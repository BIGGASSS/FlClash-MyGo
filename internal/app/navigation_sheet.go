package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// side_sheet.dart applies standardDecelerate to the route's linear controller
// in BOTH directions (not a fresh easing of the remaining painted distance).
var sheetDecelerate = ui.CubicBezier(.05, .7, .1, 1)
var sheetBarrierEase = ui.CubicBezier(.25, .1, .25, 1)

// deferSheetClose lets an owner retain its content/async generation until the
// route has left. Call it AFTER save/dirty guards and BEFORE clearing owner
// state: if a.deferSheetClose(key, a.closeOwner) { return }. A reset/reopen
// replaces the state, so an obsolete exit can never close the replacement.
func (a *App) deferSheetClose(key string, close func()) bool {
	s := a.snapSheets[key]
	if s == nil || !s.Mounted || s.Committing || s.Reduced {
		return false
	}
	if !s.Dismissing && !s.DismissRequested {
		s.DismissRequested, s.Close = true, close
		s.clearInput()
	}
	return true
}

func (s *snapSheetState) present(now time.Time, target float64) {
	if s.Compact {
		// SnapSheetRoute._presentation: no overshoot, launch capped at omega
		// times the remaining distance, with throws away from it discarded.
		s.Presentation.step(now)
		velocity := s.Presentation.Velocity
		if target == 0 {
			velocity = s.ClosingVelocity
		}
		distance := target - s.Presentation.Value
		if distance == 0 || velocity*distance <= 0 {
			velocity = 0
		} else {
			velocity = math.Copysign(min(math.Abs(velocity), sheetOmega*math.Abs(distance)), distance)
		}
		s.Presentation.Velocity = velocity
		s.Presentation.to(now, target, springSpec{sheetOmega, 1})
	} else {
		s.SideFrom, s.SideTarget, s.SideStarted = s.Controller, target, now
	}
}

func (s *snapSheetState) motion(c *ui.Context, compact bool) float32 {
	now := c.Now()
	s.Reduced = c.Preferences().ReduceMotion
	if !s.Mounted || s.Compact != compact {
		// showSnapSheet reopens the route across the mobile breakpoint. Do
		// not carry a detent, pointer capture or wheel turn into the new form.
		if s.Mounted {
			s.Stop, s.Height, s.Minimum, s.Maximum = 0, springValue{}, 0, 0
			s.LiftFollows, s.Lift = false, 0
		}
		s.clearInput()
		s.LastWheel = time.Time{}
		s.Mounted, s.Compact, s.Controller = true, compact, 0
		s.Presentation.jump(now, 0)
		target := 1.0
		if s.Dismissing {
			target = 0
		}
		s.present(now, target)
	}
	if s.DismissRequested {
		s.DismissRequested, s.Dismissing = false, true
		if compact && s.Minimum > 0 && s.Height.Value < float64(s.Minimum) {
			s.Presentation.jump(now, max(0, s.Height.Value/float64(s.Minimum)+s.Presentation.Value-1))
			s.Height.jump(now, float64(s.Minimum))
		}
		s.Height.jump(now, s.Height.Value) // reverse stops the detent activity
		s.present(now, 0)
	}
	// SnapSheetRoute._close lends its throw to this frame's accepted pop
	// only. A guard rejecting it must not fling a later ordinary dismissal.
	s.ClosingVelocity = 0
	if compact {
		s.Controller = float64(sheetSpringFrame(c, &s.Presentation, .001, .02))
		s.Barrier = float32(s.Controller)
	} else {
		duration := 300 * time.Millisecond
		if s.SideTarget == 0 {
			duration = 200 * time.Millisecond
		}
		// AnimationController scales its remaining duration by the distance
		// left in controller units when a presentation is interrupted.
		span := duration.Seconds() * math.Abs(s.SideTarget-s.SideFrom)
		t := 1.0
		if !s.Reduced && span > 0 {
			t = min(max(now.Sub(s.SideStarted).Seconds()/span, 0), 1)
		}
		s.Controller = s.SideFrom + (s.SideTarget-s.SideFrom)*t
		if t < 1 {
			c.AnimationFrame()
		}
		s.Barrier = sheetBarrierEase(float32(s.Controller))
	}
	s.Progress = float32(s.Controller)
	if !compact {
		s.Progress = sheetDecelerate(s.Progress)
	}
	return s.Progress
}

