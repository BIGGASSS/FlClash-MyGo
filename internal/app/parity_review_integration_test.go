package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestOnDemandReviewLocalizedValidationAtMinimumSize(t *testing.T) {
	for _, locale := range []string{"en", "es", "fr", "ja", "ko", "ru", "zh_CN"} {
		for _, scale := range []float32{1, 1.4} {
			t.Run(fmt.Sprintf("%s/%g", locale, scale), func(t *testing.T) {
				a := testApp(t)
				a.frameEnabled, a.framePlatform = true, "linux"
				a.cfg.Locale, a.cfg.Scale = locale, scale
				a.page = "onDemand"
				tt := ui.NewTester(a.View, 380, 400)
				if err := tt.Click(a.t("add")); err != nil || !a.onDemand.View.Open {
					t.Fatal("minimum-size add action is inaccessible", err)
				}
				for _, draft := range []string{"", strings.Repeat("x", 33)} {
					a.onDemand.View.Draft = draft
					a.saveSSID()
					tt.Frame()
					key, args := "emptyTip", map[string]any{"label": "SSID"}
					if draft != "" {
						key, args = "maxLengthTip", map[string]any{"label": "SSID", "max": 32}
					}
					if want := a.messageText(key, args); a.onDemand.View.Error != want {
						t.Errorf("validation bypassed the locale: %q, want %q", a.onDemand.View.Error, want)
					}
					for _, label := range []string{"SSID", a.t("cancel"), a.t("confirm")} {
						r, ok := tt.Find(label)
						if !ok || r.X < 0 || r.Y < a.frameHeight || r.X+r.W > 380.5 || r.Y+r.H > 400.5 {
							t.Errorf("dialog control %q outside minimum window: %+v", label, r)
						}
					}
				}
				if err := tt.Click(a.t("cancel")); err != nil || a.onDemand.View.Open || len(a.cfg.ExcludeSSIDs) != 0 {
					t.Fatal("cancel committed or retained the draft", err)
				}
				if err := tt.Click(a.t("add")); err != nil {
					t.Fatal(err)
				}
				tt.Type("unsaved identity")
				tt.Key(0, ui.KeyEscape)
				if a.onDemand.View.Open || len(a.cfg.ExcludeSSIDs) != 0 {
					t.Fatal("Escape persisted a dirty SSID draft")
				}
				a.editSSID(strings.Repeat("👨‍👩‍👧‍👦", 32), false)
				a.saveSSID()
				if a.onDemand.View.Open || len(a.cfg.ExcludeSSIDs) != 1 {
					t.Fatal("valid extended grapheme clusters were counted as code points")
				}
				a.editSSID(a.cfg.ExcludeSSIDs[0], false)
				a.saveSSID()
				if !a.onDemand.View.Open || a.onDemand.View.Error != a.messageText("existsTip", map[string]any{"label": "SSID"}) {
					t.Fatal("duplicate validation bypassed the locale")
				}
			})
		}
	}
}

func TestOnDemandReviewStatusDoesNotHideMinimumSizeList(t *testing.T) {
	for _, locale := range []string{"en", "fr", "ru", "zh_CN"} {
		t.Run(locale, func(t *testing.T) {
			a := testApp(t)
			a.frameEnabled, a.framePlatform, a.page = true, "linux", "onDemand"
			a.cfg.Locale, a.cfg.Scale = locale, 1.4
			a.cfg.ExcludeSSIDs = []string{"Excluded network"}
			a.onDemand.Known = true
			a.onDemand.State = desktop.WiFiState{Connected: true, SSID: "Current network"}
			a.onDemand.Error = "Wi-Fi permission unavailable"
			tt := ui.NewTester(a.View, 380, 400)
			if err := tt.Click(a.t("add")); err != nil || !a.onDemand.View.Open {
				t.Fatal("status panels hid the add action", err)
			}
			tt.Key(0, ui.KeyEscape)
			tt.Scroll(350, 330, 0, 500)
			if err := tt.Click("Excluded network"); err != nil || !a.onDemand.View.Open || a.onDemand.View.Original != "Excluded network" {
				t.Fatal("status panels hid the editable list", err, tt.Texts())
			}
		})
	}
}

