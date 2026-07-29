// Package plan defines the YAML change-plan schema for ros plan preview|apply.
// Plans compose existing dry-run/diff and safe-session journaling — they are
// not a separate MCP-style journal.
package plan

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/domains"
	"gopkg.in/yaml.v3"
)

// Supported mutation ops (unknown ops are rejected).
const (
	OpCreate  = "create"
	OpSet     = "set"
	OpDelete  = "delete"
	OpEnable  = "enable"
	OpDisable = "disable"
)

var knownOps = map[string]struct{}{
	OpCreate:  {},
	OpSet:     {},
	OpDelete:  {},
	OpEnable:  {},
	OpDisable: {},
}

// Step is one mutation in a plan file.
type Step struct {
	Op      string            `yaml:"op" json:"op"`
	Path    string            `yaml:"path" json:"path"`
	Props   map[string]string `yaml:"props,omitempty" json:"props,omitempty"`
	ID      string            `yaml:"id,omitempty" json:"id,omitempty"`
	Comment string            `yaml:"comment,omitempty" json:"comment,omitempty"`
}

// Document is the on-disk YAML plan schema.
//
//	device: home          # optional; else -d / default device
//	steps:
//	  - op: create|set|delete|enable|disable
//	    path: /ip/firewall/address-list   # or friendly alias
//	    props:                            # create/set
//	      list: blacklist
//	      address: 1.2.3.4
//	    id: "*1"                          # set/delete/enable/disable
//	    comment: "stable-id"              # filter/mangle comment-as-ID
type Document struct {
	Device string `yaml:"device,omitempty" json:"device,omitempty"`
	Steps  []Step `yaml:"steps" json:"steps"`
}

// ValidatedStep is a Step after path/op validation (API path resolved).
type ValidatedStep struct {
	Index     int
	Op        string
	Path      string // resolved RouterOS API base path
	PathInput string
	Props     map[string]string
	ID        string
	Comment   string
}

// Validated is a Document after successful Validate.
type Validated struct {
	Device string
	Steps  []ValidatedStep
}

// Parse unmarshals YAML bytes into a Document (no semantic validation).
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing plan YAML: %w", err)
	}
	return &doc, nil
}

// LoadFile reads and parses a plan YAML file.
func LoadFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file %q: %w", path, err)
	}
	return Parse(data)
}

// Validate checks ops/paths/targets and resolves friendly path aliases.
func Validate(doc *Document) (*Validated, error) {
	if doc == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	if len(doc.Steps) == 0 {
		return nil, fmt.Errorf("plan requires at least one step")
	}
	out := &Validated{Device: strings.TrimSpace(doc.Device)}
	out.Steps = make([]ValidatedStep, 0, len(doc.Steps))
	for i, s := range doc.Steps {
		vs, err := validateStep(i, s)
		if err != nil {
			return nil, err
		}
		out.Steps = append(out.Steps, vs)
	}
	return out, nil
}

func validateStep(index int, s Step) (ValidatedStep, error) {
	op := strings.ToLower(strings.TrimSpace(s.Op))
	if op == "" {
		return ValidatedStep{}, fmt.Errorf("step %d: op is required", index)
	}
	if _, ok := knownOps[op]; !ok {
		return ValidatedStep{}, fmt.Errorf("step %d: unknown op %q (want create|set|delete|enable|disable)", index, s.Op)
	}
	pathInput := strings.TrimSpace(s.Path)
	if pathInput == "" {
		return ValidatedStep{}, fmt.Errorf("step %d: path is required", index)
	}
	resolved, ok := domains.Resolve(pathInput)
	if !ok {
		return ValidatedStep{}, fmt.Errorf("step %d: unknown path %q (use /api/path or a known domain; see: ros domains)", index, pathInput)
	}

	id := strings.TrimSpace(s.ID)
	comment := strings.TrimSpace(s.Comment)
	if id != "" && comment != "" {
		return ValidatedStep{}, fmt.Errorf("step %d: specify either id or comment, not both", index)
	}

	props := cloneProps(s.Props)
	switch op {
	case OpCreate:
		if len(props) == 0 {
			return ValidatedStep{}, fmt.Errorf("step %d: create requires props", index)
		}
		if id != "" || comment != "" {
			return ValidatedStep{}, fmt.Errorf("step %d: create does not take id/comment (put comment in props if needed)", index)
		}
	case OpSet:
		if len(props) == 0 {
			return ValidatedStep{}, fmt.Errorf("step %d: set requires props", index)
		}
	case OpDelete, OpEnable, OpDisable:
		if id == "" && comment == "" {
			return ValidatedStep{}, fmt.Errorf("step %d: %s requires id or comment", index, op)
		}
		if len(props) > 0 {
			return ValidatedStep{}, fmt.Errorf("step %d: %s does not take props", index, op)
		}
	}

	return ValidatedStep{
		Index:     index,
		Op:        op,
		Path:      resolved,
		PathInput: pathInput,
		Props:     props,
		ID:        id,
		Comment:   comment,
	}, nil
}

// HasDeletes reports whether any step is a delete.
func (v *Validated) HasDeletes() bool {
	if v == nil {
		return false
	}
	for _, s := range v.Steps {
		if s.Op == OpDelete {
			return true
		}
	}
	return false
}

// IsDestructive reports whether the step is delete-class destructive.
func (s ValidatedStep) IsDestructive() bool {
	return s.Op == OpDelete
}

// NeedsCommentAsID reports whether the step uses comment targeting.
func (s ValidatedStep) NeedsCommentAsID() bool {
	return s.Comment != ""
}

// APIAction maps a plan op to the RouterOS API verb suffix.
func APIAction(op string) string {
	switch op {
	case OpCreate:
		return "add"
	case OpDelete:
		return "remove"
	default:
		return op
	}
}

// PropsToAPIArgs converts props (+ optional .id) into RouterOS API =key=value args.
// Keys are sorted for stable diffs/tests. .id is placed first when present.
func PropsToAPIArgs(props map[string]string, id string) []string {
	keys := make([]string, 0, len(props))
	for k := range props {
		k = strings.TrimSpace(k)
		if k == "" || k == ".id" || k == "id" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys)+1)
	if id != "" {
		out = append(out, "=.id="+id)
	}
	for _, k := range keys {
		out = append(out, "="+k+"="+props[k])
	}
	return out
}

func cloneProps(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
