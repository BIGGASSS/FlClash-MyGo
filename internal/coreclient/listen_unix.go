//go:build !windows

package coreclient

import (
	"net"
	"os"
	"path/filepath"
)

func listen() (net.Listener, string, func(), error) {
	dir, err := os.MkdirTemp("", "flclash-mygo-")
	if err != nil {
		return nil, "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "core.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		cleanup()
		return nil, "", nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		ln.Close()
		cleanup()
		return nil, "", nil, err
	}
	return ln, path, cleanup, nil
}
