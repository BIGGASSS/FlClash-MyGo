package ipquality

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
)

var OutboundSources = []string{
	"https://www.cloudflare.com/cdn-cgi/trace",
	"https://api.ip.sb/geoip",
	"https://ipinfo.io/json",
	"https://ipwho.is",
	"https://get.geojs.io/v1/ip/geo.json",
	"https://api.country.is",
	"https://api.ipquery.io/?format=json",
	"https://ident.me/json",
}

type Info struct{ IP, Country string }

func ParseOutbound(source, body string) (Info, error) {
	var ip, country string
	if source == OutboundSources[0] {
		fields := map[string]string{}
		for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
			if key, value, ok := strings.Cut(line, "="); ok {
				fields[key] = value
			}
		}
		ip, country = fields["ip"], fields["loc"]
	} else {
		var raw map[string]any
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			return Info{}, err
		}
		ip = text(raw["ip"])
		switch source {
		case OutboundSources[1], OutboundSources[3], OutboundSources[4]:
			country = text(raw["country_code"])
		case OutboundSources[2], OutboundSources[5]:
			country = text(raw["country"])
		case OutboundSources[6]:
			country = text(object(raw["location"])["country_code"])
		case OutboundSources[7]:
			country = text(raw["cc"])
		default:
			return Info{}, errors.New("unknown outbound IP source")
		}
	}
	if _, err := netip.ParseAddr(ip); err != nil {
		return Info{}, err
	}
	if country == "" {
		return Info{}, errors.New("outbound IP source has no country code")
	}
	return Info{IP: ip, Country: country}, nil
}
