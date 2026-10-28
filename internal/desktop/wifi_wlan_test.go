package desktop

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func wifiTestWindowsConnection(ssid string) []byte {
	data := make([]byte, 604) // sizeof(WLAN_CONNECTION_ATTRIBUTES)
	binary.LittleEndian.PutUint32(data, 1)
	// The profile name is unrelated to the SSID, and is always WCHAR[256].
	for i := 8; i < wifiWindowsSSIDOffset; i += 2 {
		binary.LittleEndian.PutUint16(data[i:], 0x3042)
	}
	binary.LittleEndian.PutUint32(data[wifiWindowsSSIDOffset:], uint32(len(ssid)))
	copy(data[wifiWindowsSSIDOffset+4:], ssid)
	return data
}

func TestWiFiWindowsWireSSIDIsLengthPrefixedNotUTF16(t *testing.T) {
	for _, ssid := range []string{"Home", " 東京 ", "a\x00b", strings.Repeat("x", 32)} {
		data := wifiTestWindowsConnection(ssid)
		state, err := wifiWindowsConnectionBytes(data)
		if err != nil || !state.Connected || state.SSID != ssid {
			t.Fatalf("%q: %v, %v", ssid, state, err)
		}
	}
}

func TestWiFiWindowsWireBounds(t *testing.T) {
	data := wifiTestWindowsConnection("Home")
	for size := 0; size < wifiWindowsConnectionPrefix; size++ {
		if _, err := wifiWindowsConnectionBytes(data[:size]); !errors.Is(err, ErrWiFiUnavailable) {
			t.Fatalf("accepted %d-byte connection: %v", size, err)
		}
	}
	for _, length := range []uint32{0, 33, 0xffffffff} {
		binary.LittleEndian.PutUint32(data[wifiWindowsSSIDOffset:], length)
		if _, err := wifiWindowsConnectionBytes(data); !errors.Is(err, ErrWiFiUnavailable) {
			t.Fatalf("accepted SSID length %d: %v", length, err)
		}
	}
}

func TestWiFiWindowsWireQueryDoesNotReuseDisconnectedSSID(t *testing.T) {
	data := wifiTestWindowsConnection("stale")
	binary.LittleEndian.PutUint32(data, 4)
	state, err := wifiWindowsConnectionBytes(data)
	if err != nil || state != (WiFiState{}) {
		t.Fatal("reused SSID after disconnect", state, err)
	}
	for _, status := range []uint32{0, 2, 3, 5, 6, 7, 0xffffffff} {
		binary.LittleEndian.PutUint32(data, status)
		if _, err := wifiWindowsConnectionBytes(data); !errors.Is(err, ErrWiFiUnavailable) {
			t.Fatalf("treated transient/invalid interface state %d as an association: %v", status, err)
		}
	}
}
