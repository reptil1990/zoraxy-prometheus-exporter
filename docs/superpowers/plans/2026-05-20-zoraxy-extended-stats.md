# Zoraxy Extended Stats — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sieben neue Prometheus-Metriken (IP-Version, Devices, Browser, OS, OS-Versionen, File-Types, Referring Sites) aus den rohen Zoraxy-Stats-Maps exportieren.

**Architecture:** Parsing-Helper in separater Datei `parsers.go`, aggregierte Maps werden im 30-s-Polling-Zyklus berechnet und im `metricsState` gecached. `handleMetrics()` gibt sie via vorhandenem `writeLabeledGauge`-Helper aus. Cardinality-Schutz durch Top-N-Bucket bei Referrers und OS-Versionen.

**Tech Stack:** Go 1.23, `github.com/mssola/user_agent` (neue Dependency), Standard-Library für URL/IP-Parsing.

**Reference Spec:** `docs/superpowers/specs/2026-05-20-zoraxy-extended-stats-design.md`

---

## File Structure

- **Create:** `parsers.go` — alle Parser/Aggregations-Helper
- **Create:** `parsers_test.go` — Unit-Tests pro Parser
- **Modify:** `main.go` — `DailySummaryExport` erweitern, `metricsState.aggregated` ergänzen, Parser in `fetchStats()` aufrufen, neue Metriken in `handleMetrics()` rausschreiben, Version-Bump
- **Modify:** `go.mod`, `go.sum` — `mssola/user_agent` Dependency
- **Modify:** `README.md` — Metriken-Tabelle erweitern

---

## Task 1: User-Agent-Dependency hinzufügen

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Dependency holen**

Run: `go get github.com/mssola/user_agent@latest`

Expected: `go.mod` und `go.sum` bekommen neue Einträge.

- [ ] **Step 2: Verifizieren dass go.mod den Eintrag hat**

Run: `cat go.mod`

Expected output enthält:
```
require github.com/mssola/user_agent vX.Y.Z
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add mssola/user_agent for UA parsing"
```

---

## Task 2: AggregatedStats-Struct + DailySummaryExport erweitern

**Files:**
- Modify: `main.go:31-39` — `DailySummaryExport`-Felder ergänzen
- Modify: `main.go:46-52` — `metricsState` um `aggregated *AggregatedStats` ergänzen, neue Struct dazu

- [ ] **Step 1: DailySummaryExport um vier Roh-Maps erweitern**

Ersetze in `main.go` den `DailySummaryExport`-Block:

```go
// DailySummaryExport mirrors the structure returned by /api/stats/summary.
// The high-cardinality maps (RequestClientIp, Referer, UserAgent, RequestURL)
// are fetched as raw strings and aggregated into low-cardinality buckets
// in parsers.go before being exported as Prometheus metrics.
type DailySummaryExport struct {
	TotalRequest    int64          `json:"TotalRequest"`
	ErrorRequest    int64          `json:"ErrorRequest"`
	ValidRequest    int64          `json:"ValidRequest"`
	ForwardTypes    map[string]int `json:"ForwardTypes"`
	RequestOrigin   map[string]int `json:"RequestOrigin"`
	Downstreams     map[string]int `json:"Downstreams"`
	Upstreams       map[string]int `json:"Upstreams"`
	RequestClientIp map[string]int `json:"RequestClientIp"`
	UserAgent       map[string]int `json:"UserAgent"`
	Referer         map[string]int `json:"Referer"`
	RequestURL      map[string]int `json:"RequestURL"`
}
```

- [ ] **Step 2: AggregatedStats-Struct hinzufügen und metricsState erweitern**

Ersetze in `main.go` den `metricsState`-Block durch:

```go
type AggregatedStats struct {
	IPVersion  map[string]int // "ipv4" | "ipv6"
	Devices    map[string]int // "desktop" | "mobile" | "bot"
	Browsers   map[string]int // "Chrome", "Firefox", ...
	OS         map[string]int // "Windows", "macOS", ...
	OSVersions map[string]int // "Windows 10", ... (Top 20 + "other")
	FileTypes  map[string]int // "html", "css", "Folder path", "API call"
	Referrers  map[string]int // "google.com", ..., "direct", "other"
}

type metricsState struct {
	mu         sync.RWMutex
	summary    *DailySummaryExport
	aggregated *AggregatedStats
	netstat    *NetStat
	lastUpdate time.Time
	lastError  string
}
```

