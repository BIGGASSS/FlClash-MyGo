package coreclient

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFrameRejectsOversizedAndTruncated(t *testing.T) {
	var h [4]byte
	binary.LittleEndian.PutUint32(h[:], MaxFrame+1)
	if _, err := ReadFrame(bytes.NewReader(h[:])); err == nil {
		t.Fatal("oversized frame accepted")
	}
	binary.LittleEndian.PutUint32(h[:], 10)
	if _, err := ReadFrame(bytes.NewReader(append(h[:], 1, 2))); err == nil {
		t.Fatal("truncated frame accepted")
	}
	var out bytes.Buffer
	if err := WriteFrame(&out, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	b, err := ReadFrame(&out)
	if err != nil || string(b) != "hello" {
		t.Fatalf("%q %v", b, err)
	}
}
func TestConcurrentRPCAndEvents(t *testing.T) {
	a, b := net.Pipe()
	c := New(a, t.TempDir())
	defer c.conn.Close()
	defer b.Close()
	serverDone := make(chan error, 1)
	go func() {
		var calls []struct {
			ID        string `json:"id"`
			Arguments int    `json:"arguments"`
		}
		for range 20 {
			frame, err := ReadFrame(b)
			if err != nil {
				serverDone <- err
				return
			}
			var call struct {
				ID        string `json:"id"`
				Arguments int    `json:"arguments"`
			}
			if err = json.Unmarshal(frame, &call); err != nil {
				serverDone <- err
				return
			}
			calls = append(calls, call)
		}
		for i := len(calls) - 1; i >= 0; i-- {
			data, _ := json.Marshal(map[string]any{"id": calls[i].ID, "result": calls[i].Arguments})
			if err := WriteFrame(b, data); err != nil {
				serverDone <- err
				return
			}
		}
		serverDone <- WriteFrame(b, []byte(`{"method":"message","arguments":[{"type":"delay","data":{"name":"node","value":42}}]}`))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out int
			if err := c.Call(ctx, "echo", i, &out); err != nil {
				t.Error(err)
			} else if out != i {
				t.Errorf("response %d for request %d", out, i)
			}
		}()
	}
	wg.Wait()
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-c.Events():
		if e.Type != "delay" {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
func TestDisconnectFailsPendingRequest(t *testing.T) {
	a, b := net.Pipe()
	c := New(a, t.TempDir())
	go func() { _, _ = ReadFrame(b); _ = b.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Call(ctx, "pending", nil, nil); err == nil {
		t.Fatal("closed connection accepted")
	}
}
func TestSpilledResponse(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "responses")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a, b := net.Pipe()
	c := New(a, home)
	defer a.Close()
	defer b.Close()
	go func() {
		frame, _ := ReadFrame(b)
		var call struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(frame, &call)
		body, _ := json.Marshal(map[string]any{"id": call.ID, "result": "large reply"})
		path := filepath.Join(dir, "reply.json")
		_ = os.WriteFile(path, body, 0600)
		envelope, _ := json.Marshal(map[string]any{"id": call.ID, "file": path})
		_ = WriteFrame(b, envelope)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out string
	if err := c.Call(ctx, "large", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out != "large reply" {
		t.Fatal(out)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("spill file was not removed")
	}
}
func TestRealCore(t *testing.T) {
	binaryPath := os.Getenv("FLCLASH_TEST_CORE")
	if binaryPath == "" {
		t.Skip("set FLCLASH_TEST_CORE to run against the built upstream core")
	}
	home := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := Launch(ctx, binaryPath, home, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.Call(ctx, "initClash", map[string]any{"home-dir": home, "version": 0}, nil); err != nil {
		t.Fatal(err)
	}
	config := []byte("mixed-port: 0\nmode: rule\nproxies: []\nproxy-groups: []\nrules: ['MATCH,DIRECT']\ndns: {enable: false}\ntun: {enable: false}\n")
	path := filepath.Join(home, "config.yaml")
	if err = os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	var message string
	if err = c.Call(ctx, "validateMyGoConfig", path, &message); err != nil || message != "" {
		t.Fatalf("validate: %q %v", message, err)
	}
	if err = c.Call(ctx, "setupConfig", map[string]any{}, &message); err != nil || message != "" {
		t.Fatalf("setup: %q %v", message, err)
	}
	var nodes struct {
		Proxies map[string]any `json:"proxies"`
	}
	if err = c.Call(ctx, "getProxies", nil, &nodes); err != nil {
		t.Fatal(err)
	}
	if nodes.Proxies["DIRECT"] == nil {
		t.Fatal("core did not expose DIRECT")
	}
	var providers []any
	if err = c.Call(ctx, "getExternalProviders", nil, &providers); err != nil {
		t.Fatal(err)
	}
	var routes struct {
		CoreEpoch    uint64 `json:"core-epoch"`
		RouteVersion uint64 `json:"route-version"`
		Held         []bool `json:"held"`
	}
	if err = c.Call(ctx, "checkRoutes", map[string]any{"routes": [][]any{{}, {}}}, &routes); err != nil || len(routes.Held) != 2 || routes.Held[0] || routes.Held[1] {
		t.Fatalf("empty route stamps must require a new probe: %+v %v", routes, err)
	}
	// Unknown service names exercise the contract without any outbound checks.
	var services []any
	if err = c.Call(ctx, "serviceCheck", map[string]any{"proxy-name": "DIRECT", "names": []string{"__port_contract_only__"}, "timeout": 10000}, &services); err != nil || len(services) != 0 {
		t.Fatal("service check contract", services, err)
	}
	if err = os.WriteFile(path, []byte("proxies: [{name: bad, type: made-up}]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = c.Call(ctx, "validateMyGoConfig", path, &message); err != nil {
		t.Fatal(err)
	}
	if message == "" {
		t.Fatal("invalid proxy was accepted")
	}
}
