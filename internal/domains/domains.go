// Package domains maps friendly resource names to RouterOS API paths.
package domains

import "strings"

// Alias maps "firewall/filter" style names to API base paths (no trailing action).
var Alias = map[string]string{
	// P0
	"firewall/filter":       "/ip/firewall/filter",
	"firewall/nat":          "/ip/firewall/nat",
	"firewall/address-list": "/ip/firewall/address-list",
	"ip/address":            "/ip/address",
	"ip/route":              "/ip/route",
	"ip/dns":                "/ip/dns",
	"dhcp/server":           "/ip/dhcp-server",
	"dhcp/lease":            "/ip/dhcp-server/lease",
	"dhcp/network":          "/ip/dhcp-server/network",
	"dhcp/pool":             "/ip/pool",
	"user":                  "/user",
	"user/group":            "/user/group",

	// P1
	"interface":              "/interface",
	"interface/ethernet":     "/interface/ethernet",
	"interface/wireguard":    "/interface/wireguard",
	"interface/wireguard/peers": "/interface/wireguard/peers",
	"service":                "/ip/service",
	"ip/service":             "/ip/service",
	"dhcp-client":            "/ip/dhcp-client",
	"ip/dhcp-client":         "/ip/dhcp-client",
	"firewall/mangle":        "/ip/firewall/mangle",
	"firewall/raw":           "/ip/firewall/raw",
	"system/identity":        "/system/identity",
	"system/clock":           "/system/clock",
	"system/ntp/client":      "/system/ntp/client",
	"system/package":         "/system/package",
	"system/resource":        "/system/resource",
	"radius":                 "/radius",
	"ppp/secret":             "/ppp/secret",
	"ppp/profile":            "/ppp/profile",
	"queue/simple":           "/queue/simple",
	"queue/tree":             "/queue/tree",
	"certificate":            "/certificate",
	"routing/filter":         "/routing/filter",
	"routing/rule":           "/routing/rule",
	"routing/bgp":            "/routing/bgp",
	"interface/wifi":         "/interface/wifi",
	"interface/wireless":     "/interface/wireless",
	"interface/wifiwave2":    "/interface/wifiwave2",
	"container":              "/container",
	"container/config":       "/container/config",
	"container/envs":         "/container/envs",
	"container/mounts":       "/container/mounts",
	"tool/bandwidth-test":    "/tool/bandwidth-test",
	"tool/traceroute":        "/tool/traceroute",

	// L2
	"interface/bridge":         "/interface/bridge",
	"interface/bridge/port":    "/interface/bridge/port",
	"interface/vlan":           "/interface/vlan",
	"interface/bonding":        "/interface/bonding",

	// Diag
	"log":           "/log",
	"tool/ping":     "/ping", // special: /ping
	"ping":          "/ping",
	"tool/torch":    "/tool/torch",
	"ip/neighbor":   "/ip/neighbor",
	"file":          "/file",
}

// Resolve turns a friendly name or raw path into a normalized API base path.
func Resolve(nameOrPath string) (string, bool) {
	s := strings.TrimSpace(nameOrPath)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "/") {
		return strings.TrimSuffix(s, "/"), true
	}
	key := strings.Trim(strings.ReplaceAll(s, " ", "/"), "/")
	key = strings.ToLower(key)
	if p, ok := Alias[key]; ok {
		return p, true
	}
	// try without normalizing case on last segment
	if p, ok := Alias[strings.Trim(s, "/")]; ok {
		return p, true
	}
	return "", false
}

// JoinFriendly joins cobra args like ["firewall","filter"] → "firewall/filter".
func JoinFriendly(parts []string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "/")
}

// List returns sorted alias keys.
func List() []string {
	keys := make([]string, 0, len(Alias))
	for k := range Alias {
		keys = append(keys, k)
	}
	// simple insertion sort-ish via strings
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
