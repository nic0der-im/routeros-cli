package session

import (
	"path/filepath"
	"testing"
)

func TestBeginCommit(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sess, err := store.Begin("central hub BA", true)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if sess.Status != StatusActive {
		t.Errorf("status = %q", sess.Status)
	}
	if !sess.Safe {
		t.Error("expected safe=true")
	}

	active, err := store.Active("central hub BA")
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active == nil || active.ID != sess.ID {
		t.Fatalf("active session mismatch: %+v", active)
	}

	if err := store.AppendChange(sess, Change{
		Command: "/ip/address/add",
		Args:    []string{"=address=10.0.0.1/24"},
		Inverse: []string{"/ip/address/remove", "=.id=*1"},
	}); err != nil {
		t.Fatalf("AppendChange: %v", err)
	}

	reloaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(reloaded.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(reloaded.Changes))
	}

	if err := store.Commit(reloaded); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if reloaded.Status != StatusCommitted {
		t.Errorf("status = %q", reloaded.Status)
	}

	active, err = store.Active("central hub BA")
	if err != nil {
		t.Fatalf("Active after commit: %v", err)
	}
	if active != nil {
		t.Fatal("expected no active session after commit")
	}
}

func TestBeginDuplicateRejected(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin("r1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin("r1", true); err == nil {
		t.Fatal("expected error on duplicate begin")
	}
}

func TestMarkRolledBack(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.Begin("r1", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRolledBack(sess); err != nil {
		t.Fatal(err)
	}
	if sess.Status != StatusRolledBack {
		t.Errorf("status = %q", sess.Status)
	}
	active, _ := store.Active("r1")
	if active != nil {
		t.Fatal("expected no active session")
	}
}

func TestBuildInverse(t *testing.T) {
	inv := BuildInverse("/ip/firewall/filter/add", nil, "*A")
	if len(inv) != 2 || inv[0] != "/ip/firewall/filter/remove" || inv[1] != "=.id=*A" {
		t.Errorf("unexpected inverse: %v", inv)
	}

	inv = BuildInverse("/interface/enable", nil, "*1")
	if len(inv) != 2 || inv[0] != "/interface/disable" {
		t.Errorf("unexpected enable inverse: %v", inv)
	}

	if BuildInverse("/system/reboot", nil, "") != nil {
		t.Error("expected nil inverse for reboot")
	}
}

func TestSanitize(t *testing.T) {
	got := sanitize("central hub BA")
	if got != "central_hub_BA" {
		t.Errorf("sanitize = %q", got)
	}
	_ = filepath.Join // keep import used if needed
}

func TestDefaultDir(t *testing.T) {
	d := DefaultDir()
	if filepath.Base(d) != "sessions" {
		t.Errorf("DefaultDir base = %q", filepath.Base(d))
	}
}
