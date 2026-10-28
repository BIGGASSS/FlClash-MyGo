package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) providerInfo(p Provider) {
	if p.Subscription != nil {
		a.showProviderSubscription(p)
		return
	}
	a.showJSON(p.Name, p)
}

func (a *App) providerDialogs(c *ui.Context) {
	probe := a.providerFiles[providerStateKey]
	if probe == nil || probe.state == nil || probe.state.subscription == nil {
		return
	}
	state := probe.state
	info := *state.subscription
	background, scrim := a.colors.card, float32(.2)
	if a.colors.dark {
		background, scrim = a.colors.block, .48
	}
	a.dialogBase(c.Key("provider-subscription"), &state.infoOpen, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, scrim))
		panel.Width(332).MaxWidthPercent(90).MaxHeightPercent(90).Radius(34).Background(background).Gap(0)
		ui.Text(c, a.t("subscriptionInfo")).TextAlign(ui.Center).FillWidth().Padding(21, 30, 0, 30).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600)
		ui.Scroll(c).Shrink(1).FillWidth().Padding(12, 16).Children(func() {
			ui.Column(c).FillWidth().Gap(0).Children(func() {
				a.providerSubscriptionSection(c, a.t("trafficUsage"), func() {
					a.providerSubscriptionItem(c, a.t("usedTraffic"), compactTraffic(info.Upload+info.Download), false)
					ui.Box(c).Height(1).Margin(0, 14, 0, 16).Background(a.colors.text.Alpha(.14))
					a.providerSubscriptionItem(c, a.t("totalTraffic"), compactTraffic(info.Total), false)
				})
				expire := a.t("infiniteTime")
				if info.Expire != 0 {
					expire = time.Unix(info.Expire, 0).Local().Format("2006-01-02")
				}
				a.providerSubscriptionSection(c, a.t("expireTime"), func() {
					a.providerSubscriptionItem(c, "", expire, true)
				})
			})
		})
		ui.Box(c).FillWidth().Padding(0, 16, 16, 16).Children(func() {
			ui.PrimaryButton(c, a.t("confirm")).FillWidth().Height(48).Radius(24).FontSize(fontSize(c, 17)).FontWeight(600).OnClick(func() { state.infoOpen = false })
		})
	})
}

func (a *App) providerSubscriptionSection(c *ui.Context, title string, body func()) {
	background := a.colors.block
	if a.colors.dark {
		background = a.colors.inset
	}
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		a.profileSectionHeader(c, title, nil)
		ui.Column(c).FillWidth().Gap(0).Radius(24).Background(background).Children(body)
	})
}

func (a *App) providerSubscriptionItem(c *ui.Context, label, value string, single bool) {
	ui.Column(c).FillWidth().MinHeight(56).Padding(10, 16).Gap(0).Justify(ui.Center).Children(func() {
		if !single {
			ui.Text(c, label).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22))
		}
		text := ui.Text(c, value).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).TextColor(a.colors.muted).Selectable()
		if single {
			text.FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).TextColor(a.colors.text)
		}
	})
}
