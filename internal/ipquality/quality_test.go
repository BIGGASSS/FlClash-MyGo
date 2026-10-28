package ipquality

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

const testIP = "203.0.113.9"

func TestSourceParsersMatchFlClash(t *testing.T) {
	for _, test := range []struct {
		source                  Source
		body                    string
		kind, org               string
		asn                     int
		proxy, vpn, tor, abuser bool
	}{
		{IdentMe, `{"ip":"203.0.113.9","aso":"Example Broadband","asn":64500,"type":"isp"}`, "residential", "Example Broadband", 64500, false, false, false, false},
		{IPAPICom, `{"status":"success","isp":"Example Hosting","org":"Example Cloud","as":"AS64501 Example Hosting","mobile":false,"proxy":true,"hosting":true}`, "hosting", "Example Cloud", 64501, true, false, false, false},
		{IPQuery, `{"isp":{"asn":"AS64502","org":"Example Mobile"},"risk":{"is_mobile":true,"is_tor":true}}`, "mobile", "Example Mobile", 64502, false, false, true, false},
		{IPLocate, `{"asn":{"asn":"AS64503","name":"Example Broadband","type":"isp"},"privacy":{"is_hosting":true},"company":{"name":"Example Leasing","type":"hosting"}}`, "hosting", "Example Leasing", 64503, false, false, false, false},
		{ProxyCheck, `{"status":"ok","203.0.113.9":{"network":{"type":"Wireless","organisation":"Example Wireless","asn":"AS64504"},"detections":{"vpn":true,"proxy":true}}}`, "mobile", "Example Wireless", 64504, true, true, false, false},
		{IPAPIIs, `{"company":{"type":"business","name":"Example Business"},"asn":{"asn":64505},"is_abuser":true}`, "business", "Example Business", 64505, false, false, false, true},
	} {
		got, inferred, status := Parse(test.source, testIP, []byte(test.body))
		if status != "" || inferred {
			t.Fatal(test.source, status, inferred)
		}
		want := Quality{IP: testIP, Source: test.source, Type: test.kind, Organization: test.org, ASN: test.asn, Proxy: test.proxy, VPN: test.vpn, Tor: test.tor, Abuser: test.abuser}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %+v want %+v", test.source, got, want)
		}
	}
}
func TestExplicitHostingWinsOverMobileAndDeclaredResidential(t *testing.T) {
	got, _, status := Parse(IPAPIIs, testIP, []byte(`{"is_mobile":true,"is_datacenter":true,"company":{"type":"isp"}}`))
	if status != "" || got.Type != "hosting" {
		t.Fatal(got, status)
	}
	if (Quality{Type: "residential", Proxy: true, VPN: true}).Level() != "good" || (Quality{Type: "mobile", Tor: true}).Level() != "risky" || (Quality{Type: "hosting"}).Level() != "normal" {
		t.Fatal("quality level differs from source")
	}
}
func TestParseFailuresAndInferredType(t *testing.T) {
	for _, test := range []struct {
		source       Source
		body, status string
	}{
		{IdentMe, `{"ip":"198.51.100.1","type":"isp"}`, "ipMismatch"},
		{IPLocate, `{"company":{"type":"unknown"}}`, "noType"},
		{ProxyCheck, `{"status":"denied"}`, "rateLimited"},
		{IPAPICom, `{"status":"fail"}`, "failed"},
		{IPQuery, `null`, "failed"},
	} {
		if _, _, status := Parse(test.source, testIP, []byte(test.body)); status != test.status {
			t.Fatal(test, status)
		}
	}
	quality, inferred, status := Parse(IPAPICom, testIP, []byte(`{"status":"success"}`))
	if status != "" || !inferred || quality.Type != "residential" {
		t.Fatal(quality, inferred, status)
	}
	if asn(float64(4294967295)) != 4294967295 {
		t.Fatal("numeric ASN truncated")
	}
}
func TestHedgedLookupPrefersExplicitTypeAndCancelsLosers(t *testing.T) {
	cancelled := make(chan string, 6)
	quality, err := LookupWith(context.Background(), testIP, func(ctx context.Context, address string) (Response, error) {
		u, _ := url.Parse(address)
		if u.Host == "iplocate.io" {
			return Response{Status: 200, Body: []byte(`{"company":{"type":"isp","name":"Explicit"}}`)}, nil
		}
		<-ctx.Done()
		cancelled <- u.Host
		return Response{}, ctx.Err()
	}, Options{Timeout: time.Second, Hedge: 5 * time.Millisecond})
	if err != nil || quality.Organization != "Explicit" || quality.Source != IPLocate {
		t.Fatal(quality, err)
	}
	for range 3 {
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatal("losing source was not cancelled")
		}
	}
}
func TestInferredResidentialIsOnlyUsedAfterEverySource(t *testing.T) {
	var mu sync.Mutex
	requested := map[string]bool{}
	quality, err := LookupWith(context.Background(), testIP, func(_ context.Context, address string) (Response, error) {
		u, _ := url.Parse(address)
		mu.Lock()
		requested[u.Host] = true
		mu.Unlock()
		if u.Host == "ip-api.com" {
			return Response{Status: 200, Body: []byte(`{"status":"success"}`)}, nil
		}
		return Response{Status: 503}, nil
	}, Options{Timeout: time.Second, Hedge: time.Millisecond})
	if err != nil || quality.Source != IPAPICom || quality.Type != "residential" || len(requested) != 6 {
		t.Fatal(quality, err, requested)
	}
}
func TestSourceFailureStatusesAndStableOrder(t *testing.T) {
	_, err := LookupWith(context.Background(), testIP, func(_ context.Context, address string) (Response, error) {
		u, _ := url.Parse(address)
		if u.Host == "ident.me" {
			return Response{Status: 429}, nil
		}
		if u.Host == "ip-api.com" {
			return Response{}, context.DeadlineExceeded
		}
		if u.Host == "api.ipquery.io" {
			return Response{Status: 503}, nil
		}
		return Response{Status: 200, Body: []byte(`{}`)}, nil
	}, Options{Timeout: time.Second, Hedge: time.Millisecond})
	var failed *LookupError
	if !errors.As(err, &failed) || len(failed.Failures) != 6 {
		t.Fatal(err)
	}
	if failed.Failures[0].Status != "rateLimited" || failed.Failures[1].Status != "timeout" {
		t.Fatal(failed.Failures)
	}
	for i, f := range failed.Failures {
		if f.Source != Source(i) {
			t.Fatal("failure order changed", failed.Failures)
		}
	}
	data, _ := json.Marshal(failed.Failures)
	if len(data) == 0 {
		t.Fatal("source statuses cannot be cached")
	}
}
