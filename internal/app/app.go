package app

import (
	"encoding/json"
	"image/png"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/BIGGASSS/FlClash-MyGo/internal/iconcache"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const Version = "0.1.0-dev"

type App struct {
	codeProvider            *providerEditorContext
	editorLoadSerial        uint64
	store                   *store.Store
	cfg                     store.Settings
	engine                  *Engine
	corePath                string
	safe                    bool
	window                  *mygo.Window
	updateMu                sync.Mutex
	updates                 []func()
	tray                    *mygo.Tray
	trayMenu                *mygo.Menu
	traySignature           string
	quitting                bool
	status                  string
	corePhase               string
	coreHoldUntil           time.Time
	err                     string
	message                 string
	working                 int
	wantRunning             bool
	onDemand                onDemandState
	fontPicker              *fontPickerState
	started                 time.Time
	sidebarWidth            float32
	sidebarReady            bool
	sidebarSelection        sidebarSelection
	sidebarOverlaySelection sidebarSelection
	sidebarOverlay          sidebarOverlayState
	frameEnabled            bool
	framePlatform           string
	frameHeight             float32
	caption                 windowCaptionState
	captionHost             windowCaptionHost
	page                    string
	history                 []string
	activeNavigation        string
	navigationState         map[string]navigationPageState
	navigationDock          navigationDockState
	catalog                 map[string]string
	english                 map[string]string
	locale                  string
	colors                  palette
	logo                    *ui.Bitmap
	iconCache               *iconcache.Cache
	targetIcons             map[string]*targetIcon
	iconUse                 uint64
	iconFrame               uint64
	iconMounts              map[uint64]iconMount
	sample                  Sample
	logs                    []LogEntry
	dns                     []DNSQuery
	requests                []Connection
	delayJobs               map[uint64]*proxyDelayJob
	delayGroups             map[string]uint64
	delayPending            map[string]delayPending
	delaySerial             uint64
	delayPool               *proxyDelayPool
	delays                  map[string]int
	upHistory               []int64
	downHistory             []int64
	peak                    Traffic
	services                []Service
	servicePending          map[string]uint64
	serviceLast             map[string]time.Time
	serviceSerial           uint64
	serviceRouteDue         time.Time
	serviceRouteRun         bool
	serviceRouteRev         uint64
	memoryOpen              bool
	memoryReleasing         bool
	memoryPollAt            time.Time
	memoryPolling           bool
	serviceOpen             bool
	serviceManage           bool
	serviceList             ui.ListState
	serviceHostSet          bool
	serviceProxied          bool
	outbound                *outboundProbeState
	ip                      string
	organization            string
	country                 string
	intranet                string
	group                   string
	proxyPageState
	proxyFilter        string
	filterState        *proxyFilterState
	proxyExpanded      map[string]bool
	delaySnapshot      map[string]int
	recordQuery        string
	recordFilter       string
	recordsTable       bool
	paused             bool
	dashboardEditing   bool
	dashboardList      ui.ListState
	connectionTab      int
	recordState        ui.ListState
	recordSelected     int
	recordSort         string
	frozen             *frozenRecords
	detailOpen         bool
	detail             string
	addOpen            bool
	addKind            int
	addName            string
	addSource          string
	addSecret          string
	editProfile        string
	profileTab         int
	profileDraft       store.Profile
	profileDetail      profileDetailState
	profileGrid        profileGridState
	profileShare       profileShareState
	profileGroupCounts map[string]*profileGroupCount
	picker             dashboardPickerState
	compactSheet       bool
	sheetContentHeight float32
	snapSheets         map[string]*snapSheetState
	profileUpdating    map[string]bool
	providerFiles      map[string]*providerFileProbe
	providerPending    map[string]uint64
	providerSerial     uint64
	code               *editor.State
	codePurpose        string
	codeTitle          string
	codeOriginal       string
	confirmOpen        bool
	confirmTitle       string
	confirmText        string
	confirm            func()
	probeURL           string
	probeProxy         string
	probeResult        string
	resourceDraft      string
	disclaimer         bool
	checkIP            bool
	ipPending          bool
	qualities          map[string]qualityRecord
	qualityOpen        bool
	qualityIP          string
	qualityRequest     string
	qualityBusy        bool
	qualityRevision    uint64
	qualityCancel      func()
	systemDark         bool
	document           *documentEditor
	pendingApply       time.Time
	hotkeyOpen         bool
	hotkeyAction       string
	hotkeyDraft        string
	hotkeysSuspended   bool
	hotkeyErrors       map[string]string
}

func New(dir, corePath string, safe bool) (*App, error) {
	if safe && dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(base, "FlClash-MyGo-safe")
	}
	s, cfg, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	if err = registerFonts(); err != nil {
		return nil, err
	}
	logo, _ := ui.DecodeBitmap(assets.Read("icon.png"))
	a := &App{store: s, cfg: cfg, corePath: corePath, safe: safe, status: "stopped", page: "dashboard", english: assets.Catalog("en"), logo: logo, delays: map[string]int{}, recordSelected: -1, probeURL: store.DefaultTestURL, recordsTable: true}
	a.loadQualities()
	a.disclaimer = !cfg.Disclaimer && !safe
	a.recordState.Selected = &a.recordSelected
	return a, nil
}
func (a *App) t(key string) string {
	aliases := map[string]string{"testDelay": "delayTest", "organization": "ipOrganization", "stopped": "notRunning", "hotkey": "hotkeyManagement", "primaryColor": "themeColor", "proxy": "proxies", "proxyGroups": "proxyGroup", "export": "exportFile", "clear": "clearData", "clipboard": "clipboardImport", "override": "overrideEntries", "configuration": "config", "success": "done"}
	if alias := aliases[key]; alias != "" {
		key = alias
	}
	if a.catalog != nil && a.catalog[key] != "" {
		return a.catalog[key]
	}
	if a.english[key] != "" {
		return a.english[key]
	}
	fallback := map[string]string{"chooseFile": "Choose file", "closeAllConnections": "Close all connections", "pause": "Pause", "paused": "Paused", "peak": "Peak", "resetNetwork": "Reset network", "scale": "Text size", "sourceCode": "Source code", "updateAll": "Update all", "config": "Configuration", "sidebar": "Toggle sidebar", "coreStatus": "Core status", "loading": "Loading", "success": "Done", "done": "Done", "dnsSettings": "DNS", "proxySettings": "Proxies", "coreSettings": "Core", "customResources": "Custom resources"}
	if label := fallback[key]; label != "" {
		return label
	}
	return humanize(key)
}
func (a *App) save() {
	if err := a.store.Save(a.cfg); err != nil {
		a.err = err.Error()
	}
}
func (a *App) update(fn func()) {
	if a.window != nil {
		a.window.Update(fn)
	} else {
		a.updateMu.Lock()
		a.updates = append(a.updates, fn)
		a.updateMu.Unlock()
	}
}