// A sheet can host a second page. Escape pops that page while clicking the
// backdrop dismisses the sheet, matching BaseNavigator inside the source sheet.
func (a *App) navigationSheet(c *ui.Context, key string, open *bool, close, back func(), content func(), naturalHeight ...float32) {
	if !*open {
		return
	}
	s := a.snapSheet(key)
	width, height := c.Size()
	available := max(0, height-a.frameHeight)
	compact := width <= 600
	progress := s.motion(c, compact)
	if s.Dismissing && s.Controller == 0 {
		// Only the current route may release its owner. No ui.Element or
		// content builder is retained by the dismissal state.
		commit := s.Close
		s.Close, s.Committing = nil, true
		if a.snapSheets[key] == s && commit != nil {
			commit()
		}
		s.Committing = false
		if a.snapSheets[key] != s || !*open {
			return
		}
		// A save/dirty guard may reject a request. Restore the same route,
		// retaining its content, rather than disposing it behind the guard.
		s.Dismissing = false
		s.present(c.Now(), 1)
		c.AnimationFrame()
	}
	// The owner runs its guards first, then defers the accepted close with
	// deferSheetClose. Calling it here also keeps nested Escape pops distinct
	// from dismissal, and shows dirty/save prompts without first hiding a page.
	requestClose := close
	ui.Overlay(c.Key(key), func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(a.frameHeight).Right(0).Bottom(0).Modal().Clip().OnClick(requestClose)
		backdrop.Children(func() {
			pane := ui.Column(c).Role(ui.RoleDialog).Absolute().Right(0).Background(a.colors.page).OnClick(func() {}).Focusable().FocusRing(false).Bind(&s.Focus)
			shown := available
			barrier := s.Barrier
			if compact {
				s.Width = width
				minimum, maximum := available*9/16, available*.9
				if len(naturalHeight) > 0 {
					natural := naturalHeight[0]
					if key == "dashboard-picker" && a.picker.Kind == "profiles" && a.profilePickerNavigation() == nil {
						// Match profilesGridView's rows, not the number of cards.
						// Width/text scale can change its column count while open.
						spacing := float32(8) * min(1, 1+(a.cfg.Scale-1)/2)
						columns := profileColumns(max(0, width-32), spacing, a.cfg.Scale)
						rows := (len(a.cfg.Profiles) + columns - 1) / columns
						natural = 68 + 16 + float32(rows)*(a.textHeight(22)+4+a.textHeight(18)+20) + float32(max(0, rows-1))*spacing
					}
					maximum = min(maximum, max(minimum, natural))
				}
				if s.Pressed && (!s.Focus.FocusWithin(c) || s.Minimum != minimum || s.Maximum != maximum) {
					// A covering modal can replace native capture without sending
					// another event here. Cancel during builds too, so closing it
					// never revives the lower route's obsolete press. Refitting
					// also invalidates the captured local geometry.
					s.cancel(c.Now(), available, maximum, requestClose)
				}
				shown = s.height(c, available, maximum)
				layoutHeight := max(minimum, shown)
				// _RenderSheetViewport keeps the minimum layout height while
				// a pull below it moves presentation, including the barrier.
				below := min(float32(0), shown-minimum)
				if minimum > 0 {
					barrier *= min(max(shown/minimum, 0), 1)
				}
				pane.Width(width).Height(layoutHeight).Bottom(below-(1-progress)*layoutHeight).Radius(28, 28, 0, 0).Clip()
				pane.HandleInput(func(e ui.InputEvent) bool {
					if a.snapSheets[key] != s || s.Dismissing || s.DismissRequested {
						return true
					}
					paneFocused := s.Focus.Focused(c)
					scrollFocused := s.Scroll != nil && s.Scroll.Handle.Focused(c)
					if s.Pressed && (!paneFocused && !(s.ContentDrag && scrollFocused) || s.ContentDrag && s.DragScroll != s.Scroll) {
						// Blur and a covering modal cancel, never fling the route
						// underneath. Also reject a replaced page's late release.
						return s.cancel(e.Time, available, maximum, requestClose)
					}
					editor := key == "profile-details" && a.profileDetail.Preview
					if nav := a.profilePickerNavigation(); key == "dashboard-picker" && nav != nil && nav.Page == "editor" {
						editor = true
					}
					inContent := s.inScroll(c, e)
					if e.Kind == ui.InputScroll {
						// Editors own their padding/find bar too, not just the
						// inner handler, including at either document edge.
						if editor && e.Y >= 68 {
							return false
						}
						return s.wheel(e, available, maximum, inContent)
					}
					if e.Kind == ui.InputPointerDown {
						// MyGo focuses the deepest control before dispatch. Never
						// capture buttons, fields, selectable text or editor padding:
						// the toolkit has no late gesture-arena takeover/replay.
						if !paneFocused && !(inContent && scrollFocused) || editor && e.Y >= 68 {
							return false
						}
						if inContent && e.Y >= 68 {
							return s.startContent(e)
						}
					}
					return s.input(e, available, maximum, requestClose)
				})
			} else {
				pane.Top(0).Bottom(0).Width(a.sideSheetWidth(c)).Right(-(1 - progress) * a.sideSheetWidth(c))
			}
			backdrop.Background(ui.RGBA(0, 0, 0, .32*barrier))
			pane.Disabled(s.Dismissing || s.DismissRequested).Children(func() {
				previousCompact, previousHeight := a.compactSheet, a.sheetContentHeight
				a.compactSheet, a.sheetContentHeight = compact, max(available*9/16, shown)
				defer func() { a.compactSheet, a.sheetContentHeight = previousCompact, previousHeight }()
				if compact {
					if !s.Dismissing {
						s.Lift = max(0, available*9/16-shown)
						if s.LiftFollows {
							s.Lift += (1 - progress) * max(available*9/16, shown)
						}
					}
					// Keep docked foot controls above the pulled-down overhang,
					// and clip them once there is no room. Unlike Flutter's
					// footer-only paint lift, native content also loses this room.
					ui.Column(c.Key("sheet-viewport")).FillWidth().Height(max(0, max(available*9/16, shown)-s.Lift)).Shrink(0).Clip().Children(content)
					a.navigationSheetScroll(c, key, s)
				} else {
					content()
				}
			})
		})
		// Let the editor's find bar and other focused children handle Escape
		// before falling back to popping the sheet's navigation stack.
		if backdrop.OverlayShortcut(0, ui.KeyEscape) {
			s.clearInput()
			if !s.Dismissing && !s.DismissRequested {
				back()
			}
			c.AnimationFrame()
		}
	})
}

