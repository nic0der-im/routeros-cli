// Package publicip detects the caller's public and local egress addresses.
package publicip

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const DefaultURL = "https://api.ipify.org"

// Detect fetches the public IP from url (HTTPS). Empty url uses DefaultURL.
func Detect(ctx context.Context, url string) (string, error) {
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("public ip lookup: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("public ip lookup: status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("public ip lookup: invalid response %q", ip)
	}
	return ip, nil
}

// LocalToward returns the local IP the kernel would use to reach host
// (useful when the CLI is on the same LAN as the router).
func LocalToward(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	// UDP dial does not send packets but selects a local address.
	conn, err := net.DialTimeout("udp", net.JoinHostPort(host, "80"), 2*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return "", fmt.Errorf("no local address toward %s", host)
	}
	ip := addr.IP
	if ip.IsUnspecified() {
		return "", fmt.Errorf("unspecified local address")
	}
	// Prefer IPv4 string without zone.
	if v4 := ip.To4(); v4 != nil {
		return v4.String(), nil
	}
	return ip.String(), nil
}
