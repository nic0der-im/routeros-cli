package session

import "strings"

// readOnlyKeys are RouterOS fields that must not be sent back on /add or /set restore.
var readOnlyKeys = map[string]struct{}{
	".id": {}, "id": {}, "bytes": {}, "packets": {}, "dynamic": {}, "invalid": {},
	"running": {}, "slave": {}, "inactive": {}, "actual-interface": {},
	"active-address": {}, "active-mac-address": {}, "active-client-id": {},
	"active-server": {}, "expires-after": {}, "last-seen": {}, "status": {},
	"last-logged-in": {}, "radius": {}, "creation-time": {},
}


// IsReadOnlyField reports whether a printed field should be omitted from restore args.
func IsReadOnlyField(key string) bool {
	_, ok := readOnlyKeys[strings.ToLower(key)]
	return ok
}

// BuildSetInverse restores previously changed properties via /set.
// changedArgs are the =key=value pairs from the original set (excluding .id).
func BuildSetInverse(setCommand, id string, preState map[string]string, changedArgs []string) []string {
	if id == "" || !endsWith(setCommand, "/set") || preState == nil {
		return nil
	}
	inv := []string{setCommand, "=.id=" + id}
	for _, a := range changedArgs {
		key, ok := argKey(a)
		if !ok || key == ".id" || key == "id" {
			continue
		}
		if old, exists := preState[key]; exists {
			inv = append(inv, "="+key+"="+old)
		}
	}
	if len(inv) <= 2 {
		return nil
	}
	return inv
}

// BuildRemoveInverse recreates a removed row via /add using printable writable fields.
func BuildRemoveInverse(removeCommand string, preState map[string]string) []string {
	if preState == nil || !endsWith(removeCommand, "/remove") {
		return nil
	}
	base := removeCommand[:len(removeCommand)-len("/remove")]
	inv := []string{base + "/add"}
	for k, v := range preState {
		if IsReadOnlyField(k) || v == "" {
			continue
		}
		inv = append(inv, "="+k+"="+v)
	}
	if len(inv) == 1 {
		return nil
	}
	return inv
}

// ChangedSetKeys extracts property keys from set args (excluding .id).
func ChangedSetKeys(args []string) []string {
	var keys []string
	for _, a := range args {
		if key, ok := argKey(a); ok && key != ".id" && key != "id" {
			keys = append(keys, key)
		}
	}
	return keys
}

func argKey(a string) (string, bool) {
	a = strings.TrimPrefix(a, "=")
	if i := strings.IndexByte(a, '='); i > 0 {
		return a[:i], true
	}
	return "", false
}
