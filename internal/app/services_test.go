package app

import (
	"reflect"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestServiceManagementKeepsOneEnabledAndPersistsOrder(t *testing.T) {
	a := testApp(t)
	a.cfg.ServiceOrder = []string{"spotify", "unknown", "spotify"}
	all := a.orderedServiceTargets()
	if len(all) != 13 || all[0].ID != "spotify" || all[1].ID != "google" {
		t.Fatal(all)
	}
	for _, target := range all[1:] {
		a.setServiceEnabled(target.ID, false)
	}
	a.setServiceEnabled("spotify", false)
	if enabled := a.enabledServiceTargets(); len(enabled) != 1 || enabled[0].ID != "spotify" {
		t.Fatal("last enabled service was disabled", enabled)
	}
	a.openServices()
	a.serviceManage = true
	_ = ui.NewTester(a.View, 1200, 800)
	a.serviceList.Reorder([]int{0}, 13)
	_, saved, err := store.Open(a.store.Dir)
	if err != nil || saved.ServiceOrder[12] != "spotify" || len(saved.DisabledServices) != 12 {
		t.Fatal(saved.ServiceOrder, saved.DisabledServices, err)
	}
	data, err := a.store.Backup(a.cfg)
	if err != nil {
		t.Fatal(err)
	}
	target, _, _ := store.Open(t.TempDir())
	restored, err := target.Restore(data)
	if err != nil || !reflect.DeepEqual(saved.ServiceOrder, restored.ServiceOrder) || !reflect.DeepEqual(saved.DisabledServices, restored.DisabledServices) {
		t.Fatal("service preferences lost on backup restore", restored.ServiceOrder, err)
	}
}

func TestServiceResultsCannotOverwriteNewerChecks(t *testing.T) {
	a := testApp(t)
	a.servicePending = map[string]uint64{"google": 2, "github": 1, "youtube": 3}
	a.finishServices([]string{"google", "github"}, 1, []Service{{Name: "google", Status: "blocked"}, {Name: "github", Status: "available", Chains: []string{"Node", "Proxy"}, CoreEpoch: 4, CheckedAt: 10}}, false)
	if a.serviceValue("google") != nil || a.servicePending["google"] != 2 || a.serviceValue("github").Status != "available" {
		t.Fatal("superseded result was accepted", a.services, a.servicePending)
	}
	a.finishServices([]string{"google"}, 2, nil, false)
	if a.serviceValue("google").Status != "failed" || a.servicePending["google"] != 0 {
		t.Fatal("transport failure did not finish its row")
	}
	a.finishServices([]string{"youtube"}, 3, []Service{{Name: "youtube", Status: "available"}}, true)
	if a.serviceValue("youtube") != nil || a.servicePending["youtube"] != 0 {
		t.Fatal("answer from a previous route was accepted")
	}
	a.services = append(a.services, Service{Name: "youtube", Status: "available", CheckedAt: 20})
	a.applyServiceRouteCheck([]string{"github", "youtube"}, []uint64{4, 4}, []int64{10, 19}, []bool{true, false}, 4, 8)
	if a.serviceValue("github").RouteVersion != 8 || a.serviceValue("youtube") == nil {
		t.Fatal("route check overwrote a newer service result")
	}
	a.applyServiceRouteCheck([]string{"github"}, []uint64{4}, []int64{10}, []bool{true}, 5, 1)
	if a.serviceValue("github") != nil {
		t.Fatal("previous-core result survived a core restart")
	}
}

func TestServiceDrawerListsPendingChecksAndManagement(t *testing.T) {
	a := populatedApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	a.openServices()
	tt.Frame()
	for _, label := range []string{"Google", "GitHub", "YouTube", a.t("servicePending"), a.t("serviceManage")} {
		if !tt.HasText(label) {
			// Actions carry accessibility labels, not visible text.
			if _, has := tt.Find(label); !has {
				t.Fatal("missing drawer item", label)
			}
		}
	}
	row, has := tt.Find("YouTube")
	if !has || row.X < 720 || row.X+row.W > 1201 {
		t.Fatal("service sheet did not use the original right-side width", row)
	}
	if err := tt.Click(a.t("serviceManage")); err != nil || !a.serviceManage {
		t.Fatal("could not open service management", err)
	}
	tt.Key(0, ui.KeyEscape)
	if a.serviceOpen || a.engine != nil {
		t.Fatal("escape did not dismiss the safe drawer")
	}
}
