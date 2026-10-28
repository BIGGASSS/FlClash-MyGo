package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func flutterCustomSourceIsEmpty(source map[string]any) bool {
	for key, value := range source {
		if key != "proxies" && key != "proxy-groups" && key != "rules" {
			return false
		}
		list, ok := value.([]any)
		if !ok || len(list) != 0 {
			return false
		}
	}
	return true
}

func validateFlutterAddedRules(value any) error {
	list, ok := value.([]any)
	if !ok {
		return errors.New("added rules must be a list")
	}
	for _, rule := range list {
		if _, ok := rule.(string); !ok {
			return errors.New("added rules must contain strings")
		}
	}
	return nil
}

// Materializing an overlay in subscription YAML cannot override settings that
// Flutter applies *after* reading that YAML. Native profile settings win there.
func validateFlutterMaterializedOverlay(overlay map[string]any, cfg Settings) error {
	owned := runtimeDefaults()
	owned["interface-name"], owned["interface-name-mode"] = "", "clear"
	Merge(owned, cfg.Overrides)
	if cfg.Overrides["interface-name-mode"] == nil && String(cfg.Overrides["interface-name"]) != "" {
		owned["interface-name-mode"] = "custom"
	}
	var conflicts func(any, any) bool
	conflicts = func(own, global any) bool {
		a, aok := own.(map[string]any)
		b, bok := global.(map[string]any)
		if aok && bok {
			for key, value := range a {
				if other, exists := b[key]; exists && conflicts(value, other) {
					return true
				}
			}
			return false
		}
		return jsonText(own) != jsonText(global)
	}
	for key, value := range overlay {
		if key == "rules" {
			continue
		}
		if global, exists := owned[key]; exists && conflicts(value, global) {
			return fmt.Errorf("Flutter cannot retain profile precedence for %q; use native backup", key)
		}
	}
	return nil
}

// Unlike native portable paths, Flutter's custom provider namespace is the
// current profile label. Resolve by identity first, then rename keys and uses
// simultaneously (including label swaps); never apply a global string alias to
// freshly rebuilt groups or options a second time.
func normalizeFlutterCustomProviderNames(source map[string]any, profileLabels map[string]string, rewrite func(string) (string, error)) error {
	providers := flutterJSONMap(source["proxy-providers"])
	renames, next := map[string]string{}, map[string]any{}
	for name, value := range providers {
		def, ok := value.(map[string]any)
		if !ok || String(def["type"]) != "file" {
			return fmt.Errorf("custom proxy provider %q must be a file profile reference", name)
		}
		path, err := rewrite(String(def["path"]))
		if err != nil {
			return err
		}
		label := profileLabels[path]
		if label == "" || next[label] != nil {
			return fmt.Errorf("custom proxy provider %q has a missing or ambiguous profile reference", name)
		}
		for key := range def {
			if !strings.Contains(",type,path,filter,exclude-filter,override,health-check,memberLayout,", ","+key+",") {
				return fmt.Errorf("Flutter cannot represent custom proxy provider option %q", key)
			}
		}
		for section, keys := range map[string]string{
			"health-check": ",enable,url,interval,timeout,lazy,expected-status,",
			"override":     ",additional-prefix,additional-suffix,udp,skip-cert-verify,ip-version,",
		} {
			if def[section] == nil {
				continue
			}
			options, ok := def[section].(map[string]any)
			if !ok {
				return fmt.Errorf("custom provider %s must be a mapping", section)
			}
			for key := range options {
				if !strings.Contains(keys, ","+key+",") {
					return fmt.Errorf("Flutter cannot represent custom provider %s option %q", section, key)
				}
			}
		}
		if health := flutterJSONMap(def["health-check"]); health["enable"] != nil && !flutterBool(health["enable"]) {
			return fmt.Errorf("Flutter cannot retain disabled health-check options for %q", name)
		}
		renames[name], next[label] = label, def
	}
	if source["proxy-providers"] != nil {
		source["proxy-providers"] = next
	}
	for _, value := range flutterJSONList(source["proxy-groups"]) {
		group := flutterJSONMap(value)
		use, ok := group["use"].([]any)
		if group["use"] != nil && !ok {
			return errors.New("custom group use must be a list")
		}
		for i, name := range use {
			if label := renames[String(name)]; label != "" {
				use[i] = label
			}
		}
	}
	return nil
}

