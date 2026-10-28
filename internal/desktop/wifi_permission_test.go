package desktop

import (
	"errors"
	"testing"
)

func TestWiFiPermissionReadDoesNotReadWhenDenied(t *testing.T) {
	_, err := wifiPermissionRead(func() error { return ErrWiFiPermission }, func() (WiFiState, error) {
		t.Fatal("called native API without permission")
		return WiFiState{}, nil
	})
	if !errors.Is(err, ErrWiFiPermission) {
		t.Fatal(err)
	}
}

func TestWiFiPermissionRevocationDuringReadIsNotDisconnection(t *testing.T) {
	for _, result := range []WiFiState{{}, {Connected: true, SSID: "redacted-late"}} {
		checks := 0
		state, err := wifiPermissionRead(func() error {
			checks++
			if checks == 2 {
				return ErrWiFiPermission
			}
			return nil
		}, func() (WiFiState, error) { return result, nil })
		if !errors.Is(err, ErrWiFiPermission) || state != (WiFiState{}) || checks != 2 {
			t.Fatal("accepted native result after revocation", state, err, checks)
		}
	}
}

func TestWiFiPermissionReadPreservesSuccessfulAssociationAndErrors(t *testing.T) {
	for _, expected := range []WiFiState{{}, {Connected: true, SSID: "Home"}} {
		state, err := wifiPermissionRead(func() error { return nil }, func() (WiFiState, error) { return expected, nil })
		if err != nil || state != expected {
			t.Fatal(state, err)
		}
	}
	failure := errors.New("native call failed")
	_, err := wifiPermissionRead(func() error { return nil }, func() (WiFiState, error) { return WiFiState{}, failure })
	if !errors.Is(err, failure) {
		t.Fatal("lost native API error", err)
	}
}
