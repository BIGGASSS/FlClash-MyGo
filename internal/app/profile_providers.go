package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type providerFileKind uint8

const (
	providerBinary providerFileKind = iota
	providerText
	providerRuleSet
)

type providerFileProbe struct {
	path    string
	updated time.Time
	kind    providerFileKind
	remote  bool
	state   *providerSyncState
}

func probeProviderFile(path string) providerFileKind {
	if path == "" {
		return providerBinary
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return providerText
	}
	if err != nil {
		return providerBinary
	}
	defer f.Close()
	head, err := io.ReadAll(io.LimitReader(f, 512))
	if err != nil {
		return providerBinary
	}
	if bytes.HasPrefix(head, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		return providerRuleSet
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return providerBinary
	}
	return providerText
}

func (a *App) providerKey(p Provider) string {
	return a.cfg.Active + "\x00" + p.Type + "\x00" + p.Name + "\x00" + p.Path
}

func (a *App) providerFile(p Provider) providerFileKind {
	key := a.providerKey(p)
	if a.providerFiles == nil {
		a.providerFiles = map[string]*providerFileProbe{}
	}
	probe := a.providerFiles[key]
	if probe == nil || probe.path != p.Path || !probe.updated.Equal(p.Updated) {
		probe = &providerFileProbe{path: p.Path, updated: p.Updated}
		a.providerFiles[key] = probe
		cfg, s := store.Clone(a.cfg), a.store
		go func() {
			kind := probeProviderFile(p.Path)
			remote := false
			if p.Type == "Rule" && strings.EqualFold(p.Vehicle, "File") {
				_, remote = s.RemoteRuleResource(cfg, p.Path)
			}
			a.update(func() {
				if a.providerFiles[key] == probe {
					probe.kind, probe.remote = kind, remote
				}
			})
		}()
	}
	return probe.kind
}

func compactNumber(n int) string {
	v := float64(n)
	for i, unit := range []string{"", "K", "M", "B", "T"} {
		if math.Abs(v) < 999.5 || i == 4 {
			precision := 0
			if i > 0 && v != 0 {
				precision = max(0, 2-int(math.Floor(math.Log10(math.Abs(v)))))
			}
			return strconv.FormatFloat(math.Round(v*math.Pow10(precision))/math.Pow10(precision), 'f', -1, 64) + unit
		}
		v /= 1000
	}
	return number(n)
}

