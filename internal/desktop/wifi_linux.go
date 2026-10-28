//go:build linux

package desktop

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// Linux queries nl80211's cached association, independent of NetworkManager,
// iwd or ConnMan. GET_SCAN reads the cache; TRIGGER_SCAN is deliberately absent.
func readWiFi(ctx context.Context) (WiFiState, error) {
	state, err := readLinuxWiFi(ctx)
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		err = fmt.Errorf("%w: nl80211: %v", ErrWiFiPermission, err)
	}
	return state, err
}

type wifiNetlink struct {
	fd  int
	seq uint32
}

func readLinuxWiFi(ctx context.Context) (WiFiState, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_GENERIC)
	if err != nil {
		return WiFiState{}, err
	}
	defer unix.Close(fd)
	if err = unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return WiFiState{}, err
	}
	nl := wifiNetlink{fd: fd}
	family, err := nl.query(ctx, unix.GENL_ID_CTRL, unix.CTRL_CMD_GETFAMILY, false, wifiAttribute(unix.CTRL_ATTR_FAMILY_NAME, []byte("nl80211\x00")))
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			err = fmt.Errorf("%w: nl80211 is not present", ErrWiFiUnavailable)
		}
		return WiFiState{}, err
	}
	var id uint16
	for _, message := range family {
		attrs, err := wifiAttributes(message)
		if err != nil {
			return WiFiState{}, err
		}
		if value := attrs[unix.CTRL_ATTR_FAMILY_ID]; len(value) >= 2 {
			id = binary.NativeEndian.Uint16(value)
		}
	}
	if id == 0 {
		return WiFiState{}, fmt.Errorf("%w: missing nl80211 family", ErrWiFiUnavailable)
	}
	interfaces, err := nl.query(ctx, id, unix.NL80211_CMD_GET_INTERFACE, true, nil)
	if err != nil {
		return WiFiState{}, err
	}
	for _, message := range interfaces {
		attrs, err := wifiAttributes(message)
		if err != nil {
			return WiFiState{}, err
		}
		index := attrs[unix.NL80211_ATTR_IFINDEX]
		kind := attrs[unix.NL80211_ATTR_IFTYPE]
		if len(index) != 4 || len(kind) != 4 {
			continue
		}
		switch binary.NativeEndian.Uint32(kind) {
		case unix.NL80211_IFTYPE_STATION, unix.NL80211_IFTYPE_P2P_CLIENT, unix.NL80211_IFTYPE_ADHOC:
		default:
			continue
		}
		cached, err := nl.query(ctx, id, unix.NL80211_CMD_GET_SCAN, true, wifiAttribute(unix.NL80211_ATTR_IFINDEX, index))
		if err != nil {
			// An interface may disappear during enumeration; this is not a
			// permission failure or evidence that a previous SSID was left.
			return WiFiState{}, err
		}
		for _, bss := range cached {
			state, err := wifiAssociatedBSS(bss)
			if err != nil || state.Connected {
				return state, err
			}
		}
	}
	return WiFiState{}, nil
}

func wifiAttribute(kind uint16, data []byte) []byte {
	length := 4 + len(data)
	out := make([]byte, (length+3)&^3)
	binary.NativeEndian.PutUint16(out, uint16(length))
	binary.NativeEndian.PutUint16(out[2:], kind)
	copy(out[4:], data)
	return out
}

func wifiAttributes(data []byte) (map[uint16][]byte, error) {
	out := map[uint16][]byte{}
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, errors.New("short Wi-Fi netlink attribute")
		}
		n := int(binary.NativeEndian.Uint16(data))
		if n < 4 || n > len(data) {
			return nil, errors.New("invalid Wi-Fi netlink attribute length")
		}
		kind := binary.NativeEndian.Uint16(data[2:]) & 0x3fff
		out[kind] = data[4:n]
		padded := (n + 3) &^ 3
		if padded > len(data) {
			if n == len(data) {
				break
			}
			return nil, errors.New("short Wi-Fi netlink attribute padding")
		}
		data = data[padded:]
	}
	return out, nil
}

