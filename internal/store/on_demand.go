package store

import (
	"errors"
	"slices"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// ExcludesSSID follows the source's exact, case-sensitive membership test.
// SSIDs are identities: do not trim, case-fold, or interpret them as patterns.
func (cfg Settings) ExcludesSSID(ssid string) bool {
	return slices.Contains(cfg.ExcludeSSIDs, ssid)
}

// ValidateSSID is for new edits only. Imported lists are retained unchanged.
func ValidateSSID(ssid string) error {
	if ssid == "" {
		return errors.New("SSID must not be empty")
	}
	if !utf8.ValidString(ssid) {
		return errors.New("SSID must be valid UTF-8")
	}
	// Flutter's maxLength counts extended grapheme clusters, so combining
	// marks and joined emoji must not consume several visible characters.
	iterator := uniseg.NewGraphemes(ssid)
	for count := 0; iterator.Next(); count++ {
		if count == 32 {
			return errors.New("SSID must contain at most 32 characters")
		}
	}
	return nil
}

func importFlutterOnDemand(cfg *Settings, source map[string]any) {
	cfg.ExcludeSSIDs = nil
	for _, item := range flutterJSONList(source["excludeSSIDs"]) {
		if ssid, ok := item.(string); ok {
			cfg.ExcludeSSIDs = append(cfg.ExcludeSSIDs, ssid)
		}
	}
}

func exportFlutterOnDemand(cfg Settings, out map[string]any) {
	// Always replace the owned snapshot field, including after the last removal.
	out["excludeSSIDs"] = append([]string{}, cfg.ExcludeSSIDs...)
}
