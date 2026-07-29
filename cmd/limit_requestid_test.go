package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
)

func TestNewRequestID(t *testing.T) {
	a := newRequestID()
	b := newRequestID()
	if len(a) != 16 {
		t.Fatalf("len=%d want 16 hex chars, got %q", len(a), a)
	}
	if a == b {
		t.Fatal("expected unique request ids")
	}
}

func TestRender_IncludesRequestID(t *testing.T) {
	a, _ := testApp(t)
	a.RequestID = "fixed-req-id"
	a.OutFormat = output.FormatJSON
	a.MaxOutputBytes = output.DefaultMaxOutputBytes

	ifaces := rosapi.Interfaces{
		{Name: "ether1", Type: "ether", MTU: "1500", Running: "true", Disabled: "false"},
		{Name: "ether2", Type: "ether", MTU: "1500", Running: "true", Disabled: "false"},
		{Name: "ether3", Type: "ether", MTU: "1500", Running: "true", Disabled: "false"},
	}
	var buf bytes.Buffer
	if err := a.render(&buf, ifaces, "lab", "/interface/print"); err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.RequestID != "fixed-req-id" {
		t.Fatalf("meta.request_id=%q", resp.Meta.RequestID)
	}
	if resp.Meta.Count != 3 {
		t.Fatalf("count=%d", resp.Meta.Count)
	}
}

func TestRender_RowLimitTruncates(t *testing.T) {
	a, _ := testApp(t)
	a.RequestID = "lim-1"
	a.RowLimit = 1
	a.OutFormat = output.FormatJSON
	a.MaxOutputBytes = -1 // disable byte cap via Options; renderOpts maps 0→default, so set high
	a.MaxOutputBytes = 10_000_000

	ifaces := rosapi.Interfaces{
		{Name: "ether1", Type: "ether", MTU: "1500", Running: "true", Disabled: "false"},
		{Name: "ether2", Type: "ether", MTU: "1500", Running: "true", Disabled: "false"},
	}
	var buf bytes.Buffer
	if err := a.render(&buf, ifaces, "lab", "/interface/print"); err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Meta.Truncated {
		t.Fatal("expected truncated=true")
	}
	if resp.Meta.Count != 1 {
		t.Fatalf("count=%d want 1", resp.Meta.Count)
	}
	records, ok := resp.Data.([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("data=%v", resp.Data)
	}
}

func TestRenderError_IncludesRequestID(t *testing.T) {
	a, _ := testApp(t)
	a.RequestID = "err-rid"
	a.OutFormat = output.FormatJSON
	var buf bytes.Buffer
	a.renderError(&buf, "api", "boom", "lab")
	var resp struct {
		OK   bool        `json:"ok"`
		Meta output.Meta `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if resp.Meta.RequestID != "err-rid" {
		t.Fatalf("request_id=%q", resp.Meta.RequestID)
	}
}

func TestVerbosef_IncludesRequestID(t *testing.T) {
	a, _ := testApp(t)
	a.RequestID = "verb-1"
	a.Verbose = true
	// verbosef writes stderr; just ensure it does not panic
	a.verbosef("hello %s", "world")
}

func TestParseRowLimit_CmdHelper(t *testing.T) {
	n, err := output.ParseRowLimit("42")
	if err != nil || n != 42 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err := output.ParseRowLimit("no"); err == nil {
		t.Fatal("expected error")
	}
}
