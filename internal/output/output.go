package output

import (
	"fmt"
	"io"
)

// Format represents the output format for rendering command results.
type Format string

const (
	// FormatTable renders human-readable tabwriter output.
	FormatTable Format = "table"
	// FormatJSON renders machine-readable JSON with a stable envelope.
	FormatJSON Format = "json"
)

// ParseFormat converts a string to a Format, returning an error for unknown values.
func ParseFormat(s string) (Format, error) {
	switch s {
	case string(FormatTable):
		return FormatTable, nil
	case string(FormatJSON):
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unknown output format: %q (valid: table, json)", s)
	}
}

// Renderable is implemented by types that can be rendered as table or JSON output.
type Renderable interface {
	TableHeaders() []string
	TableRows() [][]string
}

// Meta holds metadata included in the JSON response envelope.
type Meta struct {
	Device    string `json:"device"`
	Command   string `json:"command"`
	Timestamp string `json:"timestamp"`
	Count     int    `json:"count"`
}

// Render dispatches to RenderTable or RenderJSON based on the requested format.
func Render(w io.Writer, format Format, data Renderable, meta Meta) error {
	switch format {
	case FormatTable:
		return RenderTable(w, data)
	case FormatJSON:
		return RenderJSON(w, data, meta)
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}