func TestOnDemandReviewFailedWritesRetainDraftSelectionAndOrder(t *testing.T) {
	a := testApp(t)
	a.cfg.ExcludeSSIDs = []string{"Home", "Office", "Third"}
	if err := a.store.Save(a.cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(a.store.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Fail persistence deterministically, including when tests run as root.
	dir := a.store.Dir
	a.store.Dir = filepath.Join(dir, "missing", "directory")
	a.editSSID("Home", true)
	a.onDemand.View.Draft = " Home edited "
	a.saveSSID()
	if !a.onDemand.View.Open || a.onDemand.View.Error == "" || a.onDemand.View.Draft != " Home edited " {
		t.Fatal("failed rename lost its editable draft")
	}
	a.onDemand.View.Selected = map[string]bool{"Home": true, "Third": true}
	a.removeSelectedSSIDs()
	if a.err == "" || len(a.onDemand.View.Selected) != 2 {
		t.Fatal("failed deletion cleared its selection or error")
	}
	a.err = ""
	a.reorderSSIDs([]int{0, 2}, 3)
	if a.err == "" || !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Home", "Office", "Third"}) {
		t.Fatal("failed reorder changed the working order")
	}
	if after, err := os.ReadFile(filepath.Join(dir, "settings.json")); err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed edits replaced saved settings", err)
	}
	a.store.Dir = dir
	a.err = ""
	a.saveSSID()
	if a.onDemand.View.Open || !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Office", "Third", " Home edited "}) {
		t.Fatal("retry did not preserve the rename and source append order")
	}
	a.editSSID("Office", true)
	a.cfg.ExcludeSSIDs = []string{"Third", " Home edited "} // An intervening restore removed the edited identity.
	a.onDemand.View.Draft = "Obsolete"
	a.saveSSID()
	if !a.onDemand.View.Open || a.onDemand.View.Error != a.t("editorContextChanged") || slices.Contains(a.cfg.ExcludeSSIDs, "Obsolete") {
		t.Fatal("stale edit resurrected an identity removed by newer settings")
	}
}

