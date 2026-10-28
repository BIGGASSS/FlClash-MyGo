package app

import "github.com/egoist/mygo/ui/editor"

type providerEditorContext struct {
	Path     string
	Profile  string
	Engine   *Engine
	Revision uint64
}

func (p *providerEditorContext) valid(a *App) bool {
	return p != nil && p.Profile == a.cfg.Active && p.Engine == a.engine && a.engine != nil && p.Revision == a.engine.Revision()
}

func (a *App) beginEditorLoad() uint64 { a.editorLoadSerial++; return a.editorLoadSerial }

func (a *App) editorSaved(code *editor.State, tab, text string, version uint64) {
	unchanged := code.Buffer.Version() == version
	if unchanged {
		code.Buffer.MarkSaved()
	}
	if a.code == code && a.navigationParent() == tab {
		a.codeOriginal = text
		if unchanged && a.page == "editor" {
			a.back()
		}
		return
	}
	if state, ok := a.navigationState[tab]; ok && state.Code == code {
		state.CodeOriginal = text
		if unchanged && state.Page == "editor" {
			state.Page = tab
			if n := len(state.History); n > 0 {
				state.Page = state.History[n-1]
				state.History = state.History[:n-1]
			}
		}
		a.navigationState[tab] = state
	}
}
