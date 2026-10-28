package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/egoist/mygo/ui"
)

func TestProviderFileProbeMatchesSourcePrefixRules(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		data []byte
		want providerFileKind
	}{
		{"text", []byte("payload: [example.com]"), providerText},
		{"zstd", []byte{0x28, 0xb5, 0x2f, 0xfd, 0}, providerRuleSet},
		{"binary", []byte{65, 0, 66}, providerBinary},
		{"empty", nil, providerText},
		{"late-nul", append(bytes.Repeat([]byte{'x'}, 512), 0), providerText},
		{"invalid-utf8", []byte{0xff}, providerText},
	} {
		path := filepath.Join(dir, tc.name)
		if err := os.WriteFile(path, tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		if got := probeProviderFile(path); got != tc.want {
			t.Fatal(tc.name, got, tc.want)
		}
	}
	if probeProviderFile("") != providerBinary || probeProviderFile(filepath.Join(dir, "missing")) != providerText || probeProviderFile(dir) != providerBinary {
		t.Fatal("missing path/file and read errors differ from source")
	}
}

func TestCompactProviderCountsMatchOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-compact-counts.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Value int
		Label string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if got := compactNumber(tc.Value); got != tc.Label {
			t.Errorf("%d: %q, original %q", tc.Value, got, tc.Label)
		}
	}
}

func TestProviderDetailsPreviewAndUnsavedEditStayInSheet(t *testing.T) {
	a := populatedApp(t)
	path := filepath.Join(t.TempDir(), "provider.yaml")
	original := []byte("payload: [example.com]\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	p := Provider{Name: "Fixture provider", Type: "Rule", Vehicle: "File", Count: 1234, Path: path}
	a.sample.Providers = []Provider{p}
	a.openProfileDetails()
	a.openProfileProvider(p, false, providerText)
	waitApp(t, a, func() bool { return !a.profileDetail.Busy })
	tt := ui.NewTester(a.View, 1200, 1000)
	if a.profileDetail.Code == nil || !a.profileDetail.Code.ReadOnly || a.profileDetail.Code.Buffer.String() != string(original) {
		t.Fatal("provider tap did not load a read-only preview")
	}
	if err := tt.Click(a.t("back")); err != nil || !a.profileDetail.Open || a.profileDetail.Preview {
		t.Fatal("provider preview left the sheet", err)
	}
	a.openProfileProvider(p, true, providerText)
	waitApp(t, a, func() bool { return !a.profileDetail.Busy })
	a.profileDetail.Code.Buffer.Set("payload: [changed.example]\n")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !a.profileDetail.SavePrompt || !a.profileDetail.Preview {
		t.Fatal("dirty edit closed without a save prompt")
	}
	if err := tt.Click(a.t("cancel")); err != nil || !a.profileDetail.Preview {
		t.Fatal("cancel lost the edited buffer", err)
	}
	tt.Key(0, ui.KeyEscape)
	if err := tt.Click(a.t("no")); err != nil || a.profileDetail.Preview || !a.profileDetail.Open {
		t.Fatal("discard did not return to details", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("discard rewrote provider", err)
	}
	if a.page != "dashboard" {
		t.Fatal("nested provider navigation changed the background page")
	}
}

func TestProviderDetailsListVirtualizesLargeCollections(t *testing.T) {
	a := populatedApp(t)
	for i := range 10000 {
		a.sample.Providers = append(a.sample.Providers, Provider{Name: "Provider " + number(i), Type: "Rule", Count: 1234})
	}
	a.openProfileDetails()
	tt := ui.NewTester(a.View, 1200, 1000)
	if len(a.providerFiles) > 100 {
		t.Fatal("provider list eagerly built hidden rows", len(a.providerFiles))
	}
	if !tt.HasText("1.23K rules") {
		t.Fatal("provider count was not formatted", tt.Texts())
	}
	if tt.HasText("Provider 9999") {
		t.Fatal("hidden provider was built")
	}
}

func providerTestClient(t *testing.T, response func(string, json.RawMessage) any) *Engine {
	t.Helper()
	left, right := net.Pipe()
	c := coreclient.New(left, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = left.Close(); _ = right.Close() })
	go func() {
		for {
			frame, err := coreclient.ReadFrame(right)
			if err != nil {
				return
			}
			var request struct {
				ID, Method string
				Arguments  json.RawMessage
			}
			if json.Unmarshal(frame, &request) != nil {
				return
			}
			result := response(request.Method, request.Arguments)
			data, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if coreclient.WriteFrame(right, data) != nil {
				return
			}
		}
	}()
	return &Engine{client: c, ctx: ctx}
}

func TestProviderSaveKeepsFileOnCoreRejectionAndSavesAfterAcceptance(t *testing.T) {
	for _, reject := range []bool{true, false} {
		t.Run(map[bool]string{true: "reject", false: "accept"}[reject], func(t *testing.T) {
			a := populatedApp(t)
			a.safe = false
			a.cfg.Disclaimer = true
			path := filepath.Join(t.TempDir(), "provider.yaml")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			p := Provider{Name: "Local", Path: path}
			a.engine = providerTestClient(t, func(method string, args json.RawMessage) any {
				var payload map[string]string
				if err := json.Unmarshal(args, &payload); err != nil {
					return "bad payload"
				}
				data, err := base64.StdEncoding.DecodeString(payload["data-base64"])
				if method != "sideLoadExternalProvider" || payload["providerName"] != "Local" || err != nil || string(data) != "changed" {
					return "wrong provider payload"
				}
				if reject {
					return "invalid provider"
				}
				return ""
			})
			a.openProfileDetails()
			a.openProfileProvider(p, true, providerText)
			waitApp(t, a, func() bool { return !a.profileDetail.Busy })
			a.profileDetail.Code.Buffer.Set("changed")
			a.saveProfileProvider()
			waitApp(t, a, func() bool { return !a.profileDetail.Saving })
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if reject {
				if string(data) != "original" || !a.profileDetail.Preview || a.err != "invalid provider" {
					t.Fatal("rejected save lost the original", string(data), a.err)
				}
			} else if string(data) != "changed" || a.profileDetail.Preview || !a.profileDetail.Open {
				t.Fatal("accepted save did not pop the editor", string(data), a.err)
			}
		})
	}
}

func TestProviderUpdateDeduplicatesAndDropsSupersededErrors(t *testing.T) {
	a := populatedApp(t)
	a.safe = false
	release := make(chan struct{})
	started := make(chan struct{})
	a.engine = providerTestClient(t, func(_ string, _ json.RawMessage) any { close(started); <-release; return "old provider error" })
	p := Provider{Name: "Remote", Type: "Proxy"}
	a.updateDetailProvider(p)
	a.updateDetailProvider(p)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider update not dispatched")
	}
	if len(a.providerPending) != 1 {
		t.Fatal("update was not deduplicated")
	}
	a.engine.mu.Lock()
	a.engine.revision++
	a.engine.mu.Unlock()
	close(release)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if a.err != "" {
		t.Fatal("superseded provider error reached current config", a.err)
	}
}
