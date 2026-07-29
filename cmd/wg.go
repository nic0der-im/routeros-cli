package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

const wgPeersLongHelp = `List WireGuard peers (/interface/wireguard/peers/print).

Flags:
  --stale-after  Mark peers whose last-handshake is older than the duration
                 (or empty/never/unparseable). Go duration syntax: 5m, 3m30s, 1h.
                 Does not delete peers; human output adds a FINDINGS-style note.

Also available as:
  ros interface wireguard peers
  ros get wg peers`

func newWGCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wg",
		Short: "WireGuard helpers (peers)",
	}
	cmd.AddCommand(newWGPeersCmd())
	return cmd
}

func newWGPeersCmd() *cobra.Command {
	var staleAfterFlag string
	cmd := &cobra.Command{
		Use:   "peers",
		Short: "List WireGuard peers (optional --stale-after annotation)",
		Long:  wgPeersLongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			runWGPeers(cmd, staleAfterFlag)
		},
	}
	cmd.Flags().StringVar(&staleAfterFlag, "stale-after", "", "mark peers with last-handshake older than duration or empty (e.g. 5m)")
	return cmd
}

func runWGPeers(cmd *cobra.Command, staleAfterFlag string) {
	var staleAfter time.Duration
	if staleAfterFlag != "" {
		d, err := time.ParseDuration(staleAfterFlag)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: invalid --stale-after %q: %v (use Go duration, e.g. 5m, 3m30s)\n", staleAfterFlag, err)
			return
		}
		if d < 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: --stale-after must be non-negative\n")
			return
		}
		staleAfter = d
	}

	runWithClient(cmd, "/interface/wireguard/peers/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		result, err := c.Run(ctx, "/interface/wireguard/peers/print")
		if err != nil {
			return fmt.Errorf("listing WireGuard peers: %w", err)
		}
		now := time.Now()
		view := buildWGPeerView(result.Sentences, staleAfter, now)
		if err := a.render(cmd.OutOrStdout(), view, deviceName, "/interface/wireguard/peers/print"); err != nil {
			return err
		}
		if staleAfter > 0 && a.OutFormat != output.FormatJSON {
			writeWGStaleFindings(cmd.OutOrStdout(), view, staleAfter)
		}
		return nil
	})
}

