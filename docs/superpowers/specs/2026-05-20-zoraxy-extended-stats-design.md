# Zoraxy Prometheus Exporter — Erweiterte Statistiken

**Datum:** 2026-05-20
**Autor:** reptil1990 (mit Claude)
**Status:** Draft

## Ziel

Der Exporter soll sieben weitere Statistiken anbieten, die in der Zoraxy-Weboberfläche sichtbar sind, aber bisher nicht als Prometheus-Metrik exportiert werden:

1. Requests IP Version (IPv4 / IPv6)
2. Client Devices (Desktop / Mobile / Bot)
3. Client Browsers (Chrome, Firefox, Safari, ...)
4. Client OS (Windows, macOS, Linux, iOS, Android, ...)
5. OS Versions (Windows 10, macOS 14.2, ...)
6. Request File Types (html, css, png, ...)
7. Referring Sites (Top-Domains der Referer)

"Request Origins" aus der UI ist bereits durch `zoraxy_requests_by_country` abgedeckt und wird nicht erneut exportiert.

## Datenquelle

`GET /plugin/api/stats/summary` (das ohne `?fast=true` aufrufen — die `fast=true`-Variante liefert nur die drei Totals und keine Maps).

Relevante Felder im Response-JSON (alle `map[string]int`, String → Count):

- `RequestClientIp` — rohe IP-Adressen
- `UserAgent` — rohe User-Agent-Header
- `RequestURL` — rohe Request-URLs
- `Referer` — rohe Referer-Header

Diese Felder hat der bestehende Exporter bisher bewusst ignoriert, weil ein 1:1-Export aus Cardinality-Gründen unbrauchbar wäre. Dieser Spec aggregiert sie zu niedriger Cardinality, bevor sie als Metrik herausgereicht werden.

## Architektur

Drei Dateien:

- `main.go` — bisher schon vorhanden; bekommt Aufrufe der Parser eingebaut, ansonsten unverändert in Struktur
- `parsers.go` — neue Datei, enthält alle Parsing- und Aggregations-Helper
- `parsers_test.go` — Unit-Tests pro Parser

`fetchStats()` parst die rohen Maps **einmal pro Poll-Zyklus** (alle 30 s) zu aggregierten Maps und cached die zusammen mit den Roh-Totals im `metricsState`. `handleMetrics()` gibt sie nur noch unverändert raus. Damit zahlen wir die Parsing-Kosten 1× pro Polling-Intervall, nicht pro Prometheus-Scrape.

```
+----------------+    poll 30s    +-----------------+
|  Zoraxy API    | <------------- |   fetchStats()  |
| /summary       | --- JSON --->  |  - parse maps   |
+----------------+                |  - aggregate    |
                                  +--------+--------+
                                           | writes
                                           v
                                  +-----------------+
                                  |  metricsState   |
                                  | (aggregated)    |
                                  +--------+--------+
                                           | reads
                                           v
                                  +-----------------+
                                  | handleMetrics() |
                                  | -> text output  |
                                  +-----------------+
```

## State-Erweiterung

`metricsState` bekommt ein zusätzliches Feld `aggregated *AggregatedStats`:

```go
type AggregatedStats struct {
    IPVersion   map[string]int // "ipv4" | "ipv6"
    Devices     map[string]int // "desktop" | "mobile" | "bot"
    Browsers    map[string]int // "Chrome", "Firefox", ...
    OS          map[string]int // "Windows", "macOS", ...
    OSVersions  map[string]int // "Windows 10", ... (Top 20 + "other")
    FileTypes   map[string]int // "html", "css", "Folder path", "API call"
    Referrers   map[string]int // "google.com", ..., "direct", "other"
}
```

`DailySummaryExport` wird um die vier Roh-Maps erweitert (die bisher beim Unmarshal ignoriert wurden):

```go
RequestClientIp map[string]int
UserAgent       map[string]int
Referer         map[string]int
RequestURL      map[string]int
```

## Parser-Spezifikation

### `ipVersion(ips map[string]int) map[string]int`

Iteriert über die Map, bucket-Schlüssel:
- enthält `:` → `"ipv6"`
- sonst → `"ipv4"`

Die Counts werden summiert (Request-Counts, nicht unique IPs). **Bewusste Abweichung von der Zoraxy-UI**, die unique IPs zählt — bei uns ist die Request-Verteilung das richtige Maß.

### `parseUA(uas map[string]int) (browsers, os, osVersions, devices map[string]int)`

Benutzt `github.com/mssola/user_agent` (neue Dependency in `go.mod`).

Pro UA-String:
```go
ua := user_agent.New(s)
browser, _ := ua.Browser()       // "Chrome", "Firefox", ...
osInfo := ua.OSInfo()            // OSInfo.Name, OSInfo.Version
isMobile := ua.Mobile()
isBot := ua.Bot()
```

