package filexfer

import (
	"fmt"
	"net"
	"strings"
)

// MergeAddressList merges RouterOS service address lists with extra CIDRs.
// Empty previous means "allow all"; during ephemeral we still set an explicit
// allowlist, then restore empty afterward.
func MergeAddressList(previous string, extras ...string) string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		// normalize bare IPs to /32 or /128
		if ip := net.ParseIP(s); ip != nil {
			if ip.To4() != nil {
				s = ip.String() + "/32"
			} else {
				s = ip.String() + "/128"
			}
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	for _, part := range splitAddrList(previous) {
		add(part)
	}
	for _, e := range extras {
		add(e)
	}
	return strings.Join(out, ",")
}

func splitAddrList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// RouterOS accepts comma and whitespace separators.
	s = strings.ReplaceAll(s, ";", ",")
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	return fields
}

// CIDROrIP returns ip/32 (or /128) for a bare IP, or the input if already CIDR-like.
func CIDROrIP(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", fmt.Errorf("empty ip")
	}
	if strings.Contains(ip, "/") {
		_, _, err := net.ParseCIDR(ip)
		if err != nil {
			return "", err
		}
		return ip, nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid ip %q", ip)
	}
	if parsed.To4() != nil {
		return parsed.String() + "/32", nil
	}
	return parsed.String() + "/128", nil
}
