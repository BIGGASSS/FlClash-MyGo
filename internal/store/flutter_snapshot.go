package store

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Native identities are deliberately separate from Drift's integer IDs. Keep
// their correspondence through native restores (which allocate fresh files).
type flutterSnapshot struct {
	Config    map[string]any    `json:"config"`
	Tables    flutterTables     `json:"tables"`
	Profiles  map[string]string `json:"nativeProfiles,omitempty"`
	Scripts   map[string]string `json:"nativeScripts,omitempty"`
	Resources map[string]string `json:"nativeResources,omitempty"`
	Overlays  map[string]string `json:"nativeOverlays,omitempty"`
}

var flutterTableNames = []string{"profiles", "scripts", "rules", "disabled_rules", "proxy_groups", "clash_providers", "custom_proxies", "proxy_dialers", "icon_records", "icon_sets", "ip_qualities"}

func decodeFlutterSnapshot(data []byte) (flutterSnapshot, error) {
	var snapshot flutterSnapshot
	if len(data) != 0 {
		if len(data) > 32<<20 {
			return snapshot, errors.New("FlClash import snapshot exceeds size limit")
		}
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err := d.Decode(&snapshot); err != nil {
			return snapshot, err
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return snapshot, errors.New("invalid trailing FlClash snapshot data")
		}
	}
	if snapshot.Config == nil {
		snapshot.Config = map[string]any{}
	}
	if snapshot.Tables == nil {
		snapshot.Tables = flutterTables{}
	}
	for _, rows := range snapshot.Tables {
		if len(rows) > 100000 {
			return snapshot, errors.New("too many FlClash database rows")
		}
		for _, row := range rows {
			if row == nil {
				return snapshot, errors.New("invalid FlClash database row")
			}
		}
	}
	return snapshot, nil
}

// Early native imports saved source rows but no identity correspondence. Recover
// only unambiguous, still-matching labels/URLs; never guess by list position.
func recoverFlutterIdentities(cfg Settings, snapshot *flutterSnapshot) {
	if snapshot.Profiles == nil {
		snapshot.Profiles = map[string]string{}
	}
	if snapshot.Scripts == nil {
		snapshot.Scripts = map[string]string{}
	}
	if snapshot.Resources == nil {
		snapshot.Resources = map[string]string{}
	}
	claimed := map[string]map[string]bool{}
	for table, mapping := range map[string]map[string]string{"profiles": snapshot.Profiles, "scripts": snapshot.Scripts, "clash_providers": snapshot.Resources} {
		claimed[table] = map[string]bool{}
		for _, id := range mapping {
			claimed[table][id] = true
		}
	}
	unique := func(table string, match func(map[string]any) bool) string {
		id, count := "", 0
		for _, row := range snapshot.Tables[table] {
			if match(row) {
				id = String(row["id"])
				count++
			}
		}
		if count == 1 && !claimed[table][id] {
			claimed[table][id] = true
			return id
		}
		return ""
	}
	profileMatches, scriptMatches := map[string]int{}, map[string]int{}
	labelResources, resourceLabels := map[string]map[string]bool{}, map[string]map[string]bool{}
	profileKey := func(name, url, kind string) string { return jsonText([]string{name, url, kind}) }
	for _, profile := range cfg.Profiles {
		profileMatches[profileKey(profile.Name, profile.URL, profile.Type)]++
		overlay, err := ParseYAML([]byte(profile.Override))
		if err != nil {
			continue
		}
		for label, value := range flutterJSONMap(overlay["rule-providers"]) {
			path := String(flutterJSONMap(value)["path"])
			if !strings.HasPrefix(path, resourceRef) {
				continue
			}
			id := strings.TrimPrefix(path, resourceRef)
			if labelResources[label] == nil {
				labelResources[label] = map[string]bool{}
			}
			if resourceLabels[id] == nil {
				resourceLabels[id] = map[string]bool{}
			}
			labelResources[label][id], resourceLabels[id][label] = true, true
		}
	}
	for _, script := range cfg.Scripts {
		scriptMatches[script.Name]++
	}
	for _, profile := range cfg.Profiles {
		if snapshot.Profiles[profile.ID] == "" && profileMatches[profileKey(profile.Name, profile.URL, profile.Type)] == 1 {
			if old := unique("profiles", func(row map[string]any) bool {
				kind := String(row["type"])
				matches := profileMatches[profileKey(profile.Name, profile.URL, "")]
				if kind != "" {
					matches += profileMatches[profileKey(profile.Name, profile.URL, kind)]
				}
				return matches == 1 && String(row["label"]) == profile.Name && String(row["url"]) == profile.URL && (profile.Type == "" || kind == profile.Type)
			}); old != "" {
				snapshot.Profiles[profile.ID] = old
			}
		}
		if profile.Override != "" {
			overlay, err := ParseYAML([]byte(profile.Override))
			if err != nil {
				continue
			}
			for label, value := range flutterJSONMap(overlay["rule-providers"]) {
				path := String(flutterJSONMap(value)["path"])
				if !strings.HasPrefix(path, resourceRef) {
					continue
				}
				id := strings.TrimPrefix(path, resourceRef)
				if snapshot.Resources[id] != "" || len(labelResources[label]) != 1 || len(resourceLabels[id]) != 1 {
					continue
				}
				if old := unique("clash_providers", func(row map[string]any) bool {
					return String(row["label"]) == label && String(row["url"]) == "" && String(row["kind"]) == "rule"
				}); old != "" {
					snapshot.Resources[id] = old
				}
			}
		}
	}
	for _, script := range cfg.Scripts {
		if snapshot.Scripts[script.ID] == "" && scriptMatches[script.Name] == 1 {
			if old := unique("scripts", func(row map[string]any) bool { return String(row["label"]) == script.Name }); old != "" {
				snapshot.Scripts[script.ID] = old
			}
		}
	}
}

