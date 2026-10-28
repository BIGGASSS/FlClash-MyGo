package store

import "slices"

func DefaultDashboardWidgets() []string {
	return []string{"currentOutbound", "networkSpeed", "outboundMode", "vpnButton", "networkTakeover", "intranetIp", "serviceStatus", "profiles", "proxyGroups", "connectionStats"}
}

func DashboardWidgetNames() []string {
	return []string{"currentOutbound", "networkSpeed", "outboundMode", "vpnButton", "networkTakeover", "serviceStatus", "profiles", "proxyGroups", "connectionStats", "intranetIp", "memoryInfo"}
}

// NormalizeDashboardWidgets follows dashboardWidgetsFromJson in the pinned
// Flutter model, including its legacy defaults and retired card identities.
func NormalizeDashboardWidgets(saved []string) []string {
	if saved == nil {
		return DefaultDashboardWidgets()
	}
	aliases := map[string]string{
		"networkDetection": "intranetIp", "outboundModeV2": "outboundMode",
		"systemProxyButton": "networkTakeover", "tunButton": "networkTakeover",
		"connections": "connectionStats", "requests": "connectionStats", "dnsQueries": "connectionStats",
		"trafficUsage": "networkSpeed",
	}
	current := slices.Contains(saved, "currentOutbound")
	if !current {
		legacy := []string{"networkSpeed", "systemProxyButton", "tunButton", "outboundMode", "networkDetection", "trafficUsage", "intranetIp"}
		names := map[string]bool{}
		for _, name := range saved {
			if name == "outboundModeV2" {
				name = "outboundMode"
			}
			names[name] = true
		}
		match := len(names) == len(legacy)
		for _, name := range legacy {
			match = match && names[name]
		}
		if match {
			return DefaultDashboardWidgets()
		}
	}
	kept := []string{}
	retiredOnly := true
	for _, name := range saved {
		retiredOnly = retiredOnly && slices.Contains([]string{"runTime", "overrideDnsButton", "overrideNtpButton"}, name)
		if alias := aliases[name]; alias != "" {
			name = alias
		}
		if slices.Contains(DashboardWidgetNames(), name) && !slices.Contains(kept, name) {
			kept = append(kept, name)
		}
	}
	if current {
		return kept
	}
	if len(kept) == 0 && !retiredOnly {
		return DefaultDashboardWidgets()
	}
	return append([]string{"currentOutbound"}, kept...)
}
