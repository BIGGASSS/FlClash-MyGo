package store

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) Backup(cfg Settings) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	write := func(name string, b []byte) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write(b)
		return e
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = write("settings.json", b); err != nil {
		return nil, err
	}
	for _, p := range cfg.Profiles {
		b, err := os.ReadFile(s.ProfilePath(p.ID))
		if err != nil {
			return nil, err
		}
		if err = write("profiles/"+p.ID+".yaml", b); err != nil {
			return nil, err
		}
	}
	for _, id := range cfg.Resources {
		if !ValidID(id) {
			return nil, errors.New("invalid resource ID")
		}
		b, e := os.ReadFile(s.ResourcePath(id))
		if e != nil {
			return nil, e
		}
		if e = write("resources/"+id, b); e != nil {
			return nil, e
		}
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func (s *Store) Restore(data []byte) (Settings, error) {
	cfg := Defaults()
	files, err := readBackupFiles(data)
	if err != nil {
		return cfg, err
	}
	if _, flutter := files["config.json"]; flutter {
		if _, native := files["settings.json"]; native {
			return cfg, errors.New("mixed backup formats")
		}
		cfg, files, err = s.importFlutter(files)
		if err != nil {
			return Defaults(), err
		}
	} else {
		b, ok := files["settings.json"]
		if !ok {
			return cfg, errors.New("backup has no settings.json or config.json")
		}
		if err = json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	return s.restoreFiles(cfg, files)
}

func readBackupFiles(data []byte) (map[string][]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(z.File) > 20000 {
		return nil, errors.New("too many backup entries")
	}
	files := map[string][]byte{}
	total := int64(0)
	for _, f := range z.File {
		name := f.Name
		if f.FileInfo().IsDir() && (name == "profiles/" || name == "scripts/" || name == "providers/" || name == "providers/rules/" || name == "resources/") {
			continue
		}
		if !validBackupEntry(name) || f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected backup entry %q", name)
		}
		if _, exists := files[name]; exists {
			return nil, errors.New("duplicate backup entry")
		}
		if f.UncompressedSize64 > 32<<20 {
			return nil, errors.New("backup entry exceeds size limit")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(io.LimitReader(r, (32<<20)+1))
		r.Close()
		if e != nil {
			return nil, e
		}
		total += int64(len(b))
		if len(b) > 32<<20 || total > 128<<20 {
			return nil, errors.New("backup exceeds size limit")
		}
		files[name] = b
	}
	return files, nil
}
func validBackupEntry(name string) bool {
	if name == "settings.json" || name == "config.json" || name == "database.sqlite" {
		return true
	}
	for _, prefix := range []string{"profiles/", "scripts/", "providers/rules/", "resources/"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		base := strings.TrimPrefix(name, prefix)
		if prefix == "profiles/" {
			if !strings.HasSuffix(base, ".yaml") {
				return false
			}
			base = strings.TrimSuffix(base, ".yaml")
		}
		if prefix == "scripts/" {
			if !strings.HasSuffix(base, ".js") {
				return false
			}
			base = strings.TrimSuffix(base, ".js")
		}
		if base == "" || len(base) > 128 {
			return false
		}
		for _, ch := range base {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
				return false
			}
		}
		return true
	}
	return false
}
func (s *Store) restoreFiles(cfg Settings, files map[string][]byte) (Settings, error) {
	if cfg.Version != 1 {
		return cfg, errors.New("unsupported backup version")
	}
	ids, resources := map[string]string{}, map[string]string{}
	expected := map[string]bool{"settings.json": true}
	for _, p := range cfg.Profiles {
		if !ValidID(p.ID) || ids[p.ID] != "" {
			return cfg, errors.New("invalid or duplicate profile ID")
		}
		ids[p.ID] = NewID()
		name := "profiles/" + p.ID + ".yaml"
		b, ok := files[name]
		if !ok {
			return cfg, fmt.Errorf("profile %s is missing", p.Name)
		}
		if _, err := ParseYAML(b); err != nil {
			return cfg, err
		}
		if p.Override != "" {
			if _, err := ParseYAML([]byte(p.Override)); err != nil {
				return cfg, err
			}
		}
		expected[name] = true
	}
	for _, id := range cfg.Resources {
		if !ValidID(id) || resources[id] != "" {
			return cfg, errors.New("invalid or duplicate resource ID")
		}
		if _, ok := files["resources/"+id]; !ok {
			return cfg, errors.New("backup resource is missing")
		}
		expected["resources/"+id] = true
		resources[id] = NewID()
	}
	for name := range files {
		if !expected[name] {
			return cfg, fmt.Errorf("unexpected backup entry %q", name)
		}
	}
	if cfg.Active != "" && ids[cfg.Active] == "" {
		return cfg, errors.New("active profile is missing")
	}
	scriptIDs := map[string]bool{}
	for _, script := range cfg.Scripts {
		if !ValidID(script.ID) || scriptIDs[script.ID] {
			return cfg, errors.New("invalid or duplicate script ID")
		}
		scriptIDs[script.ID] = true
	}
	for _, p := range cfg.Profiles {
		if p.ScriptID != "" && !scriptIDs[p.ScriptID] {
			return cfg, fmt.Errorf("script for profile %s is missing", p.Name)
		}
	}
	if cfg.Scale < 0.8 || cfg.Scale > 1.4 {
		cfg.Scale = 1
	}
	if cfg.Selected == nil {
		cfg.Selected = map[string]string{}
	}
	if cfg.Hotkeys == nil {
		cfg.Hotkeys = map[string]string{}
	}
	normalizeHotkeys(&cfg)
	cfg.DashboardWidgets = NormalizeDashboardWidgets(cfg.DashboardWidgets)
	if cfg.Overrides == nil {
		cfg.Overrides = Defaults().Overrides
	}
	// Stage every validated file under a fresh identity. Settings are the commit
	// point; errors leave the previous settings and all referenced files intact.
	written := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, name := range written {
				_ = os.Remove(name)
			}
		}
	}()
	write := func(name string, b []byte) error {
		if e := AtomicWrite(name, b); e != nil {
			return e
		}
		written = append(written, name)
		return nil
	}
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		old := p.ID
		p.ID = ids[old]
		b, err := remapYAMLReferences(files["profiles/"+old+".yaml"], ids, resources)
		if err != nil {
			return cfg, err
		}
		if p.Override != "" {
			v, e := remapYAMLReferences([]byte(p.Override), ids, resources)
			if e != nil {
				return cfg, e
			}
			p.Override = string(v)
		}
		if err = write(s.ProfilePath(p.ID), b); err != nil {
			return cfg, err
		}
	}
	for i, old := range cfg.Resources {
		cfg.Resources[i] = resources[old]
		if err := write(s.ResourcePath(resources[old]), files["resources/"+old]); err != nil {
			return cfg, err
		}
	}
	if err := remapReferences(cfg.Overrides, ids, resources); err != nil {
		return cfg, err
	}
	cfg.Active = ids[cfg.Active]
	if err := s.Save(cfg); err != nil {
		return cfg, err
	}
	committed = true
	return cfg, nil
}
func WebDAV(client *http.Client, cfg Settings, method string, data []byte) ([]byte, error) {
	address := strings.TrimRight(cfg.WebDAVURL, "/") + "/FlClash-MyGo.zip"
	if !strings.HasPrefix(address, "https://") && !strings.HasPrefix(address, "http://") {
		return nil, errors.New("enter an HTTP or HTTPS WebDAV URL")
	}
	req, err := http.NewRequest(method, address, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfg.WebDAVUser, cfg.WebDAVPassword)
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/zip")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("WebDAV returned HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20+1))
	if len(b) > 128<<20 {
		return nil, errors.New("backup exceeds 128 MiB")
	}
	return b, err
}
func (s *Store) RemoveProfile(id string) error {
	if !ValidID(id) {
		return errors.New("invalid profile ID")
	}
	err := os.Remove(s.ProfilePath(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(filepath.Join(s.Dir, "profiles", "providers", id))
}