func remapFlutterSnapshot(cfg *Settings, profiles, resources map[string]string) error {
	if len(cfg.ImportSnapshot) == 0 {
		return nil
	}
	snapshot, err := decodeFlutterSnapshot(cfg.ImportSnapshot)
	if err != nil {
		return err
	}
	remap := func(values map[string]string, ids map[string]string) map[string]string {
		out := map[string]string{}
		for old, value := range values {
			if id := ids[old]; id != "" {
				out[id] = value
			}
		}
		return out
	}
	snapshot.Profiles = remap(snapshot.Profiles, profiles)
	snapshot.Resources = remap(snapshot.Resources, resources)
	overlays := map[string]string{}
	for old, value := range snapshot.Overlays {
		if id := profiles[old]; id != "" {
			if value != "" {
				b, e := remapYAMLReferences([]byte(value), profiles, resources)
				if e != nil {
					return e
				}
				value = string(b)
			}
			overlays[id] = value
		}
	}
	snapshot.Overlays = overlays
	cfg.ImportSnapshot, err = json.Marshal(snapshot)
	return err
}

func flutterProviderEntry(id string) string {
	digest := md5.Sum([]byte(id + "@"))
	return "providers/rules/" + hex.EncodeToString(digest[:])
}

// Preserve source bytes/comments when no path changes. Never replace arbitrary
// substrings in YAML (a password or a comment might contain the same string).
func rewriteFlutterYAMLPaths(data []byte, rewrite func(string) (string, error)) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	changed := false
	var walk func(*yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				v := n.Content[i+1]
				if n.Content[i].Value == "path" && v.Kind == yaml.ScalarNode && v.Tag == "!!str" {
					next, err := rewrite(v.Value)
					if err != nil {
						return err
					}
					if next != v.Value {
						v.Value = next
						changed = true
					}
				}
			}
		}
		for _, child := range n.Content {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(&doc); err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}
	return yaml.Marshal(&doc)
}
