package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// CommonSwitch uses FlClash's 52×32 track inside the 60×48 hit target. The
// behavior/semantics come from MyGo; its drawing and motion follow FlClash.
func (a *App) switchControl(c *ui.Context, on *bool, label string) ui.Element {
	e := ui.SwitchBase(c, on).Label(label).Size(60, 48)
	target := float32(0)
	if *on {
		target = 1
	}
	position := e.Animate("position", target, 300*time.Millisecond)
	held := float32(0)
	if e.Pressed() {
		held = 1
	}
	lift := e.Animate("lift", held, 150*time.Millisecond)
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		track := ui.Rect{X: r.X + 4, Y: r.Y + (r.H-32)/2, W: 52, H: 32}
		p.Fill(track, a.colors.inset.Mix(a.colors.accent, position), 16)
		size := 24 + 16*lift
		thumb := ui.Rect{X: track.X + 16 + 20*position - size/2, Y: track.Y + 16 - size/2, W: size, H: size}
		if lift > 0 {
			p.Shadow(thumb, size/2, 0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.2*lift))
		}
		color := a.colors.outline.Mix(a.colors.onAccent, position).Mix(ui.RGB(255, 255, 255), lift)
		p.Fill(thumb, color, size/2)
	})
	return e
}
