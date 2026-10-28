package store

import "time"

// Backups predating Drift kept their entities inside config.json and used UUID
// paths. Normalize them into the same rows used by the SQLite reader.
func legacyFlutter(source map[string]any, files map[string][]byte) flutterTables {
	tables := flutterTables{}
	if source["appSettingProps"] == nil {
		source["appSettingProps"] = source["appSetting"]
	}
	if source["proxiesStyleProps"] == nil {
		source["proxiesStyleProps"] = source["proxiesStyle"]
	}
	if source["davProps"] == nil {
		source["davProps"] = source["dav"]
	}
	scripts := flutterJSONList(source["scripts"])
	if len(scripts) == 0 {
		scripts = flutterJSONList(flutterJSONMap(source["scriptProps"])["scripts"])
	}
	for _, v := range scripts {
		script := flutterJSONMap(v)
		id := String(script["id"])
		if id == "" {
			continue
		}
		files["scripts/"+id+".js"] = []byte(String(script["content"]))
		tables["scripts"] = append(tables["scripts"], map[string]any{"id": id, "label": script["label"]})
	}
	for i, v := range flutterJSONList(source["rules"]) {
		rule := flutterJSONMap(v)
		tables["rules"] = append(tables["rules"], map[string]any{"id": rule["id"], "raw": rule["value"], "order": i})
	}
	for i, v := range flutterJSONList(source["profiles"]) {
		profile := flutterJSONMap(v)
		id := String(profile["id"])
		if id == "" {
			continue
		}
		row := map[string]any{"id": id, "type": "file", "label": profile["label"], "url": profile["url"], "auto_update_duration_millis": Int(profile["autoUpdateDuration"]) / 1000, "auto_update": true, "selected_map": profile["selectedMap"], "subscription_info": profile["subscriptionInfo"], "unfold_set": profile["unfoldSet"], "current_group_name": profile["currentGroupName"], "age_secret_key": profile["ageSecretKey"], "order": i}
		if String(profile["url"]) != "" {
			row["type"] = "url"
		}
		if auto, ok := profile["autoUpdate"]; ok {
			row["auto_update"] = auto
		}
		if updated, err := time.Parse(time.RFC3339, String(profile["lastUpdateDate"])); err == nil {
			row["last_update_date"] = updated.Unix()
		}
		overwrite := flutterJSONMap(profile["overwrite"])
		row["extend_type"] = overwrite["type"]
		row["script_id"] = flutterJSONMap(overwrite["scriptOverwrite"])["scriptId"]
		standard := flutterJSONMap(overwrite["standardOverwrite"])
		for j, v := range flutterJSONList(standard["addedRules"]) {
			rule := flutterJSONMap(v)
			tables["rules"] = append(tables["rules"], map[string]any{"id": rule["id"], "profile_id": id, "raw": rule["value"], "order": j})
		}
		for _, disabled := range flutterJSONList(standard["disabledRuleIds"]) {
			tables["disabled_rules"] = append(tables["disabled_rules"], map[string]any{"profile_id": id, "rule_id": disabled})
		}
		tables["profiles"] = append(tables["profiles"], row)
	}
	return tables
}
