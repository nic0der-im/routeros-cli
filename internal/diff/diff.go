// Package diff provides RouterOS row normalization and semantic dry-run diffs.
package diff

import "strings"

// Item is one planned create, update, or remove in a Diff.
type Item struct {
	Path   string            `json:"path,omitempty"`
	Key    string            `json:"key,omitempty"` // semantic identity
	ID     string            `json:"id,omitempty"`  // RouterOS .id when known
	Before map[string]string `json:"before,omitempty"`
	After  map[string]string `json:"after,omitempty"`
}

// Diff is a dry-run plan of row-level changes against a printed table.
type Diff struct {
	ToCreate []Item   `json:"to_create,omitempty"`
	ToUpdate []Item   `json:"to_update,omitempty"`
	ToRemove []Item   `json:"to_remove,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Empty reports whether the Diff plans no creates, updates, or removes.
func (d Diff) Empty() bool {
	return len(d.ToCreate) == 0 && len(d.ToUpdate) == 0 && len(d.ToRemove) == 0
}

// Stable warning tokens for dry-run / write-outcome consumers.
const (
	WarnAlreadyExists = "already_exists"
	WarnNotFound      = "not_found"
	WarnFirewallOrder = "firewall rule order is not restored by semantic match alone"
)

const (
	warnAlreadyExists     = WarnAlreadyExists
	warnNotFound          = WarnNotFound
	warnFirewallRuleOrder = WarnFirewallOrder
)

// DiffCreate plans adding newRow. If a matching semantic key already exists
// among existingRows, returns an empty Diff with warning "already_exists".
func DiffCreate(path string, existingRows []map[string]string, newRow map[string]string) Diff {
	path = basePath(path)
	key := SemanticKey(path, newRow)
	for _, row := range existingRows {
		if SemanticKey(path, row) == key {
			return Diff{Warnings: []string{warnAlreadyExists}}
		}
	}
	d := Diff{
		ToCreate: []Item{{
			Path:  path,
			Key:   key,
			After: NormalizeRow(newRow),
		}},
	}
	d.warnFirewallOrder(path)
	return d
}

// DiffDelete plans removing the row identified by idOrKey (.id or SemanticKey).
// If no row matches, returns an empty Diff with warning "not_found".
func DiffDelete(path string, existingRows []map[string]string, idOrKey string) Diff {
	path = basePath(path)
	idOrKey = strings.TrimSpace(idOrKey)
	if idOrKey == "" {
		return Diff{Warnings: []string{warnNotFound}}
	}
	for _, row := range existingRows {
		id := getCI(row, ".id")
		key := SemanticKey(path, row)
		if idOrKey == id || idOrKey == key {
			d := Diff{
				ToRemove: []Item{{
					Path:   path,
					Key:    key,
					ID:     id,
					Before: NormalizeRow(row),
				}},
			}
			d.warnFirewallOrder(path)
			return d
		}
	}
	return Diff{Warnings: []string{warnNotFound}}
}

// DiffSet plans a property-level update of existingRow using desiredChanges.
// Only keys present in desiredChanges are compared. Unchanged Diff when no
// property values differ.
func DiffSet(path string, existingRow, desiredChanges map[string]string) Diff {
	path = basePath(path)
	if existingRow == nil {
		existingRow = map[string]string{}
	}
	before := make(map[string]string)
	after := make(map[string]string)
	for k, newV := range desiredChanges {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" || lk == ".id" || lk == "id" || IsRuntimeKey(k) {
			continue
		}
		oldV := getCI(existingRow, k)
		if oldV == newV {
			continue
		}
		before[k] = oldV
		after[k] = newV
	}
	if len(after) == 0 {
		return Diff{}
	}
	return Diff{
		ToUpdate: []Item{{
			Path:   path,
			Key:    SemanticKey(path, existingRow),
			ID:     getCI(existingRow, ".id"),
			Before: before,
			After:  after,
		}},
	}
}

// Compare finds a row in current by matchKey (.id or semantic key). When
// matchKey is empty, it uses SemanticKey(path, desired). Missing rows become
// creates; found rows become property-level updates via DiffSet.
func Compare(path string, current []map[string]string, desired map[string]string, matchKey string) Diff {
	path = basePath(path)
	matchKey = strings.TrimSpace(matchKey)
	var found map[string]string
	if matchKey != "" {
		for _, row := range current {
			if getCI(row, ".id") == matchKey || SemanticKey(path, row) == matchKey {
				found = row
				break
			}
		}
	} else {
		want := SemanticKey(path, desired)
		for _, row := range current {
			if SemanticKey(path, row) == want {
				found = row
				break
			}
		}
	}
	if found == nil {
		return DiffCreate(path, current, desired)
	}
	return DiffSet(path, found, desired)
}

func (d *Diff) warnFirewallOrder(path string) {
	if !isFirewallOrderedPath(path) {
		return
	}
	for _, w := range d.Warnings {
		if w == warnFirewallRuleOrder {
			return
		}
	}
	d.Warnings = append(d.Warnings, warnFirewallRuleOrder)
}

func isFirewallOrderedPath(path string) bool {
	switch basePath(path) {
	case "/ip/firewall/filter", "/ip/firewall/nat", "/ip/firewall/mangle", "/ip/firewall/raw":
		return true
	default:
		return false
	}
}

func basePath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	lower := strings.ToLower(p)
	for _, suf := range []string{"/add", "/set", "/remove", "/print", "/get", "/enable", "/disable"} {
		if strings.HasSuffix(lower, suf) {
			return p[:len(p)-len(suf)]
		}
	}
	return p
}

func getCI(row map[string]string, key string) string {
	if row == nil {
		return ""
	}
	if v, ok := row[key]; ok {
		return v
	}
	want := strings.ToLower(key)
	for k, v := range row {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return ""
}