func TestOnDemandReviewNativeSelectionAndDeletion(t *testing.T) {
	a := testApp(t)
	a.frameEnabled, a.page = true, "onDemand"
	a.cfg.ExcludeSSIDs = []string{"First", "Second", "Third"}
	tt := ui.NewTester(a.View, 380, 400)
	if err := tt.RightClick("Second"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(a.t("select")); err != nil || !a.onDemand.View.Selected["Second"] || a.onDemand.View.Open {
		t.Fatal("context selection opened an editor or chose the wrong identity", err)
	}
	if err := tt.Click(a.t("selectAll")); err != nil || len(a.onDemand.View.Selected) != 3 {
		t.Fatal("select-all is unavailable at the desktop minimum", err)
	}
	if err := tt.Click("Second"); err != nil || a.onDemand.View.Selected["Second"] {
		t.Fatal("selection click failed to toggle an identity", err)
	}
	if err := tt.Click(a.t("delete")); err != nil || !slices.Equal(a.cfg.ExcludeSSIDs, []string{"Second"}) || len(a.onDemand.View.Selected) != 0 {
		t.Fatal("native delete removed an unselected identity", err, a.cfg.ExcludeSSIDs)
	}
}

func TestOnDemandReviewNativeReorderAndViewport(t *testing.T) {
	a := testApp(t)
	a.frameEnabled, a.page = true, "onDemand"
	for i := range 30 {
		a.cfg.ExcludeSSIDs = append(a.cfg.ExcludeSSIDs, fmt.Sprintf("Network %02d", i))
	}
	tt := ui.NewTester(a.View, 1200, 800)
	first, firstOK := tt.Find("Network 00")
	third, thirdOK := tt.Find("Network 02")
	eighth, eighthOK := tt.Find("Network 07")
	if !firstOK || !thirdOK || !eighthOK || eighth.Y+eighth.H > 800 {
		t.Fatal("scroll wrapper stopped the list filling a wide window", first, third, eighth)
	}
	x, y := first.X+first.W/2, first.Y+first.H/2
	tt.Press(x, y)
	tt.Move(x, third.Y+third.H-2)
	tt.Release(x, third.Y+third.H-2)
	if !slices.Equal(a.cfg.ExcludeSSIDs[:3], []string{"Network 01", "Network 02", "Network 00"}) || a.onDemand.View.Open {
		t.Fatal("native reorder did not commit or opened an editor", a.cfg.ExcludeSSIDs[:3])
	}
}

func TestBackupReviewExplicitActionsAndPrivateReplacement(t *testing.T) {
	a := testApp(t)
	a.frameEnabled = true
	a.page = "backup"
	tt := ui.NewTester(a.View, 380, 400)
	for _, label := range []string{a.t("backup"), a.t("backup") + " · FlClash (Flutter)", a.t("restore")} {
		if !tt.HasText(label) {
			t.Fatal("local backup format or restore action missing", label)
		}
	}
	for _, flutter := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "backup.zip")
		if err := os.WriteFile(path, []byte("previous archive"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeLocalBackup(a.store, store.Clone(a.cfg), path, flutter); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatal("replacing an export retained public permissions", info, err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		invalid := store.Clone(a.cfg)
		invalid.Profiles = []store.Profile{{ID: store.NewID(), Name: "Missing source"}}
		if err := writeLocalBackup(a.store, invalid, path, flutter); err == nil {
			t.Fatal("backup unexpectedly accepted a missing profile source")
		}
		if after, err := os.ReadFile(path); err != nil || !bytes.Equal(before, after) {
			t.Fatal("failed serialization replaced the existing archive", err)
		}
	}
}

func TestBackupReviewWorkerErrorIsAccessibleAtMinimumSize(t *testing.T) {
	a := testApp(t)
	a.frameEnabled, a.framePlatform, a.page = true, "linux", "backup"
	cfg := store.Clone(a.cfg)
	cfg.Profiles = []store.Profile{{ID: store.NewID(), Name: "Missing source"}}
	path := filepath.Join(t.TempDir(), "backup.zip")
	// Exercise the same worker/error presentation used by the save-dialog flow,
	// without opening a native OS dialog in a headless test.
	a.work(func() (func(), error) { return nil, writeLocalBackup(a.store, cfg, path, true) })
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.err == "" {
		t.Fatal("backup worker swallowed the serialization failure")
	}
	tt := ui.NewTester(a.View, 380, 400)
	for _, label := range []string{a.t("error"), a.t("confirm")} {
		r, ok := tt.Find(label)
		if !ok || r.X < 0 || r.Y < a.frameHeight || r.X+r.W > 380.5 || r.Y+r.H > 400.5 {
			t.Errorf("backup error control %q is clipped: %+v", label, r)
		}
	}
	if err := tt.Click(a.t("confirm")); err != nil || a.err != "" || a.page != "backup" {
		t.Fatal("backup error cannot be dismissed without leaving settings", err)
	}
}

func TestProviderReviewGlobalDialogProtectsDirtySheetAndErrors(t *testing.T) {
	for _, size := range [][2]int{{1200, 800}, {380, 400}} {
		a := populatedApp(t)
		a.frameEnabled = true
		path := filepath.Join(t.TempDir(), "provider.yaml")
		if err := store.AtomicWrite(path, []byte("original")); err != nil {
			t.Fatal(err)
		}
		a.openProfileDetails()
		a.openProfileProvider(Provider{Name: "Dirty provider", Path: path}, true, providerText)
		waitApp(t, a, func() bool { return !a.profileDetail.Busy })
		a.profileDetail.Code.Buffer.Set("edited")
		a.showProviderSubscription(Provider{Subscription: &providerSubscription{Total: 1 << 30}})
		tt := ui.NewTester(a.View, size[0], size[1])
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		a.err = "Export failed"
		tt.Frame()
		tt.Key(0, ui.KeyEscape)
		if a.err != "" || !a.providerState().infoOpen || !a.profileDetail.Open || a.profileDetail.SavePrompt {
			t.Fatal("error dismissal reached the subscription or dirty sheet", size)
		}
		tt.Key(0, ui.KeyEscape)
		if a.providerState().infoOpen || !a.profileDetail.Open || a.profileDetail.SavePrompt || a.profileDetail.Code.Buffer.String() != "edited" {
			t.Fatal("subscription Escape reached or discarded the dirty editor", size)
		}
	}
}

func TestIntegrationReviewParkedServiceProbesUseDirectRouting(t *testing.T) {
	a := testApp(t)
	a.safe, a.wantRunning, a.page = false, true, "tools"
	a.cfg.ExcludeSSIDs = []string{"Home"}
	a.onDemand.Known, a.onDemand.State = true, desktop.WiFiState{Connected: true, SSID: "Home"}
	requests := make(chan string, 4)
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		var request struct {
			Proxy string `json:"proxy-name"`
		}
		_ = json.Unmarshal(args, &request)
		requests <- method + ":" + request.Proxy
		if method == "outboundIp" {
			return outboundResult{Error: "fixture failure"}
		}
		return []Service{}
	})
	for _, parked := range []bool{true, false} {
		if !parked {
			a.onDemand.State.SSID = "Other"
		}
		a.refreshServices(serviceTargets[:1])
		a.requestOutbound("", true)
		waitApp(t, a, func() bool { return a.working == 0 })
		want := ""
		if parked {
			want = "DIRECT"
		}
		var got []string
		for range 2 {
			select {
			case request := <-requests:
				got = append(got, request)
			default:
				t.Fatal("both probes were not dispatched", got)
			}
		}
		if !slices.Contains(got, "outboundIp:"+want) || !slices.Contains(got, "serviceCheck:"+want) || !a.wantRunning {
			t.Fatal("probe routing ignored listener parking or changed user run intent", parked, got)
		}
	}
}

