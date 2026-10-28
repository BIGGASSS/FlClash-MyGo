package app

import (
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

// Mirrors RuleTarget.ruleTargetNames and profiles/extend/standard.dart in the
// pinned source. Custom profiles instead select each rule's target; setupState
// explicitly ignores their profile MatchTarget. Do not offer an ineffective
// profile-level setting in profileCustom.
var profileMatchTargetBasics = []string{"DIRECT", "REJECT", "REJECT-DROP", "PASS", "PASS-RULE", "COMPATIBLE", "GLOBAL"}

type profileMatchTargetOption struct {
	Name string
	Kind string
}

type profileMatchTargetSnapshot struct {
	Groups  []profileMatchTargetOption
	Proxies []profileMatchTargetOption
}

// Candidates come from source outbounds, not the active core or effective
// config (which contains resolved MATCH placeholders and application overlays).
// Like the source's maps, duplicate names keep their first position/last type.
func readProfileMatchTargets(text string) (profileMatchTargetSnapshot, error) {
	raw, err := store.ParseYAML([]byte(text))
	if err != nil {
		return profileMatchTargetSnapshot{}, err
	}
	options := func(value any) []profileMatchTargetOption {
		var out []profileMatchTargetOption
		indices := map[string]int{}
		list, _ := value.([]any)
		for _, item := range list {
			m, _ := item.(map[string]any)
			name, _ := m["name"].(string)
			kind, _ := m["type"].(string)
			if name == "" {
				continue
			}
			if i, ok := indices[name]; ok {
				out[i].Kind = kind
			} else {
				indices[name] = len(out)
				out = append(out, profileMatchTargetOption{name, kind})
			}
		}
		return out
	}
	return profileMatchTargetSnapshot{Groups: options(raw["proxy-groups"]), Proxies: options(raw["proxies"])}, nil
}

func (s profileMatchTargetSnapshot) hasGroup(name string) bool {
	for _, option := range s.Groups {
		if option.Name == name {
			return true
		}
	}
	return false
}

func (s profileMatchTargetSnapshot) contains(name string) bool {
	if name == "" || s.hasGroup(name) {
		return true
	}
	for _, builtin := range profileMatchTargetBasics {
		if name == builtin {
			return true
		}
	}
	for _, option := range s.Proxies {
		if name == option.Name {
			return true
		}
	}
	return false
}

// Only data lives in ui.Local, never an Element or Context. The intent key also
// changes when returning from a section editor or refreshing a subscription.
// A late read/error cannot update a replacement page, tab, profile or sheet.
type profileMatchTargetKey struct {
	Owner, Profile, Path string
	Serial, Revision     uint64
	Source               *string
	Updated              time.Time
}

type profileMatchTargetState struct {
	Key      profileMatchTargetKey
	Request  uint64
	Loading  bool
	Error    string
	Snapshot profileMatchTargetSnapshot
}

func (a *App) profileMatchTargetKey() profileMatchTargetKey {
	key := profileMatchTargetKey{Owner: a.editorOwner(), Profile: a.editProfile, Path: a.store.ProfilePath(a.editProfile), Serial: a.editorLoadSerial}
	if nav := a.profilePickerNavigation(); nav != nil {
		key.Revision, key.Source = nav.Revision, nav.Source
	}
	for _, p := range a.cfg.Profiles {
		if p.ID == key.Profile {
			key.Updated = p.Updated
			break
		}
	}
	return key
}

func (k profileMatchTargetKey) current(a *App) bool {
	page := a.page
	if nav := a.profilePickerNavigation(); nav != nil {
		page = nav.Page
	}
	return page == "profileExtend" && a.profileDraft.ID == k.Profile && a.profileDraft.Type != "custom" && k == a.profileMatchTargetKey()
}

func (a *App) loadProfileMatchTargets(state *profileMatchTargetState, key profileMatchTargetKey, read func() (string, error)) {
	state.Key, state.Loading, state.Error = key, true, ""
	state.Snapshot = profileMatchTargetSnapshot{}
	state.Request++
	request := state.Request
	a.work(func() (func(), error) {
		text, err := read()
		var snapshot profileMatchTargetSnapshot
		if err == nil {
			snapshot, err = readProfileMatchTargets(text)
		}
		return func() {
			if state.Request != request || state.Key != key || !key.current(a) {
				return
			}
			state.Loading = false
			state.Snapshot = snapshot
			if err != nil {
				state.Error = err.Error()
			}
		}, nil
	})
}

func (a *App) profileMatchTargetRow(c *ui.Context) {
	button := ui.ButtonBase(c.Key("profile-match-target")).FillWidth().Label(a.t("matchTarget"))
	state := ui.Local(button, "candidates", func() profileMatchTargetState { return profileMatchTargetState{} })
	key := a.profileMatchTargetKey()
	if state.Request == 0 || state.Key != key {
		a.loadProfileMatchTargets(state, key, a.profileSourceReader(key.Profile))
	}
	button.Disabled(state.Loading).Children(func() {
		a.row(c, "table", a.t("matchTarget"), func() {
			label, color := a.profileDraft.MatchTarget, a.colors.muted
			if label == "" {
				label = a.t("followProfile")
			} else if !state.Loading && state.Error == "" && !state.Snapshot.contains(label) {
				color = a.colors.danger
			}
			ui.Text(c, label).SingleLine().MaxWidthPercent(45).TextColor(color).Tooltip(label)
			if state.Loading {
				loading(c, 18, a.colors.accent, a.t("loading"))
			} else {
				glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted)
			}
		})
	}).Menu(func(m *ui.Menu) { a.profileMatchTargetMenu(m, state) })
	if state.Error != "" {
		ui.Text(c, state.Error).FontSize(fontSize(c, 12)).TextColor(a.colors.danger).Selectable()
	}
}