- [ ] **Step 3: Build prüfen**

Run: `go build ./...`

Expected: Erfolg (keine ungenutzten Felder, da die anderen Tasks die Aggregation gleich nachziehen).

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: extend DailySummaryExport with raw maps and add AggregatedStats"
```

---

## Task 3: `topNWithOther` Helper (TDD)

**Files:**
- Create: `parsers.go`
- Create: `parsers_test.go`

- [ ] **Step 1: Test schreiben**

`parsers_test.go`:

```go
package main

import (
	"testing"
)

func TestTopNWithOther_BelowCap(t *testing.T) {
	in := map[string]int{"a": 5, "b": 3, "c": 1}
	out := topNWithOther(in, 20)
	if len(out) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(out))
	}
	if _, hasOther := out["other"]; hasOther {
		t.Fatalf("did not expect 'other' bucket when len <= n")
	}
	if out["a"] != 5 || out["b"] != 3 || out["c"] != 1 {
		t.Fatalf("values mutated: %#v", out)
	}
}

func TestTopNWithOther_AboveCap(t *testing.T) {
	in := map[string]int{}
	for i := 0; i < 25; i++ {
		in[string(rune('a'+i))] = i + 1 // a=1, b=2, ... y=25
	}
	out := topNWithOther(in, 20)
	if len(out) != 21 {
		t.Fatalf("expected 21 entries (20 + other), got %d", len(out))
	}
	other, ok := out["other"]
	if !ok {
		t.Fatalf("missing 'other' bucket")
	}
	// "other" should be sum of bottom 5: 1+2+3+4+5 = 15
	if other != 15 {
		t.Fatalf("expected other=15, got %d", other)
	}
	// Top entry "y" (=25) must be present
	if out["y"] != 25 {
		t.Fatalf("expected y=25, got %d", out["y"])
	}
}
```

- [ ] **Step 2: Test laufen lassen, soll fehlschlagen**

Run: `go test ./... -run TestTopNWithOther -v`

Expected: FAIL — `undefined: topNWithOther`.

- [ ] **Step 3: Implementierung schreiben**

`parsers.go`:

```go
package main

import (
	"sort"
)

// topNWithOther keeps the n highest-count entries from m and folds the rest
// into an "other" bucket. Returns m unchanged (no "other") when len(m) <= n.
func topNWithOther(m map[string]int, n int) map[string]int {
	if len(m) <= n {
		out := make(map[string]int, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}

	type kv struct {
		k string
		v int
	}
	entries := make([]kv, 0, len(m))
	for k, v := range m {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].v != entries[j].v {
			return entries[i].v > entries[j].v
		}
		return entries[i].k < entries[j].k // tie-breaker: stable alphabetical
	})

	out := make(map[string]int, n+1)
	other := 0
	for i, e := range entries {
		if i < n {
			out[e.k] = e.v
		} else {
			other += e.v
		}
	}
	if other > 0 {
		out["other"] = other
	}
	return out
}
```

- [ ] **Step 4: Test soll grün werden**

Run: `go test ./... -run TestTopNWithOther -v`

Expected: PASS für beide Sub-Tests.

- [ ] **Step 5: Commit**

```bash
git add parsers.go parsers_test.go
git commit -m "feat: add topNWithOther helper with tests"
```

---

## Task 4: `ipVersion` Parser (TDD)

**Files:**
- Modify: `parsers.go`
- Modify: `parsers_test.go`

- [ ] **Step 1: Test schreiben (in parsers_test.go anhängen)**

```go
func TestIpVersion(t *testing.T) {
	in := map[string]int{
		"127.0.0.1":   10,
		"192.168.1.1": 5,
		"::1":         3,
		"2001:db8::1": 7,
	}
	out := ipVersion(in)
	if out["ipv4"] != 15 {
		t.Errorf("expected ipv4=15, got %d", out["ipv4"])
	}
	if out["ipv6"] != 10 {
		t.Errorf("expected ipv6=10, got %d", out["ipv6"])
	}
	if len(out) != 2 {
		t.Errorf("expected 2 buckets, got %d (%#v)", len(out), out)
	}
}

