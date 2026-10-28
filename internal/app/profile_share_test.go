package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

// These values come from the original Dart encoder, not the Go dependency or
// application source. Normal tests need neither Flutter nor the source checkout.
func TestProfileShareOriginalDartFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-profile-qr.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		QR              string `json:"qr"`
		ErrorCorrection string
		QuietZone       int
		Cases           []struct {
			Name, Data, SHA256 string
			Fits               bool
			Version, Modules   int
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.QR != "4.0.0" || fixture.ErrorCorrection != "medium" || fixture.QuietZone != profileShareQuietZone || len(fixture.Cases) < 20 {
		t.Fatal("unexpected original-encoder corpus")
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			code, err := encodeProfileShare(test.Data)
			if !test.Fits {
				if code != nil || !errors.Is(err, errProfileShareTooLong) {
					t.Fatalf("oversize input: code=%v err=%v", code, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if code.GetSize() != test.Modules || (code.GetSize()-17)/4 != test.Version {
				t.Fatal("source QR version/size changed", code.GetSize(), test.Modules)
			}
			bits := make([]byte, 0, test.Modules*test.Modules)
			for y := range test.Modules {
				for x := range test.Modules {
					bit := byte('0')
					if code.GetModule(x, y) {
						bit = '1'
					}
					bits = append(bits, bit)
				}
			}
			hash := sha256.Sum256(bits)
			if got := hex.EncodeToString(hash[:]); got != test.SHA256 {
				t.Fatalf("original Dart module matrix differs: got %s, want %s", got, test.SHA256)
			}
		})
	}
}

func TestProfileShareEligibilityAndErrors(t *testing.T) {
	a := testApp(t)
	for _, p := range []store.Profile{
		{ID: "local", Name: "Local"},
		{ID: "file", Type: "file", URL: "https://example.invalid/unused"},
		{ID: "custom", Type: "custom", URL: "https://example.invalid/unused"},
		{ID: "url", Type: "url"},
	} {
		a.openProfileShare(p)
		if a.profileShare.Open {
			t.Fatal("non-subscription profile acquired a QR action", p)
		}
	}
	p := store.Profile{ID: "remote", URL: "https://example.invalid/sub"}
	a.profileUpdating = map[string]bool{p.ID: true}
	a.openProfileShare(p)
	if a.profileShare.Open {
		t.Fatal("updating profile acquired an action")
	}
	a.profileUpdating[p.ID] = false
	a.openProfileShare(p)
	if !a.profileShare.Open || a.profileShare.Title != "example.invalid" || a.profileShare.Data != p.URL {
		t.Fatal("missing real-label fallback or verbatim URL")
	}
	if code, err := encodeProfileShare("https://example.invalid/\xff"); code != nil || err == nil || errors.Is(err, errProfileShareTooLong) {
		t.Fatal("invalid UTF-8 was silently rewritten or called a capacity error")
	}
	if code, err := encodeProfileShare(strings.Repeat("1", 1<<20)); code != nil || !errors.Is(err, errProfileShareTooLong) {
		t.Fatal("unbounded input reached the encoder")
	}
}

func TestProfileSharePickerContextPreservesDirtyMainAndScroll(t *testing.T) {
	a := pickerApp(t)
	a.frameEnabled = true
	a.selectNavigation("tools")
	a.openCode("Main script", "library-script:fixture", "original main")
	a.code.Buffer.Set("dirty main")
	mainCode, active := a.code, a.cfg.Active
	for i := range 60 {
		a.cfg.Profiles = append(a.cfg.Profiles, store.Profile{ID: fmt.Sprint(i), Name: fmt.Sprintf("Remote %02d", i), URL: fmt.Sprintf("https://example.invalid/%d?token=Ab%%2F+c#keep", i)})
	}
	a.openProfilePicker()
	tt := pickerTester(a)
	a.picker.Profiles.List.ScrollIntoView(25)
	tt.Frame()
	before, ok := tt.Find("Remote 24")
	if !ok {
		t.Fatal("fixture scroll row not visible")
	}
	chooseProfileAction(t, tt, "Remote 24", a.t("more"), a.t("qrcode"))
	if s := a.profileShare; !s.Open || s.Title != "Remote 24" || s.Data != a.cfg.Profiles[25].URL || s.Code == nil || a.cfg.Active != active || a.working != 0 {
		t.Fatal("sharing changed selection, loaded config, or used the active profile", s)
	}
	tt.Key(ui.Cmd, ui.KeyLeft)
	if !a.profileShare.Open || !a.picker.Open || a.picker.Navigation != nil {
		t.Fatal("a modal let the background back shortcut navigate")
	}
	tt.Key(0, ui.KeyEscape)
	if a.profileShare.Open || !a.picker.Open || a.picker.Navigation != nil || a.confirmOpen || a.code != mainCode || mainCode.Buffer.String() != "dirty main" {
		t.Fatal("QR Escape dismissed the picker or changed its dirty background")
	}
	after, ok := tt.Find("Remote 24")
	if !ok || before != after {
		t.Fatal("modal lost picker scroll", before, after)
	}
}

func TestProfileShareDismissalKeepsDirtySheet(t *testing.T) {
	for _, dismiss := range []string{"escape", "backdrop", "close"} {
		t.Run(dismiss, func(t *testing.T) {
			a := pickerApp(t)
			a.openProfilePicker()
			a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
			a.readProfileCode(a.editProfile, "profile-staged")
			waitApp(t, a, func() bool { return a.working == 0 })
			code, nav := a.code, a.picker.Navigation
			code.Buffer.Set(pickerSource + "# unsaved sheet\n")
			nav.Scroll.Y = 37
			a.openProfileShare(a.cfg.Profiles[0])
			tt := pickerTester(a)
			switch dismiss {
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "backdrop":
				tt.ClickAt(4, 100)
			case "close":
				if err := tt.Click(a.t("close")); err != nil {
					t.Fatal(err)
				}
			}
			if a.profileShare.Open || !a.picker.Open || a.picker.Navigation != nav || nav.Page != "editor" || nav.Scroll.Y != 37 || a.code != code || !code.Buffer.Modified() || a.confirmOpen {
				t.Fatal("modal dismissal touched the owned editor/navigation state")
			}
			tt.Key(0, ui.KeyEscape)
			if !a.confirmOpen || !a.picker.Open || a.code != code {
				t.Fatal("the underlying dirty guard was lost")
			}
		})
	}
}

func TestProfileShareCapacityCopyFallbackAndExistingCopyLink(t *testing.T) {
	a := pickerApp(t)
	a.page = "profiles"
	p := &a.cfg.Profiles[0]
	p.URL = "https://example.invalid/" + strings.Repeat("a", 2400) + "?token=%2f+A#keep"
	exact := p.URL
	tt := pickerTester(a)
	chooseProfileAction(t, tt, p.Name, a.t("more"), a.t("copyLink"))
	if tt.Clipboard() != exact {
		t.Fatal("existing copy-link fallback changed the source URL")
	}
	tt.SetClipboard("unchanged")
	chooseProfileAction(t, tt, p.Name, a.t("more"), a.t("qrcode"))
	if !a.profileShare.Open || a.profileShare.Code != nil || !tt.HasText(a.t("qrcodeTooLongTip")) || tt.HasText(a.t("shareQrcodeTip")) || a.err != "" || tt.Clipboard() != "unchanged" {
		t.Fatal("capacity failure was not contained in the share dialog")
	}
	if err := tt.Click(a.t("copyLink")); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != exact || !a.profileShare.Open {
		t.Fatal("fallback did not copy exactly, or discarded the modal")
	}
	tt.Key(0, ui.KeyEscape)
	if a.profileShare.Open || a.page != "profiles" {
		t.Fatal("oversize dialog did not dismiss independently")
	}
}

func TestProfileShareColorsQuietZoneAndMinimumWindow(t *testing.T) {
	for _, size := range [][2]int{{1200, 800}, {380, 400}} {
		for _, dark := range []bool{false, true} {
			for _, scale := range []float32{.8, 1, 1.4} {
				t.Run(fmt.Sprint(size, dark, scale), func(t *testing.T) {
					a := testApp(t)
					a.frameEnabled, a.framePlatform, a.cfg.Scale = true, "windows", scale
					a.cfg.Theme = "light"
					if dark {
						a.cfg.Theme = "dark"
					}
					p := store.Profile{ID: "qr", Name: "QR fixture", URL: "https://example.invalid/sub?token=AbC%2f"}
					a.openProfileShare(p)
					tt := ui.NewTester(a.View, size[0], size[1])
					r, ok := tt.Find(p.URL)
					if !ok || r.W != r.H || r.W > 240 || r.W < 96 || r.Y < a.frameHeight || r.X < 0 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
						t.Fatal("QR does not fit the body/minimum window", r)
					}
					close, ok := tt.Find(a.t("close"))
					if !ok || close.Y < a.frameHeight || close.Y+close.H > float32(size[1]) {
						t.Fatal("close action escaped the window", close)
					}
					code := a.profileShare.Code
					module := r.W / float32(code.GetSize()+8)
					img := tt.Image()
					white := color.RGBA{255, 255, 255, 255}
					black := color.RGBA{0, 0, 0, 255}
					// Mid-edge quiet zone avoids the source's rounded white corners.
					for _, pos := range [][2]float32{{r.W / 2, module * 2}, {module * 2, r.H / 2}, {r.W - 2*module, r.H / 2}, {r.W / 2, r.H - 2*module}} {
						if got := img.RGBAAt(int(r.X+pos[0]), int(r.Y+pos[1])); got != white {
							t.Fatal("quiet zone is not white in both appearances", got)
						}
					}
					// Every module center also checks painting orientation, quiet
					// zone origin, fractional scaling and uninverted source colors.
					for y := range code.GetSize() {
						for x := range code.GetSize() {
							want := white
							if code.GetModule(x, y) {
								want = black
							}
							got := img.RGBAAt(int(r.X+(float32(x)+4.5)*module), int(r.Y+(float32(y)+4.5)*module))
							if got != want {
								t.Fatalf("painted module (%d,%d) = %v, want %v", x, y, got, want)
							}
						}
					}
					if !tt.HasText(a.t("shareQrcodeTip")) {
						t.Fatal("share caption is missing")
					}
				})
			}
		}
	}
}
