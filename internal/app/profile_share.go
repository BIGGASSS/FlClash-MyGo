package app

import (
	"errors"
	"math"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	qr "github.com/piglig/go-qr"
)

const profileShareQuietZone = 4

var errProfileShareTooLong = errors.New("profile URL exceeds QR capacity")

// Only immutable encoded data is retained, never native elements or a second
// navigation/editor owner. Sharing does not load a file, fetch a URL or apply it.
type profileShareState struct {
	Open        bool
	Title, Data string
	Code        *qr.QrCode
	Err         error
}

func profileShareable(p store.Profile) bool {
	// Native URL imports historically left Type empty; explicit Flutter file
	// and custom identities must not acquire subscription actions.
	return p.URL != "" && (p.Type == "" || p.Type == "url")
}

func (a *App) openProfileShare(p store.Profile) {
	if !profileShareable(p) || a.profileUpdating[p.ID] {
		return
	}
	title := p.Name
	if strings.TrimSpace(title) == "" {
		title = p.ID
		if u, err := url.Parse(p.URL); err == nil && u.Hostname() != "" {
			title = u.Hostname()
		}
	}
	code, err := encodeProfileShare(p.URL)
	a.profileShare = profileShareState{Open: true, Title: title, Data: p.URL, Code: code, Err: err}
}

// QrPayload.fromString in the pinned qr 4.0.0 uses one numeric/alphanumeric/
// UTF-8 byte segment, adding ECI 26 only for code units above Latin-1. Do not
// percent-encode, trim, add a BOM/share-link wrapper, optimize mixed segments,
// or boost the source's default medium error correction.
func encodeProfileShare(data string) (*qr.QrCode, error) {
	if len(data) > 5596 { // Even numeric mode cannot exceed this at version 40/M.
		return nil, errProfileShareTooLong
	}
	if !utf8.ValidString(data) {
		return nil, errors.New("profile URL is not valid UTF-8")
	}
	segments, err := qr.MakeSegments(data)
	if err != nil {
		return nil, err
	}
	if data == "" { // Dart emits an empty byte segment, not an empty payload.
		segment, e := qr.MakeBytes([]byte(data))
		if e != nil {
			return nil, e
		}
		segments = []*qr.QrSegment{segment}
	}
	if strings.ContainsFunc(data, func(r rune) bool { return r > 255 }) {
		eci, e := qr.MakeEci(26)
		if e != nil {
			return nil, e
		}
		segments = append([]*qr.QrSegment{eci}, segments...)
	}
	var best *qr.QrCode
	bestScore := math.Inf(1)
	for mask := range 8 {
		code, e := qr.EncodeSegments(segments, qr.Medium, 1, 40, mask, false)
		if e != nil {
			var tooLong *qr.DataTooLongException
			if errors.As(e, &tooLong) {
				return nil, errProfileShareTooLong
			}
			return nil, e
		}
		if score := profileShareMaskScore(code); score < bestScore {
			best, bestScore = code, score
		}
	}
	return best, nil
}

// Match qr 4.0.0's QrImage mask selection, including its light format/version
// placeholders during scoring and first-mask tie break. Its eight-neighbor
// penalty is deliberately not replaced by the encoder's ISO run-length score.
func profileShareMaskScore(code *qr.QrCode) float64 {
	n := code.GetSize()
	grid := make([][]bool, n)
	for y := range n {
		grid[y] = make([]bool, n)
		for x := range n {
			grid[y][x] = code.GetModule(x, y)
		}
	}
	for i := range 15 {
		y := n - 15 + i
		if i < 6 {
			y = i
		} else if i < 8 {
			y = i + 1
		}
		grid[y][8] = false
		x := 15 - i - 1
		if i < 8 {
			x = n - i - 1
		} else if i == 8 {
			x++
		}
		grid[8][x] = false
	}
	grid[n-8][8] = false
	if n >= 45 { // Version 7 and later.
		for i := range 18 {
			grid[i/3][i%3+n-11] = false
			grid[i%3+n-11][i/3] = false
		}
	}
	score, dark := 0, 0
	for y := range n {
		for x := range n {
			v := grid[y][x]
			if v {
				dark++
			}
			same := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if (dx != 0 || dy != 0) && x+dx >= 0 && x+dx < n && y+dy >= 0 && y+dy < n && grid[y+dy][x+dx] == v {
						same++
					}
				}
			}
			if same > 5 {
				score += same - 2
			}
			if y+1 < n && x+1 < n && v == grid[y][x+1] && v == grid[y+1][x] && v == grid[y+1][x+1] {
				score += 3
			}
			if v && x+6 < n && !grid[y][x+1] && grid[y][x+2] && grid[y][x+3] && grid[y][x+4] && !grid[y][x+5] && grid[y][x+6] {
				score += 40
			}
			if v && y+6 < n && !grid[y+1][x] && grid[y+2][x] && grid[y+3][x] && grid[y+4][x] && !grid[y+5][x] && grid[y+6][x] {
				score += 40
			}
		}
	}
	return float64(score) + math.Abs(100*float64(dark)/float64(n)/float64(n)-50)/5*10
}

