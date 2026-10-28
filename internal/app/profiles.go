package app

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go.yaml.in/yaml/v3"
)

func (a *App) profilesView(c *ui.Context) {
	width, _ := a.viewSize(c)
	a.profilesGridView(c, &a.profileGrid, max(0, width-a.sidebarSize()), false)
}
func (a *App) chooseImport() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: a.t("import"), Filters: []mygo.FileFilter{{Name: "Clash configuration", Extensions: []string{"yaml", "yml", "txt"}}}, Multiple: true})
		a.update(func() {
			if err != nil {
				a.err = err.Error()
				return
			}
			for _, path := range paths {
				a.importFile(path)
			}
		})
	}()
}
func (a *App) importFile(path string) {
	name := a.addName
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	a.addOpen = false
	a.importProfile(name, "", func() ([]byte, http.Header, error) { b, e := os.ReadFile(path); return b, nil, e })
}
func (a *App) importSource() {
	if a.addKind == 3 {
		a.createCustomProfile()
		return
	}
	if a.addKind == 0 {
		if a.safe {
			a.err = "Network access is disabled in safe mode"
			return
		}
		source, name := strings.TrimSpace(a.addSource), strings.TrimSpace(a.addName)
		if name == "" {
			u, _ := url.Parse(source)
			if u != nil {
				name = u.Hostname()
			}
		}
		client := a.httpClient()
		client.Timeout = 20 * time.Second
		agent := store.String(a.cfg.Overrides["global-ua"])
		a.importProfile(name, source, func() ([]byte, http.Header, error) {
			defer client.CloseIdleConnections()
			return store.Download(client, source, agent)
		})
	} else if a.addKind == 2 {
		source, name := a.addSource, a.addName
		if name == "" {
			name = a.t("profile")
		}
		a.importProfile(name, "", func() ([]byte, http.Header, error) { return []byte(source), nil, nil })
	}
}
func (a *App) createCustomProfile() {
	name := strings.TrimSpace(a.addName)
	if name == "" {
		name = a.t("custom")
	}
	p := store.Profile{ID: store.NewID(), Name: name, Type: "custom", Selected: map[string]string{}, Dialers: map[string]string{}}
	if err := store.AtomicWrite(a.store.ProfilePath(p.ID), []byte("proxies: []\nproxy-groups: []\nrules: [\"MATCH,DIRECT\"]\n")); err != nil {
		a.err = err.Error()
		return
	}
	a.cfg.Profiles = append(a.cfg.Profiles, p)
	a.addOpen = false
	a.addName = ""
	if a.cfg.Active == "" {
		a.cfg.Active = p.ID
	}
	a.editProfile = p.ID
	a.profileDraft = p
	a.goPage("profileEdit")
	a.apply()
}
func (a *App) importProfile(name, source string, load func() ([]byte, http.Header, error)) {
	e := a.engine
	s := a.store
	secret := a.addSecret
	a.work(func() (func(), error) {
		b, h, err := load()
		if err != nil {
			return nil, err
		}
		if len(b) > 32<<20 {
			return nil, errors.New("profile exceeds 32 MiB")
		}
		b, err = decryptProfile(e, s, b, secret)
		if err != nil {
			return nil, err
		}
		if _, err = store.ParseYAML(b); err != nil && e != nil {
			var decoded [][]map[string]any
			if callErr := e.Call("decodeShareLinks", strings.Fields(string(b)), &decoded); callErr != nil {
				return nil, err
			}
			var proxies []map[string]any
			var names []string
			for _, group := range decoded {
				for _, proxy := range group {
					proxies = append(proxies, proxy)
					names = append(names, store.String(proxy["name"]))
				}
			}
			if len(proxies) == 0 {
				return nil, err
			}
			yaml, _ := store.YAML(map[string]any{"proxies": proxies, "proxy-groups": []any{map[string]any{"name": "Proxy", "type": "select", "proxies": names}}, "rules": []string{"MATCH,Proxy"}})
			b = []byte(yaml)
		}
		if _, err = store.ParseYAML(b); err != nil {
			return nil, err
		}
		if e != nil {
			if err = e.Validate(b); err != nil {
				return nil, err
			}
		}
		p := store.Profile{ID: store.NewID(), Name: name, URL: source, Updated: time.Now(), Interval: 24, Selected: map[string]string{}, AgeSecretKey: secret}
		if h != nil {
			p.Subscription(h)
		}
		if err = store.AtomicWrite(s.ProfilePath(p.ID), b); err != nil {
			return nil, err
		}
		return func() {
			a.cfg.Profiles = append(a.cfg.Profiles, p)
			if a.cfg.Active == "" {
				a.cfg.Active = p.ID
			}
			a.addOpen = false
			a.addSource = ""
			a.addSecret = ""
			a.apply()
		}, nil
	})
}
func (a *App) updateProfile(p store.Profile) {
	if p.URL == "" || a.safe || a.profileUpdating[p.ID] {
		return
	}
	if a.profileUpdating == nil {
		a.profileUpdating = map[string]bool{}
	}
	a.profileUpdating[p.ID] = true
	e := a.engine
	s := a.store
	client := a.httpClient()
	client.Timeout = 20 * time.Second
	agent := p.UserAgent
	if agent == "" {
		agent = store.String(a.cfg.Overrides["global-ua"])
	}
	a.work(func() (func(), error) {
		defer client.CloseIdleConnections()
		b, h, err := store.Download(client, p.URL, agent)
		if err != nil {
			return nil, err
		}
		b, err = decryptProfile(e, s, b, p.AgeSecretKey)
		if err != nil {
			return nil, err
		}
		if _, err = store.ParseYAML(b); err != nil {
			return nil, err
		}
		if e != nil {
			if err = e.Validate(b); err != nil {
				return nil, err
			}
		}
		if err = store.AtomicWrite(s.ProfilePath(p.ID), b); err != nil {
			return nil, err
		}
		p.Updated = time.Now()
		p.Subscription(h)
		return func() {
			for i := range a.cfg.Profiles {
				if a.cfg.Profiles[i].ID == p.ID {
					a.cfg.Profiles[i].Updated = p.Updated
					a.cfg.Profiles[i].Upload = p.Upload
					a.cfg.Profiles[i].Download = p.Download
					a.cfg.Profiles[i].Total = p.Total
					a.cfg.Profiles[i].Expire = p.Expire
				}
			}
			a.apply()
		}, nil
	}, func() { delete(a.profileUpdating, p.ID) })
}
func (a *App) updateAllProfiles() {
	for _, p := range a.cfg.Profiles {
		if p.URL != "" {
			a.updateProfile(p)
		}
	}
}
func (a *App) deleteProfile(p store.Profile) {
	a.ask(a.t("delete")+" "+p.Name, a.t("confirm"), func() {
		for i, x := range a.cfg.Profiles {
			if x.ID == p.ID {
				a.cfg.Profiles = append(a.cfg.Profiles[:i], a.cfg.Profiles[i+1:]...)
				break
			}
		}
		if a.cfg.Active == p.ID {
			next := ""
			if len(a.cfg.Profiles) > 0 {
				next = a.cfg.Profiles[0].ID
			}
			a.activateProfile(next)
		} else {
			a.apply()
		}
		if err := a.store.RemoveProfile(p.ID); err != nil {
			a.err = err.Error()
		}
	})
}
func (a *App) exportProfile(p store.Profile) {
	s := a.store
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.window, Title: a.t("export"), DefaultPath: p.Name + ".yaml"})
		if err == nil && path != "" {
			var b []byte
			b, err = os.ReadFile(s.ProfilePath(p.ID))
			if err == nil {
				err = store.AtomicWrite(path, b)
			}
		}
		if err != nil {
			a.update(func() { a.err = err.Error() })
		}
	}()
}
func (a *App) profileEdit(c *ui.Context) {
	p := &a.profileDraft
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(22).Children(func() {
			a.section(c, a.t("profile"), func() {
				a.row(c, "profiles", a.t("name"), func() { ui.TextInput(c, &p.Name).Label(a.t("name")).Width(280) })
				a.separator(c)
				a.row(c, "link", a.t("url"), func() { ui.TextInput(c, &p.URL).Label(a.t("url")).Width(360) })
				a.separator(c)
				a.row(c, "requests", a.t("autoUpdateInterval"), func() {
					v := p.UpdatePeriod().Hours()
					if ui.NumberInput(c, &v, 0, 8760, 1).Width(130).Changed() {
						p.Interval = int(v)
						p.IntervalMS = int64(v * float64(time.Hour/time.Millisecond))
					}
					ui.Text(c, "h").TextColor(a.colors.muted)
				})
				a.separator(c)
				enabled := p.UpdateInterval() > 0
				a.boolRow(c, "", a.t("autoUpdate"), &enabled, func() { p.AutoUpdate = &enabled })
				a.separator(c)
				a.row(c, "settings", "User-Agent", func() { ui.TextInput(c, &p.UserAgent).Width(280).Label("User-Agent") })
				a.separator(c)
				a.row(c, "shield", "Age secret key", func() { ui.TextInput(c, &p.AgeSecretKey).Width(280).Password().Label("Age secret key") })
			})
			a.section(c, a.t("configuration"), func() {
				a.row(c, "edit", a.t("script"), func() { a.scriptSelector(c) })
				a.separator(c)
				a.actionRow(c, "edit", a.t("edit"), func() { a.readProfileCode(p.ID, "profile") })
				a.separator(c)
				a.actionRow(c, "layers", a.t("override"), func() {
					value := map[string]any{}
					if p.Override != "" {
						if parsed, err := store.ParseYAML([]byte(p.Override)); err == nil {
							value = parsed
						}
					}
					a.openCollection(a.t("override"), "profile-override", value)
				})
				a.separator(c)
				a.actionRow(c, "edit", a.t("script"), func() {
					source := p.Script
					if source == "" {
						source = "const main = (config) => {\n  return config;\n};\n"
					}
					a.openCode(a.t("script"), "profile-script", source)
				})
				a.separator(c)
				a.actionRow(c, "table", a.t("rules"), func() { a.editProfileSection(p.ID, "rules") })
				a.separator(c)
				a.actionRow(c, "proxies", a.t("proxies"), func() { a.editProfileSection(p.ID, "proxies") })
				a.separator(c)
				a.actionRow(c, "layers", a.t("proxyGroups"), func() { a.editProfileSection(p.ID, "proxy-groups") })
				a.separator(c)
				a.actionRow(c, "cloud", a.t("providers"), func() { a.editProfileSection(p.ID, "proxy-providers") })
			})
			a.section(c, "", func() {
				a.actionRow(c, "up", a.t("export"), func() { a.exportProfile(*p) })
				a.separator(c)
				a.actionRow(c, "trash", a.t("delete"), func() { a.deleteProfile(*p) })
			})
		})
	})
}
func decryptProfile(e *Engine, s *store.Store, b []byte, key string) ([]byte, error) {
	text := strings.TrimSpace(string(b))
	if !strings.HasPrefix(text, "age-encryption.org/") && !strings.HasPrefix(text, "-----BEGIN AGE ENCRYPTED FILE-----") {
		return b, nil
	}
	if e == nil {
		return nil, errors.New("start the core to decrypt an Age profile")
	}
	f, err := os.CreateTemp(s.Dir, "encrypted-*")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	var plain string
	err = e.Call("decryptConfig", map[string]string{"path": path, "age-secret-key": key}, &plain)
	return []byte(plain), err
}
func (a *App) actionRow(c *ui.Context, icon, label string, fn func()) {
	ui.ButtonBase(c).FillWidth().OnClick(fn).Children(func() { a.row(c, icon, label, func() { glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted) }) })
}
func (a *App) saveProfile() {
	if strings.TrimSpace(a.profileDraft.Name) == "" {
		a.err = a.t("profileNameNullValidationDesc")
		return
	}
	for i := range a.cfg.Profiles {
		if a.cfg.Profiles[i].ID == a.editProfile {
			a.cfg.Profiles[i] = a.profileDraft
			a.apply()
			a.back()
			return
		}
	}
}
func (a *App) readProfileCode(id, purpose string) {
	s := a.store
	serial, title := a.beginEditorLoad(), a.profileDraft.Name
	a.work(func() (func(), error) {
		b, err := os.ReadFile(s.ProfilePath(id))
		return func() {
			if a.editorLoadSerial == serial && a.editProfile == id {
				a.openCode(title, purpose, string(b))
			}
		}, err
	})
}
func (a *App) editProfileSection(id, section string) {
	s := a.store
	serial := a.beginEditorLoad()
	a.work(func() (func(), error) {
		b, e := os.ReadFile(s.ProfilePath(id))
		if e != nil {
			return nil, e
		}
		m, e := store.ParseYAML(b)
		if e != nil {
			return nil, e
		}
		v := m[section]
		if v == nil {
			if strings.Contains(section, "providers") {
				v = map[string]any{}
			} else {
				v = []any{}
			}
		}
		return func() {
			if a.editorLoadSerial == serial && a.editProfile == id {
				a.openCollection(humanKey(section), "section:"+section, v)
			}
		}, nil
	})
}
func (a *App) saveCode() {
	if a.code == nil {
		return
	}
	text := a.code.Buffer.String()
	code, tab, version := a.code, a.navigationParent(), a.code.Buffer.Version()
	purpose := a.codePurpose
	if strings.HasPrefix(purpose, "provider:") {
		if !a.codeProvider.valid(a) {
			a.err = a.t("editorContextChanged")
			return
		}
		a.sideLoadProvider(strings.TrimPrefix(purpose, "provider:"), []byte(text), func() { a.editorSaved(code, tab, text, version) }, a.codeProvider.Path)
		return
	}
	if a.saveLibraryScript(purpose, text) {
		return
	}
	s := a.store
	e := a.engine
	id := a.editProfile
	switch purpose {
	case "document":
		var value any
		if err := yaml.Unmarshal([]byte(text), &value); err != nil {
			a.err = err.Error()
			return
		}
		if a.document != nil {
			a.document.Value = value
		}
		a.back()
		return
	case "profile-override":
		if strings.TrimSpace(text) != "" {
			if _, err := store.ParseYAML([]byte(text)); err != nil {
				a.err = err.Error()
				return
			}
		}
		a.profileDraft.Override = text
		a.persistProfileDraft()
		a.back()
		return
	case "profile-script":
		a.profileDraft.Script = text
		a.persistProfileDraft()
		a.back()
		return
	case "global":
		m, err := store.ParseYAML([]byte(text))
		if err != nil {
			a.err = err.Error()
			return
		}
		a.cfg.Overrides = m
		a.apply()
		a.back()
		return
	}
	if strings.HasPrefix(purpose, "global:") {
		key := strings.TrimPrefix(purpose, "global:")
		var value any
		if err := yaml.Unmarshal([]byte(text), &value); err != nil {
			a.err = err.Error()
			return
		}
		setPath(a.cfg.Overrides, key, value)
		a.apply()
		a.back()
		return
	}
	a.work(func() (func(), error) {
		b := []byte(text)
		if strings.HasPrefix(purpose, "section:") {
			section := strings.TrimPrefix(purpose, "section:")
			original, err := os.ReadFile(s.ProfilePath(id))
			if err != nil {
				return nil, err
			}
			m, err := store.ParseYAML(original)
			if err != nil {
				return nil, err
			}
			var value any
			if err = yaml.Unmarshal(b, &value); err != nil {
				return nil, err
			}
			m[section] = value
			out, err := store.YAML(m)
			if err != nil {
				return nil, err
			}
			b = []byte(out)
		}
		if _, err := store.ParseYAML(b); err != nil {
			return nil, err
		}
		if e != nil {
			if err := e.Validate(b); err != nil {
				return nil, err
			}
		}
		if err := store.AtomicWrite(s.ProfilePath(id), b); err != nil {
			return nil, err
		}
		return func() { a.apply(); a.editorSaved(code, tab, text, version) }, nil
	})
}