func TestIntegrationReviewProviderShapeReconcileRetainsParking(t *testing.T) {
	a := onDemandTestApp(t) // Engine loop is deliberately not started.
	a.onDemand.Known, a.onDemand.State = true, desktop.WiFiState{Connected: true, SSID: "Home"}
	profileID, resourceID := store.NewID(), store.NewID()
	a.cfg.Active, a.cfg.Profiles = profileID, []store.Profile{{ID: profileID, Name: "Fixture", Override: "# retained\nunknown: value\n"}}
	prepared := &preparedRuleSync{resource: store.RuleResource{URL: "https://never-fetch.invalid/rules"}, id: resourceID, sourceID: resourceID, behavior: "classical", format: "yaml", shapeChanged: true}
	if err := a.applyProviderRuleChanges(a.engine, []providerRuleChange{{name: "Library", prepared: prepared}}); err != nil {
		t.Fatal(err)
	}
	assertOnDemandIntent(t, a, false)
	if !a.wantRunning || !prepared.keep || !slices.Contains(a.engine.desired.Resources, resourceID) || !strings.Contains(a.cfg.Profiles[0].Override, "# retained") {
		t.Fatal("provider shape reconciliation lost parking, resources or source metadata")
	}
}

func TestIntegrationReviewSafeModeDoesNotDispatchProvidersOrProbes(t *testing.T) {
	a := populatedApp(t)
	var calls atomic.Int32
	a.engine = providerTestClient(t, func(string, json.RawMessage) any { calls.Add(1); return nil })
	a.onDemand.reader = onDemandReadFunc(func(_ context.Context) (desktop.WiFiState, error) {
		calls.Add(1)
		return desktop.WiFiState{}, nil
	})
	a.cfg.ExcludeSSIDs = []string{"Home"}
	a.onDemand.Known, a.onDemand.State = true, desktop.WiFiState{Connected: true, SSID: "Home"}
	a.serviceOpen = true
	a.syncOnDemand()
	a.updateProviderBatch([]Provider{{Name: "Remote", Type: "Rule", Vehicle: "HTTP", Path: "https://must-not-fetch.invalid/rules"}})
	a.refreshServices(serviceTargets)
	a.requestOutbound("", true)
	a.lookupQuality("203.0.113.1", true)
	a.verifyServiceRoutes()
	ui.NewTester(a.View, 380, 400).Frame()
	if calls.Load() != 0 || a.working != 0 || a.onDemand.cancel != nil || a.listenerIntent() {
		t.Fatal("safe integration started a reader, worker or IPC/network probe", calls.Load(), a.working)
	}
}
