// Package audit appends a best-effort NDJSON write-audit trail under
// ~/.config/ros/audit/.
//
// Default layout (flat by day, not per-device):
//
//	~/.config/ros/audit/writes-YYYY-MM-DD.ndjson
//
// Each line is one JSON object (snake_case). Device, profile, and env_class
// are fields on the event so a single daily file covers all inventory devices.
//
// Disable with ROS_AUDIT=0 (or ROS_NO_AUDIT=1). Audit I/O failures never fail
// the caller; callers may log them at -v.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/output"
)

// dirForTest overrides DefaultDir in unit tests.
var dirForTest string

// Event is one mutation audit record (NDJSON line).
type Event struct {
	TS        string            `json:"ts"`
	RequestID string            `json:"request_id,omitempty"`
	Device    string            `json:"device,omitempty"`
	Profile   string            `json:"profile,omitempty"`
	EnvClass  string            `json:"env_class,omitempty"`
	Verb      string            `json:"verb,omitempty"`
	Action    string            `json:"action,omitempty"`
	Outcome   string            `json:"outcome,omitempty"`
	Command   string            `json:"command,omitempty"`
	Path      string            `json:"path,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Props     map[string]string `json:"properties,omitempty"`
	Error     string            `json:"error,omitempty"`
	DryRun    bool              `json:"dry_run"`
}

// Enabled reports whether write-audit is active.
// ROS_AUDIT=0/false or ROS_NO_AUDIT=1/true disables it.
func Enabled() bool {
	if envTruthy("ROS_NO_AUDIT") {
		return false
	}
	v := strings.TrimSpace(os.Getenv("ROS_AUDIT"))
	if v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
		return false
	}
	return true
}

func envTruthy(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	return v == "1" || strings.EqualFold(v, "true")
}

// DefaultDir returns ~/.config/ros/audit (or the test override).
func DefaultDir() string {
	if dirForTest != "" {
		return dirForTest
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".config", "ros", "audit")
}

// SetDirForTest redirects the audit directory (tests only).
func SetDirForTest(dir string) {
	dirForTest = dir
}

// FileName returns writes-YYYY-MM-DD.ndjson for the given UTC time.
func FileName(t time.Time) string {
	return fmt.Sprintf("writes-%s.ndjson", t.UTC().Format("2006-01-02"))
}

// PathFor returns the full path of the daily audit file under dir.
func PathFor(dir string, t time.Time) string {
	return filepath.Join(dir, FileName(t))
}

// RedactAPIArgs returns a copy of RouterOS API args (=key=value) with secret
// values replaced by output.RedactedPlaceholder.
func RedactAPIArgs(args []string) []string {
	if args == nil {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = redactAPIArg(a)
	}
	return out
}

func redactAPIArg(a string) string {
	a = strings.TrimSpace(a)
	if a == "" {
		return a
	}
	prefix := ""
	rest := a
	switch {
	case strings.HasPrefix(a, "?="):
		prefix = "?="
		rest = a[2:]
	case strings.HasPrefix(a, "="):
		prefix = "="
		rest = a[1:]
	case strings.HasPrefix(a, "?"):
		prefix = "?"
		rest = a[1:]
	default:
		return a
	}
	i := strings.IndexByte(rest, '=')
	if i <= 0 {
		return a
	}
	key, val := rest[:i], rest[i+1:]
	return prefix + key + "=" + output.RedactValue(key, val)
}

// PropsFromAPIArgs converts =key=value API args into a redacted property map.
func PropsFromAPIArgs(args []string) map[string]string {
	raw := make(map[string]string, len(args))
	for _, a := range args {
		if !strings.HasPrefix(a, "=") {
			continue
		}
		rest := strings.TrimPrefix(a, "=")
		i := strings.IndexByte(rest, '=')
		if i <= 0 {
			continue
		}
		raw[rest[:i]] = rest[i+1:]
	}
	return output.RedactRecord(raw)
}

// Prepare returns a copy of ev with secrets redacted and ts filled if empty.
func Prepare(ev Event) Event {
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339)
	}
	if ev.Outcome == "" && ev.Action != "" {
		ev.Outcome = ev.Action
	}
	if ev.Action == "" && ev.Outcome != "" {
		ev.Action = ev.Outcome
	}
	if ev.Args != nil {
		ev.Args = RedactAPIArgs(ev.Args)
	}
	if ev.Props != nil {
		ev.Props = output.RedactRecord(ev.Props)
	} else if len(ev.Args) > 0 {
		ev.Props = PropsFromAPIArgs(ev.Args)
	}
	return ev
}

// Append writes one NDJSON line for ev under dir.
// Creates dir with 0700 and the daily file with 0600.
// Concurrent-safe enough: open O_APPEND, write one line, close.
func Append(dir string, ev Event) error {
	if dir == "" {
		dir = DefaultDir()
	}
	ev = Prepare(ev)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("audit mkdir: %w", err)
	}
	// MkdirAll does not tighten an existing dir (e.g. t.TempDir is often 0755).
	_ = os.Chmod(dir, 0o700)

	path := PathFor(dir, time.Now().UTC())
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("audit open: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Ensure mode stays 0600 even if umask loosened create bits.
	_ = f.Chmod(0o600)

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("audit marshal: %w", err)
	}
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("audit write: %w", err)
	}
	return nil
}
