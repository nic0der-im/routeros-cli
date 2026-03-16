package rosapi

import (
	"reflect"
	"testing"
)

func TestMapSentences_SystemResource(t *testing.T) {
	sentences := []map[string]string{
		{
			"uptime":            "3d12h05m",
			"version":           "7.14.3",
			"build-time":        "2024-04-01 12:00:00",
			"cpu-count":         "4",
			"cpu-load":          "12",
			"free-memory":       "512000000",
			"total-memory":      "1073741824",
			"free-hdd-space":    "100000000",
			"total-hdd-space":   "256000000",
			"architecture-name": "arm64",
			"board-name":        "RB5009UG+S+",
			"platform":          "MikroTik",
		},
	}

	results, err := MapSentences[SystemResource](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Uptime != "3d12h05m" {
		t.Errorf("Uptime = %q, want %q", r.Uptime, "3d12h05m")
	}
	if r.Version != "7.14.3" {
		t.Errorf("Version = %q, want %q", r.Version, "7.14.3")
	}
	if r.CPUCount != "4" {
		t.Errorf("CPUCount = %q, want %q", r.CPUCount, "4")
	}
	if r.ArchitectureName != "arm64" {
		t.Errorf("ArchitectureName = %q, want %q", r.ArchitectureName, "arm64")
	}
	if r.BoardName != "RB5009UG+S+" {
		t.Errorf("BoardName = %q, want %q", r.BoardName, "RB5009UG+S+")
	}
	if r.Platform != "MikroTik" {
		t.Errorf("Platform = %q, want %q", r.Platform, "MikroTik")
	}
}

func TestMapSentences_Interface(t *testing.T) {
	sentences := []map[string]string{
		{
			".id":        "*1",
			"name":       "ether1",
			"type":       "ether",
			"actual-mtu": "1500",
			"running":    "true",
			"disabled":   "false",
			"comment":    "WAN",
		},
		{
			".id":        "*2",
			"name":       "bridge1",
			"type":       "bridge",
			"actual-mtu": "1500",
			"running":    "true",
			"disabled":   "false",
			"comment":    "",
		},
	}

	results, err := MapSentences[Interface](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	first := results[0]
	if first.ID != "*1" {
		t.Errorf("ID = %q, want %q", first.ID, "*1")
	}
	if first.Name != "ether1" {
		t.Errorf("Name = %q, want %q", first.Name, "ether1")
	}
	if first.Type != "ether" {
		t.Errorf("Type = %q, want %q", first.Type, "ether")
	}
	if first.MTU != "1500" {
		t.Errorf("MTU = %q, want %q", first.MTU, "1500")
	}
	if first.Running != "true" {
		t.Errorf("Running = %q, want %q", first.Running, "true")
	}
	if first.Comment != "WAN" {
		t.Errorf("Comment = %q, want %q", first.Comment, "WAN")
	}

	second := results[1]
	if second.Name != "bridge1" {
		t.Errorf("second Name = %q, want %q", second.Name, "bridge1")
	}
	if second.Comment != "" {
		t.Errorf("second Comment = %q, want empty", second.Comment)
	}
}

func TestMapSentences_Empty(t *testing.T) {
	results, err := MapSentences[SystemResource](nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}

	results2, err := MapSentences[Interface]([]map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results2) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results2))
	}
}

func TestMapSentences_MissingFields(t *testing.T) {
	// Sentence has only a subset of the fields defined on Interface.
	sentences := []map[string]string{
		{
			"name": "ether1",
			".id":  "*1",
		},
	}

	results, err := MapSentences[Interface](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Name != "ether1" {
		t.Errorf("Name = %q, want %q", r.Name, "ether1")
	}
	if r.ID != "*1" {
		t.Errorf("ID = %q, want %q", r.ID, "*1")
	}
	// All other fields should be zero values.
	if r.Type != "" {
		t.Errorf("Type = %q, want empty (zero value)", r.Type)
	}
	if r.MTU != "" {
		t.Errorf("MTU = %q, want empty (zero value)", r.MTU)
	}
	if r.Running != "" {
		t.Errorf("Running = %q, want empty (zero value)", r.Running)
	}
	if r.Disabled != "" {
		t.Errorf("Disabled = %q, want empty (zero value)", r.Disabled)
	}
	if r.Comment != "" {
		t.Errorf("Comment = %q, want empty (zero value)", r.Comment)
	}
}

// testBoolStruct is a helper type for testing bool field mapping.
type testBoolStruct struct {
	Active bool `ros:"active"`
}

func TestMapSentences_BoolFields(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"true", "true", true},
		{"yes", "yes", true},
		{"false", "false", false},
		{"no", "no", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := []map[string]string{{"active": tt.val}}
			results, err := MapSentences[testBoolStruct](sentences)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if results[0].Active != tt.want {
				t.Errorf("Active = %v, want %v", results[0].Active, tt.want)
			}
		})
	}
}

