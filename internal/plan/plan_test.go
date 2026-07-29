package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndValidate_AddressListCreate(t *testing.T) {
	const yaml = `
device: home
steps:
  - op: create
    path: address-list
    props:
      list: blacklist
      address: 1.2.3.4
  - op: set
    path: /ip/firewall/address-list
    id: "*1"
    props:
      comment: blocked
  - op: delete
    path: firewall/address-list
    id: "*2"
`
	doc, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Device != "home" || len(doc.Steps) != 3 {
		t.Fatalf("doc=%+v", doc)
	}
	v, err := Validate(doc)
	if err != nil {
		t.Fatal(err)
	}
	if v.Steps[0].Path != "/ip/firewall/address-list" {
		t.Fatalf("path=%q", v.Steps[0].Path)
	}
	if !v.HasDeletes() {
		t.Fatal("expected HasDeletes")
	}
	args := PropsToAPIArgs(v.Steps[0].Props, "")
	if len(args) != 2 || !strings.Contains(strings.Join(args, " "), "address=1.2.3.4") {
		t.Fatalf("args=%v", args)
	}
}

func TestValidate_RejectUnknownOp(t *testing.T) {
	doc := &Document{Steps: []Step{{Op: "upsert", Path: "/ip/address", Props: map[string]string{"a": "1"}}}}
	_, err := Validate(doc)
	if err == nil || !strings.Contains(err.Error(), "unknown op") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_RejectUnknownPath(t *testing.T) {
	doc := &Document{Steps: []Step{{Op: "create", Path: "not/a/real/alias", Props: map[string]string{"a": "1"}}}}
	_, err := Validate(doc)
	if err == nil || !strings.Contains(err.Error(), "unknown path") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_DeleteRequiresTarget(t *testing.T) {
	doc := &Document{Steps: []Step{{Op: "delete", Path: "/ip/firewall/filter"}}}
	_, err := Validate(doc)
	if err == nil || !strings.Contains(err.Error(), "requires id or comment") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_CommentAndIDMutuallyExclusive(t *testing.T) {
	doc := &Document{Steps: []Step{{
		Op: "delete", Path: "/ip/firewall/filter", ID: "*1", Comment: "x",
	}}}
	_, err := Validate(doc)
	if err == nil || !strings.Contains(err.Error(), "either id or comment") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate_EnableDisableComment(t *testing.T) {
	doc := &Document{Steps: []Step{{
		Op: "disable", Path: "firewall/filter", Comment: "allow-web",
	}}}
	v, err := Validate(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Steps[0].NeedsCommentAsID() || v.Steps[0].Path != "/ip/firewall/filter" {
		t.Fatalf("%+v", v.Steps[0])
	}
}

func TestValidate_EmptySteps(t *testing.T) {
	_, err := Validate(&Document{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.yaml")
	body := "steps:\n  - op: create\n    path: /ip/dns/static\n    props:\n      name: a.lan\n      address: 1.1.1.1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Validate(doc)
	if err != nil {
		t.Fatal(err)
	}
	if v.Steps[0].Path != "/ip/dns/static" {
		t.Fatalf("path=%q", v.Steps[0].Path)
	}
}

func TestAPIAction(t *testing.T) {
	if APIAction(OpCreate) != "add" || APIAction(OpDelete) != "remove" || APIAction(OpSet) != "set" {
		t.Fatal(APIAction(OpCreate), APIAction(OpDelete), APIAction(OpSet))
	}
}
