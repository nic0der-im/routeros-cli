package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
)

func TestWGPeersFlagsRegistered(t *testing.T) {
	cmd := newWGPeersCmd()
	f := cmd.Flags().Lookup("stale-after")
	if f == nil {
		t.Fatal("missing --stale-after")
	}
	if f.DefValue != "" {
		t.Fatalf("stale-after default: %q", f.DefValue)
	}
	for _, needle := range []string{"--stale-after", "FINDINGS", "Does not delete"} {
		if !strings.Contains(cmd.Long, needle) {
			t.Fatalf("Long help missing %q:\n%s", needle, cmd.Long)
		}
	}
}

func TestWifiBGPOSPFCommandsRegistered(t *testing.T) {
	if newWifiClientsCmd().Use != "clients" {
		t.Fatal("wifi clients")
	}
	if newBGPSessionsCmd().Use != "sessions" {
		t.Fatal("bgp sessions")
	}
	if newOSPFNeighborsCmd().Use != "neighbors" {
		t.Fatal("ospf neighbors")
	}
	wg := newWGCmd()
	if len(wg.Commands()) != 1 || wg.Commands()[0].Use != "peers" {
		t.Fatalf("wg tree: %+v", wg.Commands())
	}
	iface := newInterfaceWireguardCmd()
	if len(iface.Commands()) != 1 || iface.Commands()[0].Name() != "peers" {
		t.Fatalf("interface wireguard: %+v", iface.Commands())
	}
}

func TestParseLastHandshakeAge(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.Local)

	cases := []struct {
		raw     string
		want    time.Duration
		wantOK  bool
	}{
		{"", 0, false},
		{"never", 0, false},
		{"NONE", 0, false},
		{"-", 0, false},
		{"00:01:23", 1*time.Minute + 23*time.Second, true},
		{"1:02:03", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"05:00", 5 * time.Minute, true},
		{"1m23s", 1*time.Minute + 23*time.Second, true},
		{"5m", 5 * time.Minute, true},
		{"90s", 90 * time.Second, true},
		{"123", 123 * time.Second, true},
		{"garbage", 0, false},
		{"-5s", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseLastHandshakeAge(tc.raw, now)
		if ok != tc.wantOK {
			t.Errorf("%q: ok=%v want %v", tc.raw, ok, tc.wantOK)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%q: age=%v want %v", tc.raw, got, tc.want)
		}
	}

	abs := "jul/29/2026 17:55:00"
	got, ok := parseLastHandshakeAge(abs, now)
	if !ok {
		t.Fatalf("absolute time should parse")
	}
	if got != 5*time.Minute {
		t.Fatalf("absolute age=%v want 5m", got)
	}
}

func TestIsHandshakeStale(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.Local)
	thresh := 5 * time.Minute

	if !isHandshakeStale("", thresh, now) {
		t.Fatal("empty should be stale")
	}
	if !isHandshakeStale("never", thresh, now) {
		t.Fatal("never should be stale")
	}
	if !isHandshakeStale("00:10:00", thresh, now) {
		t.Fatal("10m should be stale vs 5m")
	}
	if isHandshakeStale("00:01:00", thresh, now) {
		t.Fatal("1m should not be stale vs 5m")
	}
	if isHandshakeStale("00:05:00", thresh, now) {
		t.Fatal("exactly 5m should not be stale (age > thresh)")
	}
	if isHandshakeStale("1m", 0, now) {
		t.Fatal("staleAfter=0 disables")
	}
}

func TestBuildWGPeerView_StaleAnnotation(t *testing.T) {
	now := time.Now()
	rows := []map[string]string{
		{
			".id": "*1", "interface": "wg0", "public-key": "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			"current-endpoint-address": "1.2.3.4", "current-endpoint-port": "51820",
			"allowed-address": "10.0.0.2/32", "last-handshake": "00:01:00", "comment": "ok",
		},
		{
			".id": "*2", "interface": "wg0", "public-key": "zzzz",
			"allowed-address": "10.0.0.3/32", "last-handshake": "", "comment": "dead",
		},
		{
			".id": "*3", "interface": "wg1", "public-key": "yyyy",
			"last-handshake": "00:30:00",
		},
	}
	view := buildWGPeerView(rows, 5*time.Minute, now)
	if len(view) != 3 {
		t.Fatalf("len=%d", len(view))
	}
	if !view[0].ShowStale || view[0].Stale {
		t.Fatalf("peer0: show=%v stale=%v", view[0].ShowStale, view[0].Stale)
	}
	if !view[1].Stale || !view[2].Stale {
		t.Fatalf("peer1/2 should be stale: %v %v", view[1].Stale, view[2].Stale)
	}
	if view[0].Endpoint != "1.2.3.4:51820" {
		t.Fatalf("endpoint=%q", view[0].Endpoint)
	}
	headers := view.TableHeaders()
	if headers[len(headers)-1] != "Stale" {
		t.Fatalf("headers=%v", headers)
	}
	table := view.TableRows()
	if table[0][len(table[0])-1] != "no" || table[1][len(table[1])-1] != "yes" {
		t.Fatalf("stale cols: %v", table)
	}

	noFlag := buildWGPeerView(rows, 0, now)
	if noFlag[0].ShowStale {
		t.Fatal("without --stale-after, ShowStale should be false")
	}
	if h := noFlag.TableHeaders(); h[len(h)-1] == "Stale" {
		t.Fatalf("unexpected Stale header: %v", h)
	}
}