func wifiAssociatedBSS(message []byte) (WiFiState, error) {
	attrs, err := wifiAttributes(message)
	if err != nil {
		return WiFiState{}, err
	}
	bss, err := wifiAttributes(attrs[unix.NL80211_ATTR_BSS])
	if err != nil {
		return WiFiState{}, err
	}
	status, present := bss[unix.NL80211_BSS_STATUS]
	if !present {
		return WiFiState{}, nil
	}
	if len(status) != 4 {
		return WiFiState{}, errors.New("invalid Wi-Fi BSS status length")
	}
	value := binary.NativeEndian.Uint32(status)
	if value != unix.NL80211_BSS_STATUS_ASSOCIATED && value != unix.NL80211_BSS_STATUS_IBSS_JOINED {
		return WiFiState{}, nil
	}
	for _, key := range []uint16{unix.NL80211_BSS_INFORMATION_ELEMENTS, unix.NL80211_BSS_BEACON_IES} {
		for ies := bss[key]; len(ies) >= 2; {
			n := int(ies[1])
			if n+2 > len(ies) {
				return WiFiState{}, errors.New("invalid Wi-Fi information element")
			}
			if ies[0] == 0 && n > 0 && n <= 32 {
				return WiFiState{Connected: true, SSID: wifiLinuxSSID(ies[2 : 2+n])}, nil
			}
			ies = ies[2+n:]
		}
	}
	return WiFiState{}, fmt.Errorf("%w: associated Wi-Fi has no readable SSID", ErrWiFiUnavailable)
}

// The source Linux reader uses GLib's g_utf8_make_valid: every invalid byte
// (including an embedded NUL) becomes one replacement character. ToValidUTF8
// instead collapses consecutive invalid bytes and changes SSID identities.
func wifiLinuxSSID(raw []byte) string {
	runes := []rune(string(raw))
	for i, r := range runes {
		if r == 0 {
			runes[i] = utf8.RuneError
		}
	}
	return string(runes)
}

// query returns generic-netlink payloads without their four-byte headers.
func (nl *wifiNetlink) query(ctx context.Context, family uint16, command byte, dump bool, attrs []byte) ([][]byte, error) {
	nl.seq++
	request := make([]byte, 20+len(attrs))
	binary.NativeEndian.PutUint32(request, uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:], family)
	flags := uint16(unix.NLM_F_REQUEST)
	if dump {
		flags |= unix.NLM_F_DUMP
	}
	binary.NativeEndian.PutUint16(request[6:], flags)
	binary.NativeEndian.PutUint32(request[8:], nl.seq)
	request[16], request[17] = command, 1
	copy(request[20:], attrs)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := unix.Sendto(nl.fd, request, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	buffer := make([]byte, 128<<10)
	var out [][]byte
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		poll := []unix.PollFd{{Fd: int32(nl.fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(poll, 100); err != nil && err != unix.EINTR {
			return nil, err
		}
		n, _, recvFlags, from, err := unix.Recvmsg(nl.fd, buffer, nil, unix.MSG_DONTWAIT)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			return nil, err
		}
		if recvFlags&unix.MSG_TRUNC != 0 {
			return nil, errors.New("Wi-Fi netlink reply truncated")
		}
		if source, ok := from.(*unix.SockaddrNetlink); !ok || source.Pid != 0 {
			continue
		}
		total += n
		if total > 4<<20 {
			return nil, errors.New("Wi-Fi netlink reply exceeds limit")
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:n])
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if message.Header.Seq != nl.seq {
				continue
			}
			if message.Header.Flags&unix.NLM_F_DUMP_INTR != 0 {
				return nil, errors.New("Wi-Fi changed during query; retrying is required")
			}
			switch message.Header.Type {
			case unix.NLMSG_DONE, unix.NLMSG_ERROR:
				if len(message.Data) >= 4 {
					if code := int32(binary.NativeEndian.Uint32(message.Data)); code != 0 {
						return nil, syscall.Errno(-code)
					}
				} else if message.Header.Type == unix.NLMSG_ERROR {
					return nil, errors.New("short Wi-Fi netlink error")
				}
				return out, nil
			case family:
				if len(message.Data) < 4 {
					return nil, errors.New("short Wi-Fi generic netlink header")
				}
				out = append(out, append([]byte(nil), message.Data[4:]...))
				if !dump && message.Header.Flags&unix.NLM_F_MULTI == 0 {
					return out, nil
				}
			}
		}
	}
}
