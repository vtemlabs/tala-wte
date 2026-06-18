// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Read-only post-capture analysis of a saved pcap via tshark/capinfos; every
// sub-analysis is best-effort and yields an empty section rather than failing.
package capture

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Bounded list sizes so a large capture cannot produce an unbounded response.
const (
	maxTalkers = 15
	maxDNS     = 30
	maxHTTP    = 30
	maxCreds   = 50
)

type Analysis struct {
	Packets        int            `json:"packets"`
	FileSizeMB     float64        `json:"file_size_mb"`
	DurationSec    float64        `json:"duration_sec"`
	FirstPacket    string         `json:"first_packet"`
	LastPacket     string         `json:"last_packet"`
	Protocols      []ProtoStat    `json:"protocols"`
	TopTalkers     []Conversation `json:"top_talkers"`
	DNSQueries     []CountedItem  `json:"dns_queries"`
	HTTPRequests   []HTTPRequest  `json:"http_requests"`
	TLSServerNames []CountedItem  `json:"tls_server_names"`
	UserAgents     []CountedItem  `json:"user_agents"`
	Credentials    []Credential   `json:"credentials"`
	Note           string         `json:"note,omitempty"`
}

type ProtoStat struct {
	Protocol string `json:"protocol"`
	Packets  int    `json:"packets"`
}
type Conversation struct {
	A       string `json:"a"`
	B       string `json:"b"`
	Packets int    `json:"packets"`
	Bytes   int64  `json:"bytes"`
}
type CountedItem struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}
type HTTPRequest struct {
	Method string `json:"method"`
	Host   string `json:"host"`
	URI    string `json:"uri"`
}
type Credential struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Detail string `json:"detail"`
}

// PcapPath returns the on-disk path for a capture id.
func PcapPath(id string) string {
	return filepath.Join(CaptureDir, id+".pcapng")
}

// haveTshark reports whether tshark is available for the deeper analyses.
func haveTshark() bool {
	_, err := exec.LookPath("tshark")
	return err == nil
}

// runFields runs tshark with a display filter and -e fields, returning the
// tab-separated rows. Returns nil on any error.
func runFields(pcap, displayFilter string, fields ...string) [][]string {
	args := []string{"-r", pcap, "-Y", displayFilter, "-T", "fields"}
	for _, f := range fields {
		args = append(args, "-e", f)
	}
	out, err := exec.Command("tshark", args...).Output()
	if err != nil {
		return nil
	}
	var rows [][]string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

// Analyze reads a saved pcap and returns a structured summary.
func Analyze(id string) (*Analysis, error) {
	pcap := PcapPath(id)
	if _, err := os.Stat(pcap); err != nil {
		return nil, err
	}

	a := &Analysis{
		Packets:    PacketCount(pcap),
		FileSizeMB: fileSizeMB(pcap),
	}
	analyzeCapinfos(pcap, a)

	if !haveTshark() {
		a.Note = "tshark is not installed; only summary counts are available."
		return a, nil
	}

	analyzeProtocols(pcap, a)
	analyzeTalkers(pcap, a)
	analyzeDNS(pcap, a)
	analyzeHTTP(pcap, a)
	analyzeTLS(pcap, a)
	analyzeUserAgents(pcap, a)
	analyzeCredentials(pcap, a)
	return a, nil
}

// analyzeTLS lists the TLS SNI server names, showing which HTTPS destinations
// were contacted even though the payload is encrypted.
func analyzeTLS(pcap string, a *Analysis) {
	rows := runFields(pcap, "tls.handshake.extensions_server_name", "tls.handshake.extensions_server_name")
	counts := map[string]int{}
	for _, r := range rows {
		if len(r) > 0 && r[0] != "" {
			counts[r[0]]++
		}
	}
	a.TLSServerNames = topCounts(counts, maxDNS)
}

// analyzeUserAgents lists the HTTP User-Agent strings seen.
func analyzeUserAgents(pcap string, a *Analysis) {
	rows := runFields(pcap, "http.user_agent", "http.user_agent")
	counts := map[string]int{}
	for _, r := range rows {
		if len(r) > 0 && r[0] != "" {
			counts[r[0]]++
		}
	}
	a.UserAgents = topCounts(counts, 10)
}

func fileSizeMB(pcap string) float64 {
	info, err := os.Stat(pcap)
	if err != nil {
		return 0
	}
	return float64(info.Size()) / 1024 / 1024
}

// analyzeCapinfos fills duration and first/last timestamps from capinfos.
func analyzeCapinfos(pcap string, a *Analysis) {
	out, err := exec.Command("capinfos", "-u", "-a", "-e", pcap).Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Capture duration:"):
			a.DurationSec = parseDurationSeconds(afterColon(line))
		case strings.HasPrefix(line, "First packet time:"):
			a.FirstPacket = afterColon(line)
		case strings.HasPrefix(line, "Last packet time:"):
			a.LastPacket = afterColon(line)
		}
	}
}

