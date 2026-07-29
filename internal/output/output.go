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

// Options controls optional rendering behavior.
type Options struct {
	// Raw, when true and data implements RawRenderable, includes all RouterOS
	// fields in JSON (including .id). Secret keys are still redacted in table
	// output always; in JSON they are shown only when Raw is true (escape hatch
	// for operators who need the real values). Prefer default (Raw=false) for
	// agent-safe dumps.
	Raw bool
	// MaxBytes caps rendered output size. Zero uses DefaultMaxOutputBytes.
	// Negative disables the byte cap.
	MaxBytes int
}

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

// RawRenderable optionally exposes raw RouterOS sentence maps (including .id).
type RawRenderable interface {
	RawRecords() []map[string]string
}

// Meta holds metadata included in the JSON response envelope.
type Meta struct {
	Device    string `json:"device"`
	Command   string `json:"command"`
	Timestamp string `json:"timestamp"`
	Count     int    `json:"count"`
	// RequestID correlates a single CLI invocation across logs and envelopes.
	RequestID string `json:"request_id,omitempty"`
	// Truncated is true when row limit or byte cap dropped content.
	Truncated bool `json:"truncated,omitempty"`
	// Action is set for write-path outcomes (e.g. "dry_run") when applicable.
	Action string `json:"action,omitempty"`
}

// effectiveMaxBytes resolves Options.MaxBytes (0 → default, <0 → unlimited).
func effectiveMaxBytes(o Options) int {
	if o.MaxBytes < 0 {
		return 0 // unlimited for writeCapped
	}
	if o.MaxBytes == 0 {
		return DefaultMaxOutputBytes
	}
	return o.MaxBytes
}

// Render dispatches to RenderTable or RenderJSON based on the requested format.
func Render(w io.Writer, format Format, data Renderable, meta Meta, opts ...Options) error {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	switch format {
	case FormatTable:
		return RenderTable(w, data, o)
	case FormatJSON:
		return RenderJSON(w, data, meta, o)
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}
