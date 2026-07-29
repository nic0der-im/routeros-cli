package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseMaxOutputBytes(t *testing.T) {
	if got := ParseMaxOutputBytes(""); got != DefaultMaxOutputBytes {
		t.Fatalf("empty: got %d want %d", got, DefaultMaxOutputBytes)
	}
	if got := ParseMaxOutputBytes("1000"); got != 1000 {
		t.Fatalf("1000: got %d", got)
	}
	if got := ParseMaxOutputBytes("nope"); got != DefaultMaxOutputBytes {
		t.Fatalf("invalid: got %d", got)
	}
	if got := ParseMaxOutputBytes("-5"); got != DefaultMaxOutputBytes {
		t.Fatalf("negative: got %d", got)
	}
	if got := ParseMaxOutputBytes("0"); got != DefaultMaxOutputBytes {
		t.Fatalf("zero: got %d", got)
	}
}

func TestParseRowLimit(t *testing.T) {
	n, err := ParseRowLimit("")
	if err != nil || n != 0 {
		t.Fatalf("empty: %d %v", n, err)
	}
	n, err = ParseRowLimit("0")
	if err != nil || n != 0 {
		t.Fatalf("0: %d %v", n, err)
	}
	n, err = ParseRowLimit("25")
	if err != nil || n != 25 {
		t.Fatalf("25: %d %v", n, err)
	}
	if _, err := ParseRowLimit("-1"); err == nil {
		t.Fatal("expected error for -1")
	}
	if _, err := ParseRowLimit("x"); err == nil {
		t.Fatal("expected error for x")
	}
}

func TestLimitRenderable(t *testing.T) {
	data := newMockData()
	got, trunc := LimitRenderable(data, 0)
	if trunc || got != data {
		t.Fatal("limit 0 should be no-op")
	}
	got, trunc = LimitRenderable(data, 10)
	if trunc || got != data {
		t.Fatal("limit > rows should not truncate")
	}
	got, trunc = LimitRenderable(data, 1)
	if !trunc {
		t.Fatal("expected truncated")
	}
	if len(got.TableRows()) != 1 {
		t.Fatalf("rows=%d", len(got.TableRows()))
	}
	if got.TableRows()[0][0] != "ether1" {
		t.Fatalf("row=%v", got.TableRows()[0])
	}
}

func TestLimitRenderable_Raw(t *testing.T) {
	data := &mockRawRenderable{
		headers: []string{"name"},
		rows:    [][]string{{"a"}, {"b"}, {"c"}},
		raw: []map[string]string{
			{"name": "a"}, {"name": "b"}, {"name": "c"},
		},
	}
	got, trunc := LimitRenderable(data, 2)
	if !trunc {
		t.Fatal("expected trunc")
	}
	raw, ok := got.(RawRenderable)
	if !ok {
		t.Fatal("expected RawRenderable")
	}
	if len(raw.RawRecords()) != 2 {
		t.Fatalf("raw len=%d", len(raw.RawRecords()))
	}
}

func TestRenderJSON_RequestIDAndTruncateRows(t *testing.T) {
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{strings.Repeat("x", 40), "y"}
	}
	data := &mockRenderable{headers: []string{"name", "val"}, rows: rows}
	meta := Meta{Device: "r1", Command: "test", RequestID: "req-1", Count: 50}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, data, meta, Options{MaxBytes: 800}); err != nil {
		t.Fatal(err)
	}
	var resp JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if resp.Meta.RequestID != "req-1" {
		t.Fatalf("request_id=%q", resp.Meta.RequestID)
	}
	if !resp.Meta.Truncated {
		t.Fatal("expected meta.truncated=true")
	}
	records, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data type %T", resp.Data)
	}
	if len(records) >= 50 {
		t.Fatalf("expected row shrink, got %d", len(records))
	}
	if len(buf.Bytes()) > 800+len(TruncationMarker) {
		t.Fatalf("output too large: %d", len(buf.Bytes()))
	}
}

func TestRenderTable_ByteCapMarker(t *testing.T) {
	rows := make([][]string, 100)
	for i := range rows {
		rows[i] = []string{strings.Repeat("z", 20)}
	}
	data := &mockRenderable{headers: []string{"col"}, rows: rows}
	var buf bytes.Buffer
	if err := RenderTable(&buf, data, Options{MaxBytes: 200}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[OUTPUT TRUNCATED]") {
		t.Fatalf("expected truncation marker: %q", buf.String())
	}
	if len(buf.Bytes()) > 200 {
		// marker may push slightly? writeCapped keeps maxBytes including marker
		t.Fatalf("len=%d want <=200", len(buf.Bytes()))
	}
}

func TestRenderRawJSON_TruncatedFlag(t *testing.T) {
	big := map[string]string{"blob": strings.Repeat("A", 500)}
	meta := Meta{RequestID: "r2", Command: "x", Count: 1}
	var buf bytes.Buffer
	if err := RenderRawJSON(&buf, big, meta, Options{MaxBytes: 120}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, `"truncated": true`) || strings.Contains(out, TruncationMarker) {
		// either soft truncated meta or hard marker is OK
		return
	}
	t.Fatalf("expected truncation signal: %s", out)
}

func TestRenderJSON_UnlimitedMaxBytes(t *testing.T) {
	data := newMockData()
	meta := Meta{RequestID: "u1", Count: 2}
	var buf bytes.Buffer
	if err := RenderJSON(&buf, data, meta, Options{MaxBytes: -1}); err != nil {
		t.Fatal(err)
	}
	var resp JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Truncated {
		t.Fatal("should not truncate with MaxBytes=-1")
	}
	if resp.Meta.RequestID != "u1" {
		t.Fatal(resp.Meta.RequestID)
	}
}
