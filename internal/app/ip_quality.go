package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

type qualityRecord struct {
	Quality  *ipquality.Quality  `json:"quality,omitempty"`
	Checked  time.Time           `json:"checked"`
	Failures []ipquality.Failure `json:"failures,omitempty"`
}

func (a *App) loadQualities() {
	a.qualities = map[string]qualityRecord{}
	f, err := os.Open(filepath.Join(a.store.Dir, "ip-quality.json"))
	if err != nil {
		return
	}
	defer f.Close()
	if err = json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(&a.qualities); err != nil {
		a.qualities = map[string]qualityRecord{}
	}
	if a.qualities == nil {
		a.qualities = map[string]qualityRecord{}
	}
	for ip, record := range a.qualities {
		if q := record.Quality; q == nil || !qualityFresh(record, time.Now()) || q.IP != ip || q.Type == "" || q.Source < ipquality.IdentMe || q.Source > ipquality.IPAPIIs {
			delete(a.qualities, ip)
		}
	}
}

type qualityJob struct {
	owner  probeOwner
	cancel context.CancelFunc
}

// Independent IP lookups can finish together. Serialize durable snapshots so
// an older write cannot replace a newer snapshot containing both records.
type qualityPersistence struct {
	mu     sync.Mutex
	newest atomic.Uint64
}

func qualityTTL(record qualityRecord) time.Duration {
	if record.Quality != nil {
		return 72 * time.Hour
	}
	return 5 * time.Minute
}

func qualityFresh(record qualityRecord, now time.Time) bool {
	age := now.Sub(record.Checked)
	return age >= 0 && age < qualityTTL(record)
}

func (a *App) qualityLoading(ip string) bool {
	if a.outbound != nil && a.outbound.qualityJobs[ip] != nil {
		return true
	}
	// Also supports windowless fixture states.
	return a.qualityBusy && a.qualityRequest == ip && (a.outbound == nil || len(a.outbound.qualityJobs) == 0)
}

