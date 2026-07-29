package diff

import "strings"

// IsRuntimeKey reports whether key is runtime/counter noise that should be
// dropped from normalized rows and semantic fallback keys.
// Case-insensitive. Prefix matches: rx-*, tx-*, fp-*.
// "running" is kept (iface state); callers that need counter-only diffs can
// drop it separately.
func IsRuntimeKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "bytes", "packets", "uptime", "last-link-up-time", "link-downs":
		return true
	}
	if strings.HasPrefix(k, "rx-") || strings.HasPrefix(k, "tx-") || strings.HasPrefix(k, "fp-") {
		return true
	}
	return false
}

// NormalizeRow returns a shallow copy of row without runtime/counter noise keys.
// The input map is not modified. Nil input yields an empty map.
func NormalizeRow(row map[string]string) map[string]string {
	out := make(map[string]string, len(row))
	for k, v := range row {
		if IsRuntimeKey(k) {
			continue
		}
		out[k] = v
	}
	return out
}

// IsDynamic reports whether row is a RouterOS dynamic entry (dynamic=true|yes).
func IsDynamic(row map[string]string) bool {
	v := strings.ToLower(strings.TrimSpace(getCI(row, "dynamic")))
	return v == "true" || v == "yes"
}

// WithoutDynamic returns rows that are not dynamic. The input slice is not modified.
func WithoutDynamic(rows []map[string]string) []map[string]string {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if IsDynamic(row) {
			continue
		}
		out = append(out, row)
	}
	return out
}
