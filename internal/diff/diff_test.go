package diff

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeRow_DropsCounters(t *testing.T) {
	row := map[string]string{
		".id":               "*1",
		"name":              "ether1",
		"type":              "ether",
		"running":           "true",
		"bytes":             "999",
		"packets":           "42",
		"uptime":            "1w2d",
		"last-link-up-time": "2026-01-01",
		"link-downs":        "3",
		"rx-byte":           "100",
		"tx-byte":           "200",
		"rx-packet":         "10",
		"tx-drop":           "1",
		"fp-rx-byte":        "50",
		"RX-ERROR":          "0", // case-insensitive prefix
		"comment":           "wan",
	}
	got := NormalizeRow(row)
	want := map[string]string{
		".id":     "*1",
		"name":    "ether1",
		"type":    "ether",
		"running": "true",
		"comment": "wan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeRow = %#v, want %#v", got, want)
	}
	// input unchanged
	if row["bytes"] != "999" {
		t.Fatal("NormalizeRow mutated input")
	}
}

func TestIsDynamic_WithoutDynamic(t *testing.T) {
	rows := []map[string]string{
		{"dst-address": "0.0.0.0/0", "gateway": "1.1.1.1", "dynamic": "false"},
		{"dst-address": "10.0.0.0/8", "gateway": "10.0.0.1", "dynamic": "true"},
		{"dst-address": "192.168.0.0/16", "gateway": "192.168.0.1", "dynamic": "yes"},
	}
	if !IsDynamic(rows[1]) || !IsDynamic(rows[2]) {
		t.Fatal("expected dynamic rows")
	}
	if IsDynamic(rows[0]) {
		t.Fatal("static row marked dynamic")
	}
	static := WithoutDynamic(rows)
	if len(static) != 1 || static[0]["dst-address"] != "0.0.0.0/0" {
		t.Fatalf("WithoutDynamic = %#v", static)
	}
}

func TestSemanticKey_AddressList(t *testing.T) {
	key := SemanticKey("/ip/firewall/address-list", map[string]string{
		"list":    "blacklist",
		"address": "1.2.3.4",
		".id":     "*9",
	})
	if key != "list=blacklist|address=1.2.3.4" {
		t.Fatalf("got %q", key)
	}
}

func TestSemanticKey_DNSStatic_DefaultType(t *testing.T) {
	key := SemanticKey("/ip/dns/static", map[string]string{
		"name":    "router.lan",
		"address": "192.168.88.1",
	})
	if key != "name=router.lan|type=A" {
		t.Fatalf("got %q", key)
	}
	key2 := SemanticKey("ip/dns/static/add", map[string]string{
		"name": "router.lan",
		"type": "AAAA",
	})
	if key2 != "name=router.lan|type=AAAA" {
		t.Fatalf("got %q", key2)
	}
}

func TestSemanticKey_Route_DefaultTable(t *testing.T) {
	key := SemanticKey("/ip/route", map[string]string{
		"dst-address": "0.0.0.0/0",
		"gateway":     "192.168.88.1",
	})
	if key != "dst-address=0.0.0.0/0|gateway=192.168.88.1|routing-table=main" {
		t.Fatalf("got %q", key)
	}
}

func TestSemanticKey_IPAddress(t *testing.T) {
	key := SemanticKey("/ip/address", map[string]string{
		"address":   "10.0.0.1/24",
		"interface": "bridge",
	})
	if key != "address=10.0.0.1/24|interface=bridge" {
		t.Fatalf("got %q", key)
	}
}

func TestSemanticKey_FirewallCommentPreferred(t *testing.T) {
	row := map[string]string{
		"chain":   "forward",
		"action":  "accept",
		"comment": "allow-lan",
		".id":     "*3",
	}
	key := SemanticKey("/ip/firewall/filter", row)
	if key != "comment=allow-lan" {
		t.Fatalf("got %q", key)
	}
	// same comment identity across nat/mangle
	if SemanticKey("/ip/firewall/nat", row) != "comment=allow-lan" {
		t.Fatal("nat comment key mismatch")
	}
}

