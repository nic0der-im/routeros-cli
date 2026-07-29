package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

// FINDINGS thresholds for optional hygiene/full sections (B6).
const (
	auditWGStaleAfter     = 15 * time.Minute
	auditWGStaleLabel     = "15m"
	dnsStaticHighCount    = 50
	dnsStaticManyDisabled = 5
)

func newAuditCmd() *cobra.Command {
	var profile string
	var showPPP bool
	var skipCPUProfile bool
	var cpuProfileSec int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Read-only multi-domain snapshot for humans and agents",
		Long: `Collect a read-only configuration/status snapshot from the router.

Profiles:
  full     system, interfaces, IP, routes, DNS, firewall, DHCP, users, services (default)
  network  interfaces, IP addresses, routes, DNS, DHCP
  security firewall, users, IP services, identity
  hygiene  cloud DDNS, on-router .backup clutter, iface drop counters,
           FastTrack flags, services leftovers, DHCP lease hygiene,
           WireGuard peers, netwatch, DNS static (skips /tool/profile)

Human output is a compact summary (highlights only). PPPoE/PPP/L2TP dynamic
interfaces and their addresses are omitted by default (ISP-scale friendly);
pass --show-ppp to include them. PPP active sessions are shown as a count.

Memory/storage are shown in MB. Top CPU processes come from a short
/tool/profile sample (RouterOS does not expose per-process RAM over the API).
Use --skip-cpu-profile to skip that sample. Use -o json for full raw maps.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/audit", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				order, sections, err := collectAudit(ctx, c, profile, !skipCPUProfile, cpuProfileSec)
				if err != nil {
					return err
				}

				if profile == "hygiene" {
					recordDoctorSuccess(deviceName, cmd.ErrOrStderr())
				}

				meta := a.newMeta(deviceName, "audit --profile "+profile, len(sections))

				if a.OutFormat == output.FormatJSON {
					return a.renderRawJSON(cmd.OutOrStdout(), sections, meta)
				}

				return renderAuditHuman(cmd.OutOrStdout(), deviceName, profile, order, sections, showPPP)
			})
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "full", "audit profile: full, network, security, or hygiene")
	cmd.Flags().BoolVar(&showPPP, "show-ppp", false, "include PPPoE/PPP/L2TP interfaces and addresses in the human summary")
	cmd.Flags().BoolVar(&skipCPUProfile, "skip-cpu-profile", false, "skip /tool/profile CPU sample (faster)")
	cmd.Flags().IntVar(&cpuProfileSec, "cpu-profile-sec", 3, "seconds to sample for top CPU processes")
	return cmd
}

// auditProfiles lists valid --profile values (empty string is treated as full).
func auditProfiles() []string {
	return []string{"full", "network", "security", "hygiene"}
}

func validateAuditProfile(profile string) error {
	if profile == "" {
		return nil
	}
	for _, p := range auditProfiles() {
		if profile == p {
			return nil
		}
	}
	return fmt.Errorf("unknown audit profile %q (valid: %s)", profile, strings.Join(auditProfiles(), ", "))
}

func collectAudit(ctx context.Context, c client.Client, profile string, cpuProfile bool, cpuProfileSec int) ([]string, map[string]interface{}, error) {
	if err := validateAuditProfile(profile); err != nil {
		return nil, nil, err
	}

	type sectionSpec struct {
		key string
		cmd string
	}

	var specs []sectionSpec
	switch profile {
	case "full", "":
		specs = []sectionSpec{
			{"system_resource", "/system/resource/print"},
			{"system_identity", "/system/identity/print"},
			{"interfaces", "/interface/print"},
			{"ip_addresses", "/ip/address/print"},
			{"ip_routes", "/ip/route/print"},
			{"dns", "/ip/dns/print"},
			{"firewall_filter", "/ip/firewall/filter/print"},
			{"firewall_nat", "/ip/firewall/nat/print"},
			{"dhcp_leases", "/ip/dhcp-server/lease/print"},
			{"dhcp_servers", "/ip/dhcp-server/print"},
			{"users", "/user/print"},
			{"services", "/ip/service/print"},
		}
	case "network":
		specs = []sectionSpec{
			{"interfaces", "/interface/print"},
			{"ip_addresses", "/ip/address/print"},
			{"ip_routes", "/ip/route/print"},
			{"dns", "/ip/dns/print"},
			{"dhcp_leases", "/ip/dhcp-server/lease/print"},
			{"dhcp_servers", "/ip/dhcp-server/print"},
		}
	case "security":
		specs = []sectionSpec{
			{"system_identity", "/system/identity/print"},
			{"system_resource", "/system/resource/print"},
			{"users", "/user/print"},
			{"services", "/ip/service/print"},
			{"firewall_filter", "/ip/firewall/filter/print"},
			{"firewall_nat", "/ip/firewall/nat/print"},
		}
	case "hygiene":
		// Compact optimization snapshot; skips CPU sample and PPP listing.
		specs = []sectionSpec{
			{"system_resource", "/system/resource/print"},
			{"system_identity", "/system/identity/print"},
			{"ip_cloud", "/ip/cloud/print"},
			{"files", "/file/print"},
			{"interfaces", "/interface/print"},
			{"ip_settings", "/ip/settings/print"},
			{"services", "/ip/service/print"},
			{"dhcp_leases", "/ip/dhcp-server/lease/print"},
		}
	}

	order := make([]string, 0, len(specs)+2)
	out := make(map[string]interface{}, len(specs)+2)
	for _, s := range specs {
		result, err := c.Run(ctx, s.cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("audit section %s (%s): %w", s.key, s.cmd, err)
		}
		order = append(order, s.key)
		out[s.key] = result.Sentences
	}

	// Best-effort PPP session count (ISP routers); ignore if package/path missing.
	// Hygiene stays compact — no PPP section.
	switch profile {
	case "full", "", "network":
		if result, err := c.Run(ctx, "/ppp/active/print"); err == nil {
			order = append(order, "ppp_active")
			out["ppp_active"] = result.Sentences
		}
	}

	// Optional cheap FINDINGS inputs for hygiene/full (soft-skip if path/package missing).
	switch profile {
	case "hygiene", "full", "":
		for _, s := range []sectionSpec{
			{"wireguard_peers", "/interface/wireguard/peers/print"},
			{"netwatch", "/tool/netwatch/print"},
			{"dns_static", "/ip/dns/static/print"},
		} {
			if result, err := c.Run(ctx, s.cmd); err == nil {
				order = append(order, s.key)
				out[s.key] = result.Sentences
			}
		}
	}

	// Short CPU profile sample for top processes (no per-process RAM on RouterOS API).
	// Hygiene always skips /tool/profile (faster; low-RAM boards).
	if cpuProfile && (profile == "full" || profile == "" || profile == "security") {
		if cpuProfileSec < 1 {
			cpuProfileSec = 3
		}
		if result, err := c.Run(ctx, "/tool/profile", fmt.Sprintf("=duration=%d", cpuProfileSec)); err == nil {
			order = append(order, "cpu_profile")
			out["cpu_profile"] = result.Sentences
		}
	}

	return order, out, nil
}

func renderAuditHuman(w io.Writer, deviceName, profile string, order []string, sections map[string]interface{}, showPPP bool) error {
	width := 56
	fmt.Fprintln(w, strings.Repeat("─", width))
	fmt.Fprintf(w, "  AUDIT  %s  ·  profile=%s\n", deviceName, profile)
	fmt.Fprintln(w, strings.Repeat("─", width))

	if id := firstRow(sections["system_identity"]); id != nil || firstRow(sections["system_resource"]) != nil {
		printSystemBlock(w, firstRow(sections["system_identity"]), firstRow(sections["system_resource"]))
	}

	if rows := asRows(sections["cpu_profile"]); len(rows) > 0 {
		printSection(w, "top cpu", "sampled via /tool/profile (per-process RAM N/A on RouterOS API)", summarizeCPUProfile(rows), width)
	}

	hygiene := profile == "hygiene"
	for _, name := range order {
		rows := asRows(sections[name])
		switch name {
		case "system_resource", "system_identity", "cpu_profile":
			continue
		case "ip_cloud":
			printSection(w, "ip cloud", "no secrets", summarizeCloud(rows), width)
		case "files":
			printSection(w, "backup files", "on-router *.backup", summarizeBackupFiles(rows), width)
		case "ip_settings":
			printSection(w, "ip settings", "fast-path / FastTrack", summarizeIPSettings(rows), width)
		case "interfaces":
			if hygiene {
				printSection(w, "iface errors", "running ifaces with non-zero drops", summarizeIfaceErrors(rows), width)
			} else {
				lines, meta := summarizeInterfaces(rows, showPPP)
				printSection(w, "interfaces", meta, lines, width)
			}
		case "ip_addresses":
			lines, meta := summarizeAddresses(rows, showPPP)
			printSection(w, "addresses", meta, lines, width)
		case "ip_routes":
			printSection(w, "routes", fmt.Sprintf("%d total, active below", len(rows)), summarizeRoutes(rows), width)
		case "dns":
			printSection(w, "dns", "", summarizeDNS(rows), width)
		case "firewall_filter":
			printSection(w, "firewall filter", fmt.Sprintf("%d rules, non-dynamic below", len(rows)), summarizeFirewall(rows), width)
		case "firewall_nat":
			printSection(w, "firewall nat", "", summarizeNAT(rows), width)
		case "dhcp_leases":
			if hygiene {
				printSection(w, "dhcp hygiene", "", summarizeLeaseHygiene(rows), width)
			} else {
				printSection(w, "dhcp leases", "", summarizeLeases(rows), width)
			}
		case "dhcp_servers":
			printSection(w, "dhcp servers", "", summarizeDHCPServers(rows), width)
		case "ppp_active":
			if len(rows) == 0 {
				continue
			}
			printSection(w, "ppp active", "", summarizePPPActive(rows, showPPP), width)
		case "users":
			printSection(w, "users", "", summarizeUsers(rows), width)
		case "services":
			if hygiene {
				printSection(w, "services", "enabled + disabled mgmt leftovers", summarizeServicesHygiene(rows), width)
			} else {
				printSection(w, "services", "enabled only", summarizeServices(rows), width)
			}
		case "wireguard_peers":
			printSection(w, "wireguard peers", "stale after "+auditWGStaleLabel, summarizeWGPeersAudit(rows), width)
		case "netwatch":
			printSection(w, "netwatch", "", summarizeNetwatchAudit(rows), width)
		case "dns_static":
			printSection(w, "dns static", "clutter heuristics", summarizeDNSStaticAudit(rows), width)
		default:
			printSection(w, name, fmt.Sprintf("%d item(s)", len(rows)), nil, width)
		}
	}

	// FINDINGS footer: hygiene always; full when cheap (no extra API calls).
	if hygiene || profile == "full" || profile == "" {
		if lines := deriveFindings(sections); len(lines) > 0 {
			printSection(w, "findings", "", lines, width)
		}
	}
	return nil
}

// deriveFindings builds short actionable warn/ok/info lines from already-fetched
// audit section maps. No extra RouterOS calls.
func deriveFindings(sections map[string]interface{}) []string {
	var out []string

	if res := firstRow(sections["system_resource"]); res != nil {
		if bb := val(res, "bad-blocks"); bb != "" {
			if n, err := strconv.ParseFloat(bb, 64); err == nil {
				if n > 0 {
					out = append(out, fmt.Sprintf("warn: bad-blocks %s%% — check flash / replace storage", bb))
				} else {
					out = append(out, "ok: bad-blocks 0")
				}
			}
		}
	}

	if _, hasFiles := sections["files"]; hasFiles {
		n := countBackupFiles(asRows(sections["files"]))
		switch {
		case n > 1:
			out = append(out, fmt.Sprintf("warn: %d *.backup on router — file list; remove stale via file remove", n))
		case n == 1:
			out = append(out, "ok: 1 *.backup on router")
		default:
			out = append(out, "ok: no *.backup clutter")
		}
	}

	if _, hasIfaces := sections["interfaces"]; hasIfaces {
		if names := notableDropIfaces(asRows(sections["interfaces"])); len(names) > 0 {
			out = append(out, "warn: iface drops (TX-QUEUE-DROP/…) on "+strings.Join(names, ", ")+" — inspect get interface")
		} else {
			out = append(out, "ok: no notable iface drops")
		}
	}

	if settings := firstRow(sections["ip_settings"]); settings != nil {
		allow := isTrue(settings, "allow-fast-path")
		ft := strings.ToLower(val(settings, "ipv4-fasttrack-active"))
		ftActive := ft == "true" || ft == "yes"
		switch {
		case allow && !ftActive && ft != "":
			out = append(out, "info: allow-fast-path true but FastTrack inactive — check filter FastTrack rules")
		case allow && ftActive:
			out = append(out, "ok: FastTrack active")
		case allow && ft == "":
			// Flag missing; skip noise.
		}
	}

	if _, hasLeases := sections["dhcp_leases"]; hasLeases {
		waiting, dups := leaseHygieneCounts(asRows(sections["dhcp_leases"]))
		if waiting > 0 {
			out = append(out, fmt.Sprintf("warn: %d waiting DHCP lease(s) — delete dhcp/lease or lease cleanup-waiting", waiting))
		}
		if dups > 0 {
			out = append(out, "warn: same-MAC multi-lease (see DHCP HYGIENE) — delete stale dhcp/lease")
		}
		if waiting == 0 && dups == 0 {
			out = append(out, "ok: DHCP leases clean")
		}
	}

	if _, hasSvc := sections["services"]; hasSvc {
		if danger := enabledDangerousServices(asRows(sections["services"])); len(danger) > 0 {
			out = append(out, "warn: dangerous services enabled: "+strings.Join(danger, ",")+" — set /ip/service disabled=yes")
		} else {
			out = append(out, "ok: no dangerous cleartext services")
		}
	}

	if cloud := firstRow(sections["ip_cloud"]); cloud != nil {
		ddns := strings.ToLower(val(cloud, "ddns-enabled"))
		status := strings.ToLower(val(cloud, "status"))
		warning := strings.ToLower(val(cloud, "warning"))
		ddnsOn := ddns == "yes" || ddns == "true"
		natHint := warning != "" ||
			strings.Contains(status, "nat") ||
			strings.Contains(warning, "nat") ||
			strings.Contains(status, "warn")
		if ddnsOn && natHint {
			out = append(out, "warn: cloud DDNS behind NAT/status warning — set /ip/cloud ddns-enabled=auto (ROS≥7.17)")
		} else if ddnsOn {
			out = append(out, "ok: cloud DDNS enabled (no NAT warning)")
		} else if ddns != "" {
			out = append(out, "ok: cloud DDNS not forced on")
		}
	}

	if _, hasWG := sections["wireguard_peers"]; hasWG {
		n := countStaleWGPeers(asRows(sections["wireguard_peers"]), auditWGStaleAfter, time.Now())
		if n > 0 {
			out = append(out, fmt.Sprintf(
				"warn: %d WireGuard peer(s) stale (>%s/never) — inspect: wg peers --stale-after %s",
				n, auditWGStaleLabel, auditWGStaleLabel,
			))
		} else {
			out = append(out, fmt.Sprintf("ok: no WireGuard peers stale after %s", auditWGStaleLabel))
		}
	}

	if _, hasNW := sections["netwatch"]; hasNW {
		n := countNetwatchDown(asRows(sections["netwatch"]))
		if n > 0 {
			out = append(out, fmt.Sprintf("warn: %d netwatch host(s) down — inspect: get netwatch", n))
		} else {
			out = append(out, "ok: netwatch all up")
		}
	}

	if _, hasDNS := sections["dns_static"]; hasDNS {
		out = append(out, dnsStaticClutterFindings(asRows(sections["dns_static"]))...)
	}

	return out
}

// countStaleWGPeers counts enabled peers with empty/never/unparseable last-handshake
// or age older than staleAfter (reuses parseLastHandshakeAge / isHandshakeStale).
func countStaleWGPeers(rows []map[string]string, staleAfter time.Duration, now time.Time) int {
	n := 0
	for _, r := range rows {
		if isTrue(r, "disabled") {
			continue
		}
		hs := firstField(r, "last-handshake")
		if isHandshakeStale(hs, staleAfter, now) {
			n++
		}
	}
	return n
}

// isNetwatchDown reports host-down style status for common RouterOS field names.
// Disabled probes are ignored (intentional pause).
func isNetwatchDown(row map[string]string) bool {
	if isTrue(row, "disabled") {
		return false
	}
	status := strings.ToLower(firstField(row, "status", "host-status", "state"))
	if status != "" {
		switch status {
		case "down", "host-down", "unreachable", "timeout", "fail", "failed":
			return true
		}
		return strings.Contains(status, "down")
	}
	up := strings.ToLower(val(row, "up"))
	return up == "false" || up == "no"
}

func countNetwatchDown(rows []map[string]string) int {
	n := 0
	for _, r := range rows {
		if isNetwatchDown(r) {
			n++
		}
	}
	return n
}

// dnsStaticClutterFindings applies a simple clutter heuristic:
//   - total entries > dnsStaticHighCount, or
//   - disabled entries >= dnsStaticManyDisabled, or
//   - duplicate name+type keys (type defaults to A, same as SemanticKey)
// Emits one warn per triggered condition; otherwise a single ok line.
func dnsStaticClutterFindings(rows []map[string]string) []string {
	total := len(rows)
	disabled := 0
	keyCount := map[string]int{}
	for _, r := range rows {
		if isTrue(r, "disabled") {
			disabled++
		}
		name := strings.ToLower(val(r, "name"))
		if name == "" {
			continue
		}
		typ := strings.ToUpper(val(r, "type"))
		if typ == "" {
			typ = "A"
		}
		keyCount[name+"|"+typ]++
	}
	dups := 0
	for _, n := range keyCount {
		if n >= 2 {
			dups++
		}
	}

	var out []string
	if total > dnsStaticHighCount {
		out = append(out, fmt.Sprintf(
			"warn: %d DNS static entries (>%d) — review: dns static list",
			total, dnsStaticHighCount,
		))
	}
	if disabled >= dnsStaticManyDisabled {
		out = append(out, fmt.Sprintf(
			"warn: %d disabled DNS static — remove or enable: dns static list",
			disabled,
		))
	}
	if dups > 0 {
		out = append(out, fmt.Sprintf(
			"warn: %d duplicate DNS static name+type — resolve: dns static list",
			dups,
		))
	}
	if len(out) == 0 {
		return []string{"ok: DNS static tidy"}
	}
	return out
}

func summarizeWGPeersAudit(rows []map[string]string) []string {
	stale := countStaleWGPeers(rows, auditWGStaleAfter, time.Now())
	return []string{fmt.Sprintf("%d peer(s) · %d stale after %s", len(rows), stale, auditWGStaleLabel)}
}

func summarizeNetwatchAudit(rows []map[string]string) []string {
	down := countNetwatchDown(rows)
	return []string{fmt.Sprintf("%d probe(s) · %d down", len(rows), down)}
}

func summarizeDNSStaticAudit(rows []map[string]string) []string {
	disabled := 0
	for _, r := range rows {
		if isTrue(r, "disabled") {
			disabled++
		}
	}
	return []string{fmt.Sprintf("%d entr(y/ies) · %d disabled", len(rows), disabled)}
}

func countBackupFiles(rows []map[string]string) int {
	n := 0
	for _, r := range rows {
		if strings.HasSuffix(strings.ToLower(val(r, "name")), ".backup") {
			n++
		}
	}
	return n
}

func notableDropIfaces(rows []map[string]string) []string {
	var names []string
	for _, r := range rows {
		if !isTrue(r, "running") {
			continue
		}
		_, hasRX := counterNonZero(r, "rx-drop")
		_, hasTX := counterNonZero(r, "tx-drop")
		_, hasTQ := counterNonZero(r, "tx-queue-drop")
		if hasRX || hasTX || hasTQ {
			names = append(names, val(r, "name"))
		}
	}
	sort.Strings(names)
	return names
}

func leaseHygieneCounts(rows []map[string]string) (waiting, dupMACs int) {
	macCount := map[string]int{}
	for _, r := range rows {
		if strings.EqualFold(val(r, "status"), "waiting") {
			waiting++
		}
		mac := strings.ToLower(val(r, "mac-address"))
		if mac == "" {
			continue
		}
		macCount[mac]++
	}
	for _, n := range macCount {
		if n >= 2 {
			dupMACs++
		}
	}
	return waiting, dupMACs
}

var dangerousServiceNames = map[string]bool{
	"telnet": true, "ftp": true, "www": true,
}

func enabledDangerousServices(rows []map[string]string) []string {
	var out []string
	for _, r := range rows {
		name := strings.ToLower(val(r, "name"))
		if !dangerousServiceNames[name] {
			continue
		}
		if !isTrue(r, "disabled") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func printSystemBlock(w io.Writer, id, res map[string]string) {
	const width = 56
	fmt.Fprintln(w, "┌─ SYSTEM")
	if id != nil {
		if n := val(id, "name"); n != "" {
			fmt.Fprintf(w, "│  %s\n", n)
		}
	}
	if res == nil {
		fmt.Fprintln(w, closeBar(width))
		return
	}
	fmt.Fprintf(w, "│  %s · %s · %s\n", val(res, "board-name"), val(res, "architecture-name"), val(res, "version"))
	fmt.Fprintf(w, "│  uptime %s · cpu %s%% (%sx %s MHz · %s)\n",
		val(res, "uptime"),
		val(res, "cpu-load"),
		val(res, "cpu-count"),
		val(res, "cpu-frequency"),
		val(res, "cpu"),
	)
	fmt.Fprintf(w, "│  memory %s free / %s total\n",
		formatBytes(val(res, "free-memory")),
		formatBytes(val(res, "total-memory")),
	)
	fmt.Fprintf(w, "│  storage %s free / %s total · bad-blocks %s%%\n",
		formatBytes(val(res, "free-hdd-space")),
		formatBytes(val(res, "total-hdd-space")),
		val(res, "bad-blocks"),
	)
	if ws := val(res, "write-sect-total"); ws != "" {
		line := "│  disk writes " + ws + " sectors total"
		if since := val(res, "write-sect-since-reboot"); since != "" {
			line += " · " + since + " since reboot"
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, closeBar(width))
}

func closeBar(width int) string {
	if width < 8 {
		width = 56
	}
	return "└" + strings.Repeat("─", width-1)
}

func alignColumns(rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	_ = tw.Flush()
	out := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	return out
}

func formatBytes(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "?"
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw
	}
	const (
		kb = 1024.0
		mb = 1024.0 * 1024.0
		gb = 1024.0 * 1024.0 * 1024.0
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", n/gb)
	case n >= mb:
		return fmt.Sprintf("%.0f MB", n/mb)
	case n >= kb:
		return fmt.Sprintf("%.0f KB", n/kb)
	default:
		return fmt.Sprintf("%.0f B", n)
	}
}

func summarizeCPUProfile(rows []map[string]string) []string {
	if len(rows) == 0 {
		return []string{"(no profile samples)"}
	}
	lastSec := ""
	for _, r := range rows {
		if s := val(r, ".section"); s != "" {
			lastSec = s
		}
	}
	type item struct {
		name  string
		usage float64
	}
	var items []item
	for _, r := range rows {
		if lastSec != "" && val(r, ".section") != lastSec {
			continue
		}
		name := val(r, "name")
		if name == "" || strings.EqualFold(name, "total") || strings.EqualFold(name, "profiling") {
			continue
		}
		u, _ := strconv.ParseFloat(val(r, "usage"), 64)
		items = append(items, item{name: name, usage: u})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].usage > items[j].usage })
	if len(items) == 0 {
		return []string{"(no named processes in sample)"}
	}
	const topN = 5
	if len(items) > topN {
		items = items[:topN]
	}
	table := [][]string{{"RANK", "PROCESS", "CPU%"}}
	for i, it := range items {
		table = append(table, []string{fmt.Sprintf("#%d", i+1), it.name, fmt.Sprintf("%.1f", it.usage)})
	}
	out := alignColumns(table)
	out = append(out, "note: per-process RAM is not exposed by the RouterOS API")
	return out
}

func printSection(w io.Writer, title, meta string, lines []string, width int) {
	fmt.Fprintln(w)
	label := strings.ToUpper(title)
	fmt.Fprintf(w, "┌─ %s\n", label)
	if meta != "" {
		fmt.Fprintf(w, "│  %s\n", meta)
	}
	if len(lines) == 0 {
		fmt.Fprintln(w, "│  (none)")
		fmt.Fprintln(w, closeBar(width))
		return
	}
	for _, line := range lines {
		fmt.Fprintf(w, "│  %s\n", line)
	}
	fmt.Fprintln(w, closeBar(width))
}

func asRows(v interface{}) []map[string]string {
	rows, _ := v.([]map[string]string)
	return rows
}

func firstRow(v interface{}) map[string]string {
	rows := asRows(v)
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func val(row map[string]string, key string) string {
	if row == nil {
		return ""
	}
	return strings.TrimSpace(row[key])
}

func isTrue(row map[string]string, key string) bool {
	v := strings.ToLower(val(row, key))
	return v == "true" || v == "yes"
}

// isPPPLike reports PPPoE/PPP/L2TP/PPTP/SSTP style interfaces (and RouterOS
// dynamic names like <pppoe-user>).
func isPPPLike(row map[string]string) bool {
	t := strings.ToLower(val(row, "type"))
	name := strings.ToLower(val(row, "name"))
	for _, p := range []string{"pppoe", "ppp-", "l2tp", "pptp", "sstp"} {
		if strings.Contains(t, p) || strings.Contains(name, p) {
			return true
		}
	}
	if strings.HasPrefix(name, "<") && (strings.Contains(name, "ppp") || strings.Contains(name, "l2tp") || strings.Contains(name, "pptp") || strings.Contains(name, "sstp")) {
		return true
	}
	return false
}

func isPPPInterfaceName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	for _, p := range []string{"pppoe", "ppp-", "l2tp", "pptp", "sstp"} {
		if strings.Contains(n, p) {
			return true
		}
	}
	return strings.HasPrefix(n, "<") && (strings.Contains(n, "ppp") || strings.Contains(n, "l2tp"))
}

func summarizeInterfaces(rows []map[string]string, showPPP bool) (lines []string, meta string) {
	var pppRunning, otherRunning int
	table := [][]string{{"NAME", "TYPE", "RX", "TX", "COMMENT"}}
	for _, r := range rows {
		if !isTrue(r, "running") {
			continue
		}
		if isPPPLike(r) {
			pppRunning++
			if !showPPP {
				continue
			}
		} else {
			otherRunning++
		}
		table = append(table, []string{
			val(r, "name"),
			val(r, "type"),
			formatBytes(val(r, "rx-byte")),
			formatBytes(val(r, "tx-byte")),
			val(r, "comment"),
		})
	}
	meta = fmt.Sprintf("%d total · RX/TX = cumulative since counter reset (not live Mbps)", len(rows))
	if pppRunning > 0 && !showPPP {
		meta += fmt.Sprintf(" · %d shown, %d ppp/pppoe omitted (--show-ppp)", otherRunning, pppRunning)
	}
	if len(table) == 1 {
		if pppRunning > 0 && !showPPP {
			return []string{fmt.Sprintf("(no non-ppp running interfaces; %d ppp/pppoe up — use --show-ppp)", pppRunning)}, meta
		}
		return []string{"(no running interfaces)"}, meta
	}
	return alignColumns(table), meta
}

func summarizeAddresses(rows []map[string]string, showPPP bool) (lines []string, meta string) {
	table := [][]string{{"ADDRESS", "INTERFACE", "FLAGS"}}
	var omitted int
	for _, r := range rows {
		if isTrue(r, "disabled") {
			continue
		}
		iface := val(r, "interface")
		if !showPPP && isPPPInterfaceName(iface) {
			omitted++
			continue
		}
		flags := ""
		if isTrue(r, "dynamic") {
			flags = "dynamic"
		}
		table = append(table, []string{val(r, "address"), iface, flags})
	}
	if omitted > 0 && !showPPP {
		meta = fmt.Sprintf("%d ppp/pppoe addresses omitted (--show-ppp)", omitted)
	}
	if len(table) == 1 {
		return nil, meta
	}
	return alignColumns(table), meta
}

func summarizePPPActive(rows []map[string]string, showPPP bool) []string {
	if len(rows) == 0 {
		return []string{"0 sessions"}
	}
	out := []string{fmt.Sprintf("%d sessions", len(rows))}
	if !showPPP {
		out = append(out, "(names hidden — use --show-ppp to list)")
		return out
	}
	table := [][]string{{"NAME", "SERVICE", "ADDRESS"}}
	const maxN = 10
	for i, r := range rows {
		if i >= maxN {
			lines := alignColumns(table)
			lines = append(lines, fmt.Sprintf("… +%d more", len(rows)-maxN))
			return append(out, lines...)
		}
		name := val(r, "name")
		if name == "" {
			name = val(r, "caller-id")
		}
		table = append(table, []string{name, val(r, "service"), val(r, "address")})
	}
	return append(out, alignColumns(table)...)
}

func summarizeRoutes(rows []map[string]string) []string {
	table := [][]string{{"DST", "GATEWAY", "FLAGS"}}
	for _, r := range rows {
		if !isTrue(r, "active") {
			continue
		}
		gw := val(r, "gateway")
		if gw == "" {
			gw = val(r, "immediate-gw")
		}
		flags := ""
		if isTrue(r, "dynamic") {
			flags = "dynamic"
		}
		table = append(table, []string{val(r, "dst-address"), gw, flags})
	}
	if len(table) == 1 {
		return nil
	}
	return alignColumns(table)
}

func summarizeDNS(rows []map[string]string) []string {
	if len(rows) == 0 {
		return nil
	}
	r := rows[0]
	table := [][]string{
		{"SERVERS", "DYNAMIC", "REMOTE"},
		{val(r, "servers"), val(r, "dynamic-servers"), val(r, "allow-remote-requests")},
	}
	return alignColumns(table)
}

func summarizeFirewall(rows []map[string]string) []string {
	table := [][]string{{"CHAIN", "ACTION", "MATCH", "COMMENT"}}
	for _, r := range rows {
		if isTrue(r, "dynamic") {
			continue
		}
		match := strings.TrimSpace(strings.Join(filterEmpty(
			val(r, "protocol"),
			func() string {
				if p := val(r, "dst-port"); p != "" {
					return ":" + p
				}
				return ""
			}(),
			val(r, "src-address"),
		), " "))
		comment := val(r, "comment")
		if isTrue(r, "disabled") {
			comment = strings.TrimSpace(comment + " [disabled]")
		}
		table = append(table, []string{val(r, "chain"), val(r, "action"), match, comment})
	}
	if len(table) == 1 {
		return []string{"(only dynamic/dummy rules)"}
	}
	return alignColumns(table)
}

func summarizeNAT(rows []map[string]string) []string {
	table := [][]string{{"CHAIN", "ACTION", "OUT", "COMMENT"}}
	for _, r := range rows {
		if isTrue(r, "dynamic") {
			continue
		}
		table = append(table, []string{val(r, "chain"), val(r, "action"), val(r, "out-interface"), val(r, "comment")})
	}
	if len(table) == 1 {
		return nil
	}
	return alignColumns(table)
}

func summarizeLeases(rows []map[string]string) []string {
	var bound, waiting int
	table := [][]string{{"ADDRESS", "HOST"}}
	for _, r := range rows {
		status := strings.ToLower(val(r, "status"))
		switch status {
		case "bound":
			bound++
			addr := val(r, "active-address")
			if addr == "" {
				addr = val(r, "address")
			}
			host := val(r, "host-name")
			if host == "" {
				host = val(r, "comment")
			}
			if host == "" {
				host = val(r, "mac-address")
			}
			if len(table)-1 < 6 {
				table = append(table, []string{addr, host})
			}
		case "waiting":
			waiting++
		}
	}
	out := []string{fmt.Sprintf("%d bound, %d waiting", bound, waiting)}
	if len(table) > 1 {
		out = append(out, alignColumns(table)...)
	}
	if bound > 6 {
		out = append(out, fmt.Sprintf("… +%d more bound", bound-6))
	}
	return out
}

func summarizeDHCPServers(rows []map[string]string) []string {
	table := [][]string{{"NAME", "INTERFACE", "POOL"}}
	for _, r := range rows {
		table = append(table, []string{val(r, "name"), val(r, "interface"), val(r, "address-pool")})
	}
	if len(table) == 1 {
		return nil
	}
	return alignColumns(table)
}

func summarizeUsers(rows []map[string]string) []string {
	table := [][]string{{"NAME", "GROUP", "FLAGS"}}
	for _, r := range rows {
		flags := ""
		if isTrue(r, "disabled") {
			flags = "disabled"
		}
		table = append(table, []string{val(r, "name"), val(r, "group"), flags})
	}
	if len(table) == 1 {
		return nil
	}
	return alignColumns(table)
}

func summarizeServices(rows []map[string]string) []string {
	table := [][]string{{"NAME", "PORT", "ALLOW"}}
	for _, r := range rows {
		if isTrue(r, "disabled") {
			continue
		}
		allow := val(r, "address")
		if allow == "" {
			allow = "any"
		}
		table = append(table, []string{val(r, "name"), val(r, "port"), allow})
	}
	if len(table) == 1 {
		return []string{"(all services disabled)"}
	}
	return alignColumns(table)
}

// managementServiceNames are IP services often left enabled by mistake (or
// intentionally disabled leftovers worth noting in hygiene).
var managementServiceNames = map[string]bool{
	"ftp": true, "telnet": true, "www": true, "www-ssl": true,
	"api": true, "api-ssl": true, "winbox": true, "ssh": true,
}

func summarizeServicesHygiene(rows []map[string]string) []string {
	out := summarizeServices(rows)
	var disabled []string
	for _, r := range rows {
		name := strings.ToLower(val(r, "name"))
		if !managementServiceNames[name] {
			continue
		}
		if isTrue(r, "disabled") {
			disabled = append(disabled, name)
		}
	}
	sort.Strings(disabled)
	if len(disabled) > 0 {
		out = append(out, "disabled mgmt: "+strings.Join(disabled, ", "))
	}
	return out
}

func summarizeCloud(rows []map[string]string) []string {
	if len(rows) == 0 {
		return []string{"(no /ip/cloud data)"}
	}
	r := rows[0]
	table := [][]string{
		{"DDNS", "UPDATE-TIME", "STATUS"},
		{val(r, "ddns-enabled"), val(r, "update-time"), val(r, "status")},
	}
	out := alignColumns(table)
	// Optional non-secret status fields when present.
	extra := [][]string{}
	if pa := val(r, "public-address"); pa != "" {
		extra = append(extra, []string{"public-address", pa})
	}
	if dn := val(r, "dns-name"); dn != "" {
		extra = append(extra, []string{"dns-name", dn})
	}
	if len(extra) > 0 {
		out = append(out, alignColumns(extra)...)
	}
	return out
}

func summarizeBackupFiles(rows []map[string]string) []string {
	table := [][]string{{"NAME", "SIZE"}}
	for _, r := range rows {
		name := val(r, "name")
		if !strings.HasSuffix(strings.ToLower(name), ".backup") {
			continue
		}
		size := val(r, "size")
		if size == "" {
			size = "?"
		} else {
			size = formatBytes(size)
		}
		table = append(table, []string{name, size})
	}
	if len(table) == 1 {
		return []string{"(no .backup files on router)"}
	}
	return alignColumns(table)
}

func counterNonZero(row map[string]string, key string) (string, bool) {
	raw := val(row, key)
	if raw == "" {
		return "", false
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n == 0 {
		return "", false
	}
	return raw, true
}

func summarizeIfaceErrors(rows []map[string]string) []string {
	table := [][]string{{"NAME", "RX-DROP", "TX-DROP", "TX-QUEUE-DROP"}}
	for _, r := range rows {
		if !isTrue(r, "running") {
			continue
		}
		rx, hasRX := counterNonZero(r, "rx-drop")
		tx, hasTX := counterNonZero(r, "tx-drop")
		tq, hasTQ := counterNonZero(r, "tx-queue-drop")
		if !hasRX && !hasTX && !hasTQ {
			continue
		}
		if !hasRX {
			rx = "0"
		}
		if !hasTX {
			tx = "0"
		}
		if !hasTQ {
			tq = "0"
		}
		table = append(table, []string{val(r, "name"), rx, tx, tq})
	}
	if len(table) == 1 {
		return []string{"(no notable drops on running interfaces)"}
	}
	return alignColumns(table)
}

func summarizeIPSettings(rows []map[string]string) []string {
	if len(rows) == 0 {
		return []string{"(no /ip/settings data)"}
	}
	r := rows[0]
	keys := []string{
		"allow-fast-path",
		"ipv4-fast-path-active",
		"ipv4-fasttrack-active",
		"ipv6-fast-path-active",
	}
	table := [][]string{{"FLAG", "VALUE"}}
	for _, k := range keys {
		if v := val(r, k); v != "" {
			table = append(table, []string{k, v})
		}
	}
	if len(table) == 1 {
		return []string{"(no fast-path / FastTrack flags present)"}
	}
	return alignColumns(table)
}

func summarizeLeaseHygiene(rows []map[string]string) []string {
	var waiting int
	macCount := map[string]int{}
	for _, r := range rows {
		if strings.EqualFold(val(r, "status"), "waiting") {
			waiting++
		}
		mac := strings.ToLower(val(r, "mac-address"))
		if mac == "" {
			continue
		}
		macCount[mac]++
	}
	out := []string{fmt.Sprintf("%d waiting lease(s)", waiting)}
	var dups []string
	for mac, n := range macCount {
		if n >= 2 {
			dups = append(dups, fmt.Sprintf("%s (%d)", mac, n))
		}
	}
	sort.Strings(dups)
	if len(dups) > 0 {
		out = append(out, "warn: same MAC on 2+ leases: "+strings.Join(dups, ", "))
	}
	return out
}
