package app

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func populatedApp(t *testing.T) *App {
	a := testApp(t)
	a.status = "running"
	a.wantRunning = true
	a.started = time.Now().Add(-2*time.Hour - 13*time.Minute - 57*time.Second)
	a.ip = "203.0.113.42"
	a.organization = "Example Broadband"
	a.qualities[a.ip] = qualityRecord{Quality: &ipquality.Quality{IP: a.ip, Source: ipquality.IPLocate, Type: "residential", Organization: a.organization, ASN: 64500}, Checked: time.Now()}
	a.country = "US"
	a.intranet = "192.168.1.10"
	a.cfg.SystemProxy = true
	a.cfg.Profiles = []store.Profile{{ID: store.NewID(), Name: "Example", Unfold: []string{"Proxy", "Auto", "Streaming"}, URL: "https://example.com/subscription", Upload: 1 << 30, Download: 41 << 30, Total: 100 << 30, Updated: time.Now(), Expire: time.Now().Add(30 * 24 * time.Hour).Unix()}}
	a.cfg.Active = a.cfg.Profiles[0].ID
	a.sample.Traffic = Traffic{Up: 51814, Down: 2202009}
	a.sample.Total = Traffic{Up: 1288490189, Down: 9234179686}
	a.peak = Traffic{Up: 75264, Down: 2306867}
	a.sample.Memory = Memory{RSS: 48444211, Heap: 35 << 20, Idle: 8 << 20, Stack: 2 << 20, Other: 1 << 20}
	a.sample.AppRSS = 191574835
	for i := range 30 {
		a.upHistory = append(a.upHistory, int64(35e3+15e3*math.Sin(float64(i)/2.1)+8e3*math.Sin(float64(i)/1.2)))
		a.downHistory = append(a.downHistory, int64(1.3e6+0.6e6*math.Sin(float64(i)/2.8)+0.3e6*math.Cos(float64(i)/1.4)))
	}
	a.sample.Proxies = ProxyData{All: []string{"Proxy", "Auto", "Streaming"}, Proxies: map[string]ProxyNode{}}
	nodes := []string{"United States 01", "Hong Kong 01", "Singapore 01", "Japan 01", "United Kingdom 01", "Taiwan 01"}
	for i, name := range nodes {
		a.sample.Proxies.Proxies[name] = ProxyNode{Name: name, Type: "Shadowsocks"}
		a.delays[name] = 38 + i*17
	}
	for i, name := range a.sample.Proxies.All {
		a.sample.Proxies.Proxies[name] = ProxyNode{Name: name, Type: []string{"Selector", "URLTest", "Selector"}[i], All: nodes, Now: nodes[i]}
	}
	a.group = "Proxy"
	a.services = []Service{{Name: "google", Status: "available", Delay: 38}, {Name: "github", Status: "available", Delay: 64}}
	for i, process := range []string{"Safari", "Google Chrome", "Telegram", "Spotify", "Slack"} {
		host := []string{"www.youtube.com", "github.com", "149.154.167.51", "audio-fa.scdn.co", "wss-primary.slack.com"}[i]
		a.sample.Connections.Connections = append(a.sample.Connections.Connections, Connection{ID: store.NewID(), Metadata: Metadata{Host: host, DestinationPort: "443", SourceIP: "127.0.0.1", Process: process, Network: "tcp"}, Chains: []string{nodes[i%len(nodes)], "Proxy"}, Rule: "DOMAIN-SUFFIX", RulePayload: host, Download: int64(i+1) * 8 << 20, Upload: int64(i+1) * 92 << 10, Start: time.Now().Add(-time.Duration(i) * time.Minute)})
	}
	a.requests = append(a.requests, a.sample.Connections.Connections...)
	a.dns = []DNSQuery{{Domain: "github.com", Type: "A", Upstream: "https://dns.alidns.com/dns-query", Answers: []string{"140.82.112.3"}, Delay: 24, Time: time.Now()}, {Domain: "www.youtube.com", Type: "A", Cached: true, Answers: []string{"198.18.0.2"}, Delay: 1, Time: time.Now()}}
	a.logs = []LogEntry{{Type: "info", Payload: "[TCP] Safari → www.youtube.com:443 match DOMAIN-SUFFIX using Proxy[United States 01]", Time: time.Now()}, {Type: "info", Payload: "Initial configuration complete", Time: time.Now().Add(-time.Minute)}}
	return a
}
func TestPopulatedNativeScreens(t *testing.T) {
	dir := os.Getenv("FLCLASH_SNAPSHOT_DIR")
	for _, page := range []string{"dashboard", "dashboard-short", "dashboard-two", "dashboard-standard", "dashboard-medium", "dashboard-narrow", "dashboard-narrow-edit", "dashboard-narrow-custom", "dashboard-narrow-scaled-up", "dashboard-narrow-scaled-down", "sidebar-collapsed", "dashboard-scaled-up", "dashboard-scaled-down", "proxies-scaled-up", "proxies-scaled-down", "proxies-icons", "proxies-pending", "proxies", "profiles", "connections", "requests", "dns", "logs", "tools", "general", "network", "theme", "hotkeys", "services-sheet", "services-manage", "quality-sheet", "memory-sheet", "memory-stopped", "profile-details", "profile-details-subscription", "profile-details-providers"} {
		for _, dark := range []bool{false, true} {
			t.Run(page+map[bool]string{true: "-dark", false: "-light"}[dark], func(t *testing.T) {
				a := populatedApp(t)
				a.page = page
				width, height := 1200, 800
				switch page {
				case "dashboard-scaled-up", "dashboard-scaled-down", "proxies-scaled-up", "proxies-scaled-down":
					a.page = "dashboard"
					if page[:7] == "proxies" {
						a.page = "proxies"
					}
					a.cfg.Scale = 1.4
					if page[len(page)-4:] == "down" {
						a.cfg.Scale = 0.8
					}
				case "proxies-pending":
					a.page = "proxies"
					a.delayGroups = map[string]uint64{"Proxy": 1}
					a.delayPending = map[string]delayPending{}
					names := a.sample.Proxies.Proxies["Proxy"].All
					for i, name := range names {
						runs := 0
						if i < 2 {
							runs = 1
						}
						a.delayPending[delayKey(name, a.cfg.TestURL)] = delayPending{holds: 1, runs: runs}
					}
				case "proxies-icons":
					a.page = "proxies"
					data, err := os.ReadFile("testdata/proxy-icons.json")
					if err != nil {
						t.Fatal(err)
					}
					var icons []string
					if err := json.Unmarshal(data, &icons); err != nil {
						t.Fatal(err)
					}
					for i, name := range a.sample.Proxies.All {
						g := a.sample.Proxies.Proxies[name]
						g.Icon = icons[i]
						a.sample.Proxies.Proxies[name] = g
					}
				case "dashboard-short":
					a.page, height = "dashboard", 640
				case "dashboard-two":
					a.page, width = "dashboard", 1000
				case "sidebar-collapsed":
					a.page, a.cfg.Sidebar = "dashboard", false
				case "dashboard-narrow", "dashboard-narrow-edit", "dashboard-narrow-custom", "dashboard-narrow-scaled-up", "dashboard-narrow-scaled-down":
					a.page, width = "dashboard", 820
					a.engine = &Engine{}
					a.cfg.Profiles[0].Total, a.cfg.Profiles[0].Expire = 0, 0
					a.cfg.Profiles[0].Updated = time.Time{}
					if page == "dashboard-narrow-scaled-up" {
						a.cfg.Scale = 1.4
					} else if page == "dashboard-narrow-scaled-down" {
						a.cfg.Scale = 0.8
					}
					a.sample.Traffic = Traffic{Up: a.upHistory[len(a.upHistory)-1], Down: a.downHistory[len(a.downHistory)-1]}
					a.dashboardEditing = page == "dashboard-narrow-edit"
					if page == "dashboard-narrow-custom" {
						a.cfg.DashboardWidgets = []string{"currentOutbound", "memoryInfo", "profiles"}
					}
				case "dashboard-medium":
					a.page, width, height = "dashboard", 1440, 860
				case "dashboard-standard":
					a.page, width, height = "dashboard", 1440, 1000
				case "services-sheet", "services-manage":
					a.page = "dashboard"
					a.openServices()
					a.serviceManage = page == "services-manage"
				case "profile-details", "profile-details-subscription", "profile-details-providers":
					a.page, a.profileDetail.Open = "dashboard", true
					a.engine = &Engine{}
					a.cfg.Profiles[0].Updated = time.Time{}
					if page != "profile-details-subscription" {
						a.cfg.Profiles[0].Total, a.cfg.Profiles[0].Expire = 0, 0
					} else {
						a.cfg.Profiles[0].Expire = 1893456000
					}
					if page == "profile-details-providers" {
						a.sample.Providers = []Provider{
							{Name: "Remote nodes", Type: "Proxy", Vehicle: "HTTP", Count: 1234},
							{Name: "Domain rules", Type: "Rule", Vehicle: "File", Count: 123456},
							{Name: "Subscription nodes", Type: "Proxy", Vehicle: "HTTP", Count: 1234, Subscription: &providerSubscription{Upload: 1 << 30, Download: 41 << 30, Total: 100 << 30, Expire: 1893456000}},
						}
					}
				case "memory-sheet", "memory-stopped":
					a.page, a.memoryOpen = "dashboard", true
					if page == "memory-stopped" {
						a.status = "stopped"
					}
				case "quality-sheet":
					a.page = "dashboard"
					a.showQuality(a.ip)
				}
				if dark {
					a.cfg.Theme = "dark"
				}
				tt := ui.NewTester(a.View, width, height)
				tt.Advance(time.Second)
				if len(tt.Texts()) == 0 {
					t.Fatal("empty populated screen")
				}
				if dir != "" {
					if e := os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					name := page + map[bool]string{true: "-dark", false: "-light"}[dark] + ".png"
					f, e := os.Create(filepath.Join(dir, name))
					if e != nil {
						t.Fatal(e)
					}
					e = png.Encode(f, tt.Image())
					f.Close()
					if e != nil {
						t.Fatal(e)
					}
				}
			})
		}
	}
}