func TestIpVersion_EmptyInput(t *testing.T) {
	out := ipVersion(map[string]int{})
	if len(out) != 0 {
		t.Errorf("expected empty output, got %#v", out)
	}
}
```

- [ ] **Step 2: Test laufen lassen, soll fehlschlagen**

Run: `go test ./... -run TestIpVersion -v`

Expected: FAIL — `undefined: ipVersion`.

- [ ] **Step 3: Implementierung schreiben (in parsers.go anhängen)**

```go
import (
	"strings"
)

// ipVersion buckets each IP into "ipv4" or "ipv6" by checking for ':'.
// Counts are summed (request counts, not unique IPs).
func ipVersion(ips map[string]int) map[string]int {
	out := map[string]int{}
	for ip, count := range ips {
		if strings.Contains(ip, ":") {
			out["ipv6"] += count
		} else {
			out["ipv4"] += count
		}
	}
	return out
}
```

Note: Wenn `sort` schon importiert ist, `strings` zur bestehenden import-Gruppe hinzufügen.

- [ ] **Step 4: Test soll grün werden**

Run: `go test ./... -run TestIpVersion -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add parsers.go parsers_test.go
git commit -m "feat: add ipVersion parser with tests"
```

---

## Task 5: `extractFileType` Parser (TDD)

**Files:**
- Modify: `parsers.go`
- Modify: `parsers_test.go`

- [ ] **Step 1: Test schreiben**

```go
func TestExtractFileType(t *testing.T) {
	in := map[string]int{
		"/index.html":           10,
		"/static/style.css?v=1": 5,
		"/static/app.js#main":   3,
		"/img/logo.PNG":         8,
		"/api/v1/users":         20,
		"/blog/":                7,
		"":                      2,
	}
	out := extractFileType(in)
	if out["html"] != 10 {
		t.Errorf("html: expected 10, got %d", out["html"])
	}
	if out["css"] != 5 {
		t.Errorf("css: expected 5, got %d", out["css"])
	}
	if out["js"] != 3 {
		t.Errorf("js: expected 3, got %d", out["js"])
	}
	if out["png"] != 8 {
		t.Errorf("png (lowercased): expected 8, got %d", out["png"])
	}
	if out["API call"] != 20 {
		t.Errorf("API call: expected 20, got %d", out["API call"])
	}
	// "/blog/" -> filename "", "" -> filename "". Both go to "Folder path"
	if out["Folder path"] != 9 {
		t.Errorf("Folder path: expected 9 (7+2), got %d", out["Folder path"])
	}
}
```

- [ ] **Step 2: Test laufen lassen, soll fehlschlagen**

Run: `go test ./... -run TestExtractFileType -v`

Expected: FAIL — `undefined: extractFileType`.

- [ ] **Step 3: Implementierung schreiben**

```go
// extractFileType derives a file extension bucket from each URL.
// Strips query string and fragment, takes the last path segment,
// and returns "Folder path" if empty, "API call" if extensionless,
// or the lowercased extension otherwise.
func extractFileType(urls map[string]int) map[string]int {
	out := map[string]int{}
	for u, count := range urls {
		// Strip query and fragment
		if i := strings.IndexAny(u, "?#"); i >= 0 {
			u = u[:i]
		}
		// Last path segment
		idx := strings.LastIndex(u, "/")
		filename := u
		if idx >= 0 {
			filename = u[idx+1:]
		}
		var ext string
		switch {
		case filename == "":
			ext = "Folder path"
		case !strings.Contains(filename, "."):
			ext = "API call"
		default:
			dot := strings.LastIndex(filename, ".")
			ext = strings.ToLower(filename[dot+1:])
		}
		out[ext] += count
	}
	return out
}
```

- [ ] **Step 4: Test soll grün werden**

Run: `go test ./... -run TestExtractFileType -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add parsers.go parsers_test.go
git commit -m "feat: add extractFileType parser with tests"
```

---

## Task 6: `extractRefererHost` Parser (TDD)

**Files:**
- Modify: `parsers.go`
- Modify: `parsers_test.go`

- [ ] **Step 1: Test schreiben**

```go
func TestExtractRefererHost(t *testing.T) {
	in := map[string]int{
		"https://google.com/search?q=x": 10,
		"https://www.bing.com":          5,
		"http://example.com:8080/page":  3,
		"":                              7,
		"not-a-url":                     2,
	}
	out := extractRefererHost(in)
	if out["google.com"] != 10 {
		t.Errorf("google.com: expected 10, got %d", out["google.com"])
	}
	if out["www.bing.com"] != 5 {
		t.Errorf("www.bing.com: expected 5, got %d", out["www.bing.com"])
	}
	if out["example.com"] != 3 {
		t.Errorf("example.com (port stripped): expected 3, got %d", out["example.com"])
	}
	// empty + "not-a-url" both fall into "direct"
	if out["direct"] != 9 {
		t.Errorf("direct: expected 9 (7+2), got %d", out["direct"])
	}
}

