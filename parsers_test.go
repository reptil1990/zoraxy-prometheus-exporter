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
