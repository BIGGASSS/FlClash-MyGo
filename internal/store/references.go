package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const profileRef = "flclash-profile://"
const resourceRef = "flclash-resource://"

func (s *Store) ResourcePath(id string) string {
	if !ValidID(id) {
		return ""
	}
	return filepath.Join(s.Dir, "resources", id)
}

func walkPaths(value any, rewrite func(string) (string, error)) error {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			if k == "path" {
				if path, ok := child.(string); ok {
					next, err := rewrite(path)
					if err != nil {
						return err
					}
					v[k] = next
					continue
				}
			}
			if err := walkPaths(child, rewrite); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := walkPaths(child, rewrite); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) resolveReferences(config map[string]any) error {
	return walkPaths(config, func(path string) (string, error) {
		resolved := ""
		switch {
		case strings.HasPrefix(path, profileRef):
			resolved = s.ProfilePath(strings.TrimPrefix(path, profileRef))
		case strings.HasPrefix(path, resourceRef):
			resolved = s.ResourcePath(strings.TrimPrefix(path, resourceRef))
		default:
			return path, nil
		}
		if resolved == "" {
			return "", errors.New("invalid provider reference")
		}
		if _, err := os.Stat(resolved); err != nil {
			return "", err
		}
		return resolved, nil
	})
}
func remapReferences(config map[string]any, profiles, resources map[string]string) error {
	return walkPaths(config, func(path string) (string, error) {
		for _, entry := range []struct {
			prefix string
			ids    map[string]string
		}{{profileRef, profiles}, {resourceRef, resources}} {
			if strings.HasPrefix(path, entry.prefix) {
				id := entry.ids[strings.TrimPrefix(path, entry.prefix)]
				if id == "" {
					return "", errors.New("backup provider reference is missing")
				}
				return entry.prefix + id, nil
			}
		}
		return path, nil
	})
}
func remapYAMLReferences(data []byte, profiles, resources map[string]string) ([]byte, error) {
	// Keep source bytes (and comments) unless a portable reference is present.
	if !strings.Contains(string(data), profileRef) && !strings.Contains(string(data), resourceRef) {
		return data, nil
	}
	m, err := ParseYAML(data)
	if err != nil {
		return nil, err
	}
	if err = remapReferences(m, profiles, resources); err != nil {
		return nil, err
	}
	return yaml.Marshal(m)
}
