// Package ipquality ports FlClash's source parsers and staggered IP lookup.
package ipquality

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"time"
)

type Source int

const (
	IdentMe Source = iota
	IPAPICom
	IPQuery
	IPLocate
	ProxyCheck
	IPAPIIs
)

var Sources = []Source{IdentMe, IPAPICom, IPQuery, IPLocate, ProxyCheck, IPAPIIs}

func (s Source) String() string {
	if s < IdentMe || s > IPAPIIs {
		return "unknown"
	}
	return []string{"ident.me", "ip-api.com", "ipquery.io", "iplocate.io", "proxycheck.io", "ipapi.is"}[s]
}
func (s Source) URL(ip string) string {
	switch s {
	case IdentMe:
		return "https://ident.me/json"
	case IPAPICom:
		return "http://ip-api.com/json/" + ip + "?fields=status,isp,org,as,mobile,proxy,hosting"
	case IPQuery:
		return "https://api.ipquery.io/" + ip + "?format=json"
	case IPLocate:
		return "https://iplocate.io/api/lookup/" + ip
	case ProxyCheck:
		return "https://proxycheck.io/v3/" + ip
	default:
		return "https://api.ipapi.is/?q=" + ip
	}
}

type Quality struct {
	IP           string `json:"ip"`
	Source       Source `json:"source"`
	Type         string `json:"type"`
	Organization string `json:"organization,omitempty"`
	ASN          int    `json:"asn,omitempty"`
	Proxy        bool   `json:"isProxy"`
	VPN          bool   `json:"isVpn"`
	Tor          bool   `json:"isTor"`
	Abuser       bool   `json:"isAbuser"`
}

func (q Quality) Level() string {
	if q.Tor || q.Abuser {
		return "risky"
	}
	if q.Type == "hosting" {
		return "normal"
	}
	return "good"
}

type Failure struct {
	Source Source `json:"source"`
	Status string `json:"status"`
}
type LookupError struct{ Failures []Failure }

func (e *LookupError) Error() string { return "IP quality lookup failed" }

type Response struct {
	Status int
	Body   []byte
}
type Fetcher func(context.Context, string) (Response, error)
type Options struct{ Timeout, Hedge time.Duration }

func Lookup(ctx context.Context, ip string, fetch Fetcher) (Quality, error) {
	return LookupWith(ctx, ip, fetch, Options{Timeout: 8 * time.Second, Hedge: time.Second})
}

