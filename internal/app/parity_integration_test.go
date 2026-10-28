package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestLocalBackupFormatsAreExplicitPrivateAndTransactional(t *testing.T) {
	a := testApp(t)
	id := store.NewID()
	a.cfg.Profiles = []store.Profile{{ID: id, Name: "Fixture", Selected: map[string]string{"Proxy": "Node"}}}
	a.cfg.Active = id
	source := []byte("# source must survive export\nproxies: []\nproxy-groups: []\nrules: [MATCH,DIRECT]\n")
	if err := store.AtomicWrite(a.store.ProfilePath(id), source); err != nil {
		t.Fatal(err)
	}
	for _, flutter := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "backup.zip")
		if err := writeLocalBackup(a.store, store.Clone(a.cfg), path, flutter); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		entries := map[string]bool{}
		for _, file := range z.File {
			entries[file.Name] = true
		}
		if entries["settings.json"] == flutter || entries["config.json"] != flutter || entries["database.sqlite"] != flutter {
			t.Fatal("export silently changed formats", flutter, entries)
		}
		if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatal("credential-bearing backup is not private", info, err)
		}
		target, _, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		restored, err := target.Restore(data)
		if err != nil || len(restored.Profiles) != 1 || restored.Profiles[0].Selected["Proxy"] != "Node" {
			t.Fatal("export did not retain profile choices", err)
		}
		if got, err := os.ReadFile(a.store.ProfilePath(id)); err != nil || !bytes.Equal(got, source) {
			t.Fatal("export rewrote source", err)
		}
		invalid := store.Clone(a.cfg)
		invalid.Profiles[0].Type = "unsupported-native-type"
		if flutter {
			if err := writeLocalBackup(a.store, invalid, path, true); err == nil {
				t.Fatal("unsupported export succeeded")
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
				t.Fatal("failed export replaced a working archive", err)
			}
		}
	}
}

func TestProviderSubscriptionDialogWorksWithoutProfileSheet(t *testing.T) {
	a := populatedApp(t)
	a.page = "resources"
	a.sample.Providers = []Provider{{Name: "Fixture", Type: "Proxy", Subscription: &providerSubscription{Upload: 1 << 30, Download: 2 << 30, Total: 10 << 30}}}
	tt := ui.NewTester(a.View, 1200, 1000)
	if err := tt.Click(a.t("details") + "Fixture"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(a.t("subscriptionInfo")) || !tt.HasText("3GB") || a.profileDetail.Open || a.detailOpen {
		t.Fatal("off-sheet info lost its global subscription dialog")
	}
	if err := tt.Click(a.t("confirm")); err != nil || a.providerState().infoOpen || a.page != "resources" {
		t.Fatal("subscription dismissal lost the underlying page", err)
	}
	a.providerInfo(Provider{Name: "No subscription", Type: "Rule"})
	tt.Frame()
	if !a.detailOpen || !tt.HasText(a.t("details")) {
		t.Fatal("ordinary provider details stopped working")
	}
}

func TestOnDemandNativeFormSafeEditingAndExactIdentities(t *testing.T) {
	a := testApp(t)
	a.page = "onDemand"
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click(a.t("add")); err != nil {
		t.Fatal(err)
	}
	tt.Type(" Home ")
	if err := tt.Click(a.t("confirm")); err != nil || !slices.Equal(a.cfg.ExcludeSSIDs, []string{" Home "}) {
		t.Fatal("SSID input lost focus or normalized the identity", err, a.cfg.ExcludeSSIDs)
	}
	if a.engine != nil || a.onDemand.reader != nil || a.onDemand.cancel != nil {
		t.Fatal("safe settings created an engine or Wi-Fi reader")
	}
	if err := tt.Click(" Home "); err != nil {
		t.Fatal(err)
	}
	a.onDemand.View.Draft = "Renamed"
	tt.Frame()
	if err := tt.Click(a.t("confirm")); err != nil || !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Renamed"}) {
		t.Fatal("SSID rename was not committed", err, a.cfg.ExcludeSSIDs)
	}
	a.editSSID("Renamed", false)
	a.saveSSID()
	if !a.onDemand.View.Open || a.onDemand.View.Error == "" || len(a.cfg.ExcludeSSIDs) != 1 {
		t.Fatal("duplicate SSID was accepted")
	}
	a.onDemand.View.Open = false
	if err := a.setExcludeSSIDs([]string{"Renamed", "Second", "Third"}); err != nil {
		t.Fatal(err)
	}
	a.reorderSSIDs([]int{0, 2}, 3)
	if !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Second", "Renamed", "Third"}) {
		t.Fatal("multi-row reorder lost source order", a.cfg.ExcludeSSIDs)
	}
	a.onDemand.View.Selected = map[string]bool{"Renamed": true, "Third": true}
	a.removeSelectedSSIDs()
	if !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Second"}) {
		t.Fatal("selection delete removed the wrong identities")
	}
}

func TestOnDemandParkingAlsoControlsProbesAndProviderReconcile(t *testing.T) {
	a := onDemandTestApp(t)
	a.onDemand.Known = true
	a.onDemand.State = desktop.WiFiState{Connected: true, SSID: "Home"}
	if !a.wantRunning || a.listenerIntent() || a.probeOwner().proxied {
		t.Fatal("parked listeners were treated as a running outbound proxy")
	}
	if err := a.applyProviderRuleChanges(a.engine, nil); err != nil {
		t.Fatal(err)
	}
	assertOnDemandIntent(t, a, false)
}
