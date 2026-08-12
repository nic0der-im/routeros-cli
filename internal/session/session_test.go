package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeginWithMeta(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.BeginWith("r1", BeginOpts{
		Safe:      true,
		Note:      "force-no-backup",
		BackupDir: "/tmp/backups/r1/ts",
	})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Note != "force-no-backup" {
		t.Errorf("note=%q", reloaded.Note)
	}
	if reloaded.BackupDir != "/tmp/backups/r1/ts" {
		t.Errorf("backup_dir=%q", reloaded.BackupDir)
	}
}

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

func TestBuildSetAndRemoveInverse(t *testing.T) {
	pre := map[string]string{
		".id":        "*1",
		"lease-time": "30m",
		"name":       "dhcpNetwork",
		"bytes":      "999",
		"disabled":   "false",
	}
	inv := BuildSetInverse("/ip/dhcp-server/set", "*1", pre, []string{"=.id=*1", "=lease-time=1d"})
	if len(inv) < 3 || inv[0] != "/ip/dhcp-server/set" || inv[1] != "=.id=*1" {
		t.Fatalf("set inverse: %v", inv)
	}
	found := false
	for _, a := range inv {
		if a == "=lease-time=30m" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected restored lease-time in %v", inv)
	}

	rm := BuildRemoveInverse("/ip/firewall/filter/remove", pre)
	if len(rm) < 2 || rm[0] != "/ip/firewall/filter/add" {
		t.Fatalf("remove inverse: %v", rm)
	}
	for _, a := range rm {
		if strings.Contains(a, "=.id=") || strings.HasPrefix(a, "=bytes=") {
			t.Fatalf("read-only field leaked into inverse: %v", rm)
		}
	}
}

func TestBuildSetInverseSingleton(t *testing.T) {
	pre := map[string]string{
		"ddns-enabled":   "yes",
		"update-time":    "true",
		"public-address": "1.2.3.4",
		"status":         "updated",
	}
	inv := BuildSetInverse("/ip/cloud/set", "", pre, []string{
		"=ddns-enabled=auto",
		"=update-time=false",
	})
	if len(inv) < 3 || inv[0] != "/ip/cloud/set" {
		t.Fatalf("singleton inverse: %v", inv)
	}
	for _, a := range inv {
		if strings.HasPrefix(a, "=.id=") || strings.HasPrefix(a, "=id=") {
			t.Fatalf("singleton inverse must not include .id: %v", inv)
		}
	}
	want := map[string]bool{"=ddns-enabled=yes": false, "=update-time=true": false}
	for _, a := range inv[1:] {
		if _, ok := want[a]; ok {
			want[a] = true
		}
	}
	for a, ok := range want {
		if !ok {
			t.Fatalf("missing restored arg %s in %v", a, inv)
		}
	}

	// No matching pre-state keys → nil
	if BuildSetInverse("/ip/cloud/set", "", pre, []string{"=unknown-prop=x"}) != nil {
		t.Fatal("expected nil when no changed keys exist in pre-state")
	}
	// Empty id with nil pre-state → nil
	if BuildSetInverse("/ip/cloud/set", "", nil, []string{"=ddns-enabled=auto"}) != nil {
		t.Fatal("expected nil for nil pre-state")
	}
}

func TestSafeFalseStillActiveButNotJournaledViaAppendStillWorks(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.Begin("r1", false)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Safe {
		t.Fatal("expected safe=false")
	}
}

func TestMarkAutoRollbackPending(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.Begin("r1", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAutoRollbackPending(sess); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoRollbackPending {
		t.Fatal("expected auto_rollback_pending")
	}
}

func TestPurgeDevice(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Two journals for the target device (one closed, one still active) and
	// one belonging to a device that must survive the purge.
	closed, err := store.Begin("edge core", true)
	if err != nil {
		t.Fatalf("Begin closed: %v", err)
	}
	if err := store.Commit(closed); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	active, err := store.Begin("edge core", true)
	if err != nil {
		t.Fatalf("Begin active: %v", err)
	}
	other, err := store.Begin("branch-lab", true)
	if err != nil {
		t.Fatalf("Begin other: %v", err)
	}

	n, err := store.PurgeDevice("edge core")
	if err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if n != 2 {
		t.Errorf("removed %d journals, want 2", n)
	}

	for _, id := range []string{closed.ID, active.ID} {
		if _, err := os.Stat(filepath.Join(dir, id+".json")); !os.IsNotExist(err) {
			t.Errorf("journal %s still present: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "active-edge_core.lock")); !os.IsNotExist(err) {
		t.Errorf("active lock still present: %v", err)
	}

	// The unrelated device keeps both its journal and its active lock.
	if _, err := os.Stat(filepath.Join(dir, other.ID+".json")); err != nil {
		t.Errorf("unrelated journal removed: %v", err)
	}
	if got, err := store.Active("branch-lab"); err != nil || got == nil {
		t.Errorf("unrelated active session lost: got=%v err=%v", got, err)
	}

	// Purging a device with no state is a no-op, not an error.
	if n, err := store.PurgeDevice("never-seen"); err != nil || n != 0 {
		t.Errorf("purge of unknown device: n=%d err=%v", n, err)
	}
}
