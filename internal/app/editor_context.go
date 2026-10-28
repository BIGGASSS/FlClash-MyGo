package app

import (
	"bytes"
	"errors"
	"os"
	"reflect"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui/editor"
	"go.yaml.in/yaml/v3"
)

type providerEditorContext struct {
	Path     string
	Profile  string
	Engine   *Engine
	Revision uint64
}

func (p *providerEditorContext) valid(a *App) bool {
	return p != nil && p.Profile == a.cfg.Active && p.Engine == a.engine && a.engine != nil && p.Revision == a.engine.Revision()
}

func popSavedEditor(state *navigationPageState, tab string) {
	state.Page = tab
	if n := len(state.History); n > 0 {
		state.Page = state.History[n-1]
		state.History = state.History[:n-1]
	}
}

// Completion may mark its own document saved, but may navigate only the route
// which initiated the request. A dismissed/replaced sheet has a different owner.
func (a *App) documentSaved(d *documentEditor, owner, text string) {
	current, _ := yaml.Marshal(d.Value)
	d.Original = text
	if string(current) != text {
		return
	}
	if nav := a.profilePickerNavigation(); nav != nil {
		if nav.Owner == owner && a.document == d {
			if nav.Page == "collection" {
				d.Path = nil
				a.popProfilePickerPage()
			}
			return
		}
		if a.navigationParent() == owner && nav.Background.Document == d {
			if nav.Background.Page == "collection" {
				d.Path = nil
				popSavedEditor(&nav.Background, owner)
			}
			a.page, a.history = nav.Background.Page, nav.Background.History
			return
		}
	}
	if a.document == d && a.editorOwner() == owner {
		if a.page == "collection" {
			d.Path = nil
			a.back()
		}
		return
	}
	if state, ok := a.navigationState[owner]; ok && state.Document == d {
		if state.Page == "collection" {
			d.Path = nil
			popSavedEditor(&state, owner)
		}
		a.navigationState[owner] = state
	}
}

func (a *App) beginEditorLoad() uint64 { a.editorLoadSerial++; return a.editorLoadSerial }

func profileDraftEqual(a, b store.Profile) bool {
	return reflect.DeepEqual(mergeProfileDraft(store.Profile{}, a), mergeProfileDraft(store.Profile{}, b))
}

// A nested metadata editor owns a transaction. Custom-profile sections have no
// confirmation page and save directly; sections under Edit are staged instead.
func (a *App) pickerStagesProfile() bool {
	if nav := a.profilePickerNavigation(); nav != nil {
		if nav.Page == "profileEdit" || nav.Page == "profileExtend" {
			return true
		}
		for _, parent := range nav.Stack {
			if parent.Page == "profileEdit" || parent.Page == "profileExtend" {
				return true
			}
		}
	}
	return false
}

// Capture staged text on the UI thread. Workers must not dereference a mutable
// navigation state, including when another section is opened before they run.
func (a *App) profileSourceReader(id string) func() (string, error) {
	if nav := a.profilePickerNavigation(); nav != nil && nav.Initial.ID == id && nav.Source != nil {
		text := *nav.Source
		return func() (string, error) { return text, nil }
	}
	path := a.store.ProfilePath(id)
	return func() (string, error) { b, err := os.ReadFile(path); return string(b), err }
}

func (a *App) rememberProfileSource(id, text string) {
	if nav := a.profilePickerNavigation(); nav != nil && nav.Initial.ID == id && nav.SourceOriginal == nil && nav.Source == nil {
		nav.SourceOriginal = &text
	}
}

// Recheck after validation, which can take long enough for a subscription
// refresh or another editor to replace the file. Refuse to overwrite that work.
func writeProfileSource(s *store.Store, id string, data []byte, original *string) error {
	if original != nil {
		current, err := os.ReadFile(s.ProfilePath(id))
		if err != nil {
			return err
		}
		if !bytes.Equal(current, []byte(*original)) {
			return errors.New("Profile source changed; reopen the editor before saving")
		}
	}
	return store.AtomicWrite(s.ProfilePath(id), data)
}

func (a *App) editorSaved(code *editor.State, tab, text string, version uint64) {
	unchanged := code.Buffer.Version() == version
	if unchanged {
		code.Buffer.MarkSaved()
	}
	if nav := a.profilePickerNavigation(); nav != nil {
		if nav.Owner == tab && a.code == code {
			a.codeOriginal = text
			if unchanged && nav.Page == "editor" {
				a.popProfilePickerPage()
			}
			return
		}
		if a.navigationParent() == tab && nav.Background.Code == code {
			state := nav.Background
			state.CodeOriginal = text
			if unchanged && state.Page == "editor" {
				popSavedEditor(&state, tab)
			}
			nav.Background = state
			a.page, a.history = state.Page, state.History
			return
		}
	}
	if a.code == code && a.editorOwner() == tab {
		a.codeOriginal = text
		if unchanged && a.page == "editor" {
			a.back()
		}
		return
	}
	if state, ok := a.navigationState[tab]; ok && state.Code == code {
		state.CodeOriginal = text
		if unchanged && state.Page == "editor" {
			popSavedEditor(&state, tab)
		}
		a.navigationState[tab] = state
	}
}
