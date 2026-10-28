package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMemoryDetailsUseCurrentCoreStateAndDismiss(t *testing.T) {
	a := populatedApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	label := strings.ReplaceAll(a.t("details"), "{label}", a.t("memoryInfo"))
	if err := tt.Click(label); err != nil || !a.memoryOpen {
		t.Fatal("memory details did not open", err)
	}
	for _, key := range []string{"memoryEstimateDesc", "memoryAppResident", "memoryCoreHeapInuse", "memoryCoreHeapIdle", "memoryCoreStack", "memoryCoreRuntime", "other"} {
		if !tt.HasText(a.t(key)) {
			t.Fatal("missing memory detail", key, tt.Texts())
		}
	}
	if !tt.HasText("100%") {
		t.Fatal("resident memory is not the full app share")
	}
	if err := tt.Click(a.t("releaseMemory")); err != nil {
		t.Fatal(err)
	}
	if a.memoryReleasing || a.engine != nil {
		t.Fatal("safe mode started a release or core")
	}
	a.status = "stopped"
	tt.Frame()
	if !tt.HasText(a.t("memoryCoreNotRunning")) || tt.HasText(a.t("memoryCoreHeapInuse")) {
		t.Fatal("stopped core retained live memory breakdown")
	}
	if _, core := a.memoryTotals(); core != 0 {
		t.Fatal("stopped core retained previous RSS", core)
	}
	if err := tt.Click(a.t("close")); err != nil || a.memoryOpen {
		t.Fatal("memory sheet did not dismiss", err)
	}
}

func TestMemoryDetailsHandleEmptyAndRuntimeGreaterThanRSS(t *testing.T) {
	for _, size := range [][2]int{{800, 600}, {1200, 800}} {
		a := testApp(t)
		a.memoryOpen = true
		tt := ui.NewTester(a.View, size[0], size[1])
		if !tt.HasText("0%") || !tt.HasText(a.t("memoryCoreNotRunning")) {
			t.Fatal(tt.Texts())
		}
		a.status = "running"
		a.sample.Memory = Memory{RSS: 1 << 20, Heap: 2 << 20}
		tt.Frame()
		if tt.HasText(a.t("other")) || !tt.HasText("200%") {
			t.Fatal("runtime accounting underflowed", tt.Texts())
		}
	}
}