func TestSemanticKey_FirewallCompositeWithoutComment(t *testing.T) {
	key := SemanticKey("/ip/firewall/filter", map[string]string{
		"chain":        "input",
		"action":       "drop",
		"protocol":     "tcp",
		"dst-port":     "23",
		"src-address":  "",
		"dst-address":  "",
		"in-interface": "ether1",
		"out-interface": "",
	})
	want := "chain=input|action=drop|protocol=tcp|dst-port=23|src-address=|dst-address=|in-interface=ether1|out-interface="
	if key != want {
		t.Fatalf("got %q want %q", key, want)
	}
}

func TestSemanticKey_FallbackSorted(t *testing.T) {
	key := SemanticKey("/queue/simple", map[string]string{
		"name":   "client-a",
		"target": "10.0.0.5/32",
		".id":    "*1",
		"bytes":  "100", // runtime dropped from fallback
	})
	if key != "name=client-a|target=10.0.0.5/32" {
		t.Fatalf("got %q", key)
	}
}

func TestDiffCreate_AlreadyExists(t *testing.T) {
	existing := []map[string]string{
		{"list": "blacklist", "address": "1.2.3.4", ".id": "*1"},
	}
	d := DiffCreate("/ip/firewall/address-list", existing, map[string]string{
		"list": "blacklist", "address": "1.2.3.4",
	})
	if !d.Empty() || len(d.Warnings) != 1 || d.Warnings[0] != "already_exists" {
		t.Fatalf("got %#v", d)
	}
}

func TestDiffCreate_NewRow(t *testing.T) {
	existing := []map[string]string{
		{"list": "blacklist", "address": "1.2.3.4"},
	}
	d := DiffCreate("/ip/firewall/address-list", existing, map[string]string{
		"list": "blacklist", "address": "9.9.9.9", "timeout": "1d",
	})
	if len(d.ToCreate) != 1 {
		t.Fatalf("got %#v", d)
	}
	if d.ToCreate[0].Key != "list=blacklist|address=9.9.9.9" {
		t.Fatalf("key %q", d.ToCreate[0].Key)
	}
	if d.ToCreate[0].After["timeout"] != "1d" {
		t.Fatalf("after %#v", d.ToCreate[0].After)
	}
}

func TestDiffCreate_FirewallOrderWarning(t *testing.T) {
	d := DiffCreate("/ip/firewall/filter", nil, map[string]string{
		"chain": "forward", "action": "accept", "comment": "new-rule",
	})
	if len(d.ToCreate) != 1 {
		t.Fatalf("expected create: %#v", d)
	}
	if len(d.Warnings) != 1 || !strings.Contains(d.Warnings[0], "rule order") {
		t.Fatalf("expected firewall order warning: %#v", d.Warnings)
	}
}

func TestDiffDelete_ByIDAndKey(t *testing.T) {
	rows := []map[string]string{
		{"list": "allow", "address": "10.0.0.1", ".id": "*7"},
	}
	d := DiffDelete("/ip/firewall/address-list", rows, "*7")
	if len(d.ToRemove) != 1 || d.ToRemove[0].ID != "*7" {
		t.Fatalf("by id: %#v", d)
	}
	d2 := DiffDelete("/ip/firewall/address-list", rows, "list=allow|address=10.0.0.1")
	if len(d2.ToRemove) != 1 {
		t.Fatalf("by key: %#v", d2)
	}
	d3 := DiffDelete("/ip/firewall/address-list", rows, "*missing")
	if !d3.Empty() || d3.Warnings[0] != "not_found" {
		t.Fatalf("missing: %#v", d3)
	}
}

