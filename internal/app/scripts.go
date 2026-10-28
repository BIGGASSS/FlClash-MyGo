package app

import (
	"strings"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func (a *App) scriptsView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(14).Children(func() {
			if len(a.cfg.Scripts) == 0 {
				a.empty(c, "edit", a.t("script"), a.t("noData"), nil)
			}
			for _, script := range a.cfg.Scripts {
				a.card(c).Padding(16).Children(func() {
					ui.Row(c).Gap(14).Children(func() {
						glyph(c, "edit").FontSize(24).TextColor(a.colors.muted)
						name := script.Name
						if ui.TextInput(c.Key(script.ID), &name).Grow(1).Label(a.t("name")).Changed() {
							for i := range a.cfg.Scripts {
								if a.cfg.Scripts[i].ID == script.ID {
									a.cfg.Scripts[i].Name = name
									a.save()
									break
								}
							}
						}
						a.button(c, "edit", a.t("edit")+script.Name, func() { a.openCode(script.Name, "library-script:"+script.ID, script.Source) })
						a.button(c, "trash", a.t("delete")+script.Name, func() {
							a.ask(a.t("delete")+" "+script.Name, a.t("confirm"), func() {
								for i := range a.cfg.Scripts {
									if a.cfg.Scripts[i].ID == script.ID {
										a.cfg.Scripts = append(a.cfg.Scripts[:i], a.cfg.Scripts[i+1:]...)
										break
									}
								}
								for i := range a.cfg.Profiles {
									if a.cfg.Profiles[i].ScriptID == script.ID {
										a.cfg.Profiles[i].ScriptID = ""
									}
								}
								a.apply()
							})
						})
					})
				})
			}
			ui.Button(c, "+ "+a.t("add")).FillWidth().Radius(999).Padding(12).OnClick(a.addScript)
		})
	})
}
func (a *App) addScript() {
	script := store.Script{ID: store.NewID(), Name: a.t("script"), Source: "const main = (config) => {\n  return config;\n};\n"}
	a.cfg.Scripts = append(a.cfg.Scripts, script)
	a.save()
	a.openCode(script.Name, "library-script:"+script.ID, script.Source)
}
func (a *App) scriptSelector(c *ui.Context) {
	current := a.t("none")
	var options = []string{a.t("none")}
	byName := map[string]string{}
	for _, script := range a.cfg.Scripts {
		label := script.Name
		for byName[label] != "" {
			label += " "
		}
		options = append(options, label)
		byName[label] = script.ID
		if a.profileDraft.ScriptID == script.ID {
			current = label
		}
	}
	if ui.Select(c, &current, options).Label(a.t("script")).Width(260).Changed() {
		a.profileDraft.ScriptID = byName[current]
	}
}
func (a *App) saveLibraryScript(purpose, text string) bool {
	if !strings.HasPrefix(purpose, "library-script:") {
		return false
	}
	id := strings.TrimPrefix(purpose, "library-script:")
	for i := range a.cfg.Scripts {
		if a.cfg.Scripts[i].ID == id {
			a.cfg.Scripts[i].Source = text
			a.apply()
			a.back()
			return true
		}
	}
	a.err = "Script no longer exists"
	return true
}
