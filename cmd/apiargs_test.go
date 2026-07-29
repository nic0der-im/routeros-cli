package cmd

import "testing"

func TestParseAPIArgs(t *testing.T) {
	got := parseAPIArgs([]string{"chain=forward", "=.id=*1", "?disabled=false", "?=address=1.1.1.1"})
	want := []string{"=chain=forward", "=.id=*1", "?disabled=false", "?=address=1.1.1.1"}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizePath(t *testing.T) {
	if got := normalizePath("ip/address/"); got != "/ip/address" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveResourcePath(t *testing.T) {
	path, rest, err := resolveResourcePath([]string{"firewall", "filter", "?=disabled=false"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/ip/firewall/filter" {
		t.Fatalf("path=%q", path)
	}
	if len(rest) != 1 || rest[0] != "?=disabled=false" {
		t.Fatalf("rest=%v", rest)
	}
}
