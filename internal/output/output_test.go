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
func (m *mockRenderable) TableRows() [][]string  { return m.rows }

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

	if err := RenderJSON(&buf, data, meta, Options{}); err != nil {
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
	meta := Meta{RequestID: "abc123", Device: "router1"}

	if err := RenderError(&buf, "CONNECTION_FAILED", "dial tcp: connection refused", "router1", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code            string `json:"code"`
			Message         string `json:"message"`
			Device          string `json:"device"`
			SuggestedAction string `json:"suggested_action"`
		} `json:"error"`
		Meta Meta `json:"meta"`
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
	if resp.Error.SuggestedAction != "" {
		t.Fatalf("expected empty suggested_action, got %q", resp.Error.SuggestedAction)
	}
	if resp.Meta.RequestID != "abc123" {
		t.Fatalf("expected meta.request_id=abc123, got %q", resp.Meta.RequestID)
	}
}

func TestRenderError_SuggestedAction(t *testing.T) {
	var buf bytes.Buffer
	hint := "verify with read-only get before retry; do not blindly re-run the write"
	if err := RenderError(&buf, "timeout", "ambiguous write result", "lab", Meta{RequestID: "rid"}, hint); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Code            string `json:"code"`
			SuggestedAction string `json:"suggested_action"`
		} `json:"error"`
		Meta Meta `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Code != "timeout" || resp.Error.SuggestedAction != hint {
		t.Fatalf("%+v", resp.Error)
	}
	if resp.Meta.RequestID != "rid" {
		t.Fatalf("request_id=%q", resp.Meta.RequestID)
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

// mockRawRenderable implements Renderable + RawRenderable for secret tests.
type mockRawRenderable struct {
	headers []string
	rows    [][]string
	raw     []map[string]string
}

func (m *mockRawRenderable) TableHeaders() []string          { return m.headers }
func (m *mockRawRenderable) TableRows() [][]string           { return m.rows }
func (m *mockRawRenderable) RawRecords() []map[string]string { return m.raw }

func TestIsSecretKey(t *testing.T) {
	for _, k := range []string{"private-key", "PRIVATE-KEY", "password", "pre-shared-key", "preshared-key", "shared-secret", "secret", "wpa2-pre-shared-key", "passphrase"} {
		if !IsSecretKey(k) {
			t.Errorf("expected secret key %q", k)
		}
	}
	if IsSecretKey("name") || IsSecretKey("public-key") {
		t.Fatal("name/public-key should not be secret keys")
	}
}

func TestRedactRecord(t *testing.T) {
	got := RedactRecord(map[string]string{
		"name":        "wg0",
		"private-key": "supersecret",
		"public-key":  "pubkey",
		"password":    "hunter2",
	})
	if got["private-key"] != RedactedPlaceholder {
		t.Fatalf("private-key: %q", got["private-key"])
	}
	if got["password"] != RedactedPlaceholder {
		t.Fatalf("password: %q", got["password"])
	}
	if got["name"] != "wg0" || got["public-key"] != "pubkey" {
		t.Fatalf("non-secrets changed: %#v", got)
	}
	if RedactValue("password", "") != "" {
		t.Fatal("empty secret should stay empty")
	}
}

func TestRenderTable_RedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	data := &mockRenderable{
		headers: []string{"name", "private-key", "public-key"},
		rows:    [][]string{{"wg0", "AAAA", "BBBB"}},
	}
	if err := RenderTable(&buf, data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "AAAA") {
		t.Fatalf("table leaked private-key: %s", out)
	}
	if !strings.Contains(out, RedactedPlaceholder) {
		t.Fatalf("expected placeholder in table: %s", out)
	}
	if !strings.Contains(out, "BBBB") {
		t.Fatalf("public-key should remain: %s", out)
	}
}

func TestRenderJSON_RedactsSecretsByDefault(t *testing.T) {
	var buf bytes.Buffer
	data := &mockRawRenderable{
		headers: []string{"name", "private-key"},
		rows:    [][]string{{"wg0", "SECRET"}},
		raw:     []map[string]string{{"name": "wg0", "private-key": "SECRET", ".id": "*1"}},
	}
	meta := Meta{Device: "r1", Command: "/interface/wireguard/print", Count: 1}
	if err := RenderJSON(&buf, data, meta, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("default JSON leaked secret: %s", buf.String())
	}
	var resp JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	records, ok := resp.Data.([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("data: %#v", resp.Data)
	}
	rec, ok := records[0].(map[string]interface{})
	if !ok {
		t.Fatalf("record: %#v", records[0])
	}
	if rec["private-key"] != RedactedPlaceholder {
		t.Fatalf("private-key=%v want %q", rec["private-key"], RedactedPlaceholder)
	}
}

func TestRenderJSON_RawShowsSecrets(t *testing.T) {
	var buf bytes.Buffer
	data := &mockRawRenderable{
		headers: []string{"name", "private-key"},
		rows:    [][]string{{"wg0", "SECRET"}},
		raw:     []map[string]string{{"name": "wg0", "private-key": "SECRET", ".id": "*1"}},
	}
	meta := Meta{Device: "r1", Command: "test", Count: 1}
	if err := RenderJSON(&buf, data, meta, Options{Raw: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("--raw should show secrets: %s", buf.String())
	}
	if !strings.Contains(buf.String(), ".id") {
		t.Fatalf("--raw should include .id: %s", buf.String())
	}
}

func TestRedactPayload_NestedAuditShape(t *testing.T) {
	payload := map[string]interface{}{
		"users": []map[string]string{
			{"name": "admin", "password": "hunter2"},
		},
		"interfaces": []interface{}{
			map[string]interface{}{"name": "wg0", "private-key": "AAAA"},
		},
	}
	got, ok := RedactPayload(payload).(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", RedactPayload(payload))
	}
	users, ok := got["users"].([]map[string]string)
	if !ok || len(users) != 1 {
		t.Fatalf("users: %#v", got["users"])
	}
	if users[0]["password"] != RedactedPlaceholder || users[0]["name"] != "admin" {
		t.Fatalf("users: %#v", users[0])
	}
	ifaces, ok := got["interfaces"].([]interface{})
	if !ok || len(ifaces) != 1 {
		t.Fatalf("interfaces: %#v", got["interfaces"])
	}
	rec, ok := ifaces[0].(map[string]interface{})
	if !ok {
		t.Fatalf("iface elem: %#v", ifaces[0])
	}
	if rec["private-key"] != RedactedPlaceholder || rec["name"] != "wg0" {
		t.Fatalf("interfaces: %#v", rec)
	}
}

func TestRenderRawJSON_RedactsByDefault(t *testing.T) {
	var buf bytes.Buffer
	sections := map[string]interface{}{
		"users": []map[string]string{{"name": "admin", "password": "hunter2"}},
	}
	meta := Meta{Device: "r1", Command: "audit", Count: 1}
	if err := RenderRawJSON(&buf, sections, meta); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("audit JSON leaked password: %s", buf.String())
	}
	if !strings.Contains(buf.String(), RedactedPlaceholder) {
		t.Fatalf("expected placeholder: %s", buf.String())
	}

	buf.Reset()
	if err := RenderRawJSON(&buf, sections, meta, Options{Raw: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("--raw should keep password: %s", buf.String())
	}
}
