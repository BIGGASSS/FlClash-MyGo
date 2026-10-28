package iconcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPFreshnessETagAndDiskReuse(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("ETag", `"fixture"`)
		if n == 1 {
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = io.WriteString(w, "icon bytes")
			return
		}
		if r.Header.Get("If-None-Match") != `"fixture"` {
			t.Error("conditional request lost ETag")
		}
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()
	dir := t.TempDir()
	cache := New(dir)
	defer cache.Close()
	for range 3 {
		data, err := cache.Load(server.Client(), server.URL)
		if err != nil || string(data) != "icon bytes" {
			t.Fatal(string(data), err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("fresh icon was fetched again", calls.Load())
	}
	restarted := New(dir)
	defer restarted.Close()
	data, err := restarted.Load(server.Client(), server.URL)
	if err != nil || string(data) != "icon bytes" || calls.Load() != 2 {
		t.Fatal("private disk cache was not reused", err, calls.Load())
	}
}

func TestStaleIconSurvivesRefreshFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = io.WriteString(w, "stale")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	cache := New("")
	defer cache.Close()
	if _, err := cache.Load(server.Client(), server.URL); err != nil {
		t.Fatal(err)
	}
	data, err := cache.Load(server.Client(), server.URL)
	if err == nil || string(data) != "stale" {
		t.Fatal("refresh discarded displayed icon", string(data), err)
	}
}

func TestConcurrentFetchesAreLimitedAndSameSourceCoalesces(t *testing.T) {
	var active, maximum, calls atomic.Int32
	entered := make(chan struct{}, 16)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		calls.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "fixture")
	}))
	defer server.Close()
	cache := New("")
	defer cache.Close()
	var workers sync.WaitGroup
	for i := range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := cache.Load(server.Client(), fmt.Sprintf("%s/%d", server.URL, i)); err != nil {
				t.Error(err)
			}
		}()
	}
	for range 8 {
		select {
		case <-entered:
		case <-time.After(time.Minute):
			t.Fatal("fetches did not start")
		}
	}
	close(release)
	workers.Wait()
	if maximum.Load() > 8 || calls.Load() != 16 {
		t.Fatal("incorrect concurrent icon budget", maximum.Load(), calls.Load())
	}
	before := calls.Load()
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := cache.Load(server.Client(), server.URL+"/shared")
			if err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if calls.Load() != before+1 {
		t.Fatal("shared icon fetched repeatedly", calls.Load()-before)
	}
}

func TestCloseCancelsActiveAndQueuedRequests(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { started <- struct{}{}; <-r.Context().Done() }))
	defer server.Close()
	cache := New("")
	done := make(chan error, 1)
	go func() { _, err := cache.Load(server.Client(), server.URL); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Minute):
		t.Fatal("request did not start")
	}
	cache.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Minute):
		t.Fatal("close did not cancel the icon request")
	}
}

func TestInlineAndBodyLimits(t *testing.T) {
	data, ok, err := Inline("data:image/png;base64,aWNvbg==")
	if !ok || err != nil || string(data) != "icon" {
		t.Fatal(string(data), ok, err)
	}
	for _, source := range []string{"data:image/png;base64,aWNvbg", "data:image/png;base64,aWNvbg%3D%3D"} {
		if data, ok, err := Inline(source); !ok || err != nil || string(data) != "icon" {
			t.Fatal("original base64 normalization lost", source, err)
		}
	}
	if _, ok, err := Inline("data:image/png;base64,invalid!"); !ok || err == nil {
		t.Fatal("malformed inline image accepted")
	}
	cache := New("")
	defer cache.Close()
	if _, err := cache.Load(http.DefaultClient, "file:///etc/passwd"); err == nil {
		t.Fatal("unsupported icon URL accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(MaxBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if _, err := cache.Load(server.Client(), server.URL); err == nil {
		t.Fatal("oversized icon accepted")
	}
}

func TestMemoryCacheEvictsOldBytes(t *testing.T) {
	var calls atomic.Int32
	body := strings.Repeat("x", 6<<20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = io.WriteString(w, body) }))
	defer server.Close()
	cache := New("")
	defer cache.Close()
	for _, path := range []string{"/one", "/two", "/three", "/one"} {
		if _, err := cache.Load(server.Client(), server.URL+path); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 4 || cache.bytes > MaxBytes {
		t.Fatal("encoded icon budget was exceeded", calls.Load(), cache.bytes)
	}
}

func TestFreshnessMatchesOriginalCacheManager(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	for header, age := range map[string]time.Duration{"": 7 * 24 * time.Hour, "no-cache": 0, "max-age=120": 120 * time.Second, "max-age=0": 7 * 24 * time.Hour, "no-cache, max-age=120": 120 * time.Second} {
		h := http.Header{}
		h.Set("Cache-Control", header)
		if got := validUntil(h, now); got.Sub(now) != age {
			t.Fatal(header, got.Sub(now), age)
		}
	}
}
