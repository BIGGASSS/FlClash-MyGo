package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

const matchTargetSource = `# retained subscription source
proxies:
  - {name: Leaf, type: ss, future-proxy-field: keep}
proxy-groups:
  - {name: Chosen, type: select, proxies: [Leaf]}
  - {name: DIRECT, type: select, proxies: [Leaf]}
rules:
  - MATCH,First
  - DOMAIN,source.test,DIRECT
  - match,Last
  - MATCH,
future-field: {keep: true}
`

func matchTargetApp(t *testing.T) *App {
	t.Helper()
	a := pickerApp(t)
	a.cfg.Profiles[0].MatchTarget = "Retired"
	a.cfg.Profiles[0].Override = "rules: [\"DOMAIN,added.test,MATCH\"]\nfuture-overlay: keep\n"
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(matchTargetSource)); err != nil {
		t.Fatal(err)
	}
	return a
}

func matchTargetReady(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	waitApp(t, a, func() bool { return a.working == 0 })
	tt.Frame()
}

func chooseMatchTarget(t *testing.T, a *App, tt *ui.Tester, path ...string) {
	t.Helper()
	matchTargetReady(t, a, tt)
	if err := tt.Click(a.t("matchTarget")); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(path...); err != nil {
		t.Fatal(err)
	}
}

func assertMatchTargetSource(t *testing.T, a *App, want string) {
	t.Helper()
	b, err := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if err != nil || string(b) != want {
		t.Fatalf("source changed: %v\n%s", err, b)
	}
}

func addedMatchTarget(t *testing.T, a *App) string {
	t.Helper()
	raw, err := effectiveConfig(a.store, a.cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range raw["rules"].([]any) {
		rule := store.String(value)
		if strings.HasPrefix(rule, "DOMAIN,added.test,") {
			return strings.TrimPrefix(rule, "DOMAIN,added.test,")
		}
	}
	t.Fatal("added rule missing")
	return ""
}

func TestProfileMatchTargetsSourceCandidates(t *testing.T) {
	snapshot, err := readProfileMatchTargets(matchTargetSource + "# no source mutation\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.Groups, []profileMatchTargetOption{{"Chosen", "select"}, {"DIRECT", "select"}}) || !reflect.DeepEqual(snapshot.Proxies, []profileMatchTargetOption{{"Leaf", "ss"}}) {
		t.Fatal(snapshot)
	}
	for _, name := range append(slices.Clone(profileMatchTargetBasics), "", "Chosen", "Leaf") {
		if !snapshot.contains(name) {
			t.Fatal("missing candidate", name)
		}
	}
	for _, unknown := range []string{"MATCH", "Retired", "chosen", " Chosen "} {
		if snapshot.contains(unknown) {
			t.Fatal("invented or normalized candidate", unknown)
		}
	}
	duplicates, err := readProfileMatchTargets("proxy-groups: [{name: A, type: select}, {name: B}, {name: A, type: url-test}, {name: ''}, {name: 1}]\n")
	if err != nil || !reflect.DeepEqual(duplicates.Groups, []profileMatchTargetOption{{"A", "url-test"}, {"B", ""}}) {
		t.Fatal("source map ordering/types not retained", duplicates, err)
	}
}

func TestProfileMatchTargetPickerDraftAndChildReturn(t *testing.T) {
	a := matchTargetApp(t)
	initial := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(initial, "profileExtend")
	tt := pickerTester(a)
	matchTargetReady(t, a, tt)
	if !tt.HasText("Retired") {
		t.Fatal("unknown current target was discarded")
	}
	if err := tt.Click(a.t("matchTarget")); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tt.Menu(), a.t("unknown")+": Retired") {
		t.Fatal("unknown target not retained in menu", tt.Menu())
	}
	if err := tt.ChooseMenuItem(a.t("basicStrategy"), "DIRECT"); err == nil {
		t.Fatal("group-shadowed builtin was offered as a basic strategy")
	}
	tt.CloseMenu()
	chooseMatchTarget(t, a, tt, a.t("ruleTarget"), "Chosen · select")
	if a.profileDraft.MatchTarget != "Chosen" || a.cfg.Profiles[0].MatchTarget != "Retired" || addedMatchTarget(t, a) != "Retired" {
		t.Fatal("menu selection escaped its draft")
	}
	assertMatchTargetSource(t, a, matchTargetSource)
	if !a.pickerPageDirty("profileExtend", a.captureNavigationState()) {
		t.Fatal("target-only change was not dirty")
	}
	for _, leave := range []func(){a.backProfilePicker, a.closeDashboardPicker, func() { a.selectNavigation("tools") }} {
		leave()
		if !a.confirmOpen || a.profileDraft.MatchTarget != "Chosen" || !a.picker.Open {
			t.Fatal("target draft bypassed discard confirmation")
		}
		a.confirmOpen = false
	}
	// The existing child editor must round-trip its parent target draft.
	if err := tt.Click(a.t("addedRules")); err != nil {
		t.Fatal(err)
	}
	d := a.document
	d.Value = []any{"DOMAIN,added.test,MATCH", "DOMAIN,second.test,MATCH"}
	a.saveDocument()
	if a.picker.Navigation.Page != "profileExtend" || a.profileDraft.MatchTarget != "Chosen" || a.cfg.Profiles[0].Override != initial.Override {
		t.Fatal("child save committed/lost the parent draft")
	}
	// Save changes only editor-owned fields; a refresh/selection remains newer.
	a.cfg.Profiles[0].Download = 123
	a.cfg.Profiles[0].Selected = map[string]string{"Chosen": "Leaf"}
	a.saveProfile()
	if a.cfg.Profiles[0].MatchTarget != "Chosen" || a.cfg.Profiles[0].Download != 123 || a.cfg.Profiles[0].Selected["Chosen"] != "Leaf" || a.picker.Navigation != nil || !a.picker.Open {
		t.Fatal("parent save lost latest metadata or escaped the picker")
	}
	if addedMatchTarget(t, a) != "Chosen" || !strings.Contains(a.cfg.Profiles[0].Override, "MATCH") || !strings.Contains(a.cfg.Profiles[0].Override, "future-overlay") {
		t.Fatal("save rewrote stored MATCH placeholders or lost overlay fields")
	}
	assertMatchTargetSource(t, a, matchTargetSource)
}