func writeWGStaleFindings(w io.Writer, view wgPeerView, staleAfter time.Duration) {
	n := 0
	for _, p := range view {
		if p.Stale {
			n++
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FINDINGS")
	if n == 0 {
		fmt.Fprintf(w, "ok: no WireGuard peers stale after %s\n", staleAfter)
		return
	}
	fmt.Fprintf(w, "warn: %d WireGuard peer(s) stale after %s (last-handshake older/empty) — inspect; do not auto-delete\n", n, staleAfter)
}

// wgPeerRow is a curated WireGuard peer row for table/JSON render.
type wgPeerRow struct {
	ID            string
	Interface     string
	PublicKey     string
	Endpoint      string
	Allowed       string
	LastHandshake string
	Comment       string
	Stale         bool
	ShowStale     bool
}

type wgPeerView []wgPeerRow

func (v wgPeerView) TableHeaders() []string {
	h := []string{"Interface", "Public-Key", "Endpoint", "Allowed-Address", "Last-Handshake", "Comment"}
	if len(v) > 0 && v[0].ShowStale {
		h = append(h, "Stale")
	}
	return h
}

func (v wgPeerView) TableRows() [][]string {
	rows := make([][]string, len(v))
	showStale := len(v) > 0 && v[0].ShowStale
	for i, p := range v {
		pk := p.PublicKey
		if len(pk) > 16 {
			pk = pk[:12] + "…"
		}
		row := []string{p.Interface, pk, p.Endpoint, p.Allowed, p.LastHandshake, p.Comment}
		if showStale {
			if p.Stale {
				row = append(row, "yes")
			} else {
				row = append(row, "no")
			}
		}
		rows[i] = row
	}
	return rows
}

func (v wgPeerView) RawRecords() []map[string]string {
	out := make([]map[string]string, len(v))
	for i, p := range v {
		m := map[string]string{
			".id":               p.ID,
			"interface":         p.Interface,
			"public-key":        p.PublicKey,
			"endpoint":          p.Endpoint,
			"allowed-address":   p.Allowed,
			"last-handshake":    p.LastHandshake,
			"comment":           p.Comment,
		}
		if p.ShowStale {
			if p.Stale {
				m["stale"] = "yes"
			} else {
				m["stale"] = "no"
			}
		}
		out[i] = m
	}
	return out
}

func buildWGPeerView(sentences []map[string]string, staleAfter time.Duration, now time.Time) wgPeerView {
	showStale := staleAfter > 0
	out := make(wgPeerView, 0, len(sentences))
	for _, s := range sentences {
		hs := firstField(s, "last-handshake")
		row := wgPeerRow{
			ID:            firstField(s, ".id"),
			Interface:     firstField(s, "interface"),
			PublicKey:     firstField(s, "public-key"),
			Endpoint:      joinEndpoint(s),
			Allowed:       firstField(s, "allowed-address"),
			LastHandshake: hs,
			Comment:       firstField(s, "comment"),
			ShowStale:     showStale,
		}
		if showStale {
			row.Stale = isHandshakeStale(hs, staleAfter, now)
		}
		out = append(out, row)
	}
	return out
}

func joinEndpoint(s map[string]string) string {
	addr := firstField(s, "current-endpoint-address", "endpoint-address")
	port := firstField(s, "current-endpoint-port", "endpoint-port")
	switch {
	case addr != "" && port != "":
		return addr + ":" + port
	case addr != "":
		return addr
	default:
		return ""
	}
}

// isHandshakeStale reports whether last-handshake is empty/never/unparseable
// or older than staleAfter.
func isHandshakeStale(raw string, staleAfter time.Duration, now time.Time) bool {
	if staleAfter <= 0 {
		return false
	}
	age, ok := parseLastHandshakeAge(raw, now)
	if !ok {
		return true
	}
	return age > staleAfter
}

// parseLastHandshakeAge best-effort parses RouterOS last-handshake into an age.
// ok is false for empty, "never", or unparseable values (caller treats as stale).
//
// Accepted forms:
//   - HH:MM:SS or H:MM:SS (elapsed age, common in CLI/API)
//   - Go durations: 1m23s, 5m, 90s
//   - integer seconds: 123
//   - absolute times via parseROSDateTime layouts (age = now − t)
func parseLastHandshakeAge(raw string, now time.Time) (time.Duration, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)
	if lower == "never" || lower == "none" || lower == "-" {
		return 0, false
	}

	if age, ok := parseHMSDuration(s); ok {
		return age, true
	}

	if d, err := time.ParseDuration(s); err == nil {
		if d < 0 {
			return 0, false
		}
		return d, true
	}

	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n < 0 {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	}

	if t, err := parseROSDateTime(s, rosFullDateTimeLayouts); err == nil {
		if t.After(now) {
			return 0, true
		}
		return now.Sub(t), true
	}
	return 0, false
}

// parseHMSDuration parses HH:MM:SS / H:MM:SS / MM:SS elapsed timers.
func parseHMSDuration(s string) (time.Duration, bool) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	for _, p := range parts {
		if p == "" {
			return 0, false
		}
		for _, r := range p {
			if !unicode.IsDigit(r) {
				return 0, false
			}
		}
	}
	var h, m, sec int
	var err error
	switch len(parts) {
	case 3:
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, false
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
		sec, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, false
		}
	case 2:
		m, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, false
		}
		sec, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
	}
	if m > 59 || sec > 59 || h < 0 || m < 0 || sec < 0 {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, true
}

// firstField returns the first non-empty trimmed value among keys.
func firstField(row map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(row[k]); v != "" {
			return v
		}
	}
	return ""
}
