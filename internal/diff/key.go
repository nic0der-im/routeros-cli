package diff

import (
	"sort"
	"strings"
)

// SemanticKey returns a stable identity string for a printed RouterOS row at path.
// Known paths use curated fields; unknown paths fall back to sorted non-runtime keys.
func SemanticKey(path string, row map[string]string) string {
	path = basePath(path)
	if row == nil {
		row = map[string]string{}
	}
	switch path {
	case "/ip/firewall/address-list":
		return joinKeyParts(
			"list", getCI(row, "list"),
			"address", getCI(row, "address"),
		)
	case "/ip/dns/static":
		typ := getCI(row, "type")
		if typ == "" {
			typ = "A"
		}
		return joinKeyParts(
			"name", getCI(row, "name"),
			"type", typ,
		)
	case "/ip/route":
		table := getCI(row, "routing-table")
		if table == "" {
			table = "main"
		}
		return joinKeyParts(
			"dst-address", getCI(row, "dst-address"),
			"gateway", getCI(row, "gateway"),
			"routing-table", table,
		)
	case "/ip/address":
		return joinKeyParts(
			"address", getCI(row, "address"),
			"interface", getCI(row, "interface"),
		)
	case "/ip/firewall/filter", "/ip/firewall/nat", "/ip/firewall/mangle":
		if comment := strings.TrimSpace(getCI(row, "comment")); comment != "" {
			return joinKeyParts("comment", comment)
		}
		return joinKeyParts(
			"chain", getCI(row, "chain"),
			"action", getCI(row, "action"),
			"protocol", getCI(row, "protocol"),
			"dst-port", getCI(row, "dst-port"),
			"src-address", getCI(row, "src-address"),
			"dst-address", getCI(row, "dst-address"),
			"in-interface", getCI(row, "in-interface"),
			"out-interface", getCI(row, "out-interface"),
		)
	default:
		return fallbackSemanticKey(row)
	}
}

func joinKeyParts(kv ...string) string {
	if len(kv)%2 != 0 {
		panic("joinKeyParts: odd number of args")
	}
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		parts = append(parts, kv[i]+"="+kv[i+1])
	}
	return strings.Join(parts, "|")
}

func fallbackSemanticKey(row map[string]string) string {
	type pair struct{ k, v string }
	parts := make([]pair, 0, len(row))
	for k, v := range row {
		lk := strings.ToLower(k)
		if IsRuntimeKey(k) || lk == ".id" || lk == "id" || lk == ".nextid" {
			continue
		}
		parts = append(parts, pair{lk, v})
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].k < parts[j].k })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p.k+"="+p.v)
	}
	return strings.Join(out, "|")
}