func afterColon(s string) string {
	if i := strings.Index(s, ":"); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// parseDurationSeconds handles capinfos values like "12.345 seconds".
func parseDurationSeconds(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "seconds"))
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// analyzeProtocols counts the highest-layer protocol per packet; the last
// element of frame.protocols is the most specific.
func analyzeProtocols(pcap string, a *Analysis) {
	rows := runFields(pcap, "", "frame.protocols")
	counts := map[string]int{}
	for _, r := range rows {
		if len(r) == 0 || r[0] == "" {
			continue
		}
		parts := strings.Split(r[0], ":")
		top := parts[len(parts)-1]
		if top == "" && len(parts) > 1 {
			top = parts[len(parts)-2]
		}
		counts[top]++
	}
	for _, c := range topCounts(counts, 0) {
		a.Protocols = append(a.Protocols, ProtoStat{Protocol: c.Value, Packets: c.Count})
	}
}

// analyzeTalkers parses the IP conversation table.
func analyzeTalkers(pcap string, a *Analysis) {
	out, err := exec.Command("tshark", "-r", pcap, "-q", "-z", "conv,ip").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		// Rows: A <-> B  framesA bytesA framesB bytesB framesTotal bytesTotal ...
		if len(f) < 9 || f[1] != "<->" {
			continue
		}
		frames, _ := strconv.Atoi(f[6])
		bytes := parseHumanBytes(f[7], f[8])
		a.TopTalkers = append(a.TopTalkers, Conversation{A: f[0], B: f[2], Packets: frames, Bytes: bytes})
	}
	sort.Slice(a.TopTalkers, func(i, j int) bool { return a.TopTalkers[i].Packets > a.TopTalkers[j].Packets })
	if len(a.TopTalkers) > maxTalkers {
		a.TopTalkers = a.TopTalkers[:maxTalkers]
	}
}

// parseHumanBytes turns a number plus an optional unit (bytes/kB/MB) into bytes.
func parseHumanBytes(num, unit string) int64 {
	v, _ := strconv.ParseFloat(num, 64)
	switch strings.ToLower(unit) {
	case "kb":
		v *= 1000
	case "mb":
		v *= 1000 * 1000
	case "gb":
		v *= 1000 * 1000 * 1000
	}
	return int64(v)
}

func analyzeDNS(pcap string, a *Analysis) {
	rows := runFields(pcap, "dns.flags.response==0 && dns.qry.name", "dns.qry.name")
	counts := map[string]int{}
	for _, r := range rows {
		if len(r) > 0 && r[0] != "" {
			counts[r[0]]++
		}
	}
	a.DNSQueries = topCounts(counts, maxDNS)
}

func analyzeHTTP(pcap string, a *Analysis) {
	rows := runFields(pcap, "http.request", "http.request.method", "http.host", "http.request.uri")
	seen := map[string]bool{}
	for _, r := range rows {
		req := HTTPRequest{}
		if len(r) > 0 {
			req.Method = r[0]
		}
		if len(r) > 1 {
			req.Host = r[1]
		}
		if len(r) > 2 {
			req.URI = r[2]
		}
		key := req.Method + " " + req.Host + req.URI
		if seen[key] {
			continue
		}
		seen[key] = true
		a.HTTPRequests = append(a.HTTPRequests, req)
		if len(a.HTTPRequests) >= maxHTTP {
			break
		}
	}
}

// analyzeCredentials surfaces cleartext credentials from HTTP Basic auth headers
// and credential-like HTTP form post fields.
func analyzeCredentials(pcap string, a *Analysis) {
	for _, r := range runFields(pcap, "http.authorization", "http.host", "http.authorization") {
		host, cred := "", ""
		if len(r) > 0 {
			host = r[0]
		}
		if len(r) > 1 {
			cred = decodeBasic(r[1])
		}
		if cred == "" {
			continue
		}
		a.Credentials = append(a.Credentials, Credential{Kind: "HTTP Basic", Source: host, Detail: cred})
		if len(a.Credentials) >= maxCreds {
			return
		}
	}
	for _, r := range runFields(pcap, "http.request.method==\"POST\" && urlencoded-form", "http.host", "urlencoded-form.key", "urlencoded-form.value") {
		host := ""
		if len(r) > 0 {
			host = r[0]
		}
		if len(r) < 3 {
			continue
		}
		detail := pairCredFields(r[1], r[2])
		if detail == "" {
			continue
		}
		a.Credentials = append(a.Credentials, Credential{Kind: "HTTP form post", Source: host, Detail: detail})
		if len(a.Credentials) >= maxCreds {
			return
		}
	}
}

// pairCredFields pairs tshark's comma-separated form keys and values and keeps
// pairs whose key looks like a credential field.
func pairCredFields(keysCSV, valsCSV string) string {
	keys := strings.Split(keysCSV, ",")
	vals := strings.Split(valsCSV, ",")
	var out []string
	for i, k := range keys {
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		out = append(out, k+"="+v)
	}
	return strings.Join(out, "  ")
}

func decodeBasic(v string) string {
	v = strings.TrimSpace(strings.TrimPrefix(v, "Basic "))
	if b, err := base64.StdEncoding.DecodeString(v); err == nil {
		return string(b)
	}
	return v
}

// topCounts turns a count map into a descending-sorted slice, optionally capped.
func topCounts(counts map[string]int, limit int) []CountedItem {
	items := make([]CountedItem, 0, len(counts))
	for k, v := range counts {
		items = append(items, CountedItem{Value: k, Count: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Value < items[j].Value
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}