func (a *App) profileShareDialog(c *ui.Context) {
	s := &a.profileShare
	if !s.Open {
		a.profileShare = profileShareState{}
		return
	}
	a.dialogBase(c.Key("profile-share"), &s.Open, func(backdrop, panel ui.Element) {
		w, h := c.Size()
		width := min(float32(348), max(1, w-80)) // CommonDialog's alert width/insets.
		inner := max(1, width-60)
		maxHeight := max(1, h-a.frameHeight-48)
		_, titleHeight := c.MeasureText(inner, ui.Span{Text: s.Title, Size: fontSize(c, 17), Weight: 600})
		titleHeight = min(titleHeight, a.textHeight(22)*2)
		message := a.t("shareQrcodeTip")
		if errors.Is(s.Err, errProfileShareTooLong) {
			message = a.t("qrcodeTooLongTip")
		} else if s.Err != nil {
			message = a.t("error") + ": " + s.Err.Error()
		}
		_, messageHeight := c.MeasureText(inner, ui.Span{Text: message, Size: fontSize(c, 15)})
		actionsHeight := float32(48)
		if s.Code == nil {
			actionsHeight += 56 // Direct copy fallback as well as the source menu item.
		}
		bodyRoom := max(1, maxHeight-21-titleHeight-6-20-actionsHeight-16)
		side := min(float32(240), inner, max(96, bodyRoom-messageHeight-16))
		bodyHeight := messageHeight
		if s.Code != nil {
			bodyHeight += side + 16
		}
		backdrop.Background(ui.RGBA(0, 0, 0, .3))
		panel.Label(s.Title).Width(width).MaxHeight(maxHeight).Padding(21, 30, 16, 30).Radius(34).Background(a.colors.card)
		ui.Text(c, s.Title).FillWidth().Height(titleHeight).Shrink(0).MaxLines(2).FontSize(fontSize(c, 17)).FontWeight(600).Tooltip(s.Title)
		ui.Box(c).Height(6).Shrink(0)
		ui.Scroll(c).FillWidth().Height(min(bodyHeight, bodyRoom)).Shrink(0).Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(16).Children(func() {
				if code := s.Code; code != nil {
					ui.Box(c.Key("qr-image")).Size(side, side).Shrink(0).Radius(16).Background(ui.Hex("#ffffff")).Role(ui.RoleImage).Label(s.Data).Draw(func(p *ui.Painter, r ui.Rect) {
						paintProfileShare(p, r, code)
					})
				}
				ui.Text(c, message).FillWidth().FontSize(fontSize(c, 15)).TextAlign(ui.Center).TextColor(a.colors.muted)
			})
		})
		ui.Box(c).Height(20).Shrink(0)
		if s.Code == nil {
			ui.Button(c, a.t("copyLink")).FillWidth().Height(48).Shrink(0).Radius(999).OnClick(func() {
				c.Services().WriteClipboard(s.Data)
				a.message = a.t("copySuccess")
			})
			ui.Box(c).Height(8).Shrink(0)
		}
		ui.PrimaryButton(c, a.t("close")).FillWidth().Height(48).Shrink(0).Radius(999).OnClick(func() { s.Open = false })
	})
	if !s.Open {
		a.profileShare = profileShareState{}
	}
}

func paintProfileShare(p *ui.Painter, r ui.Rect, code *qr.QrCode) {
	n := code.GetSize()
	module := min(r.W, r.H) / float32(n+2*profileShareQuietZone)
	x0, y0 := r.X+profileShareQuietZone*module, r.Y+profileShareQuietZone*module
	path := &ui.Path{}
	for y := range n {
		for x := 0; x < n; {
			if !code.GetModule(x, y) {
				x++
				continue
			}
			start := x
			for x < n && code.GetModule(x, y) {
				x++
			}
			left, right := x0+float32(start)*module, x0+float32(x)*module
			top, bottom := y0+float32(y)*module, y0+float32(y+1)*module
			path.MoveTo(left, top).LineTo(right, top).LineTo(right, bottom).LineTo(left, bottom).Close()
		}
	}
	// Paint a single path like QrCodeView: adjoining dark rows must not
	// acquire seams from separately antialiased fractional rectangles.
	p.FillPath(path, ui.Hex("#000000"))
}
