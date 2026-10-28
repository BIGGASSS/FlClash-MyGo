package app

import (
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type profileDetailState struct {
	Open, Preview, Busy bool
	Serial              uint64
	Title               string
	Code                *editor.State
	List                ui.ListState
	Provider            *Provider
	Original            string
	Saving, SavePrompt  bool
	PendingClose        bool
}

func (a *App) openProfileDetails() {
	if p := a.store.ActiveProfile(a.cfg); p != nil && p.Type == "custom" {
		a.openCustomProfileSheet(*p)
		return
	}
	if a.store.ActiveProfile(a.cfg) == nil {
		a.openProfilePicker()
		return
	}
	a.resetSheet("profile-details")
	a.profileDetail.Serial++
	a.profileDetail.Open, a.profileDetail.Preview, a.profileDetail.Busy = true, false, false
	a.profileDetail.Code = nil
	a.profileDetail.Provider = nil
	a.profileDetail.List = ui.ListState{}
	a.profileDetail.SavePrompt, a.profileDetail.PendingClose, a.profileDetail.Saving = false, false, false
}

func (a *App) closeProfileDetails() {
	if a.profileDetail.Saving {
		return
	}
	if a.profileCodeDirty() {
		a.profileDetail.SavePrompt, a.profileDetail.PendingClose = true, true
		return
	}
	if a.deferSheetClose("profile-details", a.closeProfileDetails) {
		return
	}
	a.profileDetail.Serial++
	a.profileDetail.Open, a.profileDetail.Preview, a.profileDetail.Busy = false, false, false
	a.profileDetail.Code = nil
	a.profileDetail.Provider = nil
	a.profileDetail.SavePrompt = false
}

func (a *App) backProfileDetails() {
	if a.profileDetail.Saving {
		return
	}
	if a.profileDetail.Preview {
		if a.profileCodeDirty() {
			a.profileDetail.SavePrompt, a.profileDetail.PendingClose = true, false
			return
		}
		a.profileDetail.Serial++
		a.profileDetail.Preview, a.profileDetail.Busy = false, false
		a.profileDetail.Code = nil
		a.profileDetail.Provider = nil
		a.profileDetail.PendingClose = false
		return
	}
	a.closeProfileDetails()
}

func (a *App) previewProfileRuntime() {
	profile := a.store.ActiveProfile(a.cfg)
	if profile == nil || !a.profileDetail.Open {
		return
	}
	a.profileDetail.Serial++
	serial := a.profileDetail.Serial
	a.profileDetail.Preview, a.profileDetail.Busy = true, true
	a.profileDetail.Title = profile.Name
	a.profileDetail.Code = nil
	a.profileDetail.Provider = nil
	cfg, s := store.Clone(a.cfg), a.store
	go func() {
		raw, err := effectiveConfig(s, cfg)
		var text string
		if err == nil {
			text, err = store.YAML(raw)
		}
		a.update(func() {
			if !a.profileDetail.Open || !a.profileDetail.Preview || a.profileDetail.Serial != serial {
				return
			}
			a.profileDetail.Busy = false
			if err != nil {
				a.err = err.Error()
			}
			a.profileDetail.Code = &editor.State{Buffer: editor.NewBuffer(text), ReadOnly: true, SoftWrap: true}
			a.profileDetail.Code.RenderFindBar = a.codeFindBar
			a.profileDetail.Code.Buffer.SetLanguage(editor.LanguageFor("config.yaml"))
		})
	}()
}

func (a *App) codeFindBar(c *ui.Context, bar ui.Element, find *editor.FindBar) {
	bar.Padding(8).Gap(8).Children(func() {
		find.QueryInput(ui.SearchField(c, find.Query()).Grow(1).MinWidth(0).AutoFocus().Label(a.t("search")))
		a.sheetButton(c, "up", a.t("previous"), find.Previous).Size(32, 32)
		a.sheetButton(c, "down", a.t("next"), find.Next).Size(32, 32)
		a.sheetButton(c, "close", a.t("close"), find.Close).Size(32, 32)
	})
}

func (a *App) profileDetailSheet(c *ui.Context) {
	if !a.profileDetail.Open {
		return
	}
	profile := a.store.ActiveProfile(a.cfg)
	if profile == nil {
		a.closeProfileDetails()
		return
	}
	a.navigationSheet(c, "profile-details", &a.profileDetail.Open, a.closeProfileDetails, a.backProfileDetails, func() {
		if a.profileDetail.Preview {
			a.sheetHeader(c, a.profileDetail.Title, "back", a.t("back"), a.backProfileDetails, func() {
				a.sheetButton(c, "search", a.t("search"), func() {
					if a.profileDetail.Code != nil {
						a.profileDetail.Code.Find("")
					}
				})
				if code := a.profileDetail.Code; code != nil && !code.ReadOnly {
					a.sheetButton(c, "check", a.t("save"), a.saveProfileProvider).Disabled(a.profileDetail.Saving)
				}
			})
			if a.profileDetail.Busy {
				ui.Box(c).Fill().Center().Children(func() { loading(c, 48, a.colors.accent, a.t("loading")) })
			} else if a.profileDetail.Code != nil {
				ui.Box(c).Fill().Padding(0, 16, 16, 16).Children(func() { editor.View(c, a.profileDetail.Code).Fill().Radius(20) })
				if c.Shortcut(ui.Cmd, ui.KeyS) {
					a.saveProfileProvider()
				}
			}
			return
		}
		a.sheetHeader(c, profile.Name, "close", a.t("close"), a.closeProfileDetails, func() {
			ui.Row(c).Height(44).Shrink(0).Radius(999).Background(a.colors.card).Gap(0).Children(func() {
				if profile.URL != "" {
					if a.profileUpdating[profile.ID] {
						a.sheetLoadingButton(c, a.t("sync"))
					} else {
						ui.ButtonBase(c).Label(a.t("sync")).Tooltip(a.t("sync")).Size(44, 44).Center().OnClick(func() { a.updateProfile(*profile) }).Children(func() { glyphVariant(c, "sync", true).FontSize(24) })
					}
				}
				ui.ButtonBase(c).Label(a.t("runtimeConfig")).Tooltip(a.t("runtimeConfig")).Size(44, 44).Center().OnClick(a.previewProfileRuntime).Children(func() { glyphVariant(c, "eye", true).FontSize(24) })
			})
		})
		a.profileDetail.List.Key = func(i int) any {
			if i < 2 {
				return i
			}
			provider := a.sample.Providers[i-2]
			return provider.Type + "\x00" + provider.Name
		}
		n := 1
		if len(a.sample.Providers) > 0 {
			n += 1 + len(a.sample.Providers)
		}
		ui.List(c, &a.profileDetail.List, n, func(i int) {
			if i == 1 {
				ui.Box(c).FillWidth().Padding(0, 16).Children(func() {
					a.profileSectionHeader(c, a.t("providers"), func() {
						a.sheetButton(c, "sync", a.t("update"), a.updateDetailProviders).Size(36, 36).Disabled(a.detailProvidersUpdating())
					})
				})
				return
			}
			if i > 1 {
				a.profileDetailProvider(c, a.sample.Providers[i-2], i-2)
				return
			}
			bottom := float32(20)
			if len(a.sample.Providers) > 0 {
				bottom = 0
			}
			top := float32(12)
			if a.compactSheet {
				top = 0
			}
			ui.Column(c).FillWidth().Padding(top, 16, bottom, 16).Gap(0).Children(func() {
				counts := []string{"0", "0", "0", "0"}
				if value := a.sample.Counts; value != nil {
					proxies := value.Proxies
					for _, provider := range a.sample.Providers {
						if provider.Type == "Proxy" {
							proxies += provider.Count
						}
					}
					counts = []string{number(value.Groups), number(proxies), number(value.Rules), number(len(a.sample.Providers))}
				}
				ui.Grid(c).FillWidth().Columns(2).Gap(8).Margin(8, 0, 0, 0).Children(func() {
					for i, key := range []string{"proxyGroup", "proxies", "rules", "providers"} {
						ui.Column(c).Padding(14, 16).Gap(2).Radius(20).Background(a.colors.card).Children(func() {
							ui.Text(c, counts[i]).FontSize(fontSize(c, 22)).FixedLineHeight(a.textHeight(28)).FontWeight(600)
							ui.Text(c, a.t(key)).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted).SingleLine()
						})
					}
				})
				if profile.Total > 0 {
					a.profileInfoSection(c, a.t("subscriptionInfo"), func() {
						ui.Column(c).FillWidth().MinHeight(56).Padding(10, 16).Gap(4).Justify(ui.Center).Children(func() {
							ui.Row(c).FillWidth().Gap(12).Children(func() {
								ui.Text(c, compactTraffic(profile.Upload+profile.Download)+" / "+compactTraffic(profile.Total)).Grow(1).MinWidth(0).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).FontWeight(600).TextColor(a.colors.muted).SingleLine()
								date := a.t("infiniteTime")
								if profile.Expire != 0 {
									date = time.Unix(profile.Expire, 0).Local().Format("2006-01-02")
								}
								ui.Text(c, date).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).SingleLine()
							})
							share := min(max(float32(profile.Upload+profile.Download)/float32(profile.Total), 0), 1)
							ui.Box(c).FillWidth().Height(4).Radius(999).Background(a.colors.accent.Alpha(0.15)).Children(func() { ui.Box(c).WidthPercent(share * 100).Height(4).Radius(999).Background(a.colors.accent) })
						})
					})
				}
				a.profileInfoSection(c, a.t("profile"), func() {
					updated := ""
					if !profile.Updated.IsZero() {
						updated = a.updatedAgo(profile.Updated)
					}
					a.profileInfoRow(c, a.t("lastUpdated"), updated)
					if profile.Type != "custom" {
						ui.Box(c).Absolute().Top(55).Left(16).Right(14).Height(1).Background(a.colors.text.Alpha(0.14))
						label := a.t("standard")
						if profile.Script != "" || profile.ScriptID != "" || profile.Type == "script" {
							label = a.t("script")
						}
						a.profileInfoRow(c, a.t("extendMode"), label)
					}
				})
			})
		}).Fill()
	})
	a.profileProviderSaveDialog(c)
}

