package desktop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrWiFiPermission  = errors.New("Wi-Fi SSID permission required")
	ErrWiFiUnavailable = errors.New("Wi-Fi SSID API unavailable")
)

// WiFiState distinguishes a known disconnection from an unreadable SSID.
// A connected network with an unavailable/redacted SSID must return an error.
type WiFiState struct {
	Connected bool
	SSID      string
}

// WiFiReader is read-only: it never scans, connects, changes networking, or
// requests location authorization. Read returns promptly on cancellation.
type WiFiReader interface {
	Read(context.Context) (WiFiState, error)
}

type wiFiResult struct {
	state WiFiState
	err   error
}

// nativeWiFiReader permits only one outstanding native call. The macOS and
// Windows synchronous APIs cannot themselves be interrupted; a canceled caller
// discards the result without accumulating blocked workers.
type nativeWiFiReader struct {
	mu   sync.Mutex
	busy bool
	read func(context.Context) (WiFiState, error)
}

func NewWiFiReader() WiFiReader {
	return &nativeWiFiReader{read: readWiFi}
}

func (r *nativeWiFiReader) Read(ctx context.Context) (WiFiState, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return WiFiState{}, err
	}
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		return WiFiState{}, fmt.Errorf("%w: previous SSID query is still finishing", ErrWiFiUnavailable)
	}
	r.busy = true
	r.mu.Unlock()
	ch := make(chan wiFiResult, 1)
	// Caller cancellation/deadline remains immediate even when an OS call
	// cannot honor it. The busy flag clears only after that call returns.
	go func() {
		state, err := r.read(ctx)
		r.mu.Lock()
		r.busy = false
		r.mu.Unlock()
		ch <- wiFiResult{state, err}
	}()
	select {
	case <-ctx.Done():
		return WiFiState{}, ctx.Err()
	case result := <-ch:
		if err := ctx.Err(); err != nil {
			return WiFiState{}, err
		}
		return result.state, result.err
	}
}
