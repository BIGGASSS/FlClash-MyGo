package app

import (
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
)

// The interoperable format is an explicit choice. Never replace native backups:
// Flutter cannot represent every native-only setting or profile extension.
func (a *App) backupLocalFormat(flutter bool) {
	cfg, s := store.Clone(a.cfg), a.store
	name := "FlClash-MyGo.zip"
	if flutter {
		name = "FlClash.zip"
	}
	options := mygo.SaveDialogOptions{Parent: a.window, Title: a.t("backup"), DefaultPath: name, Filters: []mygo.FileFilter{{Name: "Backup", Extensions: []string{"zip"}}}}
	a.work(func() (func(), error) {
		path, err := mygo.Dialog.Save(options)
		if err != nil || path == "" {
			return nil, err
		}
		if err = writeLocalBackup(s, cfg, path, flutter); err != nil {
			return nil, err
		}
		return func() { a.message = a.t("success") }, nil
	})
}

func writeLocalBackup(s *store.Store, cfg store.Settings, path string, flutter bool) error {
	var data []byte
	var err error
	if flutter {
		data, err = s.FlutterBackup(cfg)
	} else {
		data, err = s.Backup(cfg)
	}
	if err != nil {
		return err
	}
	// Both archives contain credentials. AtomicWrite creates private files and
	// leaves an existing export untouched when serialization/validation fails.
	return store.AtomicWrite(path, data)
}