func TestProfileMatchTargetFollowResolvesLatestRuntimeMatch(t *testing.T) {
	a := matchTargetApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	tt := pickerTester(a)
	chooseMatchTarget(t, a, tt, a.t("followProfile"))
	if a.profileDraft.MatchTarget != "" || a.cfg.Profiles[0].MatchTarget != "Retired" || addedMatchTarget(t, a) != "Retired" {
		t.Fatal("Follow rewrote the saved profile before Save")
	}
	a.saveProfile()
	if a.cfg.Profiles[0].MatchTarget != "" || addedMatchTarget(t, a) != "Last" {
		t.Fatal("Follow did not resolve the last nonempty source MATCH")
	}
	refreshed := strings.ReplaceAll(matchTargetSource, "match,Last", "MATCH,Refreshed")
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(refreshed)); err != nil {
		t.Fatal(err)
	}
	if addedMatchTarget(t, a) != "Refreshed" || a.cfg.Profiles[0].MatchTarget != "" {
		t.Fatal("Follow froze the previous subscription target")
	}
	a.cfg.Profiles[0].Script = `const main = c => { c.rules.push("MATCH,Transformed"); return c; };`
	if addedMatchTarget(t, a) != "Transformed" || !strings.Contains(a.cfg.Profiles[0].Override, "MATCH") {
		t.Fatal("Follow bypassed runtime transform or rewrote added rules")
	}
	assertMatchTargetSource(t, a, refreshed)
}