func TestWGPeersListSmoke(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if command != "/interface/wireguard/peers/print" {
			t.Fatalf("unexpected command %s", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"interface": "wg0", "public-key": "abc", "last-handshake": "00:20:00", "comment": "stale-one"},
			{"interface": "wg0", "public-key": "def", "last-handshake": "00:00:30", "comment": "fresh"},
		}}, nil
	}

	view := buildWGPeerView(
		[]map[string]string{
			{"interface": "wg0", "public-key": "abc", "last-handshake": "00:20:00", "comment": "stale-one"},
			{"interface": "wg0", "public-key": "def", "last-handshake": "00:00:30", "comment": "fresh"},
		},
		5*time.Minute,
		time.Now(),
	)
	var buf bytes.Buffer
	if err := a.render(&buf, view, "lab", "/interface/wireguard/peers/print"); err != nil {
		t.Fatal(err)
	}
	writeWGStaleFindings(&buf, view, 5*time.Minute)
	out := buf.String()
	if !strings.Contains(out, "FINDINGS") || !strings.Contains(out, "1 WireGuard peer") {
		t.Fatalf("output: %s", out)
	}
	if !strings.Contains(out, "stale-one") || !strings.Contains(out, "fresh") {
		t.Fatalf("missing peers: %s", out)
	}
	// Ensure mock was usable for list path shape.
	res, err := mock.Run(context.Background(), "/interface/wireguard/peers/print")
	if err != nil || len(res.Sentences) != 2 {
		t.Fatalf("mock list: %v %#v", err, res)
	}
}

func TestWifiBGPOSPFListSmoke(t *testing.T) {
	a, _ := testApp(t)
	a.OutFormat = output.FormatTable

	wifi := buildWifiClientView([]map[string]string{
		{"interface": "wifi1", "mac-address": "AA:BB:CC:DD:EE:FF", "signal": "-55", "uptime": "1h", "ssid": "lab"},
	})
	var buf bytes.Buffer
	if err := a.render(&buf, wifi, "lab", "/interface/wifi/registration/print"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "AA:BB:CC:DD:EE:FF") || !strings.Contains(buf.String(), "-55") {
		t.Fatalf("wifi: %s", buf.String())
	}

	buf.Reset()
	bgp := buildBGPSessionView([]map[string]string{
		{"name": "to-isp", "remote.address": "203.0.113.1", "state": "established", "established": "2d"},
	})
	if err := a.render(&buf, bgp, "lab", "/routing/bgp/session/print"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "to-isp") || !strings.Contains(buf.String(), "203.0.113.1") {
		t.Fatalf("bgp: %s", buf.String())
	}

	buf.Reset()
	ospf := buildOSPFNeighborView([]map[string]string{
		{"router-id": "10.0.0.1", "address": "10.0.0.2", "state": "Full", "interface": "ether1", "area": "backbone"},
	})
	if err := a.render(&buf, ospf, "lab", "/routing/ospf/neighbor/print"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "10.0.0.1") || !strings.Contains(buf.String(), "Full") {
		t.Fatalf("ospf: %s", buf.String())
	}
}

func TestBuildWifiBGPOSPFViews(t *testing.T) {
	w := buildWifiClientView([]map[string]string{
		{"mac": "11:22:33:44:55:66", "signal-strength": "-70", "last-activity": "3m"},
	})
	if w[0].MAC != "11:22:33:44:55:66" || w[0].Signal != "-70" || w[0].Uptime != "3m" {
		t.Fatalf("%+v", w[0])
	}
	b := buildBGPSessionView([]map[string]string{
		{"name": "x", "remote": "1.1.1.1", "messages.state": "active", "uptime": "1h"},
	})
	if b[0].Remote != "1.1.1.1" || b[0].State != "active" || b[0].Established != "1h" {
		t.Fatalf("%+v", b[0])
	}
	o := buildOSPFNeighborView([]map[string]string{
		{"router-ID": "9.9.9.9", "neighbor-id": "8.8.8.8", "state": "2-Way", "iface": "vlan10"},
	})
	if o[0].RouterID != "9.9.9.9" || o[0].Address != "8.8.8.8" || o[0].Iface != "vlan10" {
		t.Fatalf("%+v", o[0])
	}
}