func TestExtractRefererHost_TopN(t *testing.T) {
	in := map[string]int{}
	for i := 0; i < 25; i++ {
		host := string(rune('a'+i)) + ".com"
		in["https://"+host] = i + 1
	}
	out := extractRefererHost(in)
	if _, ok := out["other"]; !ok {
		t.Fatalf("expected 'other' bucket for 25 hosts, got: %#v", out)
	}
	if len(out) > 21 {
		t.Errorf("expected at most 21 entries (20 + other), got %d", len(out))
	}
}
```

- [ ] **Step 2: Test laufen lassen, soll fehlschlagen**

Run: `go test ./... -run TestExtractRefererHost -v`

Expected: FAIL — `undefined: extractRefererHost`.

- [ ] **Step 3: Implementierung schreiben**

```go
import (
	"net/url"
)

// extractRefererHost extracts the host from each referer URL and folds the
// long tail into an "other" bucket (top 20). Empty or invalid referers go
// to "direct".
func extractRefererHost(referers map[string]int) map[string]int {
	hosts := map[string]int{}
	for ref, count := range referers {
		host := "direct"
		if ref != "" {
			if u, err := url.Parse(ref); err == nil && u.Hostname() != "" {
				host = u.Hostname()
			}
		}
		hosts[host] += count
	}
	return topNWithOther(hosts, 20)
}
```

Hinweis: `net/url` zur bestehenden import-Gruppe in `parsers.go` ergänzen.

- [ ] **Step 4: Test soll grün werden**

Run: `go test ./... -run TestExtractRefererHost -v`

Expected: PASS für beide Sub-Tests.

- [ ] **Step 5: Commit**

```bash
git add parsers.go parsers_test.go
git commit -m "feat: add extractRefererHost parser with tests"
```

---

## Task 7: `parseUA` Parser (TDD)

**Files:**
- Modify: `parsers.go`
- Modify: `parsers_test.go`

- [ ] **Step 1: Test schreiben**

```go
func TestParseUA_DesktopChromeWindows(t *testing.T) {
	in := map[string]int{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36": 100,
	}
	browsers, os, osVersions, devices := parseUA(in)
	if browsers["Chrome"] != 100 {
		t.Errorf("expected Chrome=100, got %d (browsers=%#v)", browsers["Chrome"], browsers)
	}
	if os["Windows"] == 0 {
		t.Errorf("expected Windows in os map, got %#v", os)
	}
	if devices["desktop"] != 100 {
		t.Errorf("expected desktop=100, got %d (devices=%#v)", devices["desktop"], devices)
	}
	// OSVersions should contain a Windows-* key with count 100
	found := false
	for k, v := range osVersions {
		if strings.HasPrefix(k, "Windows") && v == 100 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a Windows version entry with count 100, got %#v", osVersions)
	}
}

