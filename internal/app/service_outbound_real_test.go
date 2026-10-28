package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
)

func TestServiceOutboundRealCoreLoopbackCache(t *testing.T) {
	binary := os.Getenv("FLCLASH_TEST_CORE")
	if binary == "" {
		t.Skip("requires built core; all outbound requests stay on loopback")
	}
	var requests atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, "ip=203.0.113.20\nloc=JP\n")
	}))
	defer endpoint.Close()
	// The source parser still receives its trace format. No public source can
	// be contacted, even if the lookup races/falls back or the core fails.
	original := ipquality.OutboundSources
	ipquality.OutboundSources = []string{endpoint.URL}
	defer func() { ipquality.OutboundSources = original }()

	home := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := coreclient.Launch(ctx, binary, home, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Call(ctx, "initClash", map[string]any{"home-dir": home, "version": 0}, nil); err != nil {
		t.Fatal(err)
	}
	// No engine reconciliation, mixed/DNS listener, system proxy or TUN.
	config := "mixed-port: 0\nmode: rule\nproxies: []\nproxy-groups: []\nrules: ['MATCH,DIRECT']\ndns: {enable: false}\ntun: {enable: false}\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var message string
	if err := client.Call(ctx, "setupConfig", map[string]any{}, &message); err != nil || message != "" {
		t.Fatal(message, err)
	}
	a := testApp(t)
	a.safe, a.page = false, "tools"
	a.engine = &Engine{ctx: ctx, client: client}
	seedOutboundQuality(a, "203.0.113.20")
	a.outboundIP()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "203.0.113.20" || requests.Load() != 1 {
		t.Fatal("real outbound boundary did not publish a verified loopback result", a.ip, requests.Load())
	}
	direct := a.outbound.entries["DIRECT"]
	if direct == nil || direct.value == nil || len(direct.hits) == 0 || direct.hits[0].Proxy != "DIRECT" {
		t.Fatal("stopped dashboard did not backfill DIRECT with its actual pinned route", direct)
	}
	a.requestOutbound("DIRECT", false)
	a.markServiceRoutes()
	a.verifyOutboundRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	if requests.Load() != 1 || a.outbound.entries["DIRECT"].value == nil {
		t.Fatal("real checkRoutes discarded an unchanged pinned exit or repeated the HTTP probe")
	}
}
