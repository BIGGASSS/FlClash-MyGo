package app

import (
	"github.com/egoist/mygo/ui"
	"time"
)

func (a *App) coreConnected() bool {
	if a.engine != nil {
		return a.engine.Connected()
	}
	// Populated safe previews supply a connected core's synthetic sample.
	return a.safe && a.status == "running"
}

func (a *App) connectionPhase() string {
	if a.engine != nil && a.engine.Restarting() {
		return "connecting"
	}
	if a.coreConnected() {
		return "connected"
	}
	if a.engine != nil && a.status == "connecting" {
		return "connecting"
	}
	return "disconnected"
}

func (a *App) coreStatusControl(c *ui.Context) {
	phase := a.connectionPhase()
	if phase == "connecting" && a.corePhase != "" && a.corePhase != "connecting" && !c.Now().Before(a.coreHoldUntil) {
		a.coreHoldUntil = c.Now().Add(600 * time.Millisecond)
	}
	if phase == "disconnected" {
		a.coreHoldUntil = time.Time{}
	}
	a.corePhase = phase
	if c.Now().Before(a.coreHoldUntil) {
		phase = "connecting"
		c.After(a.coreHoldUntil.Sub(c.Now()))
	}
	bg, fg := a.colors.dangerContainer, a.colors.onDangerContainer
	if phase == "connected" {
		bg, fg = a.colors.successContainer, a.colors.onSuccessContainer
	}
	if phase == "connecting" {
		bg, fg = a.colors.accent, a.colors.onAccent
	}
	button := ui.ButtonBase(c.Key("core-status")).Label(a.t("coreStatus")).Tooltip(a.t("coreStatus")).Height(44).Radius(999).Background(bg).TextColor(fg).OnClick(func() {
		if phase == "connecting" {
			return
		}
		a.confirmCoreRestart()
	})
	if phase == "connected" {
		button.Width(44).Center()
	} else {
		button.Padding(0, 14)
	}
	button.Children(func() {
		ui.Row(c).Center().Gap(8).Children(func() {
			if phase == "connecting" {
				loading(c, a.textHeight(20), fg, a.t("connecting"))
			} else {
				icon := "refresh"
				if phase == "connected" {
					icon = "check"
				}
				side := float32(24)
				if phase != "connected" {
					side = a.textHeight(20)
				}
				glyphVariant(c, icon, true).FontSize(side)
			}
			if phase != "connected" {
				ui.Text(c, a.t(phase)).FontSize(fontSize(c, 15)).FontWeight(600).SingleLine()
			}
		})
	})
}

func (a *App) confirmCoreRestart() {
	tip := a.t("restartCoreTip")
	if a.coreConnected() {
		tip = a.t("forceRestartCoreTip")
	}
	a.ask(a.t("coreStatus"), tip, func() {
		if a.safe {
			a.message = "Safe mode"
			return
		}
		a.holdCoreConnecting()
		if a.engine == nil {
			a.begin()
			return
		}
		a.engine.Restart()
	})
}

func (a *App) holdCoreConnecting() {
	if a.page == "dashboard" && a.corePhase != "" && !time.Now().Before(a.coreHoldUntil) {
		a.coreHoldUntil = time.Now().Add(600 * time.Millisecond)
	}
}
