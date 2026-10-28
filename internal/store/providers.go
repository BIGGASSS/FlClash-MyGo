package store

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) providerRoot(profileID, kind string) string {
	if !ValidID(profileID) {
		profileID = "standalone"
	}
	return filepath.Join(s.Dir, "profiles", "providers", profileID, kind)
}

func providerHash(key string) string {
	digest := md5.Sum([]byte(key))
	return hex.EncodeToString(digest[:])
}

func (s *Store) confineProviders(raw map[string]any, profileID string) error {
	for section, kind := range map[string]string{"proxy-providers": "proxies", "rule-providers": "rules"} {
		providers, _ := raw[section].(map[string]any)
		for name, value := range providers {
			provider, ok := value.(map[string]any)
			if !ok || provider["type"] == "inline" {
				continue
			}
			path := String(provider["path"])
			if strings.HasPrefix(path, profileRef) || strings.HasPrefix(path, resourceRef) {
				continue
			}
			root := s.providerRoot(profileID, kind)
			if err := os.MkdirAll(root, 0700); err != nil {
				return err
			}
			url := String(provider["url"])
			key := section + "/" + name
			if url != "" {
				key = name + "@" + url
			}
			target := filepath.Join(root, providerHash(key))
			if url != "" {
				if err := migrateProviderCache(filepath.Join(root, providerHash(url)), target); err != nil {
					return err
				}
			}
			provider["path"] = target
		}
	}
	return nil
}

func migrateProviderCache(legacy, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	info, err := os.Lstat(legacy)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil
	}
	f, err := os.Open(legacy)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return nil
	}
	return AtomicWrite(target, data)
}
