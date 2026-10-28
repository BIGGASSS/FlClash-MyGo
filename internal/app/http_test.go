package app

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

func TestAppRequestsUseRunningProxyAndFirstCredentials(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	requests := make(chan *http.Request, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=3")
		_, _ = io.WriteString(w, "rules: ['MATCH,DIRECT']\n")
	}))
	defer proxy.Close()
	u, _ := url.Parse(proxy.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := store.Defaults()
	cfg.Overrides["mixed-port"] = port
	cfg.Overrides["authentication"] = []any{"first:p@ss:word", "second:unused"}
	client := requestClient(cfg, true)
	defer client.CloseIdleConnections()
	// Later UI edits must not change an already-issued worker's routing.
	cfg.Overrides["mixed-port"] = 1
	cfg.Overrides["authentication"] = []any{"changed:unused"}
	body, headers, err := store.Download(client, "http://subscription.invalid/profile", "fixture-agent")
	if err != nil || len(body) == 0 || headers.Get("subscription-userinfo") == "" {
		t.Fatal("subscription did not use owned proxy", err)
	}
	r := <-requests
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("first:p@ss:word"))
	if r.URL.Host != "subscription.invalid" || r.Header.Get("Proxy-Authorization") != wantAuth || r.Header.Get("User-Agent") != "fixture-agent" {
		t.Fatal("request routing/authentication differs", r.URL, r.Header)
	}
	for _, raw := range []string{"http://localhost:8080/", "https://LOCALHOST/"} {
		req, _ := http.NewRequest(http.MethodGet, raw, nil)
		selected, err := client.Transport.(*http.Transport).Proxy(req)
		if err != nil || selected != nil {
			t.Fatal("localhost must bypass mixed proxy", selected, err)
		}
	}
	stopped := requestClient(cfg, false)
	defer stopped.CloseIdleConnections()
	if stopped.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("stopped app used environment/system proxy")
	}
}

func TestAppCertificatePreferenceDefaultsToVerification(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "verified fixture") }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	cfg := store.Defaults()
	client := requestClient(cfg, false)
	defer client.CloseIdleConnections()
	if response, err := client.Get(server.URL); err == nil {
		response.Body.Close()
		t.Fatal("fresh settings accepted an untrusted certificate")
	}
	cfg.SkipCertificateVerify = true
	client = requestClient(cfg, false)
	defer client.CloseIdleConnections()
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal("explicit certificate preference was ignored", err)
	}
	response.Body.Close()
}

func TestAppRedirectLimitMatchesFiveFollowedRedirects(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count++; http.Redirect(w, r, "/next", http.StatusFound) }))
	defer server.Close()
	client := requestClient(store.Defaults(), false)
	defer client.CloseIdleConnections()
	if response, err := client.Get(server.URL); err == nil {
		response.Body.Close()
		t.Fatal("unbounded redirect loop")
	}
	if count != 6 {
		t.Fatal("did not follow exactly five redirects", count)
	}
}