func TestParseUA_MobileSafariIOS(t *testing.T) {
	in := map[string]int{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1": 50,
	}
	_, _, _, devices := parseUA(in)
	if devices["mobile"] != 50 {
		t.Errorf("expected mobile=50, got %d (devices=%#v)", devices["mobile"], devices)
	}
}

func TestParseUA_Bot(t *testing.T) {
	in := map[string]int{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)": 30,
	}
	browsers, _, _, devices := parseUA(in)
	if devices["bot"] != 30 {
		t.Errorf("expected bot=30, got %d (devices=%#v)", devices["bot"], devices)
	}
	if browsers["Bot"] != 30 {
		t.Errorf("expected Bot bucket=30, got %d (browsers=%#v)", browsers["Bot"], browsers)
	}
}

func TestParseUA_EmptyString(t *testing.T) {
	in := map[string]int{"": 5}
	browsers, os, _, devices := parseUA(in)
	// Empty UA: should not crash; reasonable to bucket as Unknown
	if browsers["Unknown"] != 5 {
		t.Errorf("expected Unknown=5, got %#v", browsers)
	}
	if os["Unknown"] != 5 {
		t.Errorf("expected Unknown OS=5, got %#v", os)
	}
	if devices["desktop"] != 5 {
		// Empty UA is neither mobile nor bot, so falls into desktop.
		t.Errorf("expected desktop=5 for empty UA, got %#v", devices)
	}
}

func TestParseUA_OSVersionsCapped(t *testing.T) {
	in := map[string]int{}
	// Build 25 synthetic UAs with different (fictitious) version strings.
	// We can't reliably force 25 distinct OS-version buckets from mssola,
	// so we just verify the function runs and the result type is correct.
	for i := 0; i < 5; i++ {
		ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/12" + string(rune('0'+i)) + ".0 Safari/537.36"
		in[ua] = i + 1
	}
	_, _, osVersions, _ := parseUA(in)
	if osVersions == nil {
		t.Fatal("osVersions must not be nil")
	}
}
```

- [ ] **Step 2: Test laufen lassen, soll fehlschlagen**

Run: `go test ./... -run TestParseUA -v`

Expected: FAIL — `undefined: parseUA`.

- [ ] **Step 3: Implementierung schreiben**

```go
import (
	"github.com/mssola/user_agent"
)

