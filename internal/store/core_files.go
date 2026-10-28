package store

import (
	"os"
	"path/filepath"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
)

// The core's safe-path root includes profiles and resources as well as its own
// files. Existing development layouts kept the latter in a core/ child.
func (s *Store) PrepareCoreFiles() error {
	for _, name := range []string{"GEOIP.metadb", "GEOIP.dat", "GEOSITE.dat", "ASN.mmdb", "cache.db"} {
		target := filepath.Join(s.Dir, name)
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		legacy := filepath.Join(s.Dir, "core", name)
		if info, err := os.Lstat(legacy); err == nil && info.Mode().IsRegular() {
			if err := os.Rename(legacy, target); err != nil {
				return err
			}
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if name != "cache.db" {
			if err := AtomicWrite(target, assets.Read("data/"+name)); err != nil {
				return err
			}
		}
	}
	return nil
}
