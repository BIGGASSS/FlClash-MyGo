package app

import "github.com/BIGGASSS/FlClash-MyGo/internal/store"

// Lifecycle application and runtime preview share the same script/override
// pipeline. Preview does not launch a core, listeners or system takeover.
func effectiveConfig(s *store.Store, cfg store.Settings) (map[string]any, error) {
	source := ""
	if profile := s.ActiveProfile(cfg); profile != nil {
		if profile.Selected != nil {
			cfg.Selected = profile.Selected
		}
		source = profile.Script
		if profile.ScriptID != "" {
			for _, script := range cfg.Scripts {
				if script.ID == profile.ScriptID {
					source = script.Source
					break
				}
			}
		}
	}
	var transform func(map[string]any) (map[string]any, error)
	if source != "" {
		transform = func(raw map[string]any) (map[string]any, error) { return runScript(raw, source) }
	}
	return s.EffectiveTransform(cfg, transform)
}
