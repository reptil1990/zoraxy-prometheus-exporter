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
