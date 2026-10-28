package core

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/metacubex/mihomo/component/resource"
	cp "github.com/metacubex/mihomo/constant/provider"
	"github.com/metacubex/mihomo/rules"
	rp "github.com/metacubex/mihomo/rules/provider"
	"github.com/metacubex/mihomo/tunnel"
)

func TestNativeProviderSideLoadPreservesBinary(t *testing.T) {
	binary := []byte{0, 255, 128, 192, 13, 10, 0, 120}
	data, err := sideLoadPayload(&SideLoadParams{DataBase64: base64.StdEncoding.EncodeToString(binary)})
	if err != nil || !bytes.Equal(data, binary) {
		t.Fatal(data, err)
	}
	data, err = sideLoadPayload(&SideLoadParams{Data: "payload: [example.com]"})
	if err != nil || string(data) != "payload: [example.com]" {
		t.Fatal("legacy payload changed", err)
	}
	if _, err = sideLoadPayload(&SideLoadParams{DataBase64: "invalid!"}); err == nil {
		t.Fatal("invalid base64 accepted")
	}
}

func TestNativeProviderWritesRejectReusedNameAtAnotherPath(t *testing.T) {
	rp.SetTunnel(tunnel.Tunnel)
	oldPath, newPath := filepath.Join(t.TempDir(), "old.yaml"), filepath.Join(t.TempDir(), "new.yaml")
	current := rp.NewRuleSetProvider("shared", cp.Domain, cp.YamlRule, 0, resource.NewFileVehicle(newPath), nil, nil, rules.ParseRule).(*rp.RuleSetProvider)
	t.Cleanup(func() { _ = current.Close() })
	initial := []byte("payload:\n  - current.example\n")
	if err := os.WriteFile(newPath, initial, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := current.SideUpdate(initial); err != nil {
		t.Fatal(err)
	}
	if current.Count() != 1 {
		t.Fatal("fixture did not load its initial rule", current.Count())
	}
	withTunnelProviders(t, nil, map[string]cp.RuleProvider{"shared": current})
	for _, err := range []error{
		handleSideLoadExternalProvider(&SideLoadParams{ProviderName: "shared", ProviderPath: oldPath, Data: "payload: [wrong.example]"}),
		handleUpdateExternalProviderAtPath(&ProviderUpdateParams{ProviderName: "shared", ProviderPath: oldPath}),
	} {
		if err == nil {
			t.Fatal("stale provider identity was accepted")
		}
		method, ok := err.(*MethodError)
		if !ok || method.Code != "provider_changed" {
			t.Fatal("wrong stale-provider failure", err)
		}
	}
	data, err := os.ReadFile(newPath)
	if err != nil || !bytes.Equal(data, initial) || current.Count() != 1 {
		t.Fatal("rejected operation changed the replacement provider", string(data), err)
	}
	if err := handleSideLoadExternalProvider(&SideLoadParams{ProviderName: "shared", ProviderPath: newPath, Data: "payload:\n  - one.example\n  - two.example\n"}); err != nil || current.Count() != 2 {
		t.Fatal("matching provider identity was rejected", err)
	}
	if err := handleSideLoadExternalProvider(&SideLoadParams{ProviderName: "shared", Data: "payload:\n  - legacy.example\n"}); err != nil || current.Count() != 1 {
		t.Fatal("legacy side-load compatibility changed", err)
	}
}
