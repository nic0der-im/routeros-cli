package output

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

const (
	// DefaultMaxOutputBytes is the default ROS_MAX_OUTPUT_BYTES cap (512 KiB).
	// Agents and terminals stay responsive; raise via env for large dumps.
	DefaultMaxOutputBytes = 512_000

	// TruncationMarker is appended when rendered output is hard-truncated.
	TruncationMarker = "\n[OUTPUT TRUNCATED]\n"
)

// MaxOutputBytesFromEnv reads ROS_MAX_OUTPUT_BYTES (positive int).
// Empty or invalid values yield DefaultMaxOutputBytes.
func MaxOutputBytesFromEnv() int {
	return ParseMaxOutputBytes(os.Getenv("ROS_MAX_OUTPUT_BYTES"))
}

// ParseMaxOutputBytes parses a byte-cap string. Empty/invalid → default.
func ParseMaxOutputBytes(s string) int {
	if s == "" {
		return DefaultMaxOutputBytes
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return DefaultMaxOutputBytes
	}
	return n
}

// ParseRowLimit parses --limit N. Empty or "0" means unlimited (0).
// Negative or non-integer values return an error.
func ParseRowLimit(s string) (int, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --limit %q: must be a non-negative integer", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid --limit %d: must be >= 0", n)
	}
	return n, nil
}

// limitedRenderable wraps a Renderable with a row cap.
type limitedRenderable struct {
	headers []string
	rows    [][]string
}

func (l *limitedRenderable) TableHeaders() []string { return l.headers }
func (l *limitedRenderable) TableRows() [][]string  { return l.rows }

// limitedRawRenderable also preserves RawRecords when the source had them.
type limitedRawRenderable struct {
	limitedRenderable
	raw []map[string]string
}

func (l *limitedRawRenderable) RawRecords() []map[string]string { return l.raw }

// LimitRenderable returns data capped to at most n rows.
// n <= 0 means no limit. The second return is true when rows were dropped.
func LimitRenderable(data Renderable, n int) (Renderable, bool) {
	if data == nil || n <= 0 {
		return data, false
	}
	rows := data.TableRows()
	if len(rows) <= n {
		return data, false
	}
	base := limitedRenderable{
		headers: data.TableHeaders(),
		rows:    append([][]string(nil), rows[:n]...),
	}
	if raw, ok := data.(RawRenderable); ok {
		records := raw.RawRecords()
		out := &limitedRawRenderable{limitedRenderable: base}
		if len(records) > n {
			out.raw = append([]map[string]string(nil), records[:n]...)
		} else {
			out.raw = append([]map[string]string(nil), records...)
		}
		return out, true
	}
	return &base, true
}

// writeCapped writes b to w, hard-truncating when maxBytes > 0 and len(b) exceeds it.
// Returns whether truncation occurred.
func writeCapped(w io.Writer, b []byte, maxBytes int) (truncated bool, err error) {
	if maxBytes <= 0 || len(b) <= maxBytes {
		_, err = w.Write(b)
		return false, err
	}
	marker := []byte(TruncationMarker)
	keep := maxBytes - len(marker)
	if keep < 0 {
		keep = 0
	}
	if keep > len(b) {
		keep = len(b)
	}
	if _, err = w.Write(b[:keep]); err != nil {
		return true, err
	}
	_, err = w.Write(marker)
	return true, err
}
