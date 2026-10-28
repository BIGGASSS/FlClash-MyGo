package store

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteRuleResourceMatchesPathNotLibraryLabel(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := NewID()
	cfg.Resources = []string{id}
	cfg.Overrides["rule-providers"] = map[string]any{"Duplicated label": map[string]any{"type": "file", "path": resourceRef + id, "url": "https://library.invalid/rules", "behavior": "domain", "format": "text"}}
	resource, ok := s.RemoteRuleResource(cfg, s.ResourcePath(id))
	if !ok || resource.URL != "https://library.invalid/rules" || resource.Path != s.ResourcePath(id) || resource.SourcePath != resource.Path {
		t.Fatal("portable remote library not resolved", resource, ok)
	}
	for _, path := range []string{"", filepath.Join(s.Dir, "profiles", "providers", "same-name"), s.ResourcePath(NewID())} {
		if _, ok := s.RemoteRuleResource(cfg, path); ok {
			t.Fatal("same-name or unowned path matched", path)
		}
	}
	cfg.Resources = nil
	if _, ok := s.RemoteRuleResource(cfg, s.ResourcePath(id)); ok {
		t.Fatal("deleted resource still matched")
	}
	cfg.Resources = []string{id}
	provider := Map(cfg.Overrides, "rule-providers")["Duplicated label"].(map[string]any)
	provider["type"] = "http"
	if _, ok := s.RemoteRuleResource(cfg, s.ResourcePath(id)); ok {
		t.Fatal("subscription HTTP definition treated as an app file")
	}
}

func TestRemoteRuleResourceRetainedSnapshotURLNamespace(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.ImportSnapshot = json.RawMessage(`{"tables":{"clash_providers":[{"id":901234567890123456,"kind":"rule","label":"Same","url":"https://library.invalid/rules","behavior":"domain","format":"yaml","unknown":"keep"}]}}`)
	digest := md5.Sum([]byte("901234567890123456@https://library.invalid/rules"))
	source := filepath.Join(s.Dir, "providers", "rules", hex.EncodeToString(digest[:]))
	resource, ok := s.RemoteRuleResource(cfg, source+".mrs")
	if !ok || resource.SourcePath != source || !resource.Compiled() {
		t.Fatal("pinned upstream cache namespace not recognized", resource, ok)
	}
	if _, ok := s.RemoteRuleResource(cfg, source); ok {
		t.Fatal("compiled set matched its editable source instead of core path")
	}
}

func TestRemoteRuleResourceUsesLatestProfileDefinition(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	oldID, nextID := NewID(), NewID()
	cfg.Resources = []string{oldID, nextID}
	cfg.Active = NewID()
	cfg.Profiles = []Profile{{ID: cfg.Active, Override: "rule-providers:\n  Same:\n    type: file\n    path: " + resourceRef + nextID + "\n    url: https://new.invalid/rules\n"}}
	cfg.Overrides["rule-providers"] = map[string]any{"Same": map[string]any{"type": "file", "path": resourceRef + oldID, "url": "https://old.invalid/rules"}}
	if _, ok := s.RemoteRuleResource(cfg, s.ResourcePath(oldID)); ok {
		t.Fatal("overridden global library still matched")
	}
	resource, ok := s.RemoteRuleResource(cfg, s.ResourcePath(nextID))
	if !ok || resource.URL != "https://new.invalid/rules" {
		t.Fatal("latest profile definition ignored", resource, ok)
	}
}

