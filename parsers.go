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
