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
	"github.com/egoist/mygo/ui/editor"
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
	scroll := ui.Scroll(c).Fill()
	if nav := a.profilePickerNavigation(); nav != nil {
		scroll.TrackScroll(&nav.Scroll)
	}
	scroll.Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.profileEditorBottomInset(c), 16).Gap(22).Children(func() {
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
				a.actionRow(c, "edit", a.t("edit"), func() {
					purpose := "profile"
					if a.profilePickerNavigation() != nil {
						purpose = "profile-staged"
					}
					a.readProfileCode(p.ID, purpose)
				})
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
func (a *App) profileSheetEditView(c *ui.Context) {
	p, nav := &a.profileDraft, a.profilePickerNavigation()
	ui.Scroll(c).Fill().TrackScroll(&nav.Scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(12, 16, 24, 16).Gap(22).Children(func() {
			a.section(c, a.t("profile"), func() {
				ui.Column(c).FillWidth().Padding(16).Gap(16).Children(func() {
					ui.TextInput(c, &p.Name).FillWidth().Label(a.t("name"))
					if nav.Initial.URL != "" {
						ui.TextInput(c, &p.URL).FillWidth().Label(a.t("url"))
						ui.TextInput(c, &p.AgeSecretKey).FillWidth().Password().Label("Age secret key")
						enabled := p.UpdateInterval() > 0
						a.boolRow(c, "", a.t("autoUpdate"), &enabled, func() { p.AutoUpdate = &enabled })
						minutes := p.UpdatePeriod().Minutes()
						if ui.NumberInput(c, &minutes, 1, 525600, 1).FillWidth().Label(a.t("autoUpdateInterval") + " (min)").Changed() {
							p.IntervalMS = int64(minutes * float64(time.Minute/time.Millisecond))
							p.Interval = int(minutes / 60)
						}
					}
				})
			})
			a.section(c, a.t("file"), func() {
				a.actionRow(c, "edit", a.t("edit"), func() { a.readProfileCode(p.ID, "profile-staged") })
			})
		})
	})
}

func (a *App) profileEditorBottomInset(c *ui.Context) float32 {
	if a.profilePickerNavigation() != nil {
		return 0
	}
	return a.bottomNavigationInset(c)
}

// These routes reuse the native structured editors. The source's specialized
// group/member/preset forms remain separate parity work.
func (a *App) profileActionView(c *ui.Context, page string) {
	p := &a.profileDraft
	scroll := ui.Scroll(c).Fill()
	if nav := a.profilePickerNavigation(); nav != nil {
		scroll.TrackScroll(&nav.Scroll)
	}
	scroll.Children(func() {
		ui.Column(c).FillWidth().Padding(8, 16, 24+a.profileEditorBottomInset(c), 16).Gap(22).Children(func() {
			if page == "profileRename" {
				a.section(c, a.t("rename"), func() {
					ui.TextInput(c, &p.Name).FillWidth().Label(a.t("name")).AutoFocus()
				})
				return
			}
			if page == "profileExtend" {
				a.section(c, a.t("extend"), func() {
					a.row(c, "edit", a.t("script"), func() { a.scriptSelector(c) })
					a.separator(c)
					a.actionRow(c, "table", a.t("addedRules"), func() {
						value := map[string]any{}
						if p.Override != "" {
							value, _ = store.ParseYAML([]byte(p.Override))
						}
						rules := value["rules"]
						if rules == nil {
							rules = []any{}
						}
						a.openCollection(a.t("addedRules"), "profile-added-rules", rules)
					})
					if p.Type != "custom" {
						a.separator(c)
						a.profileMatchTargetRow(c)
					}
					a.separator(c)
					a.actionRow(c, "layers", a.t("override"), func() {
						value := map[string]any{}
						if p.Override != "" {
							value, _ = store.ParseYAML([]byte(p.Override))
						}
						a.openCollection(a.t("override"), "profile-override", value)
					})
					a.separator(c)
					a.actionRow(c, "edit", a.t("script"), func() { a.openCode(a.t("script"), "profile-script", p.Script) })
				})
				return
			}
			a.section(c, "", func() {
				sections := []struct{ icon, key, section string }{
					{"table", "rules", "rules"}, {"cloud", "providers", "proxy-providers"},
					{"dns", "DNS", "dns"}, {"layers", "proxyGroups", "proxy-groups"},
					{"proxies", "proxies", "proxies"},
				}
				for i, row := range sections {
					if i > 0 {
						a.separator(c)
					}
					a.actionRow(c, row.icon, a.t(row.key), func() { a.editProfileSection(p.ID, row.section) })
				}
			})
			a.section(c, a.t("more"), func() {
				for i, section := range []string{"rule-providers", "ntp", "sniffer", "tun"} {
					if i > 0 {
						a.separator(c)
					}
					a.actionRow(c, "settings", humanKey(section), func() { a.editProfileSection(p.ID, section) })
				}
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

// Merge only editor-owned fields. Subscription refreshes and proxy selections
// may have changed while this draft (or its asynchronous save) was open.
func mergeProfileDraft(current, draft store.Profile) store.Profile {
	current.Name, current.URL = draft.Name, draft.URL
	current.Interval, current.IntervalMS, current.AutoUpdate = draft.Interval, draft.IntervalMS, draft.AutoUpdate
	current.UserAgent, current.AgeSecretKey = draft.UserAgent, draft.AgeSecretKey
	current.Override, current.Script, current.ScriptID = draft.Override, draft.Script, draft.ScriptID
	current.MatchTarget = draft.MatchTarget
	return current
}

// Apply only fields actually changed by this sheet, not stale copies of fields
// changed by another editor, subscription refresh or proxy selection.
func mergeProfileChanges(current, initial, draft store.Profile) store.Profile {
	if initial.Name != draft.Name {
		current.Name = draft.Name
	}
	if initial.URL != draft.URL {
		current.URL = draft.URL
	}
	if initial.Interval != draft.Interval {
		current.Interval = draft.Interval
	}
	if initial.IntervalMS != draft.IntervalMS {
		current.IntervalMS = draft.IntervalMS
	}
	if !profileDraftEqual(store.Profile{AutoUpdate: initial.AutoUpdate}, store.Profile{AutoUpdate: draft.AutoUpdate}) {
		current.AutoUpdate = draft.AutoUpdate
	}
	if initial.UserAgent != draft.UserAgent {
		current.UserAgent = draft.UserAgent
	}
	if initial.AgeSecretKey != draft.AgeSecretKey {
		current.AgeSecretKey = draft.AgeSecretKey
	}
	if initial.Override != draft.Override {
		current.Override = draft.Override
	}
	if initial.Script != draft.Script {
		current.Script = draft.Script
	}
	if initial.ScriptID != draft.ScriptID {
		current.ScriptID = draft.ScriptID
	}
	if initial.MatchTarget != draft.MatchTarget {
		current.MatchTarget = draft.MatchTarget
	}
	return current
}

func (a *App) saveProfile() {
	if strings.TrimSpace(a.profileDraft.Name) == "" {
		a.err = a.t("profileNameNullValidationDesc")
		return
	}
	draft, owner := a.profileDraft, a.editorOwner()
	nav := a.profilePickerNavigation()
	if draft.ID == "" || draft.ID != a.editProfile || nav != nil && nav.Initial.ID != draft.ID {
		a.err = a.t("editorContextChanged")
		return
	}
	if nav != nil && nav.Page == "profileEdit" && nav.Initial.URL != "" {
		if strings.TrimSpace(draft.URL) == "" {
			a.err = a.t("profileUrlNullValidationDesc")
			return
		}
		u, err := url.ParseRequestURI(draft.URL)
		if err != nil || u.Hostname() == "" || u.Scheme != "https" && u.Scheme != "http" {
			a.err = a.t("profileUrlInvalidValidationDesc")
			return
		}
	}
	rename := a.page == "profileRename"
	var revision uint64
	var initial store.Profile
	var source, original *string
	if nav != nil {
		if nav.Busy {
			return
		}
		revision, initial, source, original = nav.Revision, nav.Initial, nav.Source, nav.SourceOriginal
		rename = nav.Page == "profileRename"
	}
	done := func() {
		next := store.Clone(a.cfg)
		found := false
		for i := range next.Profiles {
			if next.Profiles[i].ID != draft.ID {
				continue
			}
			found = true
			if rename {
				next.Profiles[i].Name = draft.Name
			} else if nav != nil {
				next.Profiles[i] = mergeProfileChanges(next.Profiles[i], initial, draft)
			} else {
				next.Profiles[i] = mergeProfileDraft(next.Profiles[i], draft)
			}
			break
		}
		if !found {
			a.err = a.t("editorContextChanged")
			return
		}
		// apply() cannot signal persistence errors to callers. Do not mark
		// this draft saved or navigate until persistence succeeds.
		if err := a.store.Save(next); err != nil {
			a.err = err.Error()
			return
		}
		if source != nil {
			// Commit on the UI thread after validation, so both settings and
			// source are still this intent's snapshots. Restore preferences on
			// a rejected source write; neither the draft nor engine is advanced.
			if err := writeProfileSource(a.store, draft.ID, []byte(*source), original); err != nil {
				a.err = err.Error()
				if rollbackErr := a.store.Save(a.cfg); rollbackErr != nil {
					a.err += "; " + rollbackErr.Error()
				}
				return
			}
			delete(a.profileGroupCounts, draft.ID)
		}
		a.cfg = next
		a.apply()
		if a.editorOwner() != owner || a.editProfile != draft.ID || nav != nil && nav.Revision != revision {
			return
		}
		if nav != nil {
			nav.Initial = draft
			if source != nil {
				nav.SourceOriginal = source
				if nav.Source == source {
					nav.Source = nil
				}
			}
			if profileDraftEqual(a.profileDraft, draft) && nav.Source == nil {
				a.popProfilePickerPage()
			}
		} else if profileDraftEqual(a.profileDraft, draft) {
			a.back()
		}
	}
	if source == nil {
		done()
		return
	}
	text, e := *source, a.engine
	nav.Busy = true
	a.work(func() (func(), error) {
		if _, err := store.ParseYAML([]byte(text)); err != nil {
			return nil, err
		}
		if e != nil {
			if err := e.Validate([]byte(text)); err != nil {
				return nil, err
			}
		}
		return done, nil
	}, func() {
		if nav.Revision == revision {
			nav.Busy = false
		}
	})
}

func (a *App) readProfileCode(id, purpose string) {
	title, read := a.profileDraft.Name, a.profileSourceReader(id)
	a.loadProfileEditor(id, func() (any, error) { return read() }, func(value any) {
		text := value.(string)
		a.rememberProfileSource(id, text)
		a.openCode(title, purpose, text)
	})
}

func (a *App) loadProfileEditor(id string, load func() (any, error), open func(any)) {
	serial, owner := a.beginEditorLoad(), a.editorOwner()
	nav := a.profilePickerNavigation()
	if nav != nil {
		nav.Busy = true
	}
	a.work(func() (func(), error) {
		value, err := load()
		return func() {
			if a.editorLoadSerial != serial || a.editorOwner() != owner || a.editProfile != id {
				return
			}
			if nav != nil {
				nav.Busy = false
			}
			if err != nil {
				a.err = err.Error()
				return
			}
			open(value)
		}, nil
	})
}

func (a *App) editProfileSection(id, section string) {
	read := a.profileSourceReader(id)
	type sectionSnapshot struct {
		Source string
		Value  any
	}
	a.loadProfileEditor(id, func() (any, error) {
		text, err := read()
		if err != nil {
			return nil, err
		}
		m, err := store.ParseYAML([]byte(text))
		if err != nil {
			return nil, err
		}
		value := m[section]
		if value == nil {
			if strings.Contains(section, "providers") || section == "dns" || section == "ntp" || section == "sniffer" || section == "tun" {
				value = map[string]any{}
			} else {
				value = []any{}
			}
		}
		return sectionSnapshot{text, value}, nil
	}, func(value any) {
		snapshot := value.(sectionSnapshot)
		a.rememberProfileSource(id, snapshot.Source)
		a.openCollection(humanKey(section), "section:"+section, snapshot.Value)
		a.document.Profile, a.document.Source = id, &snapshot.Source
	})
}

func (a *App) stageProfileCode(code *editor.State, owner, text string, version uint64) {
	nav, e := a.profilePickerNavigation(), a.engine
	if nav == nil || nav.Busy {
		return
	}
	revision := nav.Revision
	nav.Busy = true
	a.work(func() (func(), error) {
		if _, err := store.ParseYAML([]byte(text)); err != nil {
			return nil, err
		}
		if e != nil {
			if err := e.Validate([]byte(text)); err != nil {
				return nil, err
			}
		}
		return func() {
			if a.profilePickerNavigation() != nav || nav.Revision != revision {
				return
			}
			nav.Source = &text
			a.editorSaved(code, owner, text, version)
		}, nil
	}, func() {
		if nav.Revision == revision {
			nav.Busy = false
		}
	})
}

func (a *App) previewPickerProfile(p store.Profile) {
	if a.picker.Open && a.picker.Kind == "profiles" {
		a.beginProfilePickerNavigation()
	}
	a.openCode(p.Name, "preview", "")
	a.code.ReadOnly, a.code.SoftWrap = true, true
	a.code.RenderFindBar = a.codeFindBar
	a.editProfile = p.ID
	cfg, s := store.Clone(a.cfg), a.store
	cfg.Active = p.ID
	code := a.code
	a.loadProfileEditor(p.ID, func() (any, error) {
		raw, err := effectiveConfig(s, cfg)
		if err != nil {
			return nil, err
		}
		return store.YAML(raw)
	}, func(value any) {
		code.Buffer.Set(value.(string))
		code.Buffer.MarkSaved()
		a.codeOriginal = value.(string)
	})
}
func (a *App) saveCode() {
	if a.code == nil {
		return
	}
	text := a.code.Buffer.String()
	code, tab, version := a.code, a.editorOwner(), a.code.Buffer.Version()
	purpose := a.codePurpose
	if a.code.ReadOnly {
		return
	}
	if purpose == "profile-staged" {
		a.stageProfileCode(code, tab, text, version)
		return
	}
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
	id, baseline := a.editProfile, a.codeOriginal
	switch purpose {
	case "document":
		var value any
		if err := yaml.Unmarshal([]byte(text), &value); err != nil {
			a.err = err.Error()
			return
		}
		if a.document == nil || a.document.Profile != "" && a.document.Profile != id {
			a.err = a.t("editorContextChanged")
			return
		}
		a.document.Value = value
		a.editorSaved(code, tab, text, version)
		return
	case "profile-override":
		if strings.TrimSpace(text) != "" {
			if _, err := store.ParseYAML([]byte(text)); err != nil {
				a.err = err.Error()
				return
			}
		}
		a.profileDraft.Override = text
		if !a.persistProfileDraft() {
			return
		}
		a.editorSaved(code, tab, text, version)
		return
	case "profile-script":
		a.profileDraft.Script = text
		if !a.persistProfileDraft() {
			return
		}
		a.editorSaved(code, tab, text, version)
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
		var expected *string
		if purpose == "profile" || purpose == "raw" {
			if _, err := os.Stat(s.ProfilePath(id)); err == nil {
				expected = &baseline
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if strings.HasPrefix(purpose, "section:") {
			section := strings.TrimPrefix(purpose, "section:")
			original, err := os.ReadFile(s.ProfilePath(id))
			if err != nil {
				return nil, err
			}
			snapshot := string(original)
			expected = &snapshot
			var value any
			if err = yaml.Unmarshal(b, &value); err != nil {
				return nil, err
			}
			b, err = store.PatchSection(original, section, value)
			if err != nil {
				return nil, err
			}
		}
		if _, err := store.ParseYAML(b); err != nil {
			return nil, err
		}
		if e != nil {
			if err := e.Validate(b); err != nil {
				return nil, err
			}
		}
		if err := writeProfileSource(s, id, b, expected); err != nil {
			return nil, err
		}
		return func() { delete(a.profileGroupCounts, id); a.apply(); a.editorSaved(code, tab, text, version) }, nil
	})
}
