//go:build linux

package desktop

import (
	"encoding/binary"
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

func wifiTestBSS(status uint32, ies []byte) []byte {
	value := make([]byte, 4)
	binary.NativeEndian.PutUint32(value, status)
	bss := append(wifiAttribute(unix.NL80211_BSS_STATUS, value), wifiAttribute(unix.NL80211_BSS_INFORMATION_ELEMENTS, ies)...)
	return wifiAttribute(unix.NL80211_ATTR_BSS|unix.NLA_F_NESTED, bss)
}

func TestWiFiLinuxReadsOnlyAssociatedCachedBSS(t *testing.T) {
	for _, ssid := range []string{"Home", ` home:back\slash `, "東京", "HOME"} {
		ies := append([]byte{1, 1, 0xff, 0, byte(len(ssid))}, []byte(ssid)...)
		for _, status := range []uint32{unix.NL80211_BSS_STATUS_ASSOCIATED, unix.NL80211_BSS_STATUS_IBSS_JOINED} {
			state, err := wifiAssociatedBSS(wifiTestBSS(status, ies))
			if err != nil || !state.Connected || state.SSID != ssid {
				t.Fatal(state, err)
			}
		}
		state, err := wifiAssociatedBSS(wifiTestBSS(unix.NL80211_BSS_STATUS_AUTHENTICATED, ies))
		if err != nil || state.Connected {
			t.Fatal("merely authenticated BSS treated as current Wi-Fi", state, err)
		}
	}
	if state, err := wifiAssociatedBSS(nil); err != nil || state.Connected {
		t.Fatal("empty cache is not disconnected", state, err)
	}
	if _, err := wifiAssociatedBSS(wifiTestBSS(unix.NL80211_BSS_STATUS_ASSOCIATED, []byte{0, 0})); !errors.Is(err, ErrWiFiUnavailable) {
		t.Fatal("associated but unreadable SSID was interpreted as disconnected", err)
	}
	if _, err := wifiAssociatedBSS(wifiTestBSS(unix.NL80211_BSS_STATUS_ASSOCIATED, []byte{0, 10, 'a'})); err == nil {
		t.Fatal("accepted truncated information element")
	}
}

func TestWiFiLinuxSSIDMatchesSourceGLibText(t *testing.T) {
	for _, tc := range []struct {
		raw  []byte
		want string
	}{
		{[]byte(" 東京 "), " 東京 "},
		{[]byte{'a', 0xff, 0xfe, 'b'}, "a��b"},
		{[]byte{0xe2, 0x82}, "��"},
		{[]byte{0xed, 0xa0, 0x80}, "���"},
		{[]byte{'a', 0, 'b'}, "a�b"},
		{[]byte("�"), "�"},
	} {
		ies := append([]byte{0, byte(len(tc.raw))}, tc.raw...)
		state, err := wifiAssociatedBSS(wifiTestBSS(unix.NL80211_BSS_STATUS_ASSOCIATED, ies))
		if err != nil || state.SSID != tc.want {
			t.Fatalf("%x: got %q (%v), want %q", tc.raw, state.SSID, err, tc.want)
		}
	}
}

func TestWiFiLinuxMalformedStatusIsNotDisconnection(t *testing.T) {
	for _, value := range [][]byte{nil, {1}, {1, 0, 0}, {1, 0, 0, 0, 0}} {
		bss := wifiAttribute(unix.NL80211_BSS_STATUS, value)
		if _, err := wifiAssociatedBSS(wifiAttribute(unix.NL80211_ATTR_BSS|unix.NLA_F_NESTED, bss)); err == nil {
			t.Fatalf("accepted malformed BSS status %x as a known disconnection", value)
		}
	}
}

func TestWiFiLinuxRejectsMalformedAttributes(t *testing.T) {
	for _, data := range [][]byte{{1}, {3, 0, 0, 0}, {100, 0, 0, 0}, {5, 0, 0, 0, 1, 0}} {
		if _, err := wifiAttributes(data); err == nil {
			t.Fatal("accepted malformed netlink attribute", data)
		}
	}
}