func TestDiffDelete_FirewallOrderWarning(t *testing.T) {
	rows := []map[string]string{
		{"chain": "input", "action": "drop", "comment": "bad", ".id": "*2"},
	}
	d := DiffDelete("/ip/firewall/nat", rows, "*2")
	if len(d.ToRemove) != 1 {
		t.Fatalf("got %#v", d)
	}
	if len(d.Warnings) != 1 || !strings.Contains(d.Warnings[0], "rule order") {
		t.Fatalf("warnings %#v", d.Warnings)
	}
}

func TestDiffSet_PropertyLevel(t *testing.T) {
	existing := map[string]string{
		".id":     "*1",
		"list":    "blacklist",
		"address": "1.2.3.4",
		"comment": "old",
	}
	d := DiffSet("/ip/firewall/address-list", existing, map[string]string{
		"comment": "new",
		"list":    "blacklist", // unchanged
	})
	if len(d.ToUpdate) != 1 {
		t.Fatalf("got %#v", d)
	}
	item := d.ToUpdate[0]
	if item.Before["comment"] != "old" || item.After["comment"] != "new" {
		t.Fatalf("before/after %#v %#v", item.Before, item.After)
	}
	if _, ok := item.After["list"]; ok {
		t.Fatal("unchanged list should not appear")
	}
	if !DiffSet("/ip/firewall/address-list", existing, map[string]string{"comment": "old"}).Empty() {
		t.Fatal("expected no_change empty Diff")
	}
}

func TestCompare_CreateAndUpdate(t *testing.T) {
	current := []map[string]string{
		{"name": "a.lan", "type": "A", "address": "10.0.0.1", ".id": "*1"},
	}
	// update existing by semantic key
	d := Compare("/ip/dns/static", current, map[string]string{
		"name": "a.lan", "address": "10.0.0.2",
	}, "")
	if len(d.ToUpdate) != 1 || d.ToUpdate[0].After["address"] != "10.0.0.2" {
		t.Fatalf("update: %#v", d)
	}
	// create missing
	d2 := Compare("/ip/dns/static", current, map[string]string{
		"name": "b.lan", "address": "10.0.0.3",
	}, "")
	if len(d2.ToCreate) != 1 {
		t.Fatalf("create: %#v", d2)
	}
	// match by .id
	d3 := Compare("/ip/dns/static", current, map[string]string{
		"ttl": "1d",
	}, "*1")
	if len(d3.ToUpdate) != 1 || d3.ToUpdate[0].ID != "*1" {
		t.Fatalf("by id: %#v", d3)
	}
}

func TestTableFixtures_AddressListDNSFirewallNormalize(t *testing.T) {
	fixtures := []struct {
		name string
		path string
		row  map[string]string
		key  string
	}{
		{
			name: "address-list",
			path: "/ip/firewall/address-list",
			row:  map[string]string{"list": "vpn", "address": "8.8.8.8", "timeout": "1h"},
			key:  "list=vpn|address=8.8.8.8",
		},
		{
			name: "dns-static",
			path: "/ip/dns/static",
			row:  map[string]string{"name": "gw.home", "address": "192.168.88.1"},
			key:  "name=gw.home|type=A",
		},
		{
			name: "firewall-comment",
			path: "/ip/firewall/mangle",
			row: map[string]string{
				"chain": "prerouting", "action": "mark-connection",
				"comment": "mark-wan", "new-connection-mark": "wan",
			},
			key: "comment=mark-wan",
		},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			if got := SemanticKey(tc.path, tc.row); got != tc.key {
				t.Fatalf("SemanticKey = %q, want %q", got, tc.key)
			}
		})
	}

	noisy := map[string]string{
		"name": "ether1", "rx-byte": "1", "tx-byte": "2", "bytes": "3", "running": "true",
	}
	norm := NormalizeRow(noisy)
	if _, ok := norm["rx-byte"]; ok {
		t.Fatal("rx-byte should be dropped")
	}
	if norm["running"] != "true" || norm["name"] != "ether1" {
		t.Fatalf("norm %#v", norm)
	}
}