func TestProfileMatchTargetMainTabRetainsDraftAndOtherEditor(t *testing.T) {
	a := matchTargetApp(t)
	a.frameEnabled = true
	a.selectNavigation("profiles")
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	tt := pickerTester(a)
	chooseMatchTarget(t, a, tt, a.t("proxies"), "Leaf · ss")
	a.selectNavigation("tools")
	a.openCode("Other buffer", "library-script:fixture", "old")
	a.code.Buffer.Set("unsaved other tab")
	code := a.code
	a.selectNavigation("profiles")
	tt.Frame()
	matchTargetReady(t, a, tt)
	if a.page != "profileExtend" || a.profileDraft.MatchTarget != "Leaf" || a.cfg.Profiles[0].MatchTarget != "Retired" {
		t.Fatal("tab navigation lost/committed the target draft")
	}
	a.saveProfile()
	if a.cfg.Profiles[0].MatchTarget != "Leaf" || addedMatchTarget(t, a) != "Leaf" {
		t.Fatal("main parent Save did not commit the draft")
	}
	a.selectNavigation("tools")
	if a.code != code || !code.Buffer.Modified() || code.Buffer.String() != "unsaved other tab" {
		t.Fatal("selector consumed another tab's dirty buffer")
	}
	assertMatchTargetSource(t, a, matchTargetSource)
}

func TestProfileMatchTargetStagedCandidatesAndConflict(t *testing.T) {
	a := matchTargetApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	nav := a.profilePickerNavigation()
	original := matchTargetSource
	staged := strings.ReplaceAll(original, "Chosen", "Staged")
	nav.SourceOriginal, nav.Source = &original, &staged
	tt := pickerTester(a)
	chooseMatchTarget(t, a, tt, a.t("ruleTarget"), "Staged · select")
	if a.profileDraft.MatchTarget != "Staged" {
		t.Fatal("candidates ignored the staged source")
	}
	latest := original + "# concurrent refresh\n"
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(latest)); err != nil {
		t.Fatal(err)
	}
	a.saveProfile()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.err == "" || a.cfg.Profiles[0].MatchTarget != "Retired" || a.profileDraft.MatchTarget != "Staged" || nav.Source != &staged || !nav.sourceDirty() {
		t.Fatal("source conflict discarded/committed the target draft", a.err)
	}
	assertMatchTargetSource(t, a, latest)
}

func TestProfileMatchTargetFailedSaveRetainsUnknownAndDraft(t *testing.T) {
	a := matchTargetApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	tt := pickerTester(a)
	chooseMatchTarget(t, a, tt, a.t("basicStrategy"), "REJECT-DROP")
	path := filepath.Join(a.store.Dir, "settings.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	a.saveProfile()
	if a.err == "" || a.cfg.Profiles[0].MatchTarget != "Retired" || a.profileDraft.MatchTarget != "REJECT-DROP" || !a.pickerPageDirty("profileExtend", a.captureNavigationState()) {
		t.Fatal("failed Save lost or committed target draft", a.err)
	}
	assertMatchTargetSource(t, a, matchTargetSource)
}

func TestProfileMatchTargetLateLoadsAreIntentScoped(t *testing.T) {
	for _, sheet := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(map[bool]string{false: "main", true: "sheet"}[sheet]+map[bool]string{false: "/success", true: "/error"}[fail], func(t *testing.T) {
				a := matchTargetApp(t)
				if sheet {
					a.openProfilePicker()
				}
				a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
				state := &profileMatchTargetState{}
				key := a.profileMatchTargetKey()
				release := make(chan struct{})
				a.loadProfileMatchTargets(state, key, func() (string, error) {
					<-release
					if fail {
						return "", errors.New("obsolete source failure")
					}
					return matchTargetSource, nil
				})
				a.back()
				a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
				a.profileDraft.MatchTarget = "New intent"
				close(release)
				waitApp(t, a, func() bool { return a.working == 0 })
				if state.Error != "" || len(state.Snapshot.Groups) != 0 || a.err != "" || a.profileDraft.MatchTarget != "New intent" {
					t.Fatal("superseded read/error reached the replacement intent")
				}
			})
		}
	}
}