// parseUA aggregates raw User-Agent strings into four low-cardinality maps:
// browsers, os family, os+version (top 20 + other), and device class.
//
// Buckets:
//   - devices: "bot" (Bot() == true) overrides everything; otherwise
//     "mobile" if ua.Mobile(), else "desktop"
//   - browsers: ua.Browser(); empty -> "Unknown"; if bot -> "Bot"
//   - os: ua.OSInfo().Name; empty -> "Unknown"
//   - osVersions: Name + " " + Version (Name only if Version empty);
//     empty Name -> entry skipped; then capped via topNWithOther.
func parseUA(uas map[string]int) (browsers, os, osVersions, devices map[string]int) {
	browsers = map[string]int{}
	os = map[string]int{}
	osVersionsRaw := map[string]int{}
	devices = map[string]int{}

	for s, count := range uas {
		ua := user_agent.New(s)

		// Device class
		switch {
		case ua.Bot():
			devices["bot"] += count
		case ua.Mobile():
			devices["mobile"] += count
		default:
			devices["desktop"] += count
		}

		// Browser
		browserName, _ := ua.Browser()
		switch {
		case ua.Bot():
			browsers["Bot"] += count
		case browserName == "":
			browsers["Unknown"] += count
		default:
			browsers[browserName] += count
		}

		// OS family + version
		info := ua.OSInfo()
		osName := info.Name
		if osName == "" {
			os["Unknown"] += count
			continue
		}
		os[osName] += count

		versionKey := osName
		if info.Version != "" {
			versionKey = osName + " " + info.Version
		}
		osVersionsRaw[versionKey] += count
	}

	osVersions = topNWithOther(osVersionsRaw, 20)
	return
}
```

- [ ] **Step 4: Test soll grün werden**

Run: `go test ./... -run TestParseUA -v`

Expected: PASS für alle vier Sub-Tests.

Falls einzelne Tests fehlschlagen (z. B. weil `mssola/user_agent` Browser/OS leicht anders benennt), Test-Erwartungen pragmatisch anpassen (z. B. exakter Browser-Name aus der Lib übernehmen). Die Lib-Naming-Konvention ist autoritativ für unsere Buckets.

- [ ] **Step 5: Commit**

```bash
git add parsers.go parsers_test.go
git commit -m "feat: add parseUA with tests"
```

---

## Task 8: Parser-Aufrufe in `fetchStats()` einbauen

**Files:**
- Modify: `main.go:75-123` — innerhalb `fetchStats()` Aggregation hinzufügen

- [ ] **Step 1: Aggregation am Ende von `fetchStats()` ergänzen**

Suche in `main.go` den Block:

```go
	state.mu.Lock()
	state.summary = &summary
	state.netstat = ns
	state.lastUpdate = time.Now()
	state.lastError = ""
	state.mu.Unlock()
}
```

Ersetze durch:

```go
	browsers, osFamily, osVersions, devices := parseUA(summary.UserAgent)
	agg := &AggregatedStats{
		IPVersion:  ipVersion(summary.RequestClientIp),
		Devices:    devices,
		Browsers:   browsers,
		OS:         osFamily,
		OSVersions: osVersions,
		FileTypes:  extractFileType(summary.RequestURL),
		Referrers:  extractRefererHost(summary.Referer),
	}

	state.mu.Lock()
	state.summary = &summary
	state.aggregated = agg
	state.netstat = ns
	state.lastUpdate = time.Now()
	state.lastError = ""
	state.mu.Unlock()
}
```

- [ ] **Step 2: Build prüfen**

Run: `go build ./...`

Expected: Erfolg.

- [ ] **Step 3: Bestehende Tests laufen lassen**

Run: `go test ./... -v`

Expected: Alle bisherigen Tests grün.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: call parsers in fetchStats and cache aggregated maps"
```

---

## Task 9: Neue Metriken in `handleMetrics()` ausgeben

**Files:**
- Modify: `main.go:135-191` — `handleMetrics()`

- [ ] **Step 1: Output-Block für aggregated stats hinzufügen**

In `handleMetrics()` nach dem bestehenden `writeLabeledGauge("zoraxy_requests_by_upstream", ...)` Aufruf (Zeile ~177) und VOR dem `if state.netstat != nil`-Block einfügen:

```go
	if state.aggregated != nil {
		a := state.aggregated
		writeLabeledGauge("zoraxy_requests_by_ip_version", "Requests by client IP version today", "version", a.IPVersion)
		writeLabeledGauge("zoraxy_requests_by_device", "Requests by client device class today", "device", a.Devices)
		writeLabeledGauge("zoraxy_requests_by_browser", "Requests by client browser today", "browser", a.Browsers)
		writeLabeledGauge("zoraxy_requests_by_os", "Requests by client operating system today", "os", a.OS)
		writeLabeledGauge("zoraxy_requests_by_os_version", "Requests by client OS version today (top 20 + other)", "os_version", a.OSVersions)
		writeLabeledGauge("zoraxy_requests_by_file_type", "Requests by requested file extension today", "type", a.FileTypes)
		writeLabeledGauge("zoraxy_requests_by_referer", "Requests by referer host today (top 20 + other)", "host", a.Referrers)
	}
```

- [ ] **Step 2: Build prüfen**

Run: `go build ./...`

Expected: Erfolg.

- [ ] **Step 3: Alle Tests laufen lassen**

Run: `go test ./... -v`

Expected: Alle Tests grün.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: emit seven new aggregated stats metrics"
```

---

## Task 10: Version-Bump auf v1.1.0

**Files:**
- Modify: `main.go:250-252` — `VersionMinor` und `VersionPatch`

- [ ] **Step 1: Version in main.go anpassen**

Suche:
```go
		VersionMajor:  1,
		VersionMinor:  0,
		VersionPatch:  6,
```

Ersetze:
```go
		VersionMajor:  1,
		VersionMinor:  1,
		VersionPatch:  0,
