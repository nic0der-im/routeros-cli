package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// mockRenderable is a test double implementing Renderable.
type mockRenderable struct {
	headers []string
	rows    [][]string
}

func (m *mockRenderable) TableHeaders() []string { return m.headers }
func (m *mockRenderable) TableRows() [][]string   { return m.rows }

func newMockData() *mockRenderable {
	return &mockRenderable{
		headers: []string{"name", "address", "disabled"},
		rows: [][]string{
			{"ether1", "192.168.1.1/24", "false"},
			{"ether2", "10.0.0.1/24", "true"},
		},
	}
}

// --- ParseFormat ---

func TestParseFormat_Table(t *testing.T) {
	f, err := ParseFormat("table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != FormatTable {
		t.Fatalf("expected %q, got %q", FormatTable, f)
	}
}

func TestParseFormat_JSON(t *testing.T) {
	f, err := ParseFormat("json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != FormatJSON {
		t.Fatalf("expected %q, got %q", FormatJSON, f)
	}
}

func TestParseFormat_Invalid(t *testing.T) {
	_, err := ParseFormat("xml")
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Fatalf("error should mention the invalid format, got: %v", err)
	}
}

// --- RenderTable ---

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	data := newMockData()

	if err := RenderTable(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (1 header + 2 rows), got %d:\n%s", len(lines), out)
	}

	// Header line must be uppercase.
	header := lines[0]
	if !strings.Contains(header, "NAME") || !strings.Contains(header, "ADDRESS") || !strings.Contains(header, "DISABLED") {
		t.Fatalf("header should contain uppercase column names, got: %q", header)
	}

	// Data rows must contain the values.
	if !strings.Contains(lines[1], "ether1") || !strings.Contains(lines[1], "192.168.1.1/24") {
		t.Fatalf("first data row missing expected values: %q", lines[1])
	}
	if !strings.Contains(lines[2], "ether2") || !strings.Contains(lines[2], "10.0.0.1/24") {
		t.Fatalf("second data row missing expected values: %q", lines[2])
	}
}

func TestRenderTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	data := &mockRenderable{
		headers: []string{"name"},
		rows:    [][]string{},
	}

	if err := RenderTable(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 header-only line, got %d", len(lines))
	}
}

// --- RenderJSON ---

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	data := newMockData()
	meta := Meta{
		Device:    "router1",
		Command:   "/interface/print",
		Timestamp: "2026-03-16T12:00:00Z",
		Count:     2,
	}

	if err := RenderJSON(&buf, data, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if resp.Meta.Device != "router1" {
		t.Fatalf("expected device=router1, got %q", resp.Meta.Device)
	}
	if resp.Meta.Command != "/interface/print" {
		t.Fatalf("expected command=/interface/print, got %q", resp.Meta.Command)
	}
	if resp.Meta.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Meta.Count)
	}

	// Data should be an array of objects.
	records, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", resp.Data)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	first, ok := records[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected record to be a map, got %T", records[0])
	}
	if first["name"] != "ether1" {
		t.Fatalf("expected name=ether1, got %v", first["name"])
	}
	if first["address"] != "192.168.1.1/24" {
		t.Fatalf("expected address=192.168.1.1/24, got %v", first["address"])
	}
}

// --- RenderError ---

func TestRenderError(t *testing.T) {
	var buf bytes.Buffer

	if err := RenderError(&buf, "CONNECTION_FAILED", "dial tcp: connection refused", "router1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Device  string `json:"device"`
		} `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if resp.Error.Code != "CONNECTION_FAILED" {
		t.Fatalf("expected code=CONNECTION_FAILED, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "dial tcp: connection refused" {
		t.Fatalf("expected message='dial tcp: connection refused', got %q", resp.Error.Message)
	}
	if resp.Error.Device != "router1" {
		t.Fatalf("expected device=router1, got %q", resp.Error.Device)
	}
}

// --- Render dispatcher ---

func TestRender_Table(t *testing.T) {
	var buf bytes.Buffer
	data := newMockData()
	meta := Meta{Device: "router1", Command: "/test", Timestamp: "now", Count: 2}

	if err := Render(&buf, FormatTable, data, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "NAME") {
		t.Fatal("table output should contain uppercase headers")
	}
}

func TestRender_JSON(t *testing.T) {
	var buf bytes.Buffer
	data := newMockData()
	meta := Meta{Device: "router1", Command: "/test", Timestamp: "now", Count: 2}

	if err := Render(&buf, FormatJSON, data, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
}

func TestRender_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	data := newMockData()
	meta := Meta{}

	err := Render(&buf, Format("yaml"), data, meta)
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}