// testIntStruct is a helper type for testing int field mapping.
type testIntStruct struct {
	Count int   `ros:"count"`
	Big   int64 `ros:"big"`
}

func TestMapSentences_IntFields(t *testing.T) {
	sentences := []map[string]string{
		{"count": "42", "big": "9223372036854775807"},
	}

	results, err := MapSentences[testIntStruct](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Count != 42 {
		t.Errorf("Count = %d, want 42", results[0].Count)
	}
	if results[0].Big != 9223372036854775807 {
		t.Errorf("Big = %d, want 9223372036854775807", results[0].Big)
	}
}

func TestMapSentences_IntEmptyIsZero(t *testing.T) {
	sentences := []map[string]string{
		{"count": "", "big": ""},
	}

	results, err := MapSentences[testIntStruct](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Count != 0 {
		t.Errorf("Count = %d, want 0", results[0].Count)
	}
	if results[0].Big != 0 {
		t.Errorf("Big = %d, want 0", results[0].Big)
	}
}

func TestMapSentences_IntParseError(t *testing.T) {
	sentences := []map[string]string{
		{"count": "not-a-number", "big": "0"},
	}

	_, err := MapSentences[testIntStruct](sentences)
	if err == nil {
		t.Fatal("expected error for invalid int, got nil")
	}
}

// testNoTagStruct verifies that fields without ros tags are skipped.
type testNoTagStruct struct {
	Tagged   string `ros:"tagged"`
	Untagged string
}

func TestMapSentences_NoTagFieldsSkipped(t *testing.T) {
	sentences := []map[string]string{
		{"tagged": "hello", "Untagged": "world"},
	}

	results, err := MapSentences[testNoTagStruct](sentences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Tagged != "hello" {
		t.Errorf("Tagged = %q, want %q", results[0].Tagged, "hello")
	}
	if results[0].Untagged != "" {
		t.Errorf("Untagged = %q, want empty (field should be skipped)", results[0].Untagged)
	}
}

func TestGenericResults_Renderable(t *testing.T) {
	gr := GenericResults{
		Items: []GenericResult{
			{Fields: map[string]string{
				"name": "ether1",
				"type": "ether",
				"mtu":  "1500",
			}},
			{Fields: map[string]string{
				"name":    "bridge1",
				"type":    "bridge",
				"mtu":     "1500",
				"comment": "LAN bridge",
			}},
		},
	}

	headers := gr.TableHeaders()
	// Headers should be sorted alphabetically and include all unique keys.
	expectedHeaders := []string{"comment", "mtu", "name", "type"}
	if !reflect.DeepEqual(headers, expectedHeaders) {
		t.Errorf("TableHeaders() = %v, want %v", headers, expectedHeaders)
	}

	rows := gr.TableRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// First row: comment is missing, should be empty string.
	// Headers order: comment, mtu, name, type
	row0 := rows[0]
	if row0[0] != "" {
		t.Errorf("row0[comment] = %q, want empty", row0[0])
	}
	if row0[1] != "1500" {
		t.Errorf("row0[mtu] = %q, want %q", row0[1], "1500")
	}
	if row0[2] != "ether1" {
		t.Errorf("row0[name] = %q, want %q", row0[2], "ether1")
	}
	if row0[3] != "ether" {
		t.Errorf("row0[type] = %q, want %q", row0[3], "ether")
	}

	// Second row: all keys present.
	row1 := rows[1]
	if row1[0] != "LAN bridge" {
		t.Errorf("row1[comment] = %q, want %q", row1[0], "LAN bridge")
	}
	if row1[2] != "bridge1" {
		t.Errorf("row1[name] = %q, want %q", row1[2], "bridge1")
	}
}

func TestGenericResults_Empty(t *testing.T) {
	var gr GenericResults

	headers := gr.TableHeaders()
	if len(headers) != 0 {
		t.Errorf("expected 0 headers for empty GenericResults, got %d", len(headers))
	}

	rows := gr.TableRows()
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty GenericResults, got %d", len(rows))
	}
}

func TestSystemResources_Renderable(t *testing.T) {
	sr := SystemResources{
		{
			BoardName:   "RB5009UG+S+",
			Platform:    "MikroTik",
			Version:     "7.14.3",
			Uptime:      "3d12h",
			CPUCount:    "4",
			CPULoad:     "12",
			FreeMemory:  "512MB",
			TotalMemory: "1024MB",
			FreeHDDSpace:  "100MB",
			TotalHDDSpace: "256MB",
		},
	}

	headers := sr.TableHeaders()
	if len(headers) != 8 {
		t.Errorf("expected 8 headers, got %d: %v", len(headers), headers)
	}

	rows := sr.TableRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row[0] != "RB5009UG+S+" {
		t.Errorf("row[0] = %q, want %q", row[0], "RB5009UG+S+")
	}
	if row[6] != "512MB/1024MB" {
		t.Errorf("row[6] = %q, want %q", row[6], "512MB/1024MB")
	}
}
