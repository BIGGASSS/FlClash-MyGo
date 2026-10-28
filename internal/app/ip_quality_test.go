package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/egoist/mygo/ui"
)

func TestIPQualityDialogAndPrivacy(t *testing.T) {
	a := populatedApp(t)
	a.page = "dashboard"
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click("203.0.113.42"); err != nil {
		t.Fatal(err)
	}
	if !a.qualityOpen || !tt.HasText("Residential") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click(a.t("ipAddress")); err != nil || tt.Clipboard() != a.ip {
		t.Fatal("IP row did not copy its address", tt.Clipboard(), err)
	}
	if row, has := tt.Find(a.t("ipAddress")); !has || row.X < 720 || row.X+row.W > 1201 {
		t.Fatal("quality details did not open in a side sheet", row)
	}
	a.cfg.HideIP = true
	tt.Frame()
	if tt.HasText("203.0.113.42") || !tt.HasText("*****") {
		t.Fatal("hidden IP leaked", tt.Texts())
	}
	tt.SetClipboard("unchanged")
	if err := tt.Click(a.t("ipAddress")); err != nil || tt.Clipboard() != "unchanged" {
		t.Fatal("hidden IP was copied", tt.Clipboard(), err)
	}
	if a.engine != nil {
		t.Fatal("safe preview started core")
	}
}

func TestIPQualitySheetLoadingAndFailures(t *testing.T) {
	a := populatedApp(t)
	delete(a.qualities, a.ip)
	a.qualityOpen, a.qualityIP = true, a.ip
	a.qualityBusy, a.qualityRequest = true, a.ip
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText(a.t("network")) || tt.HasText(a.t("ipQualityFailed")) {
		t.Fatal("loading layout omitted its eventual rows", tt.Texts())
	}
	a.qualityBusy = false
	a.qualities[a.ip] = qualityRecord{Checked: time.Now(), Failures: []ipquality.Failure{{Source: ipquality.IdentMe, Status: "rateLimited"}}}
	tt.Frame()
	if !tt.HasText(a.t("ipQualityFailed")) || !tt.HasText(a.t("ipSourceRateLimited")) || tt.HasText(a.t("ipAsn")) {
		t.Fatal("failure state did not match its available details", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.qualityOpen {
		t.Fatal("escape did not close quality details")
	}
}
func TestMalformedOptionalIPCache(t *testing.T) {
	for _, data := range []string{"null", "{", `{"203.0.113.9":{"quality":{"ip":"203.0.113.9","type":"hosting","source":999}}}`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ip-quality.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		a, err := New(dir, "", true)
		if err != nil {
			t.Fatal(err)
		}
		if a.qualities == nil || len(a.qualities) != 0 {
			t.Fatal("invalid cache retained", a.qualities)
		}
	}
}
