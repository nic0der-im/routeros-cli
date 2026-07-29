package domains

import "testing"

func TestResolvePath(t *testing.T) {
	p, ok := Resolve("/ip/firewall/filter")
	if !ok || p != "/ip/firewall/filter" {
		t.Fatalf("got %q %v", p, ok)
	}
}

func TestResolveAlias(t *testing.T) {
	p, ok := Resolve("firewall/filter")
	if !ok || p != "/ip/firewall/filter" {
		t.Fatalf("got %q %v", p, ok)
	}
	p, ok = Resolve("user")
	if !ok || p != "/user" {
		t.Fatalf("user: %q %v", p, ok)
	}
	p, ok = Resolve("radius")
	if !ok || p != "/radius" {
		t.Fatalf("radius: %q %v", p, ok)
	}
}

func TestResolveStripsTrailingPrintGet(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/interface/print", "/interface"},
		{"/interface/PRINT", "/interface"},
		{"/ip/address/get", "/ip/address"},
		{"/ip/address/Get/", "/ip/address"},
		{"interface/print", "/interface"},
		{"firewall/filter/print", "/ip/firewall/filter"},
		{"ip/cloud/print", "/ip/cloud"},
		{"/interface", "/interface"},
	}
	for _, tc := range cases {
		p, ok := Resolve(tc.in)
		if !ok || p != tc.want {
			t.Errorf("Resolve(%q)=%q,%v want %q,true", tc.in, p, ok, tc.want)
		}
	}
}

func TestStripTrailingReadAction(t *testing.T) {
	if got := StripTrailingReadAction("/system/resource/print"); got != "/system/resource" {
		t.Fatalf("got %q", got)
	}
	if got := StripTrailingReadAction("/system/resource"); got != "/system/resource" {
		t.Fatalf("unchanged: got %q", got)
	}
}

func TestResolveNewAliases(t *testing.T) {
	cases := map[string]string{
		"ip/cloud":                       "/ip/cloud",
		"system/logging":                 "/system/logging",
		"system/scheduler":               "/system/scheduler",
		"system/script":                  "/system/script",
		"system/health":                  "/system/health",
		"ip/settings":                    "/ip/settings",
		"firewall/connection":            "/ip/firewall/connection",
		"ip/neighbor/discovery-settings": "/ip/neighbor/discovery-settings",
		"tool/bandwidth-server":          "/tool/bandwidth-server",
		"interface/list":                 "/interface/list",
		"interface/list/member":          "/interface/list/member",
		// B1
		"dns/static":                            "/ip/dns/static",
		"arp":                                   "/ip/arp",
		"ip/arp":                                "/ip/arp",
		"netwatch":                              "/tool/netwatch",
		"tool/netwatch":                         "/tool/netwatch",
		"routing/table":                         "/routing/table",
		"bgp/session":                           "/routing/bgp/session",
		"routing/bgp/session":                   "/routing/bgp/session",
		"ospf":                                  "/routing/ospf/instance",
		"routing/ospf":                          "/routing/ospf/instance",
		"ospf/neighbor":                         "/routing/ospf/neighbor",
		"routing/ospf/neighbor":                 "/routing/ospf/neighbor",
		"ospf/interface":                        "/routing/ospf/interface-template",
		"ospf/interface-template":               "/routing/ospf/interface-template",
		"wifi/registration":                     "/interface/wifi/registration",
		"interface/wifi/registration":           "/interface/wifi/registration",
		"interface/wireless/registration-table": "/interface/wireless/registration-table",
		"address-list":                          "/ip/firewall/address-list",
		"wg":                                    "/interface/wireguard",
		"wg/peers":                              "/interface/wireguard/peers",
		"ipv6/address":                          "/ipv6/address",
		"ipv6/route":                            "/ipv6/route",
		"ipv6/firewall/filter":                  "/ipv6/firewall/filter",
		"queue/type":                            "/queue/type",
		"system/logging/action":                 "/system/logging/action",
	}
	for in, want := range cases {
		p, ok := Resolve(in)
		if !ok || p != want {
			t.Errorf("Resolve(%q)=%q,%v want %q,true", in, p, ok, want)
		}
	}
}

func TestJoinFriendly(t *testing.T) {
	if got := JoinFriendly([]string{"firewall", "filter"}); got != "firewall/filter" {
		t.Fatalf("got %q", got)
	}
}

func TestListNonEmpty(t *testing.T) {
	if len(List()) < 10 {
		t.Fatal("expected many aliases")
	}
	found := false
	for _, k := range List() {
		if k == "ip/cloud" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("List() missing ip/cloud")
	}
}
