package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type sidebarOverlayState struct {
	Mounted, Open bool
	From, To      float32
	Started       time.Time
}

func (s *sidebarOverlayState) value(now time.Time, reduce bool) float32 {
	if reduce {
		return s.To
	}
	t := min(max(float32(now.Sub(s.Started))/float32(250*time.Millisecond), 0), 1)
	return s.From + (s.To-s.From)*sidebarExpansion(t)
}

func (a *App) openSidebarOverlay(now time.Time) {
	s := &a.sidebarOverlay
	from := float32(0)
	if s.Mounted {
		from = s.value(now, false)
	}
	*s = sidebarOverlayState{Mounted: true, Open: true, From: from, To: 1, Started: now}
}

func (a *App) closeSidebarOverlay(now time.Time) {
	s := &a.sidebarOverlay
	if !s.Mounted || !s.Open {
		return
	}
	s.From, s.To, s.Started, s.Open = s.value(now, false), 0, now, false
}

func (a *App) sidebarOverlayView(c *ui.Context) {
	s := &a.sidebarOverlay
	if !s.Mounted {
		return
	}
	width, _ := c.Size()
	if a.frameEnabled && (width <= 600 || width > 840) {
		*s = sidebarOverlayState{}
		return
	}
	progress := s.value(c.Now(), c.Preferences().ReduceMotion)
	if !s.Open && progress < .001 {
		*s = sidebarOverlayState{}
		return
	}
	if progress != s.To {
		c.AnimationFrame()
	}
	ui.Overlay(c.Key("sidebar-overlay"), func() {
		close := func() { a.closeSidebarOverlay(c.Now()) }
		back := ui.Box(c).Absolute().Left(0).Top(a.frameHeight).Right(0).Bottom(0).Modal().OnClick(close)
		if back.OverlayShortcut(0, ui.KeyEscape) {
			close()
		}
		back.Children(func() {
			compact := a.sidebarCompactWidth(c)
			sideInset := min(float32(12), (compact-40)/2)
			pane := ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Width(compact+(220-compact)*progress).Padding(4, sideInset).Gap(0).Background(a.colors.chrome).Clip().OnClick(func() {}).
				Shadow(0, 5, 5, -3, ui.RGBA(0, 0, 0, .2)).Shadow(0, 8, 10, 1, ui.RGBA(0, 0, 0, .14)).Shadow(0, 3, 14, 2, ui.RGBA(0, 0, 0, .12))
			a.sidebarPane(c, pane, compact, progress, true, close, close, &a.sidebarOverlaySelection)
		})
	})
}
