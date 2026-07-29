package device

import "testing"

func TestLookupByID(t *testing.T) {
	inv := newTestInventory(t)
	dev := sampleDevice("192.168.1.1:8728")
	dev.ID = "eoc-frontera"
	if err := inv.Add("EOC FRONTERA", dev); err != nil {
		t.Fatalf("Add: %v", err)
	}

	name, got, err := inv.Lookup("eoc-frontera")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if name != "EOC FRONTERA" {
		t.Errorf("name = %q", name)
	}
	if got.ID != "eoc-frontera" {
		t.Errorf("id = %q", got.ID)
	}
}

func TestLookupByIP(t *testing.T) {
	inv := newTestInventory(t)
	if err := inv.Add("lab", sampleDevice("10.0.0.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	name, _, err := inv.Lookup("10.0.0.1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if name != "lab" {
		t.Errorf("name = %q", name)
	}
}

func TestLookupAmbiguousIP(t *testing.T) {
	inv := newTestInventory(t)
	_ = inv.Add("a", sampleDevice("10.0.0.1:8728"))
	_ = inv.Add("b", sampleDevice("10.0.0.1:8729"))

	if _, _, err := inv.Lookup("10.0.0.1"); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestInferTLS(t *testing.T) {
	if InferTLS("192.168.88.1:8728", false, true) != false {
		t.Error("8728 should infer tls=false")
	}
	if InferTLS("192.168.88.1:8729", false, false) != true {
		t.Error("8729 should infer tls=true")
	}
	if InferTLS("192.168.88.1:8728", true, true) != true {
		t.Error("explicit tls=true should win")
	}
}

func TestResolveByID(t *testing.T) {
	inv := newTestInventory(t)
	dev := sampleDevice("10.1.1.1:8728")
	dev.ID = "edge"
	_ = inv.Add("Edge Router", dev)

	name, _, err := inv.Resolve("edge")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Edge Router" {
		t.Errorf("got %q", name)
	}
}
