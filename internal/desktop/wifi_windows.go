//go:build windows

package desktop

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	wifiDLL        = windows.NewLazySystemDLL("wlanapi.dll")
	wifiOpen       = wifiDLL.NewProc("WlanOpenHandle")
	wifiClose      = wifiDLL.NewProc("WlanCloseHandle")
	wifiFree       = wifiDLL.NewProc("WlanFreeMemory")
	wifiInterfaces = wifiDLL.NewProc("WlanEnumInterfaces")
	wifiQuery      = wifiDLL.NewProc("WlanQueryInterface")
)

type wifiInterface struct {
	GUID        windows.GUID
	Description [256]uint16
	State       uint32
}

func wifiWindowsError(code uintptr) error {
	if code == uintptr(windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("%w: enable Location services and desktop app location access in Windows Settings > Privacy & security > Location", ErrWiFiPermission)
	}
	return fmt.Errorf("WLAN SSID query: %w", windows.Errno(code))
}

// Native WLAN calls neither scan nor modify network state. Loading lazily also
// supports Windows installations without the Wireless LAN feature.
func readWiFi(ctx context.Context) (WiFiState, error) {
	for _, proc := range []*windows.LazyProc{wifiOpen, wifiClose, wifiFree, wifiInterfaces, wifiQuery} {
		if err := proc.Find(); err != nil {
			return WiFiState{}, fmt.Errorf("%w: %v", ErrWiFiUnavailable, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return WiFiState{}, err
	}
	var handle windows.Handle
	var version uint32
	code, _, _ := wifiOpen.Call(2, 0, uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&handle)))
	if code != 0 {
		return WiFiState{}, wifiWindowsError(code)
	}
	defer wifiClose.Call(uintptr(handle), 0)
	var list unsafe.Pointer
	code, _, _ = wifiInterfaces.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&list)))
	if code != 0 {
		return WiFiState{}, wifiWindowsError(code)
	}
	if list == nil {
		return WiFiState{}, fmt.Errorf("%w: WLAN returned no interface list", ErrWiFiUnavailable)
	}
	defer wifiFree.Call(uintptr(list))
	count := *(*uint32)(list)
	if count > 1024 {
		return WiFiState{}, fmt.Errorf("%w: invalid WLAN interface count", ErrWiFiUnavailable)
	}
	interfaces := unsafe.Slice((*wifiInterface)(unsafe.Add(list, 8)), int(count))
	for i := range interfaces {
		if err := ctx.Err(); err != nil {
			return WiFiState{}, err
		}
		if interfaces[i].State != 1 { // wlan_interface_state_connected
			continue
		}
		var size uint32
		var data unsafe.Pointer
		code, _, _ = wifiQuery.Call(uintptr(handle), uintptr(unsafe.Pointer(&interfaces[i].GUID)), 7, 0,
			uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&data)), 0) // current_connection
		if code != 0 {
			return WiFiState{}, wifiWindowsError(code)
		}
		state, err := wifiWindowsConnection(data, size)
		if data != nil {
			wifiFree.Call(uintptr(data))
		}
		if err != nil || state.Connected {
			return state, err
		}
	}
	return WiFiState{}, nil
}

func wifiWindowsConnection(data unsafe.Pointer, size uint32) (WiFiState, error) {
	if data == nil || size < wifiWindowsConnectionPrefix {
		return wifiWindowsConnectionBytes(nil)
	}
	return wifiWindowsConnectionBytes(unsafe.Slice((*byte)(data), wifiWindowsConnectionPrefix))
}
