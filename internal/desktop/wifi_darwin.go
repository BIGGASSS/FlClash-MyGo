//go:build darwin

package desktop

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var wifiDarwin struct {
	once sync.Once
	err  error
}

// CoreWLAN and CoreLocation are loaded dynamically, preserving CGO_ENABLED=0.
// No location authorization prompt is requested by this read-only API.
func readWiFi(ctx context.Context) (WiFiState, error) {
	if err := ctx.Err(); err != nil {
		return WiFiState{}, err
	}
	wifiDarwin.once.Do(func() {
		for _, framework := range []string{"Foundation", "CoreWLAN", "CoreLocation"} {
			_, err := purego.Dlopen("/System/Library/Frameworks/"+framework+".framework/"+framework, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
			if err != nil {
				wifiDarwin.err = fmt.Errorf("%w: %v", ErrWiFiUnavailable, err)
				return
			}
		}
		for _, name := range []string{"NSAutoreleasePool", "CWWiFiClient", "CLLocationManager"} {
			if objc.GetClass(name) == 0 {
				wifiDarwin.err = fmt.Errorf("%w: missing Objective-C class %s", ErrWiFiUnavailable, name)
				return
			}
		}
	})
	if wifiDarwin.err != nil {
		return WiFiState{}, wifiDarwin.err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sel := objc.RegisterName
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	// CoreWLAN gates SSIDs on location authorization on macOS 14+ (Darwin 23).
	release, err := syscall.Sysctl("kern.osrelease")
	if err != nil {
		return WiFiState{}, fmt.Errorf("%w: cannot determine location requirements: %v", ErrWiFiUnavailable, err)
	}
	major, err := strconv.Atoi(strings.SplitN(release, ".", 2)[0])
	if err != nil {
		return WiFiState{}, err
	}
	checkPermission := func() error {
		if major < 23 {
			return nil
		}
		manager := objc.ID(objc.GetClass("CLLocationManager"))
		status := objc.Send[int32](manager, sel("authorizationStatus"))
		if !objc.Send[bool](manager, sel("locationServicesEnabled")) || (status != 3 && status != 4) {
			return fmt.Errorf("%w: allow Location Services for FlClash in System Settings > Privacy & Security > Location Services (authorization %d); automatic prompting is not implemented", ErrWiFiPermission, status)
		}
		return nil
	}
	return wifiPermissionRead(checkPermission, func() (WiFiState, error) {
		client := objc.ID(objc.GetClass("CWWiFiClient")).Send(sel("sharedWiFiClient"))
		if client == 0 {
			return WiFiState{}, fmt.Errorf("%w: CoreWLAN client not available", ErrWiFiUnavailable)
		}
		iface := client.Send(sel("interface"))
		if iface == 0 || !objc.Send[bool](iface, sel("powerOn")) {
			return WiFiState{}, nil
		}
		// Match upstream's ssid(), including CoreWLAN's WinLatin1 fallback
		// for raw SSIDs that are not UTF-8. Decoding ssidData ourselves with
		// replacement characters changes those network identities.
		name := iface.Send(sel("ssid"))
		if name == 0 {
			return WiFiState{}, nil
		}
		data := name.Send(sel("dataUsingEncoding:"), uint64(4)) // NSUTF8StringEncoding
		if data == 0 {
			return WiFiState{}, fmt.Errorf("%w: CoreWLAN SSID text conversion failed", ErrWiFiUnavailable)
		}
		length := objc.Send[uint64](data, sel("length")) // NSUInteger on both desktop ABIs
		bytes := objc.Send[unsafe.Pointer](data, sel("bytes"))
		// A 32-byte raw SSID can expand when CoreWLAN decodes legacy text.
		if length == 0 || length > 128 || bytes == nil {
			return WiFiState{}, fmt.Errorf("%w: CoreWLAN returned an unreadable SSID", ErrWiFiUnavailable)
		}
		ssid := string(unsafe.Slice((*byte)(bytes), int(length)))
		runtime.KeepAlive(data)
		if err := ctx.Err(); err != nil {
			return WiFiState{}, err
		}
		if !utf8.ValidString(ssid) {
			return WiFiState{}, fmt.Errorf("%w: CoreWLAN returned invalid SSID text", ErrWiFiUnavailable)
		}
		return WiFiState{Connected: true, SSID: ssid}, nil
	})
}