func (a *App) flushUpdates() {
	a.updateMu.Lock()
	updates := a.updates
	a.updates = nil
	a.updateMu.Unlock()
	for _, fn := range updates {
		fn()
	}
}
func (a *App) work(fn func() (func(), error), finally ...func()) {
	a.working++
	go func() {
		done, err := fn()
		a.update(func() {
			a.working--
			for _, finish := range finally {
				finish()
			}
			if err != nil {
				a.err = err.Error()
			} else if done != nil {
				done()
			}
		})
	}()
}
func (a *App) goPage(page string) {
	if a.pushProfilePickerPage(page) {
		return
	}
	if a.page != page {
		a.editorLoadSerial++
		if a.frameEnabled && navigationPrimary(page) {
			a.activeNavigation = page
		}
		a.history = append(a.history, a.page)
		a.page = page
		a.recordQuery = ""
		a.recordFilter = ""
		a.recordSelected = -1
		a.paused = false
	}
}
func (a *App) back() {
	if a.profilePickerNavigation() != nil {
		a.backProfilePicker()
		return
	}
	a.editorLoadSerial++
	if a.page == "collection" && a.document != nil && len(a.document.Path) > 0 {
		a.document.Path = a.document.Path[:len(a.document.Path)-1]
		a.document.List = ui.ListState{}
		return
	}
	if n := len(a.history); n > 0 {
		a.page = a.history[n-1]
		a.history = a.history[:n-1]
		a.recordSelected = -1
	} else {
		a.page = "tools"
	}
}
func (a *App) apply() {
	a.pendingApply = time.Time{}
	a.markServiceRoutes()
	a.save()
	a.syncOnDemand()
	if a.engine != nil && !a.safe {
		a.engine.Reconcile(a.cfg, a.listenerIntent())
	}
}
func (a *App) toggle() {
	if a.safe {
		a.message = "Safe mode"
		return
	}
	a.wantRunning = !a.wantRunning
	a.begin()
	a.apply()
}
func (a *App) scheduleApply(c *ui.Context) {
	a.pendingApply = c.Now().Add(500 * time.Millisecond)
	c.After(500 * time.Millisecond)
}
func (a *App) begin() {
	if a.engine != nil || a.safe || !a.cfg.Disclaimer {
		return
	}
	a.engine = NewEngine(a.store, a.corePath, os.Stderr)
	a.engine.Run()
	a.engine.Status = func(status string, err error, revision uint64) {
		a.update(func() {
			if a.engine.Revision() != revision {
				return
			}
			previous := a.status
			a.status = status
			if err != nil {
				a.err = err.Error()
				if status == "error" {
					a.wantRunning = false
				}
			}
			if status == "running" && previous != "running" && a.started.IsZero() {
				a.started = time.Now()
				a.outboundIP()
			}
			if status == "stopped" && a.wantRunning && a.onDemandSuspended() && a.started.IsZero() {
				a.started = time.Now()
			}
			if status == "stopped" && !a.wantRunning || status == "error" {
				a.started = time.Time{}
			}
			if status == "stopped" && previous != "stopped" {
				a.outboundIP()
			}
			a.syncTray()
		})
	}
	a.engine.Sample = func(s Sample) {
		a.update(func() {
			a.sample = s
			a.syncTray()
			a.upHistory = appendLimit(a.upHistory, s.Traffic.Up, 35)
			a.downHistory = appendLimit(a.downHistory, s.Traffic.Down, 35)
			a.peak.Up = max(a.peak.Up, s.Traffic.Up)
			a.peak.Down = max(a.peak.Down, s.Traffic.Down)
			if a.group == "" || a.sample.Proxies.Proxies[a.group].Name == "" {
				a.group = ""
				if p := a.currentProxyProfile(); p != nil && s.Proxies.Proxies[p.Group].Name != "" {
					a.group = p.Group
				}
				for _, name := range s.Proxies.All {
					if a.group == "" && !s.Proxies.Proxies[name].Hidden {
						a.group = name
						break
					}
				}
			}
		})
	}
	a.engine.Events = func(b Batch) {
		a.update(func() {
			if b.Refresh {
				a.markServiceRoutes()
			}
			for _, l := range b.Logs {
				a.logs = appendLimit(a.logs, l, 5000)
			}
			for _, d := range b.DNS {
				a.dns = appendLimit(a.dns, d, 3000)
			}
			for _, r := range b.Requests {
				a.requests = appendLimit(a.requests, r, 2000)
			}
			for _, d := range b.Delays {
				a.setProxyDelay(d.Name, d.URL, d.Value)
			}
			if len(b.RequestTraffic) > 0 {
				for i := range a.requests {
					if traffic, ok := b.RequestTraffic[a.requests[i].ID]; ok {
						a.requests[i].Upload = traffic.Up
						a.requests[i].Download = traffic.Down
					}
				}
			}
		})
	}
	a.holdCoreConnecting()
	a.status = "connecting"
	a.syncOnDemand()
	a.engine.Reconcile(a.cfg, a.listenerIntent())
	a.autoUpdate()
	go func() {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, address := range addrs {
				ip, _, _ := net.ParseCIDR(address.String())
				if ip.To4() != nil {
					a.update(func() { a.intranet = ip.String() })
					return
				}
			}
		}
	}()
}
func (a *App) Run() error {
	mygo.App.SetName("FlClash MyGo")
	mygo.App.SetVersion(Version)
	if !a.safe && !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	mygo.App.OnOpenURL(func(raw string) { a.openLink(raw) })
	mygo.App.OnOpenFile(func(path string) { a.importFile(path) })
	mygo.App.WhenReady(func() {
		a.systemDark = mygo.Theme.IsDark()
		mygo.Theme.OnUpdated(func() { a.update(func() { a.systemDark = mygo.Theme.IsDark() }) })
		a.frameEnabled = true
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: "FlClash", Width: a.cfg.WindowWidth, Height: a.cfg.WindowHeight, MinWidth: 380, MinHeight: 400, TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: runtime.GOOS != "darwin", StateKey: "main", Content: ui.View(a.View)})
		a.captionHost = a.window
		a.window.OnMaximize(func() { a.update(func() { a.caption.Maximized = true }) })
		a.window.OnUnmaximize(func() { a.update(func() { a.caption.Maximized = false }) })
		a.window.OnEnterFullScreen(func() { a.update(func() { a.caption.FullScreen = true }) })
		a.window.OnLeaveFullScreen(func() { a.update(func() { a.caption.FullScreen = false }) })
		_ = a.window.SetIcon(assets.Read("icon.png"))
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if !a.quitting && a.cfg.CloseToTray && a.tray != nil {
				e.PreventDefault()
				a.window.Hide()
			}
		})
		a.window.OnResize(func() { a.cfg.WindowWidth, a.cfg.WindowHeight = a.window.Size(); a.save() })
		mygo.App.OnActivate(func(visible bool) {
			if !visible {
				a.window.Show()
			}
		})
		a.installMenu()
		a.installTray()
		a.installHotkeys()
		if a.cfg.StartHidden && a.tray != nil {
			a.window.Hide()
		}
		if os.Getenv("FLCLASH_ACCEPT_DISCLAIMER") == "1" {
			a.cfg.Disclaimer = true
			a.disclaimer = false
			a.save()
		}
		if !a.disclaimer {
			a.wantRunning = a.cfg.AutoStart && !a.safe
			a.begin()
		}
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.quitting = true
			a.stopOnDemand()
			a.cancelDelayTests()
			if a.iconCache != nil {
				a.iconCache.Close()
			}
			a.save()
			go func() {
				if a.engine != nil {
					a.engine.Close()
				}
				mygo.App.Quit()
			}()
		})
	})
	return mygo.App.Run()
}
func (a *App) installMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "File", Submenu: []*mygo.MenuItem{{Label: a.t("addProfile"), Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.addOpen = true }) }}, {Label: a.t("import"), Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { a.chooseImport() }}, mygo.Separator(), {Role: mygo.RoleClose}}},
		{Role: mygo.RoleEditMenu},
		{Label: "View", Submenu: []*mygo.MenuItem{{Label: a.t("dashboard"), Accelerator: "CmdOrCtrl+1", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.selectNavigation("dashboard") }) }}, {Label: a.t("proxies"), Accelerator: "CmdOrCtrl+2", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.selectNavigation("proxies") }) }}, {Label: a.t("profiles"), Accelerator: "CmdOrCtrl+3", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.selectNavigation("profiles") }) }}, {Role: mygo.RoleToggleFullScreen}}},
		{Role: mygo.RoleWindowMenu},
		{Label: "Help", Submenu: []*mygo.MenuItem{{Label: a.t("about"), Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.goPage("about") }) }}}},
	}))
}
func (a *App) buildTrayMenu() *mygo.Menu {
	if a.safe {
		return mygo.NewMenu([]*mygo.MenuItem{{Label: "FlClash — SAFE MODE", Click: func(*mygo.MenuItem, *mygo.Window) { a.window.Show() }}, {Label: a.t("exit"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }}})
	}
	items := []*mygo.MenuItem{
		{Label: "FlClash", Click: func(*mygo.MenuItem, *mygo.Window) { a.window.Show(); a.window.Focus() }}, mygo.Separator(),
		{ID: "toggle", Label: a.t("start"), Click: func(*mygo.MenuItem, *mygo.Window) { a.update(a.toggle) }},
		{ID: "system-proxy", Label: a.t("systemProxy"), Type: mygo.MenuItemCheckbox, Checked: a.cfg.SystemProxy, Click: func(i *mygo.MenuItem, _ *mygo.Window) { a.update(func() { a.cfg.SystemProxy = i.Checked; a.apply() }) }},
		{ID: "tun", Label: "TUN", Type: mygo.MenuItemCheckbox, Checked: store.Bool(store.Map(a.cfg.Overrides, "tun")["enable"]), Click: func(i *mygo.MenuItem, _ *mygo.Window) {
			a.update(func() { store.Map(a.cfg.Overrides, "tun")["enable"] = i.Checked; a.apply() })
		}},
		mygo.Separator(), {Label: a.t("rule"), Type: mygo.MenuItemRadio, Checked: store.String(a.cfg.Overrides["mode"]) == "rule", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.cfg.Overrides["mode"] = "rule"; a.apply() }) }},
		{Label: a.t("global"), Type: mygo.MenuItemRadio, Checked: store.String(a.cfg.Overrides["mode"]) == "global", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.cfg.Overrides["mode"] = "global"; a.apply() }) }},
		{Label: a.t("direct"), Type: mygo.MenuItemRadio, Checked: store.String(a.cfg.Overrides["mode"]) == "direct", Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.cfg.Overrides["mode"] = "direct"; a.apply() }) }},
		mygo.Separator(), {Label: a.t("exit"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	}
	exit := items[len(items)-1]
	items = items[:len(items)-2]
	if len(a.cfg.Profiles) > 0 {
		var profiles []*mygo.MenuItem
		for _, p := range a.cfg.Profiles {
			profiles = append(profiles, &mygo.MenuItem{Label: p.Name, Type: mygo.MenuItemRadio, Checked: p.ID == a.cfg.Active, Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.activateProfile(p.ID) }) }})
		}
		items = append(items, &mygo.MenuItem{Label: a.t("profiles"), Submenu: profiles})
	}
	var groups []*mygo.MenuItem
	for _, name := range a.sample.Proxies.All {
		g := a.sample.Proxies.Proxies[name]
		if g.Hidden {
			continue
		}
		var nodes []*mygo.MenuItem
		for _, node := range g.All {
			nodes = append(nodes, &mygo.MenuItem{Label: node, Type: mygo.MenuItemRadio, Checked: g.Now == node, Click: func(*mygo.MenuItem, *mygo.Window) { a.update(func() { a.selectProxy(name, node) }) }})
		}
		groups = append(groups, &mygo.MenuItem{Label: name, Submenu: nodes})
	}
	if len(groups) > 0 {
		items = append(items, &mygo.MenuItem{Label: a.t("proxies"), Submenu: groups})
	}
	items = append(items, mygo.Separator(), exit)
	return mygo.NewMenu(items)
}
func (a *App) installTray() {
	a.trayMenu = a.buildTrayMenu()
	icon := "tray/unix/status_1.png"
	template := false
	if runtime.GOOS == "darwin" {
		icon = "tray/macos/status_1.png"
		template = true
	}
	if a.safe {
		icon = "tray/unix/status_4.png"
		if runtime.GOOS == "darwin" {
			icon = "tray/macos/status_4.png"
		}
	}
	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{ID: "FlClash-MyGo", Icon: assets.Read(icon), IconIsTemplate: template, ToolTip: "FlClash", Menu: a.trayMenu})
	if err != nil {
		log.Printf("tray: %v", err)
	}
}
func (a *App) syncTray() {
	if a.tray == nil || a.safe {
		return
	}
	var key strings.Builder
	key.WriteString(a.status)
	key.WriteString(a.cfg.Active)
	key.WriteString(store.String(a.cfg.Overrides["mode"]))
	if a.wantRunning {
		key.WriteByte('r')
	}
	if a.cfg.SystemProxy {
		key.WriteByte('p')
	}
	if store.Bool(store.Map(a.cfg.Overrides, "tun")["enable"]) {
		key.WriteByte('t')
	}
	for _, p := range a.cfg.Profiles {
		key.WriteString(p.ID)
		key.WriteString(p.Name)
	}
	for _, name := range a.sample.Proxies.All {
		g := a.sample.Proxies.Proxies[name]
		key.WriteString(name)
		key.WriteString(g.Now)
		for _, node := range g.All {
			key.WriteString(node)
		}
	}
	signature := key.String()
	if signature == a.traySignature {
		return
	}
	a.traySignature = signature
	a.trayMenu = a.buildTrayMenu()
	label := a.t("start")
	if a.wantRunning {
		label = a.t("stop")
	}
	a.trayMenu.ItemByID("toggle").SetLabel(label)
	a.tray.SetMenu(a.trayMenu)
	if runtime.GOOS != "darwin" {
		icon := "status_1"
		if a.status == "running" {
			icon = "status_2"
			if store.Bool(store.Map(a.cfg.Overrides, "tun")["enable"]) {
				icon = "status_3"
			}
		}
		_ = a.tray.SetIcon(assets.Read("tray/unix/"+icon+".png"), false)
	}
}
func (a *App) installHotkeys() {
	a.hotkeyErrors = map[string]string{}
	if a.safe {
		return
	}
	mygo.GlobalShortcut.UnregisterAll()
	if a.hotkeyOpen {
		a.hotkeysSuspended = true
		return
	}
	for _, binding := range hotkeyBindings {
		action := binding.action
		acc := a.cfg.Hotkeys[action]
		if acc == "" {
			continue
		}
		if !validHotkey(acc) {
			a.hotkeyErrors[action] = strings.ReplaceAll(a.t("hotkeyNeedsModifier"), "{modifiers}", "Ctrl, Alt, Shift, Super")
			continue
		}
		if err := mygo.GlobalShortcut.Register(acc, func() { a.update(func() { a.performHotkey(action) }) }); err != nil {
			a.hotkeyErrors[action] = err.Error()
		}
	}
}
func (a *App) Render(path string, width, height float32, dark bool) error {
	if dark {
		a.cfg.Theme = "dark"
	}
	a.disclaimer = false
	a.safe = true
	t := ui.NewTester(a.View, int(width), int(height))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, t.Image())
}
func (a *App) openLink(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		return
	}
	if u.Scheme != "clash" && u.Scheme != "clashmeta" && u.Scheme != "flclash" && u.Scheme != "flclash-mygo" {
		return
	}
	source := u.Query().Get("url")
	if source == "" {
		return
	}
	a.update(func() {
		a.addName = u.Query().Get("name")
		a.addSource = source
		a.addKind = 0
		a.addOpen = true
		a.window.Show()
	})
}
func (a *App) ask(title, text string, fn func()) {
	a.confirmTitle = title
	a.confirmText = text
	a.confirm = fn
	a.confirmOpen = true
}
func (a *App) coreAction(method string, args any) {
	if a.engine == nil || a.safe {
		return
	}
	e := a.engine
	a.work(func() (func(), error) {
		err := e.Call(method, args, nil)
		return func() { a.message = a.t("success") }, err
	})
}
func (a *App) coreMessage(method string, args any) {
	if a.engine == nil || a.safe {
		return
	}
	e := a.engine
	a.work(func() (func(), error) {
		err := e.Message(method, args)
		return func() { a.message = a.t("success") }, err
	})
}
func (a *App) showJSON(title string, value any) {
	b, err := store.YAML(value)
	if err != nil {
		a.err = err.Error()
		return
	}
	a.detail = title + "\n\n" + b
	a.detailOpen = true
}
func (a *App) openCode(title, purpose, text string) {
	a.editorLoadSerial++
	a.goPage("editor")
	a.codeProvider = nil
	if strings.HasPrefix(purpose, "provider:") && a.engine != nil {
		a.codeProvider = &providerEditorContext{Profile: a.cfg.Active, Engine: a.engine, Revision: a.engine.Revision()}
	}
	a.code = &editor.State{Buffer: editor.NewBuffer(text)}
	name := "config.yaml"
	if strings.Contains(purpose, "script") {
		name = "override.js"
	}
	a.code.Buffer.SetLanguage(editor.LanguageFor(name))
	a.codePurpose = purpose
	a.codeTitle = title
	a.codeOriginal = text
}
func (a *App) outboundIP()                  { a.requestDashboardOutbound() }
func jsonUnmarshal(b []byte, out any) error { return json.Unmarshal(b, out) }
func (a *App) autoUpdate() {
	if a.safe {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-a.engine.ctx.Done():
				return
			case <-ticker.C:
				a.update(func() {
					if a.working > 0 {
						return
					}
					for _, p := range a.cfg.Profiles {
						if p.UpdateDue(time.Now()) {
							a.updateProfile(p)
							break
						}
					}
				})
			}
		}
	}()
}

func (a *App) activateProfile(id string) {
	if a.cfg.Active == id {
		return
	}
	a.cfg.Active = id
	a.resetProxyProfile()
	a.apply()
}

func (a *App) resetProxyProfile() {
	a.editorLoadSerial++
	a.cancelDelayTests()
	a.profileDetail = profileDetailState{Serial: a.profileDetail.Serial + 1}
	a.providerFiles, a.providerPending = nil, nil
	a.group = ""
	a.cfg.Selected = map[string]string{}
	if p := a.currentProxyProfile(); p != nil {
		a.group = p.Group
		for name, value := range p.Selected {
			a.cfg.Selected[name] = value
		}
	}
	a.proxyList = ui.ListState{}
	if a.picker.Open && a.picker.Kind == "proxies" {
		a.picker.Proxy = proxyPageState{}
	}
	a.proxyScroll = ui.ScrollState{}
	a.proxyGrids = nil
	a.proxyLocate = ""
	a.delays = map[string]int{}
	a.delaySnapshot = nil
}
func (a *App) restoredSettings(cfg store.Settings) {
	a.cfg = cfg
	a.resetProxyProfile()
	a.filterState = nil
	a.installHotkeys()
	a.apply()
}
