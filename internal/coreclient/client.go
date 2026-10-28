package coreclient

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const MaxFrame = 64 << 20

type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
	File   string          `json:"file"`
}
type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

type Client struct {
	conn    net.Conn
	home    string
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan Response
	seq     atomic.Uint64
	done    chan struct{}
	once    sync.Once
	events  chan Event
	err     error
	child   *child
	cleanup func()
}

func Launch(ctx context.Context, binaryPath, home string, stderr io.Writer) (*Client, error) {
	return LaunchWithPrivileges(ctx, binaryPath, home, stderr, false)
}
func LaunchWithPrivileges(ctx context.Context, binaryPath, home string, stderr io.Writer, elevated bool) (*Client, error) {
	ln, address, cleanup, err := listen()
	if err != nil {
		return nil, err
	}
	child, err := startChild(binaryPath, address, stderr, elevated)
	if err != nil {
		ln.Close()
		cleanup()
		return nil, fmt.Errorf("start core: %w", err)
	}
	type accepted struct {
		conn net.Conn
		err  error
	}
	ch := make(chan accepted, 1)
	go func() { conn, e := ln.Accept(); ch <- accepted{conn, e} }()
	var conn net.Conn
	acceptRead := false
	select {
	case a := <-ch:
		conn, err = a.conn, a.err
		acceptRead = true
	case <-ctx.Done():
		err = ctx.Err()
	case <-child.done:
		err = errors.New("core exited before connecting")
	}
	ln.Close()
	if err != nil {
		_ = child.kill()
		select {
		case <-child.done:
		case <-time.After(time.Second):
		}
		cleanup()
		if !acceptRead {
			go func() {
				a := <-ch
				if a.conn != nil {
					a.conn.Close()
				}
			}()
		}
		return nil, err
	}
	c := New(conn, home)
	c.child = child
	c.cleanup = cleanup
	return c, nil
}
func New(conn net.Conn, home string) *Client {
	c := &Client{conn: conn, home: home, pending: map[string]chan Response{}, done: make(chan struct{}), events: make(chan Event, 2048)}
	go c.readLoop()
	return c
}
func (c *Client) Events() <-chan Event  { return c.events }
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) Err() error            { c.mu.Lock(); defer c.mu.Unlock(); return c.err }

func ReadFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n > MaxFrame {
		return nil, fmt.Errorf("IPC frame exceeds %d bytes", MaxFrame)
	}
	b := make([]byte, int(n))
	_, err := io.ReadFull(r, b)
	return b, err
}
func WriteFrame(w io.Writer, b []byte) error {
	if len(b) > MaxFrame {
		return errors.New("IPC request too large")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(b)))
	for _, part := range [][]byte{header[:], b} {
		for len(part) > 0 {
			n, err := w.Write(part)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}
func (c *Client) Call(ctx context.Context, method string, args, out any) error {
	id := strconv.FormatUint(c.seq.Add(1), 10)
	b, err := json.Marshal(struct {
		ID        string `json:"id"`
		Method    string `json:"method"`
		Arguments any    `json:"arguments"`
	}{id, method, args})
	if err != nil {
		return err
	}
	ch := make(chan Response, 1)
	c.mu.Lock()
	if c.err != nil {
		err = c.err
		c.mu.Unlock()
		return err
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	c.writeMu.Lock()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}
	err = c.conn.SetWriteDeadline(deadline)
	if err == nil {
		err = WriteFrame(c.conn, b)
	}
	c.writeMu.Unlock()
	if err != nil {
		c.fail(err)
		return err
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return r.Error
		}
		if out != nil && len(r.Result) > 0 {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Err()
	}
}
func (c *Client) fail(err error) {
	c.once.Do(func() { c.mu.Lock(); c.err = err; c.mu.Unlock(); c.conn.Close(); close(c.done) })
}
func (c *Client) readLoop() {
	defer close(c.events)
	for {
		b, err := ReadFrame(c.conn)
		if err != nil {
			c.fail(err)
			return
		}
		var envelope struct {
			Method    string          `json:"method"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err = json.Unmarshal(b, &envelope); err != nil {
			c.fail(err)
			return
		}
		if envelope.Method == "message" {
			var batch []Event
			if len(envelope.Arguments) > 0 && envelope.Arguments[0] == '[' {
				err = json.Unmarshal(envelope.Arguments, &batch)
			} else {
				var e Event
				err = json.Unmarshal(envelope.Arguments, &e)
				batch = []Event{e}
			}
			if err != nil {
				continue
			}
			for _, e := range batch {
				select {
				case c.events <- e:
				default:
				}
			}
			continue
		}
		var r Response
		if err = json.Unmarshal(b, &r); err != nil {
			c.fail(err)
			return
		}
		if r.File != "" {
			path, e := filepath.Abs(r.File)
			if e != nil {
				c.fail(e)
				return
			}
			parent, e := filepath.Abs(filepath.Join(c.home, "responses"))
			if e != nil || filepath.Dir(path) != parent {
				c.fail(errors.New("core returned a spill path outside its response directory"))
				return
			}
			info, e := os.Lstat(path)
			if e != nil || !info.Mode().IsRegular() || info.Size() > MaxFrame {
				c.fail(errors.New("invalid core spill file"))
				return
			}
			b, e = os.ReadFile(path)
			_ = os.Remove(path)
			if e != nil {
				c.fail(e)
				return
			}
			if e = json.Unmarshal(b, &r); e != nil {
				c.fail(e)
				return
			}
		}
		c.mu.Lock()
		ch := c.pending[r.ID]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- r:
			default:
			}
		}
	}
}
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = c.Call(ctx, "shutdown", nil, nil)
	c.fail(errors.New("core closed"))
	if c.child != nil {
		select {
		case <-c.child.done:
		case <-ctx.Done():
			_ = c.child.kill()
			select {
			case <-c.child.done:
			case <-time.After(time.Second):
				if c.cleanup != nil {
					c.cleanup()
				}
				return errors.New("core did not exit after its IPC connection closed")
			}
		}
	}
	if c.cleanup != nil {
		c.cleanup()
	}
	return nil
}
