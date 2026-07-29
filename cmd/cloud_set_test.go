package cmd

import (
	"strings"
	"testing"
)

func TestNormalizeCloudDDNSArgs_NoError(t *testing.T) {
	_, tip, err := normalizeCloudDDNSArgs("/ip/cloud", []string{"=ddns-enabled=no", "=update-time=false"})
	if err == nil {
		t.Fatal("expected error for ddns-enabled=no")
	}
	if tip != "" {
		t.Fatalf("tip should be empty on error, got %q", tip)
	}
	if !strings.Contains(err.Error(), "yes|auto") {
		t.Fatalf("error should mention yes|auto: %v", err)
	}

	_, _, err = normalizeCloudDDNSArgs("/ip/cloud", []string{"=ddns-enabled=NO"})
	if err == nil {
		t.Fatal("expected error for ddns-enabled=NO")
	}
}

func TestNormalizeCloudDDNSArgs_FalseToAuto(t *testing.T) {
	out, tip, err := normalizeCloudDDNSArgs("ip/cloud", []string{"=ddns-enabled=false", "=update-time=false"})
	if err != nil {
		t.Fatal(err)
	}
	if tip == "" {
		t.Fatal("expected tip for false→auto")
	}
	if len(out) != 2 || out[0] != "=ddns-enabled=auto" || out[1] != "=update-time=false" {
		t.Fatalf("got %v", out)
	}
	out, tip, err = normalizeCloudDDNSArgs("/ip/cloud", []string{"=ddns-enabled=FALSE"})
	if err != nil || tip == "" || out[0] != "=ddns-enabled=auto" {
		t.Fatalf("FALSE: out=%v tip=%q err=%v", out, tip, err)
	}
}

func TestNormalizeCloudDDNSArgs_Passthrough(t *testing.T) {
	in := []string{"=ddns-enabled=auto", "=update-time=false"}
	out, tip, err := normalizeCloudDDNSArgs("/ip/cloud", in)
	if err != nil || tip != "" {
		t.Fatalf("err=%v tip=%q", err, tip)
	}
	if len(out) != 2 || out[0] != in[0] {
		t.Fatalf("got %v", out)
	}
	// Non-cloud path unchanged.
	out, tip, err = normalizeCloudDDNSArgs("/ip/address", []string{"=ddns-enabled=no"})
	if err != nil || tip != "" || out[0] != "=ddns-enabled=no" {
		t.Fatalf("non-cloud: out=%v tip=%q err=%v", out, tip, err)
	}
}
