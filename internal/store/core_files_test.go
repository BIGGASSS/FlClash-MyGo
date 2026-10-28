package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoreHomeMigrationPreservesUpdatedGeoDataAndCache(t *testing.T) {
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GEOIP.metadb", "GEOIP.dat", "GEOSITE.dat", "ASN.mmdb", "cache.db"} {
		if err := AtomicWrite(filepath.Join(s.Dir, "core", name), []byte("updated "+name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := AtomicWrite(filepath.Join(s.Dir, "GEOIP.dat"), []byte("already migrated")); err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareCoreFiles(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GEOIP.metadb", "GEOIP.dat", "GEOSITE.dat", "ASN.mmdb", "cache.db"} {
		want := "updated " + name
		if name == "GEOIP.dat" {
			want = "already migrated"
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
		if err != nil || string(data) != want {
			t.Fatal("updated data was replaced during migration", name, string(data), err)
		}
	}
	if err := s.PrepareCoreFiles(); err != nil {
		t.Fatal("repeated preparation failed", err)
	}
}
