package main

import (
	"sort"
	"strings"
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