func (a *App) profileDetailProvider(c *ui.Context, p Provider, index int) {
	kind := a.providerFile(p)
	busy := a.providerPending[a.providerKey(p)] != 0
	top, bottom := float32(0), float32(0)
	if index == 0 {
		top = 24
	}
	if index == len(a.sample.Providers)-1 {
		bottom = 24
	}
	marginBottom := float32(0)
	if bottom > 0 {
		marginBottom = 20
	}
	ui.Box(c).FillWidth().Padding(0, 16, marginBottom, 16).Children(func() {
		row := ui.ButtonBase(c).FillWidth().MinHeight(56).Padding(10, 16).Radius(top, top, bottom, bottom).Background(a.colors.card).Label(p.Name)
		if kind != providerBinary && !busy {
			row.OnClick(func() { a.openProfileProvider(p, false, kind) })
		}
		row.Children(func() {
			ui.Row(c).FillWidth().Gap(16).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, p.Name).Shrink(1).MinWidth(0).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).SingleLine()
						if info := p.Subscription; info != nil && info.Total > 0 {
							ui.Text(c, compactNumber(p.Count)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(14)).FontWeight(600).Padding(2, 8).Radius(999).Background(a.colors.block).TextColor(a.colors.muted)
							if info.Expire > 0 {
								date := time.Unix(info.Expire, 0).Local()
								label := date.Format("2006-01-02")
								width, _ := c.Size()
								available := min(max(width*.4, 360), min(width, 560)) - 64
								titleWidth, _ := c.MeasureText(0, ui.Span{Text: p.Name, Size: fontSize(c, 17)})
								countWidth, _ := c.MeasureText(0, ui.Span{Text: compactNumber(p.Count), Size: fontSize(c, 13)})
								dateWidth, _ := c.MeasureText(0, ui.Span{Text: label, Size: fontSize(c, 13), Weight: 600})
								if available-countWidth-dateWidth-48 >= min(titleWidth, available/2) {
									color := a.colors.accent
									if date.Before(time.Now()) {
										color = a.colors.danger
									}
									ui.Text(c, label).Shrink(0).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(14)).FontWeight(600).Padding(2, 8).Radius(999).Background(color.Alpha(.15)).TextColor(color).Tooltip(a.t("expireTime"))
								}
							}
						}
					})
					if info := p.Subscription; info != nil && info.Total > 0 {
						ui.Column(c).FillWidth().Gap(6).Children(func() {
							ui.Text(c, compactTraffic(info.Upload+info.Download)+" / "+compactTraffic(info.Total)).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).TextColor(a.colors.muted).SingleLine()
							ui.Box(c).FillWidth().Height(4).Radius(999).Background(a.colors.accent.Alpha(.15)).Children(func() {
								ui.Box(c).WidthPercent(min(max(float32(info.Upload+info.Download)/float32(info.Total), 0), 1) * 100).Height(4).Radius(999).Background(a.colors.accent)
							})
						})
					} else {
						key := "rulesCount"
						if p.Type == "Proxy" {
							key = "proxiesCount"
						}
						ui.Text(c, a.messageText(key, map[string]any{"count": p.Count, "label": compactNumber(p.Count)})).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).TextColor(a.colors.muted).SingleLine()
					}
				})
				if busy {
					loading(c, 24, a.colors.accent, a.t("loading"))
				} else {
					updated := ""
					if p.Updated.UnixMicro() > 0 {
						updated = a.updatedAgoCompact(p.Updated)
					}
					ui.Text(c, updated).MinWidth(16).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted).SingleLine()
				}
			})
			if bottom == 0 {
				ui.Box(c).Absolute().Bottom(0).Left(16).Right(14).Height(1).Background(a.colors.text.Alpha(.14))
			}
		})
		if !busy {
			row.ContextMenu(func(m *ui.Menu) {
				if kind == providerText && m.Item(a.t("edit")).Chosen() {
					a.openProfileProvider(p, true, kind)
				}
				if kind == providerRuleSet && m.Item(a.t("preview")).Chosen() {
					a.openProfileProvider(p, false, kind)
				}
				if m.Item(a.t("upload")).Disabled(a.safe).Chosen() {
					a.importProvider(p)
				}
				if (strings.EqualFold(p.Vehicle, "HTTP") || a.providerFiles[a.providerKey(p)].remote) && m.Item(a.t("sync")).Disabled(a.safe).Chosen() {
					a.updateDetailProvider(p)
				}
				if info := p.Subscription; info != nil && info.Total > 0 {
					if m.Item(a.t("subscriptionInfo")).Chosen() {
						a.showProviderSubscription(p)
					}
				}
			})
		}
	})
}

func (a *App) updatedAgoCompact(at time.Time) string {
	if at.Year() <= 1970 {
		return a.t("unknown")
	}
	for _, item := range []struct {
		duration time.Duration
		key      string
	}{
		{365 * 24 * time.Hour, "yearsAgoShort"}, {30 * 24 * time.Hour, "monthsAgoShort"}, {24 * time.Hour, "daysAgoShort"}, {time.Hour, "hoursAgoShort"}, {time.Minute, "minutesAgoShort"},
	} {
		if duration := time.Since(at); duration >= item.duration {
			return a.messageText(item.key, map[string]any{"count": int(duration / item.duration)})
		}
	}
	return a.t("justNow")
}

func (a *App) openProfileProvider(p Provider, edit bool, kind providerFileKind) {
	if !a.profileDetail.Open || kind == providerBinary {
		return
	}
	a.profileDetail.Serial++
	serial, active := a.profileDetail.Serial, a.cfg.Active
	a.profileDetail.Preview, a.profileDetail.Busy = true, true
	a.profileDetail.Title, a.profileDetail.Code = p.Name, nil
	a.profileDetail.Provider = &p
	e := a.engine
	nonTextMessage := a.t("nonTextProviderFile")
	var dump func() (string, error)
	if e != nil {
		e.mu.Lock()
		client := e.client
		e.mu.Unlock()
		if client != nil {
			dump = func() (string, error) {
				ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
				defer cancel()
				var text string
				err := client.Call(ctx, "dumpRuleSet", p.Path, &text)
				return text, err
			}
		}
	}
	go func() {
		var data []byte
		var err error
		if kind == providerRuleSet {
			var text string
			if dump == nil {
				err = errors.New("core is not connected")
			} else {
				text, err = dump()
			}
			data = []byte(text)
		} else {
			data, err = readProviderFile(p.Path)
			if err == nil && !utf8.Valid(data) {
				err = errors.New(nonTextMessage)
			}
		}
		a.update(func() {
			if !a.profileDetail.Open || a.profileDetail.Serial != serial || a.cfg.Active != active {
				return
			}
			a.profileDetail.Busy = false
			if err != nil {
				a.err = err.Error()
				a.backProfileDetails()
				return
			}
			a.profileDetail.Original = string(data)
			a.profileDetail.Code = &editor.State{Buffer: editor.NewBuffer(string(data)), ReadOnly: !edit || kind == providerRuleSet, SoftWrap: true, RenderFindBar: a.codeFindBar}
			a.profileDetail.Code.Buffer.SetLanguage(editor.LanguageFor("provider.yaml"))
		})
	}()
}

