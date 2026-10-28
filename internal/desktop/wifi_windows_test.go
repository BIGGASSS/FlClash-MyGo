//go:build windows

package desktop

import (
	"errors"
	"testing"
	"unsafe"
)

// Compile-time ABI checks also run when the Windows tests are cross-compiled.
// WLAN_INTERFACE_INFO contains no pointer-sized fields on either target.
var (
	_ [532 - int(unsafe.Sizeof(wifiInterface{}))]byte
	_ [int(unsafe.Sizeof(wifiInterface{})) - 532]byte
	_ [528 - int(unsafe.Offsetof(wifiInterface{}.State))]byte
	_ [int(unsafe.Offsetof(wifiInterface{}.State)) - 528]byte
)

func TestWiFiWindowsNativeConnectionBounds(t *testing.T) {
	data := wifiTestWindowsConnection("Home")
	for _, size := range []uint32{0, 4, wifiWindowsConnectionPrefix - 1} {
		if _, err := wifiWindowsConnection(unsafe.Pointer(&data[0]), size); !errors.Is(err, ErrWiFiUnavailable) {
			t.Fatal("accepted short native connection", size, err)
		}
	}
	if _, err := wifiWindowsConnection(nil, uint32(len(data))); !errors.Is(err, ErrWiFiUnavailable) {
		t.Fatal("accepted nil native connection", err)
	}
	state, err := wifiWindowsConnection(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil || state.SSID != "Home" {
		t.Fatal(state, err)
	}
}
