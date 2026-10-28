package app

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
)

func readProviderFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if len(data) > 32<<20 {
		return nil, errors.New("provider exceeds 32 MiB")
	}
	return data, err
}
func (a *App) editProvider(p Provider) {
	if a.safe || a.engine == nil {
		return
	}
	e := a.engine
	serial := a.beginEditorLoad()
	e.mu.Lock()
	client, revision := e.client, e.revision
	e.mu.Unlock()
	if client == nil {
		return
	}
	active := a.cfg.Active
	a.work(func() (func(), error) {
		data, err := readProviderFile(p.Path)
		binary := err == nil && (!utf8.Valid(data) || strings.ContainsRune(string(data), 0))
		var text string
		if binary {
			ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
			defer cancel()
			err = client.Call(ctx, "dumpRuleSet", p.Path, &text)
		}
		return func() {
			if a.editorLoadSerial != serial || a.cfg.Active != active || a.engine != e || !providerClientCurrent(e, client, revision) || !a.providerPathCurrent(p.Name, p.Path) {
				return
			}
			if err != nil {
				a.err = err.Error()
			} else if binary {
				a.detail = p.Name + "\n\n" + text
				a.detailOpen = true
			} else {
				a.openCode(p.Name, "provider:"+p.Name, string(data))
				a.codeProvider.Path = p.Path
			}
		}, nil
	})
}
func (a *App) sideLoadProvider(name string, data []byte, done func(), paths ...string) {
	if a.safe || a.engine == nil {
		return
	}
	e := a.engine
	e.mu.Lock()
	client, revision := e.client, e.revision
	e.mu.Unlock()
	if client == nil {
		a.err = "core is not connected"
		return
	}
	path := ""
	if len(paths) > 0 {
		path = paths[0]
	}
	active := a.cfg.Active
	a.work(func() (func(), error) {
		var prepared *store.PreparedResource
		var err error
		if path != "" {
			prepared, err = store.PrepareResource(path, data)
		}
		ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
		defer cancel()
		var message string
		if err == nil {
			err = client.Call(ctx, "sideLoadExternalProvider", map[string]string{"providerName": name, "provider-path": path, "data-base64": base64.StdEncoding.EncodeToString(data)}, &message)
		}
		if err == nil && message != "" {
			err = errors.New(message)
		}
		return func() {
			if prepared != nil {
				defer prepared.Discard()
			}
			if a.engine != e || a.cfg.Active != active || !providerClientCurrent(e, client, revision) || !a.providerPathCurrent(name, path) {
				return
			}
			if err == nil && prepared != nil {
				err = prepared.Commit()
			}
			if err != nil {
				a.err = err.Error()
				return
			}
			a.message = a.t("success")
			if done != nil {
				done()
			}
		}, nil
	})
}
func (a *App) providerPathCurrent(name, path string) bool {
	for _, p := range a.sample.Providers {
		if p.Name == name && p.Path != path {
			return false
		}
	}
	return true
}

func (a *App) importProvider(p Provider) {
	if a.safe {
		return
	}
	active, e := a.cfg.Active, a.engine
	revision := uint64(0)
	if e != nil {
		revision = e.Revision()
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: a.t("import"), Filters: []mygo.FileFilter{{Name: "Provider", Extensions: []string{"yaml", "yml", "txt", "mrs"}}}})
		if err != nil {
			a.update(func() { a.err = err.Error() })
			return
		}
		if len(paths) == 0 {
			return
		}
		data, err := readProviderFile(paths[0])
		a.update(func() {
			if a.cfg.Active != active || a.engine != e || (e != nil && e.Revision() != revision) {
				return
			}
			for _, current := range a.sample.Providers {
				if current.Name == p.Name && current.Type == p.Type && current.Path != p.Path {
					return
				}
			}
			if err != nil {
				a.err = err.Error()
				return
			}
			a.sideLoadProvider(p.Name, data, nil, p.Path)
		})
	}()
}
func (a *App) exportProvider(p Provider) {
	if a.safe {
		return
	}
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.window, Title: a.t("export"), DefaultPath: p.Name + filepath.Ext(p.Path)})
		if err != nil {
			a.update(func() { a.err = err.Error() })
			return
		}
		if path == "" {
			return
		}
		data, err := readProviderFile(p.Path)
		if err == nil {
			err = os.WriteFile(path, data, 0600)
		}
		a.update(func() {
			if err != nil {
				a.err = err.Error()
			} else {
				a.message = a.t("success")
			}
		})
	}()
}
