package app

import (
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestInheritedSettingCanBeOverriddenAndRestored(t *testing.T) {
	a := testApp(t)
	p := store.Profile{ID: store.NewID(), Name: "Inherited"}
	a.cfg.Profiles, a.cfg.Active = []store.Profile{p}, p.ID
	source := []byte("# Keep subscription bytes\nunified-delay: false\nrules: [MATCH,DIRECT]\n")
	if err := store.AtomicWrite(a.store.ProfilePath(p.ID), source); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		a.configRow(c, field("unified-delay", "unifiedDelay", "bool"))
	}, 800, 180)
	if err := tt.Click(a.t("followProfile")); err != nil {
		t.Fatal(err)
	}
	effective, err := a.store.Effective(a.cfg)
	if err != nil || effective["unified-delay"] != true {
		t.Fatal("override did not replace subscription value", effective, err)
	}
	if err := tt.Click(a.t("followProfile")); err != nil {
		t.Fatal(err)
	}
	effective, err = a.store.Effective(a.cfg)
	if err != nil || effective["unified-delay"] != false || a.cfg.Overrides["unified-delay"] != nil {
		t.Fatal("follow profile did not restore the subscription value", effective, err)
	}
}
