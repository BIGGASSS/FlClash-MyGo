package app

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/egoist/mygo/ui"
)

// App RSS remains observable when the owned core is stopped. Headless fixtures
// retain their supplied samples and never start background work.
func (a *App) watchMemory(c *ui.Context) {
	if a.window == nil || a.safe || a.page != "dashboard" && !a.memoryOpen {
		return
	}
	if !a.memoryPolling && !c.Now().Before(a.memoryPollAt) {
		a.memoryPolling = true
		a.memoryPollAt = c.Now().Add(2 * time.Second)
		go func() {
			rss := desktop.ResidentMemory()
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			a.update(func() { a.sample.AppRSS, a.sample.AppHeap = rss, stats.HeapInuse; a.memoryPolling = false })
		}()
	}
	if a.memoryPolling {
		c.After(2 * time.Second)
	} else {
		c.After(max(a.memoryPollAt.Sub(c.Now()), time.Millisecond))
	}
}

func (a *App) memoryTotals() (uint64, uint64) {
	if !a.coreConnected() {
		return a.sample.AppRSS, 0
	}
	return a.sample.AppRSS, a.sample.Memory.RSS
}

func (a *App) memoryFigure(c *ui.Context, bytes uint64, numberSize, unitSize float32) {
	parts := strings.SplitN(bytesText(int64(bytes)), " ", 2)
	ui.Row(c).AlignItems(ui.Baseline).Gap(5).Children(func() {
		ui.Text(c, parts[0]).FontSize(fontSize(c, numberSize)).FixedLineHeight(a.textHeight(numberSize + 6)).FontWeight(600)
		ui.Text(c, parts[1]).FontSize(fontSize(c, unitSize)).FontWeight(600).TextColor(a.colors.muted)
	})
}

func (a *App) memoryBar(c *ui.Context, app, core uint64) {
	ui.Row(c).Height(8).FillWidth().Radius(999).Clip().Gap(2).Children(func() {
		if app+core == 0 {
			ui.Box(c).Fill().Background(a.colors.inset)
			return
		}
		for i, n := range []uint64{app, core} {
			if n == 0 {
				continue
			}
			color := a.colors.accent
			if i == 1 {
				color = a.colors.card.Mix(a.colors.accent, 0.45)
			}
			percent := float32((n*100 + (app+core)/2) / (app + core))
			ui.Box(c).Grow(max(percent, 0.01)).MinWidth(0).FillHeight().Background(color)
		}
	})
}

func (a *App) dashboardIconButton(c *ui.Context, icon, label string, action func()) ui.Element {
	return ui.ButtonBase(c).Label(label).Tooltip(label).Size(32, 32).Center().OnClick(action).Children(func() { glyph(c, icon).FontSize(18).TextColor(a.colors.muted) })
}

func (a *App) memorySheet(c *ui.Context) {
	a.sideSheet(c, "memory", &a.memoryOpen, func() {
		a.sheetHeader(c, a.t("memoryInfo"), "close", a.t("close"), func() { a.memoryOpen = false }, func() {
			if a.memoryReleasing {
				ui.Box(c).Label(a.t("releaseMemory")).Size(44, 44).Radius(999).Background(a.colors.card).Center().Children(func() { loading(c, 24, a.colors.accent, a.t("loading")) })
			} else {
				a.sheetButton(c, "broom", a.t("releaseMemory"), a.releaseMemory)
			}
		})
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(20, 16, 20, 16).Gap(24).Children(func() {
				app, core := a.memoryTotals()
				ui.Column(c).FillWidth().Padding(0, 5).Gap(0).Children(func() {
					ui.Text(c, a.t("total")).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).TextColor(a.colors.muted)
					ui.Box(c).Height(4)
					a.memoryFigure(c, app+core, 28, 17)
					ui.Box(c).Height(16)
					a.memoryBar(c, app, core)
					ui.Text(c, a.t("memoryEstimateDesc")).Margin(8, 0, 0, 0).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted)
				})
				a.memorySection(c, a.t("app"), app, func() { a.memoryRow(c, a.t("memoryAppResident"), app, app) })
				a.memorySection(c, a.t("core"), core, func() {
					if !a.coreConnected() {
						ui.Text(c, a.t("memoryCoreNotRunning")).FontSize(fontSize(c, 17)).Padding(10, 16).MinHeight(56)
						return
					}
					m := a.sample.Memory
					rows := []struct {
						label string
						value uint64
					}{{"memoryCoreHeapInuse", m.Heap}, {"memoryCoreHeapIdle", m.Idle}, {"memoryCoreStack", m.Stack}, {"memoryCoreRuntime", m.Other}}
					used := m.Heap + m.Idle + m.Stack + m.Other
					if core > used {
						rows = append(rows, struct {
							label string
							value uint64
						}{"other", core - used})
					}
					for i, row := range rows {
						ui.Column(c).FillWidth().Children(func() {
							a.memoryRow(c, a.t(row.label), row.value, core)
							if i > 0 {
								ui.Box(c).Absolute().Top(0).Left(16).Right(16).Height(0.5).Background(a.colors.line.Alpha(0.6))
							}
						})
					}
				})
			})
		})
	})
}

func (a *App) memorySection(c *ui.Context, label string, bytes uint64, content func()) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Padding(0, 16).Children(func() {
			ui.Text(c, label).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).TextColor(a.colors.muted).Grow(1)
			ui.Text(c, strings.ReplaceAll(bytesText(int64(bytes)), " ", "")).FontSize(fontSize(c, 13)).TextColor(a.colors.muted)
		})
		ui.Column(c).FillWidth().Radius(24).Background(a.colors.card).Children(content)
	})
}
func (a *App) memoryRow(c *ui.Context, label string, bytes, total uint64) {
	percent := float64(0)
	if total > 0 {
		percent = float64(bytes) * 100 / float64(total)
	}
	ui.Row(c).FillWidth().MinHeight(56).Padding(10, 16).Gap(20).Children(func() {
		ui.Text(c, label).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).Grow(1)
		ui.Text(c, strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", percent), "0"), ".")+"%").FontSize(fontSize(c, 15)).TextColor(a.colors.muted)
	})
}

func (a *App) releaseMemory() {
	if a.memoryReleasing {
		return
	}
	if a.safe || a.engine == nil {
		a.message = a.t("releaseMemoryFailed")
		return
	}
	e, revision := a.engine, a.engine.Revision()
	app, core := a.memoryTotals()
	before := app + core
	a.memoryReleasing = true
	go func() {
		start := time.Now()
		err := e.Call("forceGc", nil, nil)
		var m Memory
		if err == nil {
			err = e.Call("getMemoryStats", nil, &m)
		}
		rss := desktop.ResidentMemory()
		if remaining := 500*time.Millisecond - time.Since(start); remaining > 0 {
			time.Sleep(remaining)
		}
		a.update(func() {
			a.memoryReleasing = false
			if a.engine != e || e.Revision() != revision {
				return
			}
			if err != nil {
				a.message = a.t("releaseMemoryFailed")
				return
			}
			a.sample.Memory, a.sample.AppRSS = m, rss
			a.message = a.t("memoryReleased")
			if after := m.RSS + rss; before > after {
				a.message = strings.ReplaceAll(a.t("memoryReleasedSize"), "{size}", bytesText(int64(before-after)))
			}
		})
	}()
}
