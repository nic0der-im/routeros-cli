// Package guardrails enforces production/staging write safety (safe sessions,
// path allow/deny, session blast-radius limits, maintenance windows, and ros
// exec command policy).
package guardrails

import (
	"fmt"
	"path"
	"strings"
)

// BuiltinDeniedPrefixes are always refused, regardless of device allowlists.
var BuiltinDeniedPrefixes = []string{
	"/system/reset-configuration",
	"/system/routerboard",
}

// BuiltinExecDeny are always refused for ros exec, regardless of device allowlists.
// Patterns use matchExecGlob (path.Match plus prefix / any-depth helpers).
var BuiltinExecDeny = []string{
	"/system/reboot*",
	"/system/reset-configuration*",
	"/system/routerboard*",
	"/disk/format*",
	"/file/format*",
	"/quit",
	"/password*",
	"*/private-key",
	"*/private-key/*",
	"*show-sensitive*",
}

// RequireSafeSession reports whether writes require an active safe session.
// True for prod and staging; false for lab (and empty/unknown treated as lab).
func RequireSafeSession(envClass string) bool {
	switch strings.ToLower(strings.TrimSpace(envClass)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

// RequireBackupBeforeSafeSession reports whether session begin --safe must take
// a local text backup first. Always true for prod; also true when the device
// sets require_backup_before_write (even for staging/lab).
func RequireBackupBeforeSafeSession(envClass string, deviceRequire bool) bool {
	if deviceRequire {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(envClass)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

// ErrSafeSessionRequired is returned when a prod/staging write lacks a safe session.
type ErrSafeSessionRequired struct {
	EnvClass   string
	DeviceName string
}

func (e *ErrSafeSessionRequired) Error() string {
	dev := e.DeviceName
	if dev == "" {
		dev = "<device>"
	}
	return fmt.Sprintf(
		"%s device %q requires a safe session before writes; run: ros -d %s session begin --safe",
		e.EnvClass, e.DeviceName, dev,
	)
}

// CheckSafeSession returns ErrSafeSessionRequired when envClass requires a safe
// session and hasSafeSession is false.
func CheckSafeSession(envClass, deviceName string, hasSafeSession bool) error {
	if !RequireSafeSession(envClass) {
		return nil
	}
	if hasSafeSession {
		return nil
	}
	return &ErrSafeSessionRequired{EnvClass: envClass, DeviceName: deviceName}
}

// ErrMaxChanges is returned when a session would exceed its change cap.
type ErrMaxChanges struct {
	Current int
	Max     int
}

func (e *ErrMaxChanges) Error() string {
	return fmt.Sprintf("session change limit reached (%d/%d); commit or rollback before more writes", e.Current, e.Max)
}

// CheckMaxChanges refuses the next change when current >= max.
// max <= 0 means unlimited.
func CheckMaxChanges(current, max int) error {
	if max <= 0 {
		return nil
	}
	if current >= max {
		return &ErrMaxChanges{Current: current, Max: max}
	}
	return nil
}

// ErrPathDenied is returned when a write path is blocked by deny/allow rules.
type ErrPathDenied struct {
	Path   string
	Reason string
}

func (e *ErrPathDenied) Error() string {
	return fmt.Sprintf("write path %q denied: %s", e.Path, e.Reason)
}

// NormalizePath canonicalizes a RouterOS API path for allow/deny matching.
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return strings.ToLower(p)
}

// pathMatchesPrefix reports whether path equals prefix or is under prefix/.
func pathMatchesPrefix(path, prefix string) bool {
	path = NormalizePath(path)
	prefix = NormalizePath(prefix)
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// CheckWritePath enforces builtin + user denied prefixes, then optional allowlist.
// Empty allowed means all non-denied paths are permitted.
func CheckWritePath(path string, allowed, denied []string) error {
	norm := NormalizePath(path)
	if norm == "/" || norm == "" {
		return &ErrPathDenied{Path: path, Reason: "empty or root path"}
	}

	for _, d := range BuiltinDeniedPrefixes {
		if pathMatchesPrefix(norm, d) {
			return &ErrPathDenied{Path: norm, Reason: "builtin deny " + NormalizePath(d)}
		}
	}
	for _, d := range denied {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if pathMatchesPrefix(norm, d) {
			return &ErrPathDenied{Path: norm, Reason: "denied_write_paths " + NormalizePath(d)}
		}
	}

	if len(allowed) == 0 {
		return nil
	}
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if pathMatchesPrefix(norm, a) {
			return nil
		}
	}
	return &ErrPathDenied{Path: norm, Reason: "not in allowed_write_paths"}
}

// ErrExecDenied is returned when an exec command is blocked by deny/allow rules.
type ErrExecDenied struct {
	Command string
	Reason  string
}

func (e *ErrExecDenied) Error() string {
	return fmt.Sprintf("exec %q denied: %s", e.Command, e.Reason)
}

// CheckExec enforces builtin + user deny globs, then optional exec allowlist.
// Empty allow means all non-denied commands are permitted.
func CheckExec(command string, allow, deny []string) error {
	norm := NormalizePath(command)
	if norm == "/" || norm == "" {
		return &ErrExecDenied{Command: command, Reason: "empty or root command"}
	}

	for _, d := range BuiltinExecDeny {
		ok, err := matchExecGlob(d, norm)
		if err != nil {
			return fmt.Errorf("builtin exec deny pattern %q: %w", d, err)
		}
		if ok {
			return &ErrExecDenied{Command: norm, Reason: "builtin deny " + d}
		}
	}
	for _, d := range deny {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		ok, err := matchExecGlob(d, norm)
		if err != nil {
			return fmt.Errorf("exec_deny pattern %q: %w", d, err)
		}
		if ok {
			return &ErrExecDenied{Command: norm, Reason: "exec_deny " + d}
		}
	}

	if len(allow) == 0 {
		return nil
	}
	for _, a := range allow {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		ok, err := matchExecGlob(a, norm)
		if err != nil {
			return fmt.Errorf("exec_allow pattern %q: %w", a, err)
		}
		if ok {
			return nil
		}
	}
	return &ErrExecDenied{Command: norm, Reason: "not in exec_allow"}
}

func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// normalizeExecPattern lowercases and canonicalizes slashes while preserving glob meta.
func normalizeExecPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	for strings.Contains(pattern, "//") {
		pattern = strings.ReplaceAll(pattern, "//", "/")
	}
	return strings.ToLower(pattern)
}

// matchExecGlob reports whether command matches pattern.
// path.Match rules apply (* does not cross '/'). Extensions:
//   - literal (no meta) → path prefix match
//   - trailing "*" with literal base → nested prefix match
//   - leading "*/" → match that suffix at any depth
//   - pattern with no '/' → * matches across '/' (substring-style)
func matchExecGlob(pattern, command string) (bool, error) {
	pat := normalizeExecPattern(pattern)
	cmd := NormalizePath(command)
	if pat == "" {
		return false, nil
	}

	if !hasGlobMeta(pat) {
		return pathMatchesPrefix(cmd, pat), nil
	}

	// Trailing-star prefix: "/disk/format*" → any command with that string prefix
	// (covers same-segment suffixes and nested paths).
	if strings.HasSuffix(pat, "*") {
		base := strings.TrimSuffix(pat, "*")
		if !hasGlobMeta(base) {
			if base == "" {
				return true, nil
			}
			if cmd == base || strings.HasPrefix(cmd, base) {
				return true, nil
			}
		}
	}

	// "*/suffix" or "*/suffix/*" — any depth.
	if strings.HasPrefix(pat, "*/") {
		rest := strings.TrimPrefix(pat, "*") // starts with /
		under := false
		suf := rest
		if strings.HasSuffix(rest, "/*") {
			suf = strings.TrimSuffix(rest, "/*")
			under = true
		}
		if suf != "" && !hasGlobMeta(suf) {
			if cmd == suf || strings.HasSuffix(cmd, suf) {
				return true, nil
			}
			if under && strings.Contains(cmd, suf+"/") {
				return true, nil
			}
		}
	}

	// No slash in pattern: allow * to span segments (e.g. *show-sensitive*).
	if !strings.Contains(pat, "/") {
		return matchStarSpan(pat, cmd), nil
	}

	return path.Match(pat, cmd)
}

// matchStarSpan is a minimal * / ? matcher where * matches any run of runes (including '/').
func matchStarSpan(pattern, s string) bool {
	type state struct{ pi, si int }
	stack := []state{{0, 0}}
	seen := map[state]bool{}
	for len(stack) > 0 {
		st := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[st] {
			continue
		}
		seen[st] = true
		pi, si := st.pi, st.si
		if pi == len(pattern) {
			if si == len(s) {
				return true
			}
			continue
		}
		switch pattern[pi] {
		case '*':
			stack = append(stack, state{pi + 1, si})
			if si < len(s) {
				stack = append(stack, state{pi, si + 1})
			}
		case '?':
			if si < len(s) {
				stack = append(stack, state{pi + 1, si + 1})
			}
		default:
			if si < len(s) && s[si] == pattern[pi] {
				stack = append(stack, state{pi + 1, si + 1})
			}
		}
	}
	return false
}
