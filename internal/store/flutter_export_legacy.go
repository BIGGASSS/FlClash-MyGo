package store

import (
	"errors"
	"strconv"
)

// Legacy UUIDs and raw rules are an importer convenience, not Drift SQL values.
// Promote them consistently before reconciling the current native entities.
func normalizeFlutterExportSnapshot(snapshot *flutterSnapshot) error {
	next := int64(1)
	for _, rows := range snapshot.Tables {
		for _, row := range rows {
			if n, e := strconv.ParseInt(String(row["id"]), 10, 64); e == nil && n >= next {
				if n >= 1<<62 {
					return errors.New("FlClash ID exceeds export range")
				}
				next = n + 1
			}
		}
	}
	ids := map[string]map[string]string{}
	for _, name := range []string{"profiles", "scripts", "rules", "proxy_groups", "clash_providers", "custom_proxies", "icon_sets"} {
		mapping, normalized := map[string]string{}, map[int64]bool{}
		ids[name] = mapping
		for _, row := range snapshot.Tables[name] {
			old := String(row["id"])
			if old == "" {
				return errors.New("missing FlClash snapshot identity")
			}
			if _, exists := mapping[old]; exists {
				return errors.New("duplicate FlClash snapshot identity")
			}
			n, e := strconv.ParseInt(old, 10, 64)
			if e != nil || n <= 0 {
				n = next
				next++
			}
			if normalized[n] {
				return errors.New("ambiguous FlClash snapshot identity")
			}
			normalized[n] = true
			mapping[old] = strconv.FormatInt(n, 10)
			row["id"] = n
		}
	}
	for _, item := range []struct {
		values map[string]string
		table  string
	}{{snapshot.Profiles, "profiles"}, {snapshot.Scripts, "scripts"}, {snapshot.Resources, "clash_providers"}} {
		claimed := map[string]bool{}
		for native, old := range item.values {
			value := ids[item.table][old]
			if value == "" || claimed[value] {
				return errors.New("missing or ambiguous FlClash identity correspondence")
			}
			claimed[value] = true
			item.values[native] = value
		}
	}
	for name, rows := range snapshot.Tables {
		for i, row := range rows {
			for field, table := range map[string]string{"profile_id": "profiles", "script_id": "scripts", "rule_id": "rules", "proxy_id": "custom_proxies"} {
				if row[field] != nil {
					if mapped := ids[table][String(row[field])]; mapped != "" {
						row[field] = mustFlutterID(mapped)
					}
				}
			}
			if name == "rules" && row["raw"] != nil {
				parsed, err := exportFlutterRule(String(row["raw"]))
				if err != nil {
					return err
				}
				for key, value := range parsed {
					row[key] = value
				}
				delete(row, "raw")
				row["order"] = flutterOrder(i)
			}
		}
	}
	return nil
}