func profileMatchTargetOptionLabel(option profileMatchTargetOption) string {
	if option.Kind == "" {
		return option.Name
	}
	return option.Name + " · " + option.Kind
}

func (a *App) profileMatchTargetMenu(m *ui.Menu, state *profileMatchTargetState) {
	if state.Loading || !state.Key.current(a) {
		return
	}
	selected := a.profileDraft.MatchTarget
	item := func(menu *ui.Menu, label, value string) {
		if menu.Item(label).Checked(selected == value).Chosen() {
			// Only this draft changes. Follow is stored as empty, never as the
			// current subscription MATCH. EffectiveTransform resolves it anew
			// against the last nonempty transformed MATCH at each runtime build.
			a.profileDraft.MatchTarget = value
		}
	}
	item(m, a.t("followProfile"), "")
	if selected != "" && !state.Snapshot.contains(selected) {
		m.Item(a.t("unknown") + ": " + selected).Checked(true).Disabled(true)
	}
	m.Separator()
	m.Submenu(a.t("basicStrategy"), func(sub *ui.Menu) {
		for _, name := range profileMatchTargetBasics {
			if !state.Snapshot.hasGroup(name) {
				item(sub, name, name)
			}
		}
	})
	for _, section := range []struct {
		Label   string
		Options []profileMatchTargetOption
	}{{a.t("ruleTarget"), state.Snapshot.Groups}, {a.t("proxies"), state.Snapshot.Proxies}} {
		if len(section.Options) == 0 {
			continue
		}
		m.Submenu(section.Label, func(sub *ui.Menu) {
			for _, option := range section.Options {
				item(sub, profileMatchTargetOptionLabel(option), option.Name)
			}
		})
	}
	m.Separator()
	label := a.t("update")
	if state.Error != "" {
		label = a.t("retry")
	}
	if m.Item(label).Chosen() {
		a.loadProfileMatchTargets(state, state.Key, a.profileSourceReader(state.Key.Profile))
	}
}
