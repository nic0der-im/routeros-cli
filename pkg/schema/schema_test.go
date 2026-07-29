package schema

import "testing"

func TestListNonEmpty(t *testing.T) {
	names := List()
	if len(names) == 0 {
		t.Fatal("expected registered schemas")
	}
}

func TestGetKnown(t *testing.T) {
	s, ok := Get("system-info")
	if !ok || s.Type == "" {
		t.Fatal("expected system-info schema")
	}
}

func TestGetUnknown(t *testing.T) {
	if _, ok := Get("does-not-exist"); ok {
		t.Fatal("expected miss")
	}
}

func TestEnvelopeSchema(t *testing.T) {
	s := EnvelopeSchema()
	if s.Type != "object" {
		t.Fatalf("type = %q", s.Type)
	}
}
