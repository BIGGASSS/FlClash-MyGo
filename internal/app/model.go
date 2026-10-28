package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type ProxyNode struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	All      []string `json:"all"`
	Now      string   `json:"now"`
	Fixed    string   `json:"fixed"`
	Hidden   bool     `json:"hidden"`
	Provider string   `json:"provider-name"`
	TestURL  string   `json:"testUrl"`
	Icon     string   `json:"icon"`
}
type ProxyData struct {
	Proxies map[string]ProxyNode `json:"proxies"`
	All     []string             `json:"all"`
}
type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}
type Memory struct {
	RSS   uint64 `json:"rss"`
	Heap  uint64 `json:"heapInuse"`
	Idle  uint64 `json:"heapIdle"`
	Stack uint64 `json:"stackInuse"`
	Other uint64 `json:"runtimeOther"`
}
type Connection struct {
	ID          string    `json:"id"`
	Metadata    Metadata  `json:"metadata"`
	Upload      int64     `json:"upload"`
	Download    int64     `json:"download"`
	Start       time.Time `json:"start"`
	Chains      []string  `json:"chains"`
	Rule        string    `json:"rule"`
	RulePayload string    `json:"rulePayload"`
}
type Metadata struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	Host            string `json:"host"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Process         string `json:"process"`
	ProcessPath     string `json:"processPath"`
	DNSMode         string `json:"dnsMode"`
	SpecialProxy    string `json:"specialProxy"`
}

func (c Connection) Target() string {
	host := c.Metadata.Host
	if host == "" {
		host = c.Metadata.DestinationIP
	}
	if c.Metadata.DestinationPort != "" {
		host += ":" + c.Metadata.DestinationPort
	}
	return host
}
func (c Connection) Source() string {
	if c.Metadata.Process != "" {
		return c.Metadata.Process
	}
	if c.Metadata.ProcessPath != "" {
		return filepath.Base(c.Metadata.ProcessPath)
	}
	return c.Metadata.SourceIP
}
func (c Connection) Exit() string {
	if len(c.Chains) > 0 {
		return c.Chains[0]
	}
	return "—"
}
func (c Connection) Search() string {
	return strings.ToLower(c.Target() + " " + c.Source() + " " + c.Metadata.Network + " " + c.Rule + " " + c.RulePayload + " " + strings.Join(c.Chains, " "))
}

type ConnectionData struct {
	Connections []Connection `json:"connections"`
	Upload      int64        `json:"uploadTotal"`
	Download    int64        `json:"downloadTotal"`
	Memory      uint64       `json:"memory"`
}
type DNSQuery struct {
	Domain    string    `json:"domain"`
	Type      string    `json:"type"`
	Initiator string    `json:"initiator"`
	Upstream  string    `json:"upstream"`
	Cached    bool      `json:"cached"`
	Answers   []string  `json:"answers"`
	Rcode     string    `json:"rcode"`
	Error     string    `json:"error"`
	Delay     int64     `json:"delay"`
	Time      time.Time `json:"time"`
}
type LogEntry struct {
	Type    string    `json:"type"`
	Payload string    `json:"payload"`
	Time    time.Time `json:"time"`
}
type Service struct {
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	Region       string     `json:"region"`
	Delay        int64      `json:"delay"`
	Chains       []string   `json:"chains,omitempty"`
	CheckedAt    int64      `json:"checked-at"`
	CoreEpoch    uint64     `json:"core-epoch"`
	RouteVersion uint64     `json:"route-version"`
	Hits         []routeHit `json:"hits,omitempty"`
}

type routeHit struct {
	Host  string `json:"host"`
	Proxy string `json:"proxy,omitempty"`
	Exit  string `json:"exit"`
}
type Provider struct {
	Name         string                `json:"name"`
	Type         string                `json:"type"`
	Vehicle      string                `json:"vehicle-type"`
	Count        int                   `json:"count"`
	Path         string                `json:"path"`
	Updated      time.Time             `json:"update-at"`
	Subscription *providerSubscription `json:"subscription-info,omitempty"`
}

type providerSubscription struct {
	Upload, Download, Total, Expire int64
}
type Sample struct {
	Counts      *ConfigCounts
	Traffic     Traffic
	Total       Traffic
	Connections ConnectionData
	Memory      Memory
	Proxies     ProxyData
	Providers   []Provider
	AppHeap     uint64
	AppRSS      uint64
}
type ConfigCounts struct{ Groups, Proxies, Rules int }
type NodeDelay struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Value int    `json:"value"`
}
type Batch struct {
	Logs           []LogEntry
	DNS            []DNSQuery
	Requests       []Connection
	Delays         []NodeDelay
	Refresh        bool
	RequestTraffic map[string]Traffic
}

func Decode[T any](b json.RawMessage) T { var t T; _ = json.Unmarshal(b, &t); return t }
func bytesText(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
func number(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
func elapsed(t time.Time) string {
	if t.IsZero() {
		return "00:00:00"
	}
	d := time.Since(t)
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}