func (a *App) profileCodeDirty() bool {
	state := &a.profileDetail
	return state.Preview && state.Code != nil && !state.Code.ReadOnly && state.Code.Buffer.String() != state.Original
}

func (a *App) saveProfileProvider() {
	state := &a.profileDetail
	if a.safe || a.engine == nil || state.Code == nil || state.Code.ReadOnly || state.Provider == nil || state.Saving {
		return
	}
	e := a.engine
	e.mu.Lock()
	client, revision := e.client, e.revision
	e.mu.Unlock()
	if client == nil {
		return
	}
	p, data, serial, active, code := *state.Provider, []byte(state.Code.Buffer.String()), state.Serial, a.cfg.Active, state.Code
	state.Saving = true
	go func() {
		prepared, prepareErr := store.PrepareResource(p.Path, data)
		if prepareErr != nil {
			a.update(func() {
				if state.Serial == serial && state.Open && a.cfg.Active == active && a.engine == e && state.Code == code {
					state.Saving = false
					if providerClientCurrent(e, client, revision) && a.providerPathCurrent(p.Name, p.Path) {
						a.err = prepareErr.Error()
					}
				}
			})
			return
		}
		ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
		defer cancel()
		var message string
		err := client.Call(ctx, "sideLoadExternalProvider", map[string]string{"providerName": p.Name, "provider-path": p.Path, "data-base64": base64.StdEncoding.EncodeToString(data)}, &message)
		if err == nil && message != "" {
			err = errors.New(message)
		}
		if err == nil && e.Revision() != revision {
			err = errors.New("provider save superseded by a config change")
		}
		a.update(func() {
			defer prepared.Discard()
			if state.Serial != serial || !state.Open || a.cfg.Active != active || a.engine != e || state.Code != code {
				return
			}
			state.Saving = false
			if !providerClientCurrent(e, client, revision) || !a.providerPathCurrent(p.Name, p.Path) {
				return
			}
			if err == nil {
				err = prepared.Commit()
			}
			if err != nil {
				a.err = err.Error()
				return
			}
			state.Original = string(data)
			if code.Buffer.String() != string(data) {
				state.PendingClose = false
				return
			}
			if state.PendingClose {
				a.closeProfileDetails()
			} else {
				a.backProfileDetails()
			}
		})
	}()
}

func (a *App) profileProviderSaveDialog(c *ui.Context) {
	wasOpen := a.profileDetail.SavePrompt
	a.dialogBase(c.Key("profile-provider-save"), &a.profileDetail.SavePrompt, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, .32))
		panel.Width(440).MaxWidthPercent(90).Padding(24).Gap(20).Radius(28).Background(a.colors.card)
		ui.Text(c, a.t("saveChanges")).FontSize(fontSize(c, 22)).Bold()
		ui.Text(c, a.profileDetail.Title)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			ui.Button(c, a.t("cancel")).OnClick(func() { a.profileDetail.SavePrompt = false; a.profileDetail.PendingClose = false })
			ui.Button(c, a.t("no")).OnClick(func() {
				a.profileDetail.SavePrompt = false
				a.profileDetail.Code.ReadOnly = true
				if a.profileDetail.PendingClose {
					a.closeProfileDetails()
				} else {
					a.backProfileDetails()
				}
			})
			ui.PrimaryButton(c, a.t("save")).Disabled(a.safe || a.profileDetail.Saving).OnClick(func() { a.profileDetail.SavePrompt = false; a.saveProfileProvider() })
		})
	})
	if wasOpen && !a.profileDetail.SavePrompt && !a.profileDetail.Saving {
		a.profileDetail.PendingClose = false
	}
}

func (a *App) detailProvidersUpdating() bool {
	for _, p := range a.sample.Providers {
		if a.providerPending[a.providerKey(p)] != 0 {
			return true
		}
	}
	return false
}

func (a *App) updateDetailProviders() {
	a.updateProviderBatch(a.sample.Providers)
}

func (a *App) updateDetailProvider(p Provider) {
	a.updateProviderBatch([]Provider{p})
}

func (a *App) showProviderSubscription(p Provider) {
	if p.Subscription == nil {
		return
	}
	state := a.providerState()
	copy := *p.Subscription
	state.subscription, state.infoOpen = &copy, true
}
