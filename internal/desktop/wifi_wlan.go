package desktop

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// WLAN_CONNECTION_ATTRIBUTES uses DWORDs and 16-bit WCHARs on both Windows
// amd64 and arm64. Its association attributes start with a length-prefixed
// DOT11_SSID, not the UTF-16 profile name that precedes it.
const wifiWindowsSSIDOffset = 8 + 256*2
const wifiWindowsConnectionPrefix = wifiWindowsSSIDOffset + 4 + 32

// Keep wire parsing separate from native pointers so all boundary cases are
// testable with synthetic bytes on every host.
func wifiWindowsConnectionBytes(data []byte) (WiFiState, error) {
	if len(data) < wifiWindowsConnectionPrefix {
		return WiFiState{}, fmt.Errorf("%w: short WLAN connection", ErrWiFiUnavailable)
	}
	switch binary.LittleEndian.Uint32(data) {
	case 1: // wlan_interface_state_connected
	case 4: // wlan_interface_state_disconnected (enumeration/query raced)
		return WiFiState{}, nil
	default:
		return WiFiState{}, fmt.Errorf("%w: WLAN connection is transitioning", ErrWiFiUnavailable)
	}
	length := binary.LittleEndian.Uint32(data[wifiWindowsSSIDOffset:])
	if length == 0 || length > 32 {
		return WiFiState{}, fmt.Errorf("%w: connected WLAN has no readable SSID", ErrWiFiUnavailable)
	}
	ssid := string(data[wifiWindowsSSIDOffset+4 : wifiWindowsSSIDOffset+4+int(length)])
	return WiFiState{Connected: true, SSID: strings.ToValidUTF8(ssid, "�")}, nil
}
