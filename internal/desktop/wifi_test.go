package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWiFiReaderCancellationBoundsNativeWorkers(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	r := &nativeWiFiReader{read: func(context.Context) (WiFiState, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release // Model a native API which cannot be interrupted.
			return WiFiState{Connected: true, SSID: "stale"}, nil
		}
		return WiFiState{Connected: true, SSID: "fresh"}, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := r.Read(ctx); result <- err }()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation waited for the native API")
	}
	for range 10 {
		if _, err := r.Read(context.Background()); !errors.Is(err, ErrWiFiUnavailable) {
			t.Fatal("started another native worker", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("unbounded workers", calls.Load())
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		r.mu.Lock()
		busy := r.busy
		r.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("native worker did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	state, err := r.Read(context.Background())
	if err != nil || state.SSID != "fresh" || calls.Load() != 2 {
		t.Fatal("reused a canceled result", state, err, calls.Load())
	}
}

func TestWiFiReaderDoesNotStartForCanceledContext(t *testing.T) {
	r := &nativeWiFiReader{read: func(context.Context) (WiFiState, error) {
		t.Error("called native API for a canceled request")
		return WiFiState{}, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
