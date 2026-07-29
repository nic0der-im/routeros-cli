// Package winbox parses MikroTik Winbox address-book files (WBX / CDB)
// for import into the ros device inventory.
package winbox

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"
)

// DefaultAPIPort is the RouterOS API port used when a Winbox entry has no
// explicit port. Winbox itself uses a different (winbox) port, so imported
// addresses may need correction by the user.
const DefaultAPIPort = "8728"

// Entry is one saved connection from a Winbox address book.
type Entry struct {
	Name     string
	Address  string
	Username string
	Password string
	Comment  string
	Group    string
}

// SanitizeName turns a free-form Winbox label into a usable inventory name.
// Empty input yields "device".
func SanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "device"
	}
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteRune(r)
				prevDash = true
			}
		case unicode.IsSpace(r) || r == '/' || r == '\\' || r == ':' || r == '@':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			// drop other punctuation
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "device"
	}
	return out
}

// PreferredName picks a device name from comment, group, explicit name, or host.
func PreferredName(e Entry) string {
	for _, candidate := range []string{e.Comment, e.Group, e.Name, hostOnly(e.Address)} {
		if strings.TrimSpace(candidate) != "" {
			return SanitizeName(candidate)
		}
	}
	return "device"
}

// UniqueName returns base, or base-2, base-3, … when taken reports the name is used.
func UniqueName(base string, taken func(string) bool) string {
	base = SanitizeName(base)
	if !taken(base) {
		return base
	}
	for i := 2; i < 10000; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !taken(cand) {
			return cand
		}
	}
	return fmt.Sprintf("%s-%d", base, 10000)
}

// NormalizeAddress ensures host:port form, defaulting to DefaultAPIPort when
// the Winbox entry has no port. Bare IPv6 without brackets is bracketed.
// MAC addresses are left unchanged (Winbox L2; not a TCP API endpoint).
func NormalizeAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	// Already host:port (including [ipv6]:port).
	if host, port, err := net.SplitHostPort(addr); err == nil {
		if port == "" {
			port = DefaultAPIPort
		}
		return net.JoinHostPort(host, port)
	}
	if looksLikeMAC(addr) {
		return addr
	}
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = strings.TrimPrefix(strings.TrimSuffix(addr, "]"), "[")
	}
	// host or host:non-numeric — if trailing :digits treat as port.
	if i := strings.LastIndex(addr, ":"); i > 0 && strings.Count(addr, ":") == 1 {
		maybePort := addr[i+1:]
		if _, err := strconv.Atoi(maybePort); err == nil {
			return addr
		}
	}
	return net.JoinHostPort(addr, DefaultAPIPort)
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func looksLikeMAC(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return false
	}
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		for _, c := range p {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
