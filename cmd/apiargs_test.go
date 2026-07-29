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

func TestParseWhereFilters(t *testing.T) {
	got, err := parseWhereFilters([]string{"name=ether1", "disabled=false", "?chain=forward", "?=address=1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"?name=ether1", "?disabled=false", "?chain=forward", "?=address=1.1.1.1"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
	_, err = parseWhereFilters([]string{"nofilter"})
	if err == nil {
		t.Fatal("expected error for missing =")
	}
}

func TestNormalizePath(t *testing.T) {
	if got := normalizePath("ip/address/"); got != "/ip/address" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePath("/interface/print"); got != "/interface" {
		t.Fatalf("strip print: got %q", got)
	}
	if got := normalizePath("/ip/address/get"); got != "/ip/address" {
		t.Fatalf("strip get: got %q", got)
	}
}

func TestResolveResourcePath_StripsPrint(t *testing.T) {
	path, rest, err := resolveResourcePath([]string{"/interface/print"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/interface" {
		t.Fatalf("path=%q want /interface", path)
	}
	if len(rest) != 0 {
		t.Fatalf("rest=%v", rest)
	}
	if got := pathCommand(path, "print"); got != "/interface/print" {
		t.Fatalf("pathCommand=%q", got)
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