Mapping:
- **Devices**: `bot` → `"bot"`, sonst `mobile` → `"mobile"`, sonst `"desktop"`
- **Browsers**: `browser` direkt; leer → `"Unknown"`; bei Bot → `"Bot"`
- **OS**: `osInfo.Name` direkt; leer → `"Unknown"`
- **OS-Versionen**: `osInfo.Name + " " + osInfo.Version`, leere Version → nur Name; leerer Name → übersprungen. Danach `topNWithOther(map, 20)`.

### `extractFileType(urls map[string]int) map[string]int`

Pro URL:
1. Query-String und Fragment abschneiden (alles ab `?` bzw. `#`)
2. `filename := lastPathSegment(url)` — alles nach dem letzten `/`
3. Wenn `filename == ""` → `"Folder path"`
4. Wenn `!strings.Contains(filename, ".")` → `"API call"`
5. Sonst → alles nach dem letzten `.`, lowercased

Counts werden über alle URLs mit gleicher Extension summiert.

### `extractRefererHost(referers map[string]int) map[string]int`

Pro Referer:
1. Leerer String → `"direct"`
2. `url.Parse(s)` — bei Fehler oder leerem Host → `"direct"`
3. Sonst → `u.Hostname()` (ohne Port)

Danach `topNWithOther(map, 20)`.

### `topNWithOther(m map[string]int, n int) map[string]int`

Sortiert die Map nach Count absteigend, behält die Top-N, sammelt den Rest in `"other"`. Wenn `len(m) <= n`, wird die Map unverändert zurückgegeben (kein `"other"`-Bucket).

## Neue Prometheus-Metriken

Alle gauges, Tagesreset wie die bestehenden:

| Metrik | Label | Quelle |
|---|---|---|
| `zoraxy_requests_by_ip_version` | `version` | `ipVersion()` |
| `zoraxy_requests_by_device` | `device` | `parseUA()` Devices |
| `zoraxy_requests_by_browser` | `browser` | `parseUA()` Browsers |
| `zoraxy_requests_by_os` | `os` | `parseUA()` OS |
| `zoraxy_requests_by_os_version` | `os_version` | `parseUA()` OSVersions |
| `zoraxy_requests_by_file_type` | `type` | `extractFileType()` |
| `zoraxy_requests_by_referer` | `host` | `extractRefererHost()` |

Output-Logik: Wiederverwendung des bestehenden `writeLabeledGauge(...)`-Helpers aus `main.go`.

## Cardinality-Schutz

- **Top-N (= 20) + `"other"`** für `Referrers` und `OSVersions`.
- Browsers / OS / Devices / IP-Version / FileTypes sind von Natur aus niedrige-Cardinality (wenige Dutzend Werte max.).
- Leere Schlüssel werden zu `"unknown"` (wie bisher schon in `writeLabeledGauge`).

## Error-Handling

- API-Fehler werden wie bisher in `state.lastError` geloggt; aggregierte Maps bleiben dann beim letzten guten Snapshot.
- UA-Parsing-Fehler treten praktisch nicht auf (`user_agent.New` ignoriert ungültige Strings), aber für den Fall, dass Browser/OS leer zurückkommen, wird `"Unknown"` als Label gesetzt.
- Ungültige URL-Strings in `extractRefererHost` / `extractFileType` fallen in `"direct"` bzw. `"API call"`.

## Tests

`parsers_test.go` enthält Tabellen-Tests pro Parser:

- **IP-Version**: `127.0.0.1`, `::1`, `2001:db8::1`, `192.168.1.1`, leerer String
- **UA**: Beispiele für Chrome/Win, Firefox/Linux, Safari/iOS, Chrome/Android, GoogleBot, leerer String
- **File-Type**: `/index.html`, `/static/style.css?v=1`, `/api/v1/users`, `/blog/`, `/img/foo.png`
- **Referer**: `https://google.com/search?q=x`, `https://bing.com`, leerer String, `not-a-url`
- **topNWithOther**: Map mit 3 Einträgen bei n=20 (kein `other`), Map mit 25 Einträgen bei n=20 (mit `other`)

## Versionierung & Doku

- `VersionMinor` von 0 auf 1 erhöhen, `VersionPatch` zurück auf 0 → **v1.1.0**
- README-Tabelle um die sieben neuen Metriken erweitern
- Beispiel-Output-Snippet in der README aktualisieren

## Out-of-Scope

- Trafficmap-Endpoint (`/api/stats/trafficmap`) — separater Spec, wenn überhaupt
- Netstatgraph (`/api/stats/netstatgraph?array=true`) — bestehende Netstat-Metriken reichen
- Per-Hostname-Breakdown (z. B. Browsers pro Downstream-Hostname) — würde Cardinality multiplizieren
- Persistente Top-N-Snapshots über mehrere Polls hinweg — beim Tagesreset von Zoraxy starten wir sowieso bei 0
