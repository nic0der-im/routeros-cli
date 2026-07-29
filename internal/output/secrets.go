package output

import "strings"

// RedactedPlaceholder replaces secret field values in human table and default JSON.
const RedactedPlaceholder = "***"

// knownSecretKeys are RouterOS property names that should never appear in agent-facing
// output by default (WireGuard keys, WiFi PSKs, passwords, etc.).
var knownSecretKeys = map[string]struct{}{
	"private-key":         {},
	"pre-shared-key":      {},
	"preshared-key":       {},
	"password":            {},
	"shared-secret":       {},
	"secret":              {},
	"wpa2-pre-shared-key": {},
	"passphrase":          {},
}

// IsSecretKey reports whether a field name is a known secret (case-insensitive).
func IsSecretKey(key string) bool {
	_, ok := knownSecretKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// RedactValue returns RedactedPlaceholder for secret keys with non-empty values;
// otherwise returns value unchanged. Empty values stay empty so missing secrets
// are distinguishable from redacted ones.
func RedactValue(key, value string) string {
	if value == "" || !IsSecretKey(key) {
		return value
	}
	return RedactedPlaceholder
}

// RedactRecord returns a copy of m with known secret values replaced.
func RedactRecord(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = RedactValue(k, v)
	}
	return out
}

// RedactRecords redacts each map in records.
func RedactRecords(records []map[string]string) []map[string]string {
	if records == nil {
		return nil
	}
	out := make([]map[string]string, len(records))
	for i, r := range records {
		out[i] = RedactRecord(r)
	}
	return out
}

// RedactPayload walks arbitrary JSON-shaped data (audit sections, nested maps/slices)
// and redacts known secret string fields. Non-map values are returned unchanged.
func RedactPayload(data interface{}) interface{} {
	switch v := data.(type) {
	case nil:
		return nil
	case map[string]string:
		return RedactRecord(v)
	case []map[string]string:
		return RedactRecords(v)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				out[k] = RedactValue(k, s)
				continue
			}
			out[k] = RedactPayload(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = RedactPayload(item)
		}
		return out
	case []map[string]interface{}:
		out := make([]map[string]interface{}, len(v))
		for i, item := range v {
			redacted, _ := RedactPayload(item).(map[string]interface{})
			out[i] = redacted
		}
		return out
	default:
		return data
	}
}