func TestRuleResourceShapeOverlayPreservesUnknownFieldsAndSource(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.Active = NewID()
	cfg.Profiles = []Profile{{ID: cfg.Active, Override: "# retained\nunknown: {source: fixture}\nrule-providers:\n  Other: {type: inline, payload: [other.invalid]}\n"}}
	source := []byte("# original source\nrules: ['MATCH,DIRECT']\n")
	if err := AtomicWrite(s.ProfilePath(cfg.Active), source); err != nil {
		t.Fatal(err)
	}
	old := RuleResource{URL: "https://library.invalid/rules", Path: s.ResourcePath(NewID()), Behavior: "classical", Format: "yaml"}
	id, sourceID := NewID(), NewID()
	if err := AtomicWrite(s.ResourcePath(id), []byte{0x28, 0xb5, 0x2f, 0xfd, 0}); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(s.ResourcePath(sourceID), []byte("payload: [new.invalid]\n")); err != nil {
		t.Fatal(err)
	}
	next, err := s.WithRuleResource(cfg, "Remote", old, id, sourceID, "domain", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles[0].Override == next.Profiles[0].Override || !bytes.Contains([]byte(next.Profiles[0].Override), []byte("# retained")) {
		t.Fatal("overlay patch changed original or lost comment")
	}
	raw, err := s.Effective(next)
	if err != nil {
		t.Fatal(err)
	}
	def := Map(Map(raw, "rule-providers"), "Remote")
	if def["path"] != s.ResourcePath(id) || def["behavior"] != "domain" || def["type"] != "file" || def["format"] != "mrs" {
		t.Fatal("new source shape did not reach runtime", def)
	}
	if Map(raw, "unknown")["source"] != "fixture" || Map(raw, "rule-providers")["Other"] == nil {
		t.Fatal("unrelated override lost", raw)
	}
	got, _ := os.ReadFile(s.ProfilePath(cfg.Active))
	if !bytes.Equal(got, source) {
		t.Fatal("resource activation rewrote profile source")
	}
	archive, err := s.Backup(next)
	if err != nil {
		t.Fatal(err)
	}
	target, _, _ := Open(t.TempDir())
	restored, err := target.Restore(archive)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := target.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	corePath := String(Map(Map(effective, "rule-providers"), "Remote")["path"])
	resource, ok := target.RemoteRuleResource(restored, corePath)
	rawSource, _ := os.ReadFile(resource.SourcePath)
	if !ok || resource.Path == resource.SourcePath || resource.Format != "yaml" || string(rawSource) != "payload: [new.invalid]\n" {
		t.Fatal("native restore lost source/compiled identities", resource, ok)
	}
}

func TestImportedHTTPRemoteRuleResourceRequiresExactLibraryProvenance(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.Active = NewID()
	cfg.Profiles = []Profile{{ID: cfg.Active, Type: "custom", Override: "rule-providers:\n  Library: {type: http, url: 'https://library.invalid/rules', behavior: domain, format: text}\n"}}
	path := filepath.Join(s.Dir, "rules", providerHash("https://library.invalid/rules"))
	if _, ok := s.RemoteRuleResource(cfg, path); ok {
		t.Fatal("HTTP provider without retained library metadata was matched")
	}
	cfg.ImportSnapshot = json.RawMessage(`{"tables":{"clash_providers":[{"id":42,"kind":"rule","label":"Library","url":"https://library.invalid/rules","behavior":"domain","format":"text"}]}}`)
	resource, ok := s.RemoteRuleResource(cfg, path)
	if !ok || !resource.NeedsFileDefinition || resource.URL != "https://library.invalid/rules" {
		t.Fatal("native import's HTTP library was not recognized", resource, ok)
	}
	if _, ok := s.RemoteRuleResource(cfg, path+"-replacement"); ok {
		t.Fatal("same label matched another provider path")
	}
	cfg.Profiles[0].Override = ""
	if _, ok := s.RemoteRuleResource(cfg, path); ok {
		t.Fatal("subscription with same name/URL was mistaken for an app override")
	}
}

func TestPreparedResourceOnlyCommitsAfterOwnerCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source")
	if err := AtomicWrite(path, []byte("working")); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareResource(path, []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "working" {
		t.Fatal("preparing changed live file")
	}
	prepared.Discard()
	got, _ = os.ReadFile(path)
	if string(got) != "working" {
		t.Fatal("discarding changed live file")
	}
	prepared, err = PrepareResource(path, []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Discard()
	if err := prepared.Commit(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "replacement" {
		t.Fatal("commit did not replace file")
	}
	files, _ := os.ReadDir(filepath.Dir(path))
	if len(files) != 1 {
		t.Fatal("staging leaked", files)
	}
}