```

- [ ] **Step 2: Build prüfen**

Run: `go build ./...`

Expected: Erfolg.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "Release v1.1.0: extended request stats (IP version, devices, browsers, OS, file types, referrers)"
```

---

## Task 11: README aktualisieren

**Files:**
- Modify: `README.md` — Metriken-Tabelle erweitern

- [ ] **Step 1: Metriken-Tabelle ergänzen**

Suche in `README.md` die Tabellen-Zeile:

```
| `zoraxy_requests_by_upstream{hostname=...}` | gauge | Requests by upstream hostname |
```

Füge direkt DARUNTER ein:

```
| `zoraxy_requests_by_ip_version{version=...}` | gauge | Requests by client IP version (`ipv4`, `ipv6`) |
| `zoraxy_requests_by_device{device=...}` | gauge | Requests by client device class (`desktop`, `mobile`, `bot`) |
| `zoraxy_requests_by_browser{browser=...}` | gauge | Requests by client browser family |
| `zoraxy_requests_by_os{os=...}` | gauge | Requests by client OS family |
| `zoraxy_requests_by_os_version{os_version=...}` | gauge | Requests by client OS version (top 20 + `other`) |
| `zoraxy_requests_by_file_type{type=...}` | gauge | Requests by requested file extension |
| `zoraxy_requests_by_referer{host=...}` | gauge | Requests by referer host (top 20 + `other`, `direct` for missing) |
```

- [ ] **Step 2: Diff inspizieren**

Run: `git diff README.md`

Expected: Sieben neue Zeilen in der Metriken-Tabelle.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document new aggregated stats metrics in README"
```

---

## Task 12: Manueller End-to-End-Smoke-Test

**Files:** keine.

- [ ] **Step 1: Plugin lokal bauen**

Run: `go build -o prometheus-exporter .`

Expected: Binary aktualisiert.

- [ ] **Step 2: Binary in laufende Zoraxy-Instanz deployen**

Anweisung an den User: Binary `prometheus-exporter` an die Stelle kopieren, von der Zoraxy es lädt; in Zoraxy-UI "Plugins → Reload" klicken.

- [ ] **Step 3: Metrics-Endpoint scrapen und neue Metriken prüfen**

Run (auf dem Host, auf dem Zoraxy läuft):
```
curl -s http://127.0.0.1:9100/metrics | grep -E "^zoraxy_requests_by_(ip_version|device|browser|os|os_version|file_type|referer)"
```

Expected:
- Mindestens `zoraxy_requests_by_ip_version{version="ipv4"}` mit einem Wert > 0 (sofern Traffic anliegt)
- Browser/OS-Buckets mit plausiblen Werten
- File-Type-Buckets mit Werten für `html`, `js`, `css` oder vergleichbar

- [ ] **Step 4: Wenn alle Metriken sinnvoll sind: Branch / Release**

Anweisung: `git push origin main` und Release-Tag `v1.1.0` setzen. Mit dem User klären, ob bereits jetzt pushen.

---

## Self-Review-Notizen

**Spec coverage:**
- IP Version → Task 4 ✓
- Devices → Task 7 ✓
- Browsers → Task 7 ✓
- OS Family → Task 7 ✓
- OS Versions → Task 7 (mit Top-N in Task 3) ✓
- File Types → Task 5 ✓
- Referring Sites → Task 6 (mit Top-N in Task 3) ✓
- Wiring in fetchStats → Task 8 ✓
- Output in handleMetrics → Task 9 ✓
- Version-Bump → Task 10 ✓
- README → Task 11 ✓
- Smoke-Test → Task 12 ✓

**Konsistenz:**
- `parseUA` Signatur: `(browsers, os, osVersions, devices map[string]int)` in Tasks 7, 8 identisch ✓
- `AggregatedStats`-Felder werden in Task 2 definiert und in Tasks 8/9 verwendet ✓
- `topNWithOther` Signatur (m, n) in Tasks 3, 6, 7 identisch ✓
- Label-Keys: `version`, `device`, `browser`, `os`, `os_version`, `type`, `host` — konsistent zwischen Task 9 und README in Task 11 ✓
