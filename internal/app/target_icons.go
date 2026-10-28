package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"image"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/BIGGASSS/FlClash-MyGo/internal/iconcache"
	"github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
)

const decodedIconBudget = 64 << 20

type iconMount struct {
	source string
	frame  uint64
}

type targetIcon struct {
	digest  [32]byte
	svg     *ui.SVG
	bitmap  *ui.Bitmap
	cost    int
	used    uint64
	pending bool
	retry   time.Time
}

func groupMark(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return strings.ToUpper(string(r))
		}
	}
	var text segmenter.Segmenter
	text.Init([]rune(name))
	iterator := text.GraphemeIterator()
	if iterator.Next() {
		return string(iterator.Grapheme().Text)
	}
	return ""
}

func decodeTargetIcon(data []byte, source string) (*targetIcon, error) {
	if len(data) == 0 || len(data) > iconcache.MaxBytes {
		return nil, errors.New("invalid icon size")
	}
	entry := &targetIcon{digest: sha256.Sum256(data)}
	isSVG := strings.HasPrefix(source, "data:image/svg+xml")
	if !strings.HasPrefix(source, "data:") {
		if parsed, err := url.Parse(source); err == nil {
			isSVG = strings.HasSuffix(strings.ToLower(parsed.Path), ".svg")
		}
	}
	if isSVG {
		var err error
		entry.svg, err = ui.ParseSVG(data)
		if err != nil {
			return nil, err
		}
		entry.cost = len(data) * 4
	} else {
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16<<20 {
			return nil, errors.New("icon dimensions exceed decode budget")
		}
		entry.bitmap, entry.cost, err = iconBitmap(data)
		if err != nil {
			return nil, err
		}
	}
	if entry.cost > decodedIconBudget {
		return nil, errors.New("icon exceeds decode budget")
	}
	return entry, nil
}

// Native windows commit completed loads through Window.Update. Safe/headless
// previews decode inline fixtures only and never fetch a remote icon.
func (a *App) targetIcon(c *ui.Context, source, name string, side float32, color ui.Color) {
	host := ui.Box(c).Center()
	if a.iconMounts == nil {
		a.iconMounts = map[uint64]iconMount{}
	}
	previous, seen := a.iconMounts[host.ID()]
	mounted := !seen || previous.source != source || previous.frame+1 < a.iconFrame
	a.iconMounts[host.ID()] = iconMount{source, a.iconFrame}
	host.Children(func() { a.targetIconContent(c, source, name, side, color, mounted) })
}

func (a *App) targetIconContent(c *ui.Context, source, name string, side float32, color ui.Color, mounted bool) {
	var entry *targetIcon
	if source != "" {
		if a.targetIcons == nil {
			a.targetIcons = map[string]*targetIcon{}
		}
		a.iconUse++
		entry = a.targetIcons[source]
		if entry == nil || !entry.pending && entry.svg == nil && entry.bitmap == nil && (mounted || time.Now().After(entry.retry)) {
			if a.makeIconRoom() {
				entry = &targetIcon{used: a.iconUse}
				a.targetIcons[source] = entry
				if a.window == nil || a.safe {
					if data, inline, err := iconcache.Inline(source); inline && err == nil {
						if decoded, err := decodeTargetIcon(data, source); err == nil {
							entry.svg, entry.bitmap, entry.cost, entry.digest = decoded.svg, decoded.bitmap, decoded.cost, decoded.digest
						}
					}
					entry.retry = time.Now().Add(time.Minute)
					a.trimTargetIcons()
				} else {
					a.startTargetIcon(source, entry)
				}
			}
		}
		if entry != nil {
			entry.used = a.iconUse
		}
		if mounted && entry != nil && !entry.pending && (entry.svg != nil || entry.bitmap != nil) && a.window != nil && !a.safe && !strings.Contains(source, "base64,") {
			a.startTargetIcon(source, entry)
		}
	}
	if entry != nil && entry.svg != nil {
		w, h := entry.svg.Size()
		w, h = fitIcon(w, h, side)
		ui.Image(c, entry.svg).Label(name+" icon").Fit(ui.Contain).Size(w, h)
		return
	}
	if entry != nil && entry.bitmap != nil {
		width, height := entry.bitmap.Size()
		w, h := fitIcon(float32(width), float32(height), side)
		ui.Image(c, entry.bitmap).Label(name+" icon").Fit(ui.Contain).Size(w, h)
		return
	}
	mark := groupMark(name)
	if mark == "" {
		glyph(c, "photos").FontSize(side * 0.7).TextColor(color)
		return
	}
	ui.Text(c, mark).FontSize(fontSize(c, side*0.62)).FontWeight(600).TextColor(color).SingleLine()
}

func (a *App) startTargetIcon(source string, entry *targetIcon) {
	if a.iconCache == nil {
		a.iconCache = iconcache.New(filepath.Join(a.store.Dir, "cache", "icons"))
	}
	cache, client, window := a.iconCache, a.httpClient(), a.window
	entry.pending = true
	previousDigest, hasImage := entry.digest, entry.svg != nil || entry.bitmap != nil
	go func() {
		defer client.CloseIdleConnections()
		data, _ := cache.Load(client, source)
		var decoded *targetIcon
		var err error
		if !hasImage || sha256.Sum256(data) != previousDigest {
			decoded, err = decodeTargetIcon(data, source)
		}
		window.Update(func() {
			if a.targetIcons[source] != entry || a.quitting {
				return
			}
			entry.pending = false
			if err == nil && decoded != nil {
				entry.svg, entry.bitmap, entry.cost, entry.digest = decoded.svg, decoded.bitmap, decoded.cost, decoded.digest
			}
			entry.retry = time.Now().Add(time.Minute)
			a.trimTargetIcons()
		})
	}()
}

func (a *App) makeIconRoom() bool {
	if len(a.targetIcons) < 128 {
		return true
	}
	return a.evictTargetIcon()
}
func (a *App) evictTargetIcon() bool {
	oldest := ""
	var used uint64
	for source, entry := range a.targetIcons {
		if !entry.pending && (oldest == "" || entry.used < used) {
			oldest, used = source, entry.used
		}
	}
	if oldest == "" {
		return false
	}
	delete(a.targetIcons, oldest)
	return true
}
func (a *App) trimTargetIcons() {
	for {
		cost := 0
		for _, entry := range a.targetIcons {
			cost += entry.cost
		}
		if cost <= decodedIconBudget {
			return
		}
		if !a.evictTargetIcon() {
			return
		}
	}
}

func fitIcon(width, height, side float32) (float32, float32) {
	if width <= 0 || height <= 0 {
		return side, side
	}
	scale := min(float32(1), side/max(width, height))
	return width * scale, height * scale
}
