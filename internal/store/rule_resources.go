package store

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// RuleResource describes an application-owned file, not an HTTP provider from
// a subscription that happens to use the same name.
type RuleResource struct {
	URL, Path, SourcePath, Behavior, Format string
	NeedsFileDefinition                     bool
}

func (r RuleResource) Compiled() bool {
	return r.Format != "mrs" && r.Behavior != "classical"
}

func (s *Store) RemoteRuleResource(cfg Settings, path string) (RuleResource, bool) {
	if path == "" || len(cfg.ImportSnapshot) > 32<<20 {
		return RuleResource{}, false
	}
	definitions := []map[string]any{}
	if p := s.ActiveProfile(cfg); p != nil {
		if source, err := os.ReadFile(s.ProfilePath(p.ID)); err == nil {
			if raw, err := ParseYAML(source); err == nil {
				definitions = append(definitions, ruleProviderDefinitions(raw))
			}
		}
	}
	definitions = append(definitions, ruleProviderDefinitions(cfg.Overrides))
	if p := s.ActiveProfile(cfg); p != nil {
		if raw, err := ParseYAML([]byte(p.Override)); err == nil {
			definitions = append(definitions, ruleProviderDefinitions(raw))
		}
	}
	merged := map[string]any{}
	for _, providers := range definitions {
		for name, value := range providers {
			definition, ok := value.(map[string]any)
			if !ok {
				merged[name] = value
				continue
			}
			copy := map[string]any{}
			if previous, ok := merged[name].(map[string]any); ok {
				for key, field := range previous {
					copy[key] = field
				}
			}
			for key, field := range definition {
				copy[key] = field
			}
			merged[name] = copy
		}
	}
	owned := make(map[string]bool, len(cfg.Resources))
	for _, id := range cfg.Resources {
		owned[id] = true
	}
	for _, value := range merged {
		p, ok := value.(map[string]any)
		if !ok || String(p["type"]) != "file" || String(p["url"]) == "" {
			continue
		}
		ref := String(p["path"])
		if !strings.HasPrefix(ref, resourceRef) {
			continue
		}
		id := strings.TrimPrefix(ref, resourceRef)
		if !owned[id] || s.ResourcePath(id) != path {
			continue
		}
		resource := RuleResource{URL: String(p["url"]), Path: path, SourcePath: path, Behavior: ruleDefault(String(p["behavior"]), "classical"), Format: ruleDefault(String(p["format"]), "yaml")}
		if source, ok := p["flclash-source"].(map[string]any); ok {
			ref := String(source["path"])
			sourceID := strings.TrimPrefix(ref, resourceRef)
			if !strings.HasPrefix(ref, resourceRef) || !owned[sourceID] || s.ResourcePath(sourceID) == "" {
				continue
			}
			resource.SourcePath, resource.Format = s.ResourcePath(sourceID), String(source["format"])
		}
		return resource, true
	}
	var importedHTTP map[string]any
	if p := s.ActiveProfile(cfg); p != nil && p.Type == "custom" {
		if raw, err := ParseYAML([]byte(p.Override)); err == nil {
			importedHTTP = ruleProviderDefinitions(raw)
		}
	}
	var snapshot struct {
		Tables map[string][]map[string]any `json:"tables"`
	}
	decoder := json.NewDecoder(bytes.NewReader(cfg.ImportSnapshot))
	decoder.UseNumber()
	if decoder.Decode(&snapshot) == nil {
		for _, row := range snapshot.Tables["clash_providers"] {
			remote := String(row["url"])
			if String(row["kind"]) != "rule" || remote == "" {
				continue
			}
			digest := md5.Sum([]byte(String(row["id"]) + "@" + remote))
			source := filepath.Join(s.Dir, "providers", "rules", hex.EncodeToString(digest[:]))
			r := RuleResource{URL: remote, SourcePath: source, Path: source, Behavior: ruleDefault(String(row["behavior"]), "classical"), Format: ruleDefault(String(row["format"]), "yaml")}
			if r.Compiled() {
				r.Path += ".mrs"
			}
			if r.Path == path {
				return r, true
			}
			// Earlier native imports represented library sets as HTTP overrides.
			// Only migrate an exact retained library URL and its private cache.
			name := String(row["label"])
			def, _ := importedHTTP[name].(map[string]any)
			if String(def["type"]) == "http" && String(def["url"]) == remote && String(def["path"]) == "" {
				legacy := filepath.Join(s.Dir, "rules", providerHash(remote))
				confined := filepath.Join(s.providerRoot(cfg.Active, "rules"), providerHash(name+"@"+remote))
				if path == legacy || path == confined {
					return RuleResource{URL: remote, SourcePath: path, Path: path, Behavior: ruleDefault(String(def["behavior"]), r.Behavior), Format: ruleDefault(String(def["format"]), r.Format), NeedsFileDefinition: true}, true
				}
			}
		}
	}
	return RuleResource{}, false
}

func ruleProviderDefinitions(raw map[string]any) map[string]any {
	providers, _ := raw["rule-providers"].(map[string]any)
	return providers
}

func ruleDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// A shape change gets a fresh portable file. A failed runtime apply therefore
// cannot change the file used by the last working config.
func (s *Store) WithRuleResource(cfg Settings, name string, previous RuleResource, id, sourceID, behavior, format string) (Settings, error) {
	if !ValidID(id) || !ValidID(sourceID) {
		return cfg, errors.New("invalid rule resource identity")
	}
	if behavior != "classical" && format != "mrs" && id == sourceID {
		return cfg, errors.New("compiled rule resource needs a separate source identity")
	}
	next := Clone(cfg)
	p := s.ActiveProfile(next)
	if p == nil {
		return cfg, errors.New("no active profile")
	}
	providers := map[string]any{}
	if strings.TrimSpace(p.Override) != "" {
		raw, err := ParseYAML([]byte(p.Override))
		if err != nil {
			return cfg, err
		}
		providers = Map(raw, "rule-providers")
	}
	definition, _ := providers[name].(map[string]any)
	if definition == nil {
		definition = map[string]any{}
	}
	coreFormat := format
	if behavior != "classical" && format != "mrs" {
		coreFormat = "mrs"
	}
	definition["type"], definition["url"], definition["path"], definition["behavior"], definition["format"] = "file", previous.URL, resourceRef+id, behavior, coreFormat
	sourceMetadata, _ := definition["flclash-source"].(map[string]any)
	if sourceMetadata == nil {
		sourceMetadata = map[string]any{}
	}
	sourceMetadata["path"], sourceMetadata["format"] = resourceRef+sourceID, format
	definition["flclash-source"] = sourceMetadata
	providers[name] = definition
	source := []byte(p.Override)
	if strings.TrimSpace(p.Override) == "" {
		source = []byte("{}\n")
	}
	patched, err := PatchSection(source, "rule-providers", providers)
	if err != nil {
		return cfg, err
	}
	p.Override = string(patched)
	next.Resources = append(next.Resources, id)
	if id != sourceID {
		next.Resources = append(next.Resources, sourceID)
	}
	return next, nil
}
