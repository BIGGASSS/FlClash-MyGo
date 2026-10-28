package store

// FlutterBackup targets the pinned FlClash v1 archive / Drift v21 schema, not
// the native backup format. See common/backup.dart and database/*.dart upstream.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed flutter_schema.sql
var flutterSchema string

// FlutterBackup leaves native preferences and source files untouched. Native
// features with no faithful Flutter representation fail rather than silently
// disappearing. The result contains credentials; callers must save it privately.
func (s *Store) FlutterBackup(settings Settings) ([]byte, error) {
	if len(settings.Profiles)+len(settings.Scripts)+len(settings.Resources)+2 > 20000 {
		return nil, errors.New("too many exported backup entries")
	}
	if len(settings.ImportSnapshot) > 32<<20 || len(settings.LocalProxies) > 100000 {
		return nil, errors.New("FlClash export input exceeds size limit")
	}
	for _, script := range settings.Scripts {
		if len(script.Source) > 32<<20 {
			return nil, errors.New("script exceeds size limit")
		}
		if !utf8.ValidString(script.Source) {
			return nil, errors.New("script is not UTF-8")
		}
	}
	for _, profile := range settings.Profiles {
		if len(profile.Script) > 32<<20 || len(profile.Override) > 32<<20 {
			return nil, errors.New("profile extension exceeds size limit")
		}
		if !utf8.ValidString(profile.Script) {
			return nil, errors.New("inline script is not UTF-8")
		}
	}
	// Unlike the convenience Clone helper, export must not swallow a failed
	// JSON clone (for example an unsupported value in a native override).
	serialized, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if len(serialized) > 128<<20 {
		return nil, errors.New("export settings exceed size limit")
	}
	var cfg Settings
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.UseNumber()
	if err = decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.Version != 1 {
		return nil, errors.New("unsupported settings version")
	}
	if value, exists := cfg.Overrides["rules"]; exists {
		if err = validateFlutterAddedRules(value); err != nil {
			return nil, err
		}
	}
	snapshot, err := decodeFlutterSnapshot(cfg.ImportSnapshot)
	if err != nil {
		return nil, err
	}
	recoverFlutterIdentities(cfg, &snapshot)
	if err = normalizeFlutterExportSnapshot(&snapshot); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := map[string][]byte{}
	total := 0
	add := func(name string, b []byte) error {
		_, exists := files[name]
		if !validBackupEntry(name) || exists {
			return fmt.Errorf("invalid or duplicate export entry %q", name)
		}
		if len(b) > 32<<20 || total+len(b) > 128<<20 || len(files) >= 20000 {
			return errors.New("backup exceeds size limit")
		}
		total += len(b)
		files[name] = b
		return nil
	}
	read := func(name string) ([]byte, error) {
		info, e := root.Lstat(name)
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() || info.Size() > 32<<20 {
			return nil, fmt.Errorf("invalid or oversized backup file %q", name)
		}
		f, e := root.Open(name)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		opened, e := f.Stat()
		if e != nil {
			return nil, e
		}
		if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
			return nil, errors.New("backup file changed while opening")
		}
		b, e := io.ReadAll(io.LimitReader(f, (32<<20)+1))
		if len(b) > 32<<20 {
			return nil, errors.New("backup file exceeds size limit")
		}
		return b, e
	}
	tables := flutterTables{}
	for name, rows := range snapshot.Tables {
		tables[name] = append([]map[string]any(nil), rows...)
	}
	next := int64(1)
	for _, rows := range tables {
		if len(rows) > 100000 {
			return nil, errors.New("too many FlClash database rows")
		}
		for _, row := range rows {
			if id, e := strconv.ParseInt(String(row["id"]), 10, 64); e == nil && id >= next {
				if id >= 1<<62 {
					return nil, errors.New("FlClash ID exceeds export range")
				}
				next = id + 1
			}
		}
	}
	newID := func() int64 { id := next; next++; return id }
	originalRows := map[string]map[string]map[string]any{}
	for name, rows := range snapshot.Tables {
		index := map[string]map[string]any{}
		for _, row := range rows {
			if row["id"] != nil {
				index[String(row["id"])] = row
			}
		}
		originalRows[name] = index
	}
	original := func(table, id string) map[string]any { return copyFlutterRow(originalRows[table][id]) }
	idFor := func(old string) int64 {
		if n, e := strconv.ParseInt(old, 10, 64); e == nil && n > 0 && n < 1<<62 {
			return n
		}
		return newID()
	}
	profileIDs, scriptIDs, resourcePaths := map[string]string{}, map[string]string{}, map[string]string{}
	for _, p := range cfg.Profiles {
		if !ValidID(p.ID) || profileIDs[p.ID] != "" {
			return nil, errors.New("invalid or duplicate profile ID")
		}
		profileIDs[p.ID] = strconv.FormatInt(idFor(snapshot.Profiles[p.ID]), 10)
	}
	if cfg.Active != "" && profileIDs[cfg.Active] == "" {
		return nil, errors.New("active profile is missing")
	}
	profileLabels, aliases, resourceLabels := map[string]string{}, map[string]string{}, map[string]string{}
	oldLabels := map[string]int{}
	for _, row := range snapshot.Tables["profiles"] {
		if String(row["type"]) != "custom" {
			oldLabels[String(row["label"])]++
		}
	}
	for _, p := range cfg.Profiles {
		if p.Type == "custom" {
			continue
		}
		profileLabels["profiles/"+profileIDs[p.ID]+".yaml"] = p.Name
		old := String(original("profiles", snapshot.Profiles[p.ID])["label"])
		if old != "" && old != p.Name {
			if oldLabels[old] != 1 {
				return nil, fmt.Errorf("ambiguous original profile label %q", old)
			}
			aliases[old] = p.Name
		}
	}
	tables["profiles"], tables["scripts"], tables["clash_providers"] = nil, nil, nil
	for i, script := range cfg.Scripts {
		if !ValidID(script.ID) || scriptIDs[script.ID] != "" {
			return nil, errors.New("invalid or duplicate script ID")
		}
		row := original("scripts", snapshot.Scripts[script.ID])
		id := idFor(snapshot.Scripts[script.ID])
		scriptIDs[script.ID] = strconv.FormatInt(id, 10)
		row["id"], row["label"], row["order"] = id, script.Name, i
		if row["last_update_time"] == nil {
			row["last_update_time"] = int64(0)
		}
		tables["scripts"] = append(tables["scripts"], row)
		if err = add("scripts/"+scriptIDs[script.ID]+".js", []byte(script.Source)); err != nil {
			return nil, err
		}
	}
	for i, id := range cfg.Resources {
		if !ValidID(id) || resourcePaths[id] != "" {
			return nil, errors.New("invalid or duplicate resource ID")
		}
		old := snapshot.Resources[id]
		row := original("clash_providers", old)
		fid := idFor(old)
		row["id"], row["kind"], row["url"], row["order"] = fid, "rule", "", i
		if row["label"] == nil {
			row["label"] = "Resource " + strconv.FormatInt(fid, 10)
		}
		if row["behavior"] == nil {
			row["behavior"] = "classical"
		}
		if row["format"] == nil {
			row["format"] = "yaml"
		}
		path := flutterProviderEntry(strconv.FormatInt(fid, 10))
		resourcePaths[id] = path
		b, e := read("resources/" + id)
		if e != nil {
			return nil, e
		}
		if e = add(path, b); e != nil {
			return nil, e
		}
		tables["clash_providers"] = append(tables["clash_providers"], row)
	}
	// Keep remote rule-set metadata, but never package a core-owned download cache.
	for _, row := range snapshot.Tables["clash_providers"] {
		if String(row["url"]) != "" && String(row["kind"]) == "rule" {
			tables["clash_providers"] = append(tables["clash_providers"], row)
		}
	}
	rewrite := func(path string) (string, error) {
		if strings.HasPrefix(path, profileRef) {
			if id := profileIDs[strings.TrimPrefix(path, profileRef)]; id != "" {
				return "profiles/" + id + ".yaml", nil
			}
			return "", errors.New("missing profile reference")
		}
		if strings.HasPrefix(path, resourceRef) {
			if path := resourcePaths[strings.TrimPrefix(path, resourceRef)]; path != "" {
				return path, nil
			}
			return "", errors.New("missing resource reference")
		}
		// Do not export arbitrary workstation paths or silently depend on an old home.
		if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.Contains(path, ":") || containsParentSegment(path) {
			return "", fmt.Errorf("nonportable provider path %q", path)
		}
		return path, nil
	}
	// Local proxy IDs and empty-options survive imports. Reconcile by unique name.
	oldProxies := map[string]map[string]any{}
	for _, row := range tables["custom_proxies"] {
		name := String(flutterJSONMap(row["definition"])["name"])
		if oldProxies[name] != nil {
			return nil, fmt.Errorf("ambiguous original local proxy name %q", name)
		}
		oldProxies[name] = row
	}
	tables["custom_proxies"] = nil
	proxyIDs, proxyDefinitions := map[string]int64{}, map[string]string{}
	if len(cfg.LocalProxies) > 100000 {
		return nil, errors.New("too many exported local proxies")
	}
	for i, definition := range cfg.LocalProxies {
		name := String(definition["name"])
		if name == "" || proxyIDs[name] != 0 {
			return nil, errors.New("invalid or duplicate local proxy name")
		}
		row := copyFlutterRow(oldProxies[name])
		id := idFor(String(row["id"]))
		row["id"], row["definition"], row["order"] = id, jsonText(definition), flutterOrder(i)
		proxyIDs[name] = id
		signature := copyFlutterRow(definition)
		delete(signature, "dialer-proxy")
		proxyDefinitions[name] = jsonText(signature)
		tables["custom_proxies"] = append(tables["custom_proxies"], row)
	}
	// Retain original own/global/disabled rules when the imported overlay has not
	// been edited. Rebuilt profiles disable retained globals to avoid duplication.
	preserved := map[string]bool{}
	for _, p := range cfg.Profiles {
		if baseline, ok := snapshot.Overlays[p.ID]; ok && baseline == p.Override && String(original("profiles", snapshot.Profiles[p.ID])["type"]) == p.Type && cfg.Overrides["rules"] == nil {
			if p.Type == "custom" {
				body, e := read("profiles/" + p.ID + ".yaml")
				if e != nil {
					return nil, e
				}
				source, e := ParseYAML(body)
				if e != nil {
					return nil, e
				}
				if !flutterCustomSourceIsEmpty(source) {
					continue
				}
				overlay, e := ParseYAML([]byte(p.Override))
				if e != nil {
					return nil, e
				}
				missingProxy := false
				for _, value := range flutterJSONList(overlay["proxies"]) {
					if proxyIDs[String(flutterJSONMap(value)["name"])] == 0 {
						missingProxy = true
					}
				}
				if missingProxy {
					continue
				}
			}
			preserved[profileIDs[p.ID]] = true
		}
	}
	for _, name := range []string{"rules", "disabled_rules", "proxy_groups", "proxy_dialers"} {
		old := tables[name]
		tables[name] = nil
		for _, row := range old {
			if row["profile_id"] == nil && name == "rules" || preserved[String(row["profile_id"])] {
				tables[name] = append(tables[name], row)
			}
		}
	}
	retainedGlobals := map[string]bool{}
	tables["rules"] = orderedRows(tables["rules"])
	globalCount := 0
	for _, rule := range tables["rules"] {
		if rule["profile_id"] == nil {
			retainedGlobals[String(rule["id"])] = true
			rule["order"] = flutterOrder(globalCount)
			globalCount++
		}
	}
	if len(tables["rules"])+len(flutterJSONList(cfg.Overrides["rules"])) > 100000 {
		return nil, errors.New("too many exported rules")
	}
	for i, value := range flutterJSONList(cfg.Overrides["rules"]) {
		rule, e := exportFlutterRule(String(value))
		if e != nil {
			return nil, e
		}
		rule["id"], rule["order"] = newID(), flutterOrder(globalCount+i)
		tables["rules"] = append(tables["rules"], rule)
	}
	providerDefinitions := map[string]string{}
	for i, p := range cfg.Profiles {
		fid := profileIDs[p.ID]
		dialers := p.Dialers
		row := original("profiles", snapshot.Profiles[p.ID])
		row["id"], row["label"], row["url"], row["order"] = mustFlutterID(fid), p.Name, p.URL, i
		kind := p.Type
		if kind == "" {
			kind = "file"
			if p.URL != "" {
				kind = "url"
			}
		}
		if kind != "file" && kind != "url" && kind != "custom" {
			return nil, fmt.Errorf("unsupported profile type %q", kind)
		}
		row["type"], row["current_group_name"], row["age_secret_key"], row["match_target"] = kind, p.Group, p.AgeSecretKey, p.MatchTarget
		row["last_update_date"] = nil
		if !p.Updated.IsZero() {
			row["last_update_date"] = p.Updated.Unix()
		}
		period := p.IntervalMS
		if period == 0 {
			period = p.UpdatePeriod().Milliseconds()
		}
		row["auto_update_duration_millis"], row["auto_update"] = period, p.AutoUpdate == nil || *p.AutoUpdate
		info := flutterJSONMap(row["subscription_info"])
		info["upload"], info["download"], info["total"], info["expire"] = p.Upload, p.Download, p.Total, p.Expire
		row["subscription_info"] = jsonText(info)
		selected := p.Selected
		if selected == nil && p.ID == cfg.Active {
			selected = cfg.Selected
		}
		if selected == nil {
			selected = map[string]string{}
		}
		unfold := p.Unfold
		if unfold == nil {
			unfold = []string{}
		}
		row["selected_map"], row["unfold_set"] = jsonText(selected), jsonText(unfold)
		row["extend_type"], row["script_id"] = "standard", nil
		if p.ScriptID != "" {
			id := scriptIDs[p.ScriptID]
			if id == "" {
				return nil, fmt.Errorf("script for profile %s is missing", p.Name)
			}
			row["extend_type"], row["script_id"] = "script", mustFlutterID(id)
		}
		if p.Script != "" {
			id := newID()
			// A selected shared script wins over the dormant inline buffer in
			// the native engine. Keep that buffer as an unselected library item.
			if p.ScriptID == "" {
				row["extend_type"], row["script_id"] = "script", id
			}
			tables["scripts"] = append(tables["scripts"], map[string]any{"id": id, "label": p.Name + " script", "last_update_time": 0, "order": len(tables["scripts"])})
			if err = add(fmt.Sprintf("scripts/%d.js", id), []byte(p.Script)); err != nil {
				return nil, err
			}
		}
		if kind == "custom" && row["extend_type"] == "script" {
			return nil, errors.New("Flutter custom profiles do not support scripts")
		}
		body, e := read("profiles/" + p.ID + ".yaml")
		if e != nil {
			return nil, e
		}
		source, e := ParseYAML(body)
		if e != nil {
			return nil, e
		}
		if kind == "custom" && preserved[fid] && !flutterCustomSourceIsEmpty(source) {
			return nil, errors.New("custom profile changed while exporting")
		}
		overlay := map[string]any{}
		if p.Override != "" {
			overlay, e = ParseYAML([]byte(p.Override))
			if e != nil {
				return nil, e
			}
		}
		if e = validateFlutterJSON(overlay); e != nil {
			return nil, e
		}
		if value, exists := overlay["rules"]; exists {
			if e = validateFlutterAddedRules(value); e != nil {
				return nil, e
			}
		}
		if e = walkPaths(overlay, rewrite); e != nil {
			return nil, e
		}
		if row["extend_type"] == "script" && (len(overlay) > 0 || len(flutterJSONList(cfg.Overrides["rules"])) > 0) {
			return nil, errors.New("Flutter script extensions cannot also apply native YAML overrides or added rules")
		}
		if kind == "custom" && preserved[fid] {
			if e = validateFlutterCustomProviders(overlay, profileLabels, resourceLabels, aliases, rewrite); e != nil {
				return nil, e
			}
			if e = validateFlutterRuleProviderDefinitions(overlay, providerDefinitions); e != nil {
				return nil, e
			}
		}
		if !preserved[fid] {
			if len(tables["disabled_rules"])+len(retainedGlobals) > 100000 {
				return nil, errors.New("too many exported disabled rules")
			}
			for _, global := range tables["rules"] {
				if global["profile_id"] == nil && retainedGlobals[String(global["id"])] {
					tables["disabled_rules"] = append(tables["disabled_rules"], map[string]any{"profile_id": mustFlutterID(fid), "rule_id": global["id"]})
				}
			}
			rules := append([]any{}, flutterJSONList(overlay["rules"])...)
			if kind == "custom" {
				rules = append(rules, flutterJSONList(cfg.Overrides["rules"])...)
			}
			if len(tables["rules"])+len(rules) > 100000 {
				return nil, errors.New("too many exported rules")
			}
			for j, value := range rules {
				rule, e := exportFlutterRule(String(value))
				if e != nil {
					return nil, e
				}
				rule["id"], rule["profile_id"], rule["order"] = newID(), mustFlutterID(fid), flutterOrder(j)
				tables["rules"] = append(tables["rules"], rule)
			}
			delete(overlay, "rules")
			if kind == "custom" {
				MergeConfig(source, overlay)
				if e = validateFlutterJSON(source); e != nil {
					return nil, e
				}
				if e = normalizeFlutterCustomProviderNames(source, profileLabels, rewrite); e != nil {
					return nil, e
				}
				if e = walkPaths(source, rewrite); e != nil {
					return nil, e
				}
				if e = validateFlutterRuleProviderDefinitions(source, providerDefinitions); e != nil {
					return nil, e
				}
				if e = validateFlutterCustomProviders(source, profileLabels, resourceLabels, nil, rewrite); e != nil {
					return nil, e
				}
				dialers, e = reconcileFlutterCustomProxies(source, cfg.LocalProxies, p.Dialers)
				if e != nil {
					return nil, e
				}
				if e = exportFlutterCustom(source, row, &tables, proxyIDs, proxyDefinitions, newID, rewrite); e != nil {
					return nil, fmt.Errorf("profile %s: %w", p.Name, e)
				}
			} else {
				if e = validateFlutterMaterializedOverlay(overlay, cfg); e != nil {
					return nil, e
				}
				for key, value := range overlay {
					source, e := ParseYAML(body)
					if e != nil {
						return nil, e
					}
					MergeConfig(source, map[string]any{key: value})
					body, e = PatchSection(body, key, source[key])
					if e != nil {
						return nil, e
					}
				}
			}
		}
		if kind == "custom" {
			kept := tables["proxy_dialers"][:0]
			for _, dialer := range tables["proxy_dialers"] {
				if String(dialer["profile_id"]) != fid {
					kept = append(kept, dialer)
				}
			}
			tables["proxy_dialers"] = kept
			if len(kept)+len(dialers) > 100000 {
				return nil, errors.New("too many exported proxy dialers")
			}
			for name, target := range dialers {
				if proxyIDs[name] == 0 {
					return nil, fmt.Errorf("missing dialer proxy %q", name)
				}
				tables["proxy_dialers"] = append(tables["proxy_dialers"], map[string]any{"profile_id": mustFlutterID(fid), "proxy_id": proxyIDs[name], "target": target})
			}
		}
		body, e = rewriteFlutterYAMLPaths(body, rewrite)
		if e != nil {
			return nil, e
		}
		if kind != "custom" {
			if e = validateFlutterFileProviders(body); e != nil {
				return nil, e
			}
		}
		if e = add("profiles/"+fid+".yaml", body); e != nil {
			return nil, e
		}
		tables["profiles"] = append(tables["profiles"], row)
	}
	// Flutter's custom provider library is keyed by profile label, whereas the
	// native portable reference is keyed by identity. Carry source renames into
	// group use lists and their saved option maps.
	providerLabels := map[string]int{}
	for _, p := range cfg.Profiles {
		if p.Type != "custom" {
			providerLabels[p.Name]++
		}
	}
	for _, group := range tables["proxy_groups"] {
		use := flutterJSONList(group["use"])
		for i, name := range use {
			if next := aliases[String(name)]; preserved[String(group["profile_id"])] && next != "" {
				use[i] = next
			}
			if providerLabels[String(use[i])] != 1 {
				return nil, fmt.Errorf("custom provider %q needs a unique matching profile label", use[i])
			}
		}
		if group["use"] != nil {
			group["use"] = jsonText(use)
		}
	}
	for _, row := range tables["profiles"] {
		if String(row["type"]) != "custom" || !preserved[String(row["id"])] {
			continue
		}
		overrides := flutterJSONMap(row["overrides"])
		options := map[string]any{}
		for old, value := range flutterJSONMap(overrides["proxy-providers"]) {
			name := old
			if next := aliases[old]; next != "" {
				name = next
			}
			if _, exists := options[name]; exists {
				return nil, fmt.Errorf("ambiguous custom provider options %q", name)
			}
			options[name] = value
		}
		if len(options) > 0 {
			overrides["proxy-providers"] = options
			row["overrides"] = jsonText(overrides)
		}
	}
	for id, path := range resourcePaths {
		if snapshot.Resources[id] == "" && providerDefinitions["path:"+path] == "" {
			return nil, fmt.Errorf("resource %q has no unambiguous Flutter rule-provider metadata", id)
		}
	}
	labels := map[string]bool{}
	for _, provider := range tables["clash_providers"] {
		label := String(provider["label"])
		if labels[label] {
			return nil, fmt.Errorf("ambiguous rule provider label %q", label)
		}
		labels[label] = true
	}
	customIDs := map[string]bool{}
	for _, profile := range tables["profiles"] {
		customIDs[String(profile["id"])] = String(profile["type"]) == "custom"
	}
	for _, rule := range tables["rules"] {
		if String(rule["rule_action"]) == "RULE_SET" && customIDs[String(rule["profile_id"])] && !labels[String(rule["rule_provider"])] {
			return nil, fmt.Errorf("custom rule provider %q is missing", rule["rule_provider"])
		}
	}
	config, err := exportFlutterSettings(cfg, snapshot.Config)
	if err != nil {
		return nil, err
	}
	config["version"] = 1
	config["currentProfileId"] = nil
	if cfg.Active != "" {
		config["currentProfileId"] = mustFlutterID(profileIDs[cfg.Active])
	}
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	if err = add("config.json", b); err != nil {
		return nil, err
	}
	b, err = s.writeFlutterSQLite(tables)
	if err != nil {
		return nil, err
	}
	if err = add("database.sqlite", b); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, e := z.CreateHeader(h)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(files[name]); e != nil {
			return nil, e
		}
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func copyFlutterRow(row map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		out[k] = v
	}
	return out
}
func mustFlutterID(id string) int64 { n, _ := strconv.ParseInt(id, 10, 64); return n }
func jsonText(v any) string         { b, _ := json.Marshal(v); return string(b) }

