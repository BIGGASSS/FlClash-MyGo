package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

type frozenRecords struct {
	connections, requests []Connection
	dns                   []DNSQuery
	logs                  []LogEntry
}

func (a *App) records(c *ui.Context) {
	if a.paused && a.frozen == nil {
		a.frozen = &frozenRecords{connections: append([]Connection(nil), a.sample.Connections.Connections...), requests: append([]Connection(nil), a.requests...), dns: append([]DNSQuery(nil), a.dns...), logs: append([]LogEntry(nil), a.logs...)}
	} else if !a.paused {
		a.frozen = nil
	}
	ui.Column(c).Fill().Padding(0, 16, 16, 16).Gap(14).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.SearchField(c, &a.recordQuery).Label(a.t("search")).Radius(999).Padding(10, 14).Grow(1)
			filters := []string{"all", "tcp", "udp", "direct", "reject"}
			if a.page == "logs" {
				filters = []string{"all", "debug", "info", "warning", "error"}
			} else if a.page == "dns" {
				filters = []string{"all", "A", "AAAA", "cached", "error"}
			}
			if a.recordFilter == "" {
				a.recordFilter = "all"
			}
			ui.Select(c, &a.recordFilter, filters).Width(125).Label(a.t("filter"))
			if a.page == "connections" || a.page == "requests" {
				if a.recordSort == "" {
					a.recordSort = "start"
				}
				ui.Select(c, &a.recordSort, []string{"start", "download", "upload", "source", "target"}).Width(135).Label(a.t("sort"))
			}
		})
		switch a.page {
		case "connections", "requests":
			a.connectionRecords(c)
		case "dns":
			a.dnsRecords(c)
		case "logs":
			a.logRecords(c)
		}
	})
}
func (a *App) connectionRecords(c *ui.Context) {
	data := a.sample.Connections.Connections
	if a.page == "requests" {
		data = a.requests
	}
	if a.frozen != nil {
		data = a.frozen.connections
		if a.page == "requests" {
			data = a.frozen.requests
		}
	}
	var rows []Connection
	for _, r := range data {
		if !searchMatches(a.recordQuery, r.Search()) {
			continue
		}
		f := a.recordFilter
		if f == "tcp" || f == "udp" {
			if !strings.EqualFold(r.Metadata.Network, f) {
				continue
			}
		} else if f == "direct" || f == "reject" {
			if !strings.Contains(strings.ToLower(r.Exit()), f) {
				continue
			}
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		x, y := rows[i], rows[j]
		switch a.recordSort {
		case "download":
			return x.Download > y.Download
		case "upload":
			return x.Upload > y.Upload
		case "source":
			return x.Source() < y.Source()
		case "target":
			return x.Target() < y.Target()
		default:
			return x.Start.After(y.Start)
		}
	})
	if a.recordSelected >= len(rows) {
		a.recordSelected = -1
	}
	a.recordState.Key = func(i int) any { return rows[i].ID }
	a.recordState.RoundRows = true
	if len(rows) == 0 {
		a.empty(c, "connections", a.t("noData"), "", nil)
		return
	}
	if a.recordsTable {
		columns := []ui.TableColumn{{Title: a.t("source"), Width: 140}, {Title: a.t("target"), MinWidth: 150}, {Title: a.t("network"), Width: 65}, {Title: a.t("proxy"), Width: 130}, {Title: a.t("download"), Width: 85}, {Title: a.t("upload"), Width: 85}, {Title: "", Width: 38}}
		table := ui.Table(c, &a.recordState, columns, len(rows), func(row, col int) {
			r := rows[row]
			if col == 6 {
				a.button(c, "close", a.t("close")+" "+r.ID, func() { a.coreAction("closeConnection", r.ID) }).Padding(3).Background(ui.Transparent)
				return
			}
			text := []string{r.Source(), r.Target(), strings.ToUpper(r.Metadata.Network), r.Exit(), bytesText(r.Download), bytesText(r.Upload)}[col]
			ui.Text(c, text).FontSize(fontSize(c, 13)).SingleLine()
		}).Grow(1)
		if table.Submitted() && a.recordSelected >= 0 {
			a.showJSON(a.t("details"), rows[a.recordSelected])
		}
	} else {
		ui.List(c, &a.recordState, len(rows), func(i int) {
			r := rows[i]
			ui.ButtonBase(c.Key(r.ID)).FillWidth().Background(a.colors.card).Radius(24).Padding(18).Margin(0, 0, 8, 0).OnClick(func() { a.showJSON(a.t("details"), r) }).Children(func() {
				ui.Column(c).Gap(8).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, r.Source()).Bold()
						ui.Text(c, r.Target()).Grow(1).SingleLine()
						a.chip(c, strings.ToUpper(r.Metadata.Network), a.colors.muted)
						a.button(c, "close", a.t("close")+" "+r.ID, func() { a.coreAction("closeConnection", r.ID) }).Padding(3).Background(ui.Transparent)
					})
					ui.Text(c, strings.Join(r.Chains, "  ←  ")).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).SingleLine()
					ui.Row(c).Gap(14).Children(func() {
						ui.Text(c, r.Rule+" "+r.RulePayload).Grow(1).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
						ui.Text(c, "↓ "+bytesText(r.Download)+"    ↑ "+bytesText(r.Upload)).FontSize(fontSize(c, 12))
					})
				})
			})
		}).Grow(1)
	}
	ui.Row(c).Gap(12).Children(func() {
		ui.Textf(c, "%d / %d", len(rows), len(data)).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
		ui.Spacer(c)
		if a.recordSelected >= 0 && a.recordSelected < len(rows) {
			ui.Button(c, a.t("details")).Radius(999).OnClick(func() { a.showJSON(a.t("details"), rows[a.recordSelected]) })
		}
		if a.paused {
			a.chip(c, a.t("paused"), a.colors.warning)
		}
	})
}
func (a *App) dnsRecords(c *ui.Context) {
	data := a.dns
	if a.frozen != nil {
		data = a.frozen.dns
	}
	rows := make([]DNSQuery, 0, len(data))
	for i := len(data) - 1; i >= 0; i-- {
		d := data[i]
		if !searchMatches(a.recordQuery, d.Domain, d.Upstream, d.Type, d.Initiator, strings.Join(d.Answers, "\n")) {
			continue
		}
		f := a.recordFilter
		if (f == "A" || f == "AAAA") && d.Type != f {
			continue
		}
		if f == "cached" && !d.Cached {
			continue
		}
		if f == "error" && d.Error == "" {
			continue
		}
		rows = append(rows, d)
	}
	if len(rows) == 0 {
		a.empty(c, "dns", a.t("noData"), "", nil)
		return
	}
	if a.recordSelected >= len(rows) {
		a.recordSelected = -1
	}
	a.recordState.Key = func(i int) any { return fmt.Sprintf("%s/%s/%s/%d", rows[i].Time, rows[i].Domain, rows[i].Type, i) }
	if a.recordsTable {
		e := ui.Table(c, &a.recordState, []ui.TableColumn{{Title: "Domain", MinWidth: 150}, {Title: "Type", Width: 65}, {Title: "Upstream", Width: 160}, {Title: "Answer", MinWidth: 150}, {Title: a.t("delay"), Width: 90}}, len(rows), func(row, col int) {
			d := rows[row]
			text := []string{d.Domain, d.Type, d.Upstream, strings.Join(d.Answers, ", "), fmt.Sprintf("%d ms", d.Delay)}[col]
			if d.Error != "" && col == 3 {
				text = d.Error
			}
			ui.Text(c, text).SingleLine().FontSize(fontSize(c, 13))
		}).Grow(1)
		if e.Submitted() && a.recordSelected >= 0 {
			a.showJSON("DNS", rows[a.recordSelected])
		}
	} else {
		ui.List(c, &a.recordState, len(rows), func(i int) {
			d := rows[i]
			ui.ButtonBase(c.Key(i)).FillWidth().Background(a.colors.card).Radius(24).Padding(18).Margin(0, 0, 8, 0).OnClick(func() { a.showJSON("DNS", d) }).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, d.Domain).Bold().Grow(1).SingleLine()
						a.chip(c, d.Type, a.colors.muted)
						ui.Textf(c, "%d ms", d.Delay).FontSize(fontSize(c, 12))
					})
					answer := strings.Join(d.Answers, ", ")
					if d.Error != "" {
						answer = d.Error
					}
					ui.Text(c, answer).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).SingleLine()
					ui.Text(c, d.Upstream+"  ·  "+d.Time.Format("15:04:05")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
				})
			})
		}).Grow(1)
	}
	ui.Textf(c, "%d / %d", len(rows), len(data)).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
}
func (a *App) logRecords(c *ui.Context) {
	data := a.logs
	if a.frozen != nil {
		data = a.frozen.logs
	}
	rows := make([]LogEntry, 0, len(data))
	for i := len(data) - 1; i >= 0; i-- {
		l := data[i]
		if !searchMatches(a.recordQuery, l.Type, l.Payload) {
			continue
		}
		if a.recordFilter != "all" && a.recordFilter != l.Type {
			continue
		}
		rows = append(rows, l)
	}
	if len(rows) == 0 {
		a.empty(c, "logs", a.t("noData"), "", nil)
		return
	}
	if a.recordSelected >= len(rows) {
		a.recordSelected = -1
	}
	a.recordState.Key = func(i int) any { return fmt.Sprintf("%s/%d", rows[i].Time, i) }
	color := func(level string) ui.Color {
		switch level {
		case "error":
			return a.colors.danger
		case "warning":
			return a.colors.warning
		case "info":
			return a.colors.accent
		default:
			return a.colors.muted
		}
	}
	if a.recordsTable {
		ui.Table(c, &a.recordState, []ui.TableColumn{{Title: a.t("time"), Width: 95}, {Title: a.t("logLevel"), Width: 85}, {Title: a.t("logs")}}, len(rows), func(row, col int) {
			l := rows[row]
			if col == 0 {
				ui.Text(c, l.Time.Format("15:04:05")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
			} else if col == 1 {
				ui.Text(c, strings.ToUpper(l.Type)).FontSize(fontSize(c, 12)).TextColor(color(l.Type))
			} else {
				ui.Text(c, l.Payload).FontSize(fontSize(c, 13)).SingleLine()
			}
		}).Grow(1)
	} else {
		ui.List(c, &a.recordState, len(rows), func(i int) {
			l := rows[i]
			ui.Column(c.Key(i)).FillWidth().Radius(24).Background(a.colors.card).Padding(16).Margin(0, 0, 8, 0).Gap(8).Children(func() {
				ui.Row(c).Children(func() {
					a.chip(c, strings.ToUpper(l.Type), color(l.Type))
					ui.Spacer(c)
					ui.Text(c, l.Time.Format("15:04:05")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
				})
				ui.Text(c, l.Payload).FontSize(fontSize(c, 14)).Selectable()
			})
		}).Grow(1)
	}
	ui.Textf(c, "%d / %d", len(rows), len(data)).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
}
