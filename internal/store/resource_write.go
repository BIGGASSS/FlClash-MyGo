package store

import (
	"os"
	"path/filepath"
)

// PreparedResource does the potentially large write and fsync off the UI thread.
// Commit only replaces the directory entry after the UI checks its owner.
type PreparedResource struct {
	path, temp string
}

func PrepareResource(path string, data []byte) (*PreparedResource, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".resource-*")
	if err != nil {
		return nil, err
	}
	r := &PreparedResource{path: path, temp: f.Name()}
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		r.Discard()
		return nil, err
	}
	return r, nil
}

func (r *PreparedResource) Commit() error {
	if err := replaceFile(r.temp, r.path); err != nil {
		return err
	}
	r.temp = ""
	return nil
}

func (r *PreparedResource) Discard() {
	if r != nil && r.temp != "" {
		_ = os.Remove(r.temp)
	}
}