func LookupWith(ctx context.Context, ip string, fetch Fetcher, options Options) (Quality, error) {
	if _, err := netip.ParseAddr(ip); err != nil {
		return Quality{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	waves := [][]Source{{IdentMe, IPAPICom, IPQuery}, {IPLocate}, {IPAPIIs}, {ProxyCheck}}
	type answer struct {
		quality  Quality
		inferred bool
		failure  *Failure
	}
	answers := make(chan answer, 6)
	var fallback *Quality
	failures := []Failure{}
	wave, inflight := 0, 0
	timer := time.NewTimer(options.Hedge)
	defer timer.Stop()
	var tick <-chan time.Time
	launch := func() {
		if wave >= len(waves) {
			tick = nil
			return
		}
		for _, source := range waves[wave] {
			inflight++
			go func() {
				request, cancel := context.WithTimeout(ctx, options.Timeout)
				defer cancel()
				response, err := fetch(request, source.URL(ip))
				quality, inferred, status := Quality{}, false, ""
				if err != nil {
					status = "failed"
					if errors.Is(err, context.DeadlineExceeded) {
						status = "timeout"
					}
				} else if response.Status == 429 {
					status = "rateLimited"
				} else if response.Status != 200 {
					status = "failed"
				} else {
					quality, inferred, status = Parse(source, ip, response.Body)
				}
				result := answer{quality: quality, inferred: inferred}
				if status != "" {
					result.failure = &Failure{Source: source, Status: status}
				}
				select {
				case answers <- result:
				case <-ctx.Done():
				}
			}()
		}
		wave++
		timer.Stop()
		if wave < len(waves) {
			timer.Reset(options.Hedge)
			tick = timer.C
		} else {
			tick = nil
		}
	}
	launch()
	for {
		if inflight == 0 && wave >= len(waves) {
			if fallback != nil {
				return *fallback, nil
			}
			sort.Slice(failures, func(i, j int) bool { return failures[i].Source < failures[j].Source })
			return Quality{}, &LookupError{Failures: failures}
		}
		select {
		case <-ctx.Done():
			return Quality{}, ctx.Err()
		case <-tick:
			launch()
		case result := <-answers:
			inflight--
			if result.failure != nil {
				failures = append(failures, *result.failure)
			} else if !result.inferred {
				return result.quality, nil
			} else if fallback == nil {
				value := result.quality
				fallback = &value
			}
			if inflight == 0 {
				launch()
			}
		}
	}
}

func Parse(source Source, ip string, data []byte) (Quality, bool, string) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return Quality{}, false, "failed"
	}
	quality := Quality{IP: ip, Source: source}
	inferred := false
	switch source {
	case IdentMe:
		if text(raw["ip"]) != ip {
			return quality, false, "ipMismatch"
		}
		quality.Type = declared(raw["type"])
		quality.Organization = text(raw["aso"])
		quality.ASN = asn(raw["asn"])
	case IPAPICom:
		if raw["status"] != "success" {
			return quality, false, "failed"
		}
		quality.Type = pick(flag(raw["hosting"]), flag(raw["mobile"]), "")
		inferred = quality.Type == ""
		if inferred {
			quality.Type = "residential"
		}
		quality.Organization = first(text(raw["org"]), text(raw["isp"]))
		quality.ASN = asn(raw["as"])
		quality.Proxy = flag(raw["proxy"])
	case IPQuery:
		risk, isp := object(raw["risk"]), object(raw["isp"])
		quality.Type = pick(flag(risk["is_datacenter"]), flag(risk["is_mobile"]), "")
		inferred = quality.Type == ""
		if inferred {
			quality.Type = "residential"
		}
		quality.Organization = first(text(isp["org"]), text(isp["isp"]))
		quality.ASN = asn(isp["asn"])
		quality.Proxy, quality.VPN, quality.Tor = flag(risk["is_proxy"]), flag(risk["is_vpn"]), flag(risk["is_tor"])
	case IPLocate:
		privacy, company, network := object(raw["privacy"]), object(raw["company"]), object(raw["asn"])
		quality.Type = pick(flag(privacy["is_hosting"]), false, first(declared(company["type"]), declared(network["type"])))
		quality.Organization = first(text(company["name"]), text(network["name"]))
		quality.ASN = asn(network["asn"])
		quality.Proxy, quality.VPN, quality.Tor, quality.Abuser = flag(privacy["is_proxy"]), flag(privacy["is_vpn"]), flag(privacy["is_tor"]), flag(privacy["is_abuser"])
	case ProxyCheck:
		if raw["status"] == "denied" {
			return quality, false, "rateLimited"
		}
		if raw["status"] != "ok" && raw["status"] != "warning" {
			return quality, false, "failed"
		}
		result := object(raw[ip])
		network, detections := object(result["network"]), object(result["detections"])
		declaredType := ""
		if network["type"] == "Residential" {
			declaredType = "residential"
		} else if network["type"] == "Business" {
			declaredType = "business"
		}
		quality.Type = pick(flag(detections["hosting"]) || network["type"] == "Hosting", network["type"] == "Wireless", declaredType)
		quality.Organization = text(network["organisation"])
		quality.ASN = asn(network["asn"])
		quality.Proxy, quality.VPN, quality.Tor = flag(detections["proxy"]), flag(detections["vpn"]), flag(detections["tor"])
	case IPAPIIs:
		company, network := object(raw["company"]), object(raw["asn"])
		quality.Type = pick(flag(raw["is_datacenter"]), flag(raw["is_mobile"]), first(declared(company["type"]), declared(network["type"])))
		quality.Organization = first(text(company["name"]), text(network["org"]))
		quality.ASN = asn(network["asn"])
		quality.Proxy, quality.VPN, quality.Tor, quality.Abuser = flag(raw["is_proxy"]), flag(raw["is_vpn"]), flag(raw["is_tor"]), flag(raw["is_abuser"])
	}
	if quality.Type == "" {
		return quality, false, "noType"
	}
	return quality, inferred, ""
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func flag(v any) bool             { return v == true }
func text(v any) string           { s, _ := v.(string); return s }
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func declared(v any) string {
	switch v {
	case "isp":
		return "residential"
	case "business", "education", "government", "banking":
		return "business"
	case "hosting":
		return "hosting"
	}
	return ""
}
func pick(hosting, mobile bool, declared string) string {
	if hosting || declared == "hosting" {
		return "hosting"
	}
	if mobile {
		return "mobile"
	}
	return declared
}

var digits = regexp.MustCompile(`\d+`)

func asn(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	value := digits.FindString(fmt.Sprint(v))
	n, _ := strconv.Atoi(value)
	return n
}