func TestProfileMatchTargetRefreshAndRetry(t *testing.T) {
	a := matchTargetApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte("broken: [")); err != nil {
		t.Fatal(err)
	}
	tt := pickerTester(a)
	matchTargetReady(t, a, tt)
	if a.err != "" || a.profileDraft.MatchTarget != "Retired" {
		t.Fatal("candidate read failure escaped the local selector")
	}
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(matchTargetSource)); err != nil {
		t.Fatal(err)
	}
	chooseMatchTarget(t, a, tt, a.t("retry"))
	matchTargetReady(t, a, tt)
	// Subscription refresh invalidates visible candidates even on the same page.
	refreshed := strings.ReplaceAll(matchTargetSource, "Chosen", "New source")
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(refreshed)); err != nil {
		t.Fatal(err)
	}
	a.cfg.Profiles[0].Updated = a.cfg.Profiles[0].Updated.Add(time.Second)
	tt.Frame()
	chooseMatchTarget(t, a, tt, a.t("ruleTarget"), "New source · select")
	if a.profileDraft.MatchTarget != "New source" || a.cfg.Profiles[0].MatchTarget != "Retired" {
		t.Fatal("subscription refresh used stale candidates or committed the draft")
	}
	assertMatchTargetSource(t, a, refreshed)
}

func TestProfileMatchTargetUnknownSurvivesNarrowMenuAndSave(t *testing.T) {
	for _, width := range []int{380, 560, 1200} {
		t.Run(number(width), func(t *testing.T) {
			a := matchTargetApp(t)
			a.frameEnabled = true
			unknown := "  Missing " + strings.Repeat("route ", 25) + "  "
			a.cfg.Profiles[0].MatchTarget = unknown
			a.openProfilePicker()
			a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
			tt := ui.NewTester(a.View, width, 800)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			matchTargetReady(t, a, tt)
			if bounds, ok := tt.Find(unknown); !ok || bounds.X < 0 || bounds.X+bounds.W > float32(width) {
				t.Fatal("long unknown target escaped the narrow row", bounds, ok)
			}
			if err := tt.Click(a.t("matchTarget")); err != nil {
				t.Fatal(err)
			}
			tt.CloseMenu()
			if a.pickerPageDirty("profileExtend", a.captureNavigationState()) {
				t.Fatal("opening a menu dirtied an unknown target")
			}
			a.saveProfile()
			if a.cfg.Profiles[0].MatchTarget != unknown || !strings.Contains(a.cfg.Profiles[0].Override, "MATCH") {
				t.Fatal("unchanged Save normalized/erased unknown target or placeholders")
			}
			assertMatchTargetSource(t, a, matchTargetSource)
		})
	}
}

func TestProfileMatchTargetNewerLoadOwnsCandidates(t *testing.T) {
	a := matchTargetApp(t)
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	state := &profileMatchTargetState{}
	key := a.profileMatchTargetKey()
	release := make(chan struct{})
	a.loadProfileMatchTargets(state, key, func() (string, error) {
		<-release
		return "", errors.New("superseded load")
	})
	a.loadProfileMatchTargets(state, key, func() (string, error) {
		return matchTargetSource, nil
	})
	waitApp(t, a, func() bool { return a.working == 1 && !state.Loading })
	close(release)
	waitApp(t, a, func() bool { return a.working == 0 })
	if state.Error != "" || !state.Snapshot.contains("Chosen") || a.err != "" {
		t.Fatal("an older read replaced a newer snapshot", state.Error)
	}
}

func TestProfileMatchTargetNotInventedForCustom(t *testing.T) {
	a := matchTargetApp(t)
	p := a.cfg.Profiles[0]
	p.Type = "custom"
	a.cfg.Profiles[0] = p
	a.openCustomProfileSheet(p)
	tt := pickerTester(a)
	if tt.HasText(a.t("matchTarget")) || a.working != 0 || a.profileDraft.MatchTarget != "Retired" {
		t.Fatal("custom profile gained an ineffective MatchTarget setting")
	}
}