func (a *App) lookupQuality(ip string, force bool) {
	if ip == "" || a.engine == nil || a.safe || !a.probeVisible() {
		return
	}
	s := a.outboundState()
	o := a.probeOwner()
	if !a.ownsProbe(o) {
		return
	}
	if job := s.qualityJobs[ip]; job != nil {
		if !force && job.owner == o {
			return
		}
		job.cancel()
	}
	if record, ok := a.qualities[ip]; ok && !force && qualityFresh(record, time.Now()) {
		if record.Quality != nil && a.ip == ip {
			a.organization = record.Quality.Organization
		}
		return
	}
	// An expired or explicitly refreshed record must not keep displaying as
	// fresh while its replacement is pending (or after that request cancels).
	delete(a.qualities, ip)
	if a.ip == ip {
		a.organization = ""
	}
	proxy := ""
	if !o.proxied {
		proxy = "DIRECT"
	}
	ctx, cancel := context.WithCancel(o.engine.ctx)
	job := &qualityJob{owner: o, cancel: cancel}
	s.qualityJobs[ip] = job
	a.qualityBusy, a.qualityRequest = true, ip
	a.work(func() (func(), error) {
		quality, err := ipquality.Lookup(ctx, ip, func(ctx context.Context, url string) (ipquality.Response, error) {
			var result struct {
				Status int    `json:"status-code"`
				Body   string `json:"body"`
				Error  string `json:"error"`
			}
			err := o.call(ctx, "probe", map[string]any{"proxy-name": proxy, "url": url, "timeout": 8000, "max-body": 1 << 20, "headers": map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"}}, &result)
			if err == nil && result.Error != "" {
				if result.Error == "timeout" {
					err = context.DeadlineExceeded
				} else {
					err = errors.New(result.Error)
				}
			}
			return ipquality.Response{Status: result.Status, Body: []byte(result.Body)}, err
		})
		return func() {
			defer cancel()
			if s.qualityJobs[ip] != job {
				return
			}
			delete(s.qualityJobs, ip)
			a.qualityBusy = len(s.qualityJobs) > 0
			if !a.ownsProbe(o) || ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			record := qualityRecord{Checked: time.Now()}
			if err == nil {
				record.Quality = &quality
			} else {
				var failed *ipquality.LookupError
				if errors.As(err, &failed) {
					record.Failures = failed.Failures
				}
			}
			if a.qualities == nil {
				a.qualities = map[string]qualityRecord{}
			}
			a.qualities[ip] = record
			if quality.Organization != "" && a.ip == ip {
				a.organization = quality.Organization
			}
			a.saveQualities()
		}, nil
	})
}
func (a *App) saveQualities() {
	for ip, record := range a.qualities {
		if !qualityFresh(record, time.Now()) {
			delete(a.qualities, ip)
		}
	}
	persisted := map[string]qualityRecord{}
	for ip, record := range a.qualities {
		if record.Quality != nil {
			persisted[ip] = record
		}
	}
	data, err := json.Marshal(persisted)
	if err != nil {
		return
	}
	path := filepath.Join(a.store.Dir, "ip-quality.json")
	writes := &a.outboundState().qualitySaves
	serial := writes.newest.Add(1)
	a.work(func() (func(), error) {
		writes.mu.Lock()
		defer writes.mu.Unlock()
		if writes.newest.Load() == serial {
			_ = store.AtomicWrite(path, data)
		}
		return nil, nil
	})
}
func (a *App) shownIP(ip string) string {
	if a.cfg.HideIP {
		return "*****"
	}
	return ip
}
func (a *App) qualityLabel(level string) string { return a.t("ipQuality" + titleCase(level)) }
func (a *App) qualityType(kind string) string   { return a.t("ipType" + titleCase(kind)) }
func (a *App) qualityBadges(c *ui.Context, ip string) {
	if record := a.qualities[ip]; record.Quality != nil {
		quality := record.Quality
		level := quality.Level()
		color := a.colors.muted
		if level == "good" {
			color = a.colors.success
		} else if level == "risky" {
			color = a.colors.warning
		}
		ui.Row(c).Gap(6).Children(func() {
			a.chip(c, a.qualityType(quality.Type), a.colors.muted)
			a.chip(c, a.t("ipQualityShort")+" "+a.qualityLabel(level), color)
		})
	} else if a.qualityLoading(ip) {
		ui.Text(c, a.t("loading")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
	}
}
func (a *App) showQuality(ip string) {
	if ip == "" {
		return
	}
	a.outboundState().qualityFromService = false
	a.qualityIP = ip
	a.qualityOpen = true
	a.lookupQuality(ip, false)
}
func (a *App) qualityDialog(c *ui.Context) {
	a.sideSheet(c, "ip-quality", &a.qualityOpen, func() {
		icon, label := "close", a.t("close")
		if a.outbound != nil && a.outbound.qualityFromService && a.serviceOpen {
			icon, label = "back", a.t("back")
		}
		a.sheetHeader(c, a.t("outboundIp"), icon, label, func() { a.qualityOpen = false }, func() {
			a.sheetButton(c, "refresh", a.t("ipQualityRetry"), func() { a.lookupQuality(a.qualityIP, true) }).Disabled(a.qualityLoading(a.qualityIP))
		})
		ip := a.qualityIP
		record := a.qualities[ip]
		quality := record.Quality
		loading := quality == nil && a.qualityLoading(ip)
		failed := quality == nil && !loading
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Padding(16, 16, 20, 16).Gap(24).Children(func() {
				ui.Text(c, a.t("detectionTip")).FontSize(fontSize(c, 17)).TextColor(a.colors.muted).Margin(8, 5, 0, 5)
				a.groupedSection(c, a.t("basicInfo"), func() {
					copyIP, color := ip, a.colors.muted
					if a.cfg.HideIP {
						copyIP = ""
					}
					if quality != nil {
						color = a.qualityColor(quality.Level())
					}
					a.qualityDetailRow(c, "ipAddress", a.shownIP(ip), copyIP, color, false)
					a.detailSeparator(c)
					if failed {
						a.qualityDetailRow(c, "ipType", a.t("ipQualityFailed"), "", a.colors.danger, false)
						return
					}
					level, kind, flags := "", "", a.t("none")
					if quality != nil {
						level, kind = a.qualityLabel(quality.Level()), a.qualityType(quality.Type)
						var labels []string
						for _, flag := range []struct {
							on    bool
							label string
						}{
							{quality.Tor, "ipFlagTor"}, {quality.Abuser, "ipFlagAbuser"}, {quality.VPN, "ipFlagVpn"}, {quality.Proxy, "ipFlagProxy"},
						} {
							if flag.on {
								labels = append(labels, a.t(flag.label))
							}
						}
						if len(labels) > 0 {
							flags = strings.Join(labels, " · ")
						}
					}
					a.qualityDetailRow(c, "ipQualityLevel", level, "", color, loading)
					a.detailSeparator(c)
					a.qualityDetailRow(c, "ipType", kind, "", a.colors.muted, loading)
					a.detailSeparator(c)
					a.qualityDetailRow(c, "ipFlags", flags, "", a.colors.muted, loading)
				})
				if !failed {
					a.groupedSection(c, a.t("network"), func() {
						organization, asn, source := "—", "—", "—"
						if quality != nil {
							if quality.Organization != "" {
								organization = quality.Organization
							}
							if quality.ASN > 0 {
								asn = fmt.Sprintf("AS%d", quality.ASN)
							}
							source = quality.Source.String()
						}
						a.qualityDetailRow(c, "ipOrganization", organization, organization, a.colors.muted, loading)
						a.detailSeparator(c)
						a.qualityDetailRow(c, "ipAsn", asn, asn, a.colors.muted, loading)
						a.detailSeparator(c)
						a.qualityDetailRow(c, "ipQualitySource", source, "", a.colors.muted, loading)
					})
				}
				if failed && len(record.Failures) > 0 {
					a.groupedSection(c, a.t("ipQualitySources"), func() {
						for i, failure := range record.Failures {
							if i > 0 {
								a.detailSeparator(c)
							}
							key := map[string]string{"noType": "ipSourceNoType", "timeout": "timeout", "rateLimited": "ipSourceRateLimited", "failed": "serviceFailed", "ipMismatch": "ipSourceIpMismatch"}[failure.Status]
							a.qualityDetailRow(c, failure.Source.String(), a.t(key), "", a.colors.muted, false)
						}
					})
				}
				a.groupedSection(c, a.t("other"), func() { a.boolRow(c, "", a.t("hideIp"), &a.cfg.HideIP, a.save) })
			})
		})
	})
}

func (a *App) qualityColor(level string) ui.Color {
	switch level {
	case "good":
		return a.colors.success
	case "risky":
		return a.colors.danger
	default:
		return a.colors.muted
	}
}

func (a *App) detailSeparator(c *ui.Context) {
	ui.Box(c).Height(1).Margin(0, 16).Background(a.colors.line.Alpha(0.6))
}

func (a *App) qualityDetailRow(c *ui.Context, key, value, copyText string, color ui.Color, pending bool) {
	label := a.t(key)
	if strings.Contains(key, ".") {
		label = key
	}
	row := ui.Row(c).FillWidth().MinHeight(56).Padding(12, 16).Gap(20).Label(label)
	if copyText != "" && copyText != "—" && !pending {
		copyValue := func() { c.Services().WriteClipboard(copyText); a.message = a.t("copySuccess") }
		row.OnClick(copyValue).Tooltip(a.t("copy")).ContextMenu(func(menu *ui.Menu) {
			if menu.Item(a.t("copy")).Chosen() {
				copyValue()
			}
		})
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(fontSize(c, 17))
		if pending {
			ui.Spacer(c)
			ui.Box(c).Width(72).Height(15).Radius(8).Background(a.colors.inset)
		} else {
			ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.End).Children(func() {
				if key == "ipFlags" && value != a.t("none") {
					ui.Row(c).Gap(6).Children(func() {
						for _, flag := range strings.Split(value, " · ") {
							a.chip(c, flag, a.colors.muted)
						}
					})
				} else {
					text := ui.Text(c, value).FontSize(fontSize(c, 15)).TextColor(color).MaxWidth(245).SingleLine().Tooltip(value)
					if key == "ipAddress" && a.cfg.HideIP {
						text.Font("JetBrains Mono")
					}
				}
			})
		}
	})
}
