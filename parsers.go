package main

import (
	"net/url"
	"sort"
	"strings"

	"github.com/mssola/user_agent"
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