// Resolve only the current build's scroller; retain its numeric ScrollState,
// not an element. Existing native lists otherwise hide partial first-row
// offsets, which are essential to the source's wheel-at-top handoff.
func (a *App) navigationSheetScroll(c *ui.Context, key string, s *snapSheetState) {
	s.Scroll = nil
	var handle ui.Handle
	switch key {
	case "dashboard-picker":
		if nav := a.profilePickerNavigation(); nav != nil {
			switch nav.Page {
			case "editor":
				return
			case "collection":
				if a.document != nil {
					handle = a.document.List.Handle
				}
			default:
				s.Scroll = &nav.Scroll
				return
			}
		} else if a.picker.Kind == "profiles" {
			handle = a.picker.Profiles.List.Handle
		} else if a.cfg.ProxyStyle != "tab" {
			s.Scroll = &a.picker.Proxy.proxyScroll
			return
		} else if grid := a.picker.Proxy.proxyGrids[a.cfg.Active+"/"+a.group]; grid != nil {
			handle = grid.Handle
		}
	case "profile-details":
		if !a.profileDetail.Preview {
			handle = a.profileDetail.List.Handle
		}
	}
	if element := c.Resolve(handle); element.Valid() {
		if s.Scrolls == nil {
			s.Scrolls = map[ui.Handle]*ui.ScrollState{}
		}
		track := s.Scrolls[handle]
		if track == nil {
			track = &ui.ScrollState{}
			s.Scrolls[handle] = track
		}
		element.TrackScroll(track)
		s.Scroll = track
	}
}
