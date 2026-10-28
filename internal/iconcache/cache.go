// Package iconcache retains the original bounded icon-byte cache and HTTP
// freshness behavior without depending on Flutter's cache manager.
package iconcache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

const MaxBytes = 16 << 20
const maxDiskBytes = 64 << 20

type record struct {
	Data       []byte    `json:"data"`
	ETag       string    `json:"etag,omitempty"`
	ValidUntil time.Time `json:"validUntil"`
}
type memoryEntry struct {
	key    string
	record record
}
type flight struct {
	done chan struct{}
	data []byte
	err  error
}

type Cache struct {
	dir     string
	ctx     context.Context
	cancel  context.CancelFunc
	slots   chan struct{}
	mu      sync.Mutex
	memory  map[string]*list.Element
	lru     list.List
	bytes   int
	flights map[string]*flight
}

func New(dir string) *Cache {
	ctx, cancel := context.WithCancel(context.Background())
	return &Cache{dir: dir, ctx: ctx, cancel: cancel, slots: make(chan struct{}, 8), memory: map[string]*list.Element{}, flights: map[string]*flight{}}
}
func (c *Cache) Close() { c.cancel() }

// Inline recognizes the same base64 form accepted by CommonTargetIcon.
func Inline(source string) ([]byte, bool, error) {
	_, encoded, ok := strings.Cut(source, "base64,")
	if !ok {
		return nil, false, nil
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(MaxBytes) {
		return nil, true, errors.New("icon exceeds 16 MiB")
	}
	encoded, err := url.PathUnescape(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return nil, true, err
	}
	encoded = strings.NewReplacer("-", "+", "_", "/").Replace(encoded)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err == nil && len(data) > MaxBytes {
		err = errors.New("icon exceeds 16 MiB")
	}
	return data, true, err
}

func (c *Cache) Load(client *http.Client, source string) ([]byte, error) {
	if data, inline, err := Inline(source); inline {
		return data, err
	}
	u, err := url.Parse(source)
	if err != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("invalid icon URL")
	}
	c.mu.Lock()
	var previous record
	if element := c.memory[source]; element != nil {
		cached := element.Value.(memoryEntry).record
		previous = cached
		c.lru.MoveToBack(element)
		if time.Now().Before(cached.ValidUntil) {
			c.mu.Unlock()
			return cached.Data, nil
		}
	}
	if pending := c.flights[source]; pending != nil {
		c.mu.Unlock()
		select {
		case <-pending.done:
			return pending.data, pending.err
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		}
	}
	pending := &flight{done: make(chan struct{})}
	c.flights[source] = pending
	c.mu.Unlock()
	data, err := c.load(client, source, previous)
	c.mu.Lock()
	pending.data, pending.err = data, err
	delete(c.flights, source)
	close(pending.done)
	c.mu.Unlock()
	return data, err
}

func (c *Cache) load(client *http.Client, source string, cached record) ([]byte, error) {
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
	if len(cached.Data) == 0 {
		cached, _ = c.readDisk(source)
	}
	if len(cached.Data) > 0 && time.Now().Before(cached.ValidUntil) {
		c.remember(source, cached)
		return cached.Data, nil
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	if cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	response, err := client.Do(req)
	if err != nil {
		return cached.Data, err
	}
	defer response.Body.Close()
	updated := record{ETag: response.Header.Get("ETag"), ValidUntil: validUntil(response.Header, time.Now())}
	switch response.StatusCode {
	case http.StatusNotModified:
		if len(cached.Data) == 0 {
			return nil, errors.New("icon cache has no body for HTTP 304")
		}
		updated.Data = cached.Data
		if updated.ETag == "" {
			updated.ETag = cached.ETag
		}
	case http.StatusOK:
		if response.ContentLength > MaxBytes {
			return cached.Data, errors.New("icon exceeds 16 MiB")
		}
		updated.Data, err = io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
		if err != nil {
			return cached.Data, err
		}
		if len(updated.Data) > MaxBytes {
			return cached.Data, errors.New("icon exceeds 16 MiB")
		}
	default:
		return cached.Data, fmt.Errorf("icon returned HTTP %d", response.StatusCode)
	}
	c.remember(source, updated)
	if c.dir != "" {
		if err := os.MkdirAll(c.dir, 0700); err == nil {
			if encoded, err := json.Marshal(updated); err == nil {
				_ = store.AtomicWrite(c.path(source), encoded)
			}
			c.trimDisk()
		}
	}
	return updated.Data, nil
}

func validUntil(headers http.Header, now time.Time) time.Time {
	age := 7 * 24 * time.Hour
	for _, setting := range strings.Split(headers.Get("Cache-Control"), ",") {
		setting = strings.ToLower(strings.TrimSpace(setting))
		if setting == "no-cache" {
			age = 0
		}
		if value, ok := strings.CutPrefix(setting, "max-age="); ok {
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil && seconds > 0 && seconds < int64((time.Duration(1<<63-1))/time.Second) {
				age = time.Duration(seconds) * time.Second
			}
		}
	}
	return now.Add(age)
}
func (c *Cache) remember(key string, value record) {
	if len(value.Data) > MaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old := c.memory[key]; old != nil {
		c.bytes -= len(old.Value.(memoryEntry).record.Data)
		c.lru.Remove(old)
	}
	c.memory[key] = c.lru.PushBack(memoryEntry{key, value})
	c.bytes += len(value.Data)
	for c.bytes > MaxBytes || c.lru.Len() > 200 {
		old := c.lru.Front()
		entry := old.Value.(memoryEntry)
		delete(c.memory, entry.key)
		c.bytes -= len(entry.record.Data)
		c.lru.Remove(old)
	}
}
func (c *Cache) path(source string) string {
	digest := sha256.Sum256([]byte(source))
	return filepath.Join(c.dir, hex.EncodeToString(digest[:])+".json")
}
func (c *Cache) readDisk(source string) (record, error) {
	if c.dir == "" {
		return record{}, os.ErrNotExist
	}
	file, err := os.Open(c.path(source))
	if err != nil {
		return record{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes*4/3+4096 {
		return record{}, errors.New("invalid cached icon")
	}
	var cached record
	err = json.NewDecoder(io.LimitReader(file, MaxBytes*4/3+4096)).Decode(&cached)
	if err == nil && len(cached.Data) <= MaxBytes {
		_ = os.Chtimes(c.path(source), time.Now(), time.Now())
		return cached, nil
	}
	return record{}, errors.New("invalid cached icon")
}
func (c *Cache) trimDisk() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type item struct {
		name     string
		size     int64
		modified time.Time
	}
	var files []item
	total := int64(0)
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 69 || !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, item{name, info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	for i, file := range files {
		if len(files)-i <= 200 && total <= maxDiskBytes && time.Since(file.modified) < 30*24*time.Hour {
			break
		}
		_ = os.Remove(filepath.Join(c.dir, file.name))
		total -= file.size
	}
}