func (a *App) sheetLoadingButton(c *ui.Context, label string) {
	ui.Box(c).Size(44, 44).Children(func() {
		ui.ButtonBase(c).Label(label).Fill().Disabled(true)
		ui.Box(c).Absolute().Left(0).Top(0).Size(44, 44).Center().PassThrough().Children(func() { loading(c, 24, a.colors.accent, a.t("loading")) })
	})
}

func compactTraffic(value int64) string {
	parts := strings.SplitN(bytesText(value), " ", 2)
	return strings.TrimSuffix(parts[0], ".0") + parts[1]
}

func (a *App) profileInfoSection(c *ui.Context, title string, body func()) {
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		a.profileSectionHeader(c, title, nil)
		ui.Column(c).FillWidth().Gap(0).Radius(24).Background(a.colors.card).Children(body)
	})
}

func (a *App) profileSectionHeader(c *ui.Context, title string, actions func()) {
	adaptive := min(1+(a.cfg.Scale-1)/2, 1)
	overhang := (36 - a.textHeight(22)) / 2
	ui.Box(c).FillWidth().Padding(max(0, 24*adaptive-overhang), 16*adaptive, max(6, 8*adaptive-overhang), 16*adaptive).Children(func() {
		ui.Row(c).FillWidth().MinHeight(36).Children(func() {
			ui.Text(c, title).Grow(1).MinWidth(0).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).TextColor(a.colors.muted).MaxLines(2)
			if actions != nil {
				actions()
			}
		})
	})
}

func (a *App) profileInfoRow(c *ui.Context, label, value string) {
	ui.Row(c).FillWidth().MinHeight(56).Padding(10, 16).Gap(16).Children(func() {
		ui.Text(c, label).Grow(1).MinWidth(0).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).SingleLine()
		ui.Text(c, value).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).TextColor(a.colors.muted).SingleLine()
	})
}
