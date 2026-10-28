package app

import (
	"encoding/json"
	"errors"

	"github.com/BIGGASSS/FlClash-MyGo/internal/script"
)

func runScript(config map[string]any, source string) (map[string]any, error) {
	input, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	var original map[string]any
	if err = json.Unmarshal(input, &original); err != nil {
		return nil, err
	}
	if original["proxy-providers"] == nil {
		original["proxy-providers"] = map[string]any{}
	}
	input, err = json.Marshal(original)
	if err != nil {
		return nil, err
	}
	encoded, err := script.Evaluate(source, input)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err = json.Unmarshal(encoded, &out); err != nil {
		return nil, errors.New("script did not return a configuration object")
	}
	if out == nil {
		return nil, errors.New("script did not return a configuration object")
	}
	return out, nil
}