// The integer sequence from upstream common/indexing.dart: a0..az, b00..bzz,
// c000... . Valid fractional-index keys remain editable after a Flutter restore.
func flutterOrder(i int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	width, capacity := 1, 62
	for i >= capacity {
		i -= capacity
		width++
		capacity *= 62
	}
	key := make([]byte, width+1)
	key[0] = byte('a' + width - 1)
	for n := width; n > 0; n-- {
		key[n] = digits[i%62]
		i /= 62
	}
	return string(key)
}

func (s *Store) writeFlutterSQLite(tables flutterTables) ([]byte, error) {
	for _, rows := range tables {
		if len(rows) > 100000 {
			return nil, errors.New("too many exported database rows")
		}
	}
	dir, err := os.MkdirTemp(s.Dir, ".flutter-export-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "database.sqlite")
	if err = AtomicWrite(path, nil); err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, "PRAGMA page_size=4096; PRAGMA max_page_count=8192; PRAGMA journal_mode=OFF; PRAGMA trusted_schema=OFF; PRAGMA foreign_keys=ON;"+flutterSchema); err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Foreign-key dependencies, not the import reader's presentation order.
	for _, name := range []string{"profiles", "scripts", "rules", "disabled_rules", "proxy_groups", "clash_providers", "custom_proxies", "proxy_dialers", "icon_records", "icon_sets", "ip_qualities"} {
		info, e := tx.QueryContext(ctx, `PRAGMA table_info("`+name+`")`)
		if e != nil {
			return nil, e
		}
		cols := []string{}
		for info.Next() {
			var n, notnull, pk int
			var col, kind string
			var def any
			if e = info.Scan(&n, &col, &kind, &notnull, &def, &pk); e != nil {
				info.Close()
				return nil, e
			}
			cols = append(cols, col)
		}
		e = info.Err()
		info.Close()
		if e != nil {
			return nil, e
		}
		for _, row := range tables[name] {
			keys, marks, values := []string{}, []string{}, []any{}
			for _, col := range cols {
				if value, ok := row[col]; ok {
					if n, ok := value.(json.Number); ok {
						value, e = n.Int64()
						if e != nil {
							return nil, e
						}
					}
					keys = append(keys, `"`+col+`"`)
					marks = append(marks, "?")
					values = append(values, value)
				}
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO "`+name+`" (`+strings.Join(keys, ",")+`) VALUES (`+strings.Join(marks, ",")+`)`, values...); e != nil {
				return nil, fmt.Errorf("export %s: %w", name, e)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if err = db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func encodeFlutterDAVPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	data := []byte(password)
	mask := []byte{0x9d, 0x42, 0xe7, 0x1b, 0x68, 0xb4, 0x35, 0xca, 0x7f, 0x20, 0xd1, 0x56, 0x83, 0xfa, 0x0c, 0xa9}
	for i := range data {
		data[i] ^= nonce[i%16] ^ mask[i%16]
	}
	return "v1." + base64.URLEncoding.EncodeToString(nonce) + "." + base64.URLEncoding.EncodeToString(data), nil
}
