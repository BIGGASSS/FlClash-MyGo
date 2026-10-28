package ipquality

import "testing"

func TestAllPinnedOutboundSourceFormats(t *testing.T) {
	bodies := []string{
		"fl=example\nip=203.0.113.9\nloc=GB\n",
		`{"ip":"203.0.113.9","country_code":"GB"}`,
		`{"ip":"203.0.113.9","country":"GB"}`,
		`{"ip":"203.0.113.9","country_code":"GB"}`,
		`{"ip":"203.0.113.9","country_code":"GB"}`,
		`{"ip":"203.0.113.9","country":"GB"}`,
		`{"ip":"203.0.113.9","location":{"country_code":"GB"}}`,
		`{"ip":"203.0.113.9","cc":"GB"}`,
	}
	for i, body := range bodies {
		info, err := ParseOutbound(OutboundSources[i], body)
		if err != nil || info.IP != testIP || info.Country != "GB" {
			t.Fatal(OutboundSources[i], info, err)
		}
	}
	for _, body := range []string{"ip=not-an-address\nloc=GB\n", "ip=203.0.113.9\n"} {
		if _, err := ParseOutbound(OutboundSources[0], body); err == nil {
			t.Fatal("invalid source reply accepted")
		}
	}
}