// Flutter's library is global; different definitions cannot share a label or
// one local resource identity, even when the native profiles are independent.
func validateFlutterRuleProviderDefinitions(source map[string]any, definitions map[string]string) error {
	for name, value := range flutterJSONMap(source["rule-providers"]) {
		def, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid rule provider %q", name)
		}
		for key := range def {
			if !strings.Contains(",type,path,url,behavior,format,interval,", ","+key+",") {
				return fmt.Errorf("Flutter cannot represent custom rule provider option %q", key)
			}
		}
		remote, kind := String(def["url"]), String(def["type"])
		if remote == "" && kind != "file" || remote != "" && kind != "http" {
			return fmt.Errorf("unsupported custom rule provider %q", name)
		}
		if def["interval"] != nil && Int(def["interval"]) != 86400 {
			return fmt.Errorf("Flutter cannot retain custom rule provider interval for %q", name)
		}
		behavior, format := String(def["behavior"]), String(def["format"])
		if behavior == "" {
			behavior = "classical"
		}
		if format == "" {
			format = "yaml"
		}
		if !strings.Contains(",classical,domain,ipcidr,", ","+behavior+",") || !strings.Contains(",yaml,text,mrs,", ","+format+",") || format == "mrs" && behavior == "classical" {
			return fmt.Errorf("unsupported custom rule provider format/behavior for %q", name)
		}
		identity := "url:" + remote
		if remote == "" {
			identity = "path:" + String(def["path"])
		}
		signature := jsonText([]string{identity, behavior, format})
		if old := definitions["label:"+name]; old != "" && old != signature {
			return fmt.Errorf("conflicting custom rule provider label %q", name)
		}
		if old := definitions[identity]; remote == "" && old != "" && old != jsonText([]string{name, behavior, format}) {
			return fmt.Errorf("conflicting custom rule provider identity %q", name)
		}
		definitions["label:"+name] = signature
		definitions[identity] = jsonText([]string{name, behavior, format})
	}
	return nil
}

// Native custom source proxies can be profile-local, but Flutter stores them in
// one library and obtains dialers from a separate per-profile table. Match the
// native library's precedence before checking for cross-profile collisions.
func reconcileFlutterCustomProxies(source map[string]any, library []map[string]any, savedDialers map[string]string) (map[string]string, error) {
	dialers := map[string]string{}
	for name, target := range savedDialers {
		dialers[name] = target
	}
	named, groups := map[string]bool{}, map[string]bool{}
	for _, value := range flutterJSONList(source["proxy-groups"]) {
		group := flutterJSONMap(value)
		name := String(group["name"])
		if name == "" || groups[name] {
			return nil, errors.New("invalid or duplicate custom group name")
		}
		groups[name] = true
		for _, key := range []string{"proxies", "use"} {
			list, ok := group[key].([]any)
			if group[key] != nil && !ok {
				return nil, fmt.Errorf("custom group %s must be a list", key)
			}
			for _, value := range list {
				name, ok := value.(string)
				if !ok || name == "" {
					return nil, errors.New("custom group members must be nonempty strings")
				}
				if key == "proxies" {
					named[name] = true
				}
			}
		}
	}
	proxies := flutterJSONList(source["proxies"])
	seen := map[string]bool{}
	for _, value := range proxies {
		definition := flutterJSONMap(value)
		name := String(definition["name"])
		if name == "" || seen[name] {
			return nil, errors.New("invalid or duplicate custom proxy name")
		}
		seen[name] = true
		if !named[name] {
			return nil, fmt.Errorf("Flutter cannot retain unreferenced custom source proxy %q", name)
		}
		delete(dialers, name)
		if target := String(definition["dialer-proxy"]); target != "" {
			dialers[name] = target
		}
		delete(definition, "dialer-proxy")
	}
	for _, definition := range library {
		name := String(definition["name"])
		if !named[name] {
			continue
		}
		definition = copyFlutterRow(definition)
		delete(definition, "dialer-proxy")
		MergeConfig(source, map[string]any{"proxies": []any{definition}})
		delete(dialers, name)
		if target := savedDialers[name]; target != "" {
			dialers[name] = target
		}
	}
	return dialers, nil
}

func validateFlutterJSON(value any) error {
	_, err := json.Marshal(value)
	return err
}
