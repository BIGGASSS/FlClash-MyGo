package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

type Proxy struct {
	mu       sync.Mutex
	path     string
	snapshot map[string]string
}

func NewProxy(dataDir string) *Proxy {
	return &Proxy{path: filepath.Join(dataDir, "system-proxy.json")}
}
func (p *Proxy) Enable(port int, bypass string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.snapshot == nil {
		if _, err := os.Stat(p.path); err == nil {
			return errors.New("a previous system proxy session needs restoration; turn system proxy off first")
		}
		snap, err := readProxy()
		if err != nil {
			return err
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if err = store.AtomicWrite(p.path, b); err != nil {
			return err
		}
		p.snapshot = snap
	}
	if err := setProxy(port, bypass); err != nil {
		_ = restoreProxy(p.snapshot)
		return err
	}
	return nil
}
func (p *Proxy) Restore() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.snapshot == nil {
		b, e := os.ReadFile(p.path)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &p.snapshot); e != nil {
			return e
		}
	}
	if err := restoreProxy(p.snapshot); err != nil {
		return err
	}
	p.snapshot = nil
	return os.Remove(p.path)
}
func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return "", errors.New(name + ": " + strings.TrimSpace(string(b)) + ": " + e.Error())
	}
	return strings.TrimSpace(string(b)), nil
}
