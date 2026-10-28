package app

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

const testGroupSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#ff0000" d="M0 0h12v24H0z"/><path fill="#00ff00" d="M12 0h12v24H12z"/></svg>`

func TestGroupIconsRenderInlineColorsAndRespectHiddenMode(t *testing.T) {
	a := populatedApp(t)
	a.page = "proxies"
	source := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(testGroupSVG))
	group := a.sample.Proxies.Proxies["Proxy"]
	group.Icon = source
	a.sample.Proxies.Proxies["Proxy"] = group
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find("Proxy icon"); !ok {
		t.Fatal("custom group icon was not rendered")
	}
	entry := a.targetIcons[source]
	if entry == nil || entry.svg == nil {
		t.Fatal("inline icon did not decode")
	}
	tt.Frame()
	if a.targetIcons[source] != entry {
		t.Fatal("icon decoded again every frame")
	}
	a.cfg.ProxyIcon = "hidden"
	tt.Frame()
	if _, ok := tt.Find("Proxy icon"); ok {
		t.Fatal("hidden mode displayed custom icon")
	}
	// A multicolor SVG uses Image rather than monochrome Icon rendering.
	view := func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Size(48, 48).Center().Children(func() { a.targetIcon(c, source, "Fixture", 48, a.colors.text) })
	}
	im := ui.NewTester(view, 48, 48).Image()
	red, green := color.NRGBAModel.Convert(im.At(18, 24)).(color.NRGBA), color.NRGBAModel.Convert(im.At(30, 24)).(color.NRGBA)
	if red.R < 240 || red.G > 10 || green.G < 240 || green.R > 10 {
		t.Fatal("custom SVG lost its original colors", red, green)
	}
}

func TestSafeRemoteIconsStayOfflineAndUseMonogram(t *testing.T) {
	a := populatedApp(t)
	a.page = "proxies"
	group := a.sample.Proxies.Proxies["Proxy"]
	group.Icon = "http://127.0.0.1:1/unreachable.svg"
	a.sample.Proxies.Proxies["Proxy"] = group
	_ = ui.NewTester(a.View, 1200, 800)
	entry := a.targetIcons[group.Icon]
	if entry == nil || entry.pending || a.iconCache != nil {
		t.Fatal("safe preview started an icon download")
	}
	for name, want := range map[string]string{"🇭🇰 Hong Kong": "H", "🔒 日本": "日", " 42 nodes": "4", "👨‍👩‍👧‍👦": "👨‍👩‍👧‍👦", "": ""} {
		if got := groupMark(name); got != want {
			t.Fatal(name, got, want)
		}
	}
}

func TestRasterIconsDecodeOnceAtOriginalPreviewExtent(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		t.Fatal(err)
	}
	entry, err := decodeTargetIcon(encoded.Bytes(), "https://fixture.invalid/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	w, h := entry.bitmap.Size()
	if w != 192 || h != 96 || entry.cost != 192*96*4 {
		t.Fatal("raster was not resized before caching", w, h, entry.cost)
	}
	if _, err := decodeTargetIcon([]byte(testGroupSVG), "https://fixture.invalid/icon.SVG?version=1#symbol"); err != nil {
		t.Fatal("SVG URL suffix ignored query/fragment", err)
	}
	if _, err := decodeTargetIcon([]byte("not an image"), "broken.png"); err == nil {
		t.Fatal("invalid raster decoded")
	}
	if _, err := decodeTargetIcon([]byte("<broken"), "broken.svg"); err == nil {
		t.Fatal("invalid vector decoded")
	}
}

func TestJPEGIconOrientationSurvivesThumbnailing(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 400, 200)), nil); err != nil {
		t.Fatal(err)
	}
	for _, orientation := range []byte{2, 3, 4, 5, 6, 7, 8} {
		// A little-endian TIFF IFD with the SHORT Orientation tag.
		tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, orientation, 0, 0, 0, 0, 0, 0, 0}
		payload := append([]byte("Exif\x00\x00"), tiff...)
		segment := make([]byte, 4)
		segment[0], segment[1] = 0xff, 0xe1
		binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
		data := append([]byte{}, encoded.Bytes()[:2]...)
		data = append(data, segment...)
		data = append(data, payload...)
		data = append(data, encoded.Bytes()[2:]...)
		entry, err := decodeTargetIcon(data, "icon.jpg")
		if err != nil {
			t.Fatal(err)
		}
		w, h := entry.bitmap.Size()
		if orientation >= 5 && (w != 96 || h != 192) || orientation < 5 && (w != 192 || h != 96) {
			t.Fatal("JPEG orientation lost", orientation, w, h)
		}
	}
}
