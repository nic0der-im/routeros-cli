package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAgent(t *testing.T) {
	a, err := ParseAgent("cursor")
	if err != nil || a != AgentCursor {
		t.Fatalf("got %v %v", a, err)
	}
	if _, err := ParseAgent("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDirForUserCursor(t *testing.T) {
	dir, err := DirFor(AgentCursor, ScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dir, ".cursor") || !strings.HasSuffix(dir, "skills") {
		t.Fatalf("unexpected dir %q", dir)
	}
}

func TestInstallUninstall(t *testing.T) {
	root := t.TempDir()
	results, err := Install(InstallOptions{
		Agent:   AgentCursor,
		Scope:   ScopeProject,
		Project: root,
		Force:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results=%d", len(results))
	}
	skillFile := filepath.Join(root, ".cursor", "skills", "ros", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("missing skill: %v", err)
	}

	results, err = Install(InstallOptions{
		Agent:   AgentCursor,
		Scope:   ScopeProject,
		Project: root,
		Force:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "skipped" {
		t.Fatalf("status=%s", results[0].Status)
	}

	_, err = Uninstall(InstallOptions{
		Agent:   AgentCursor,
		Scope:   ScopeProject,
		Project: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Fatal("expected removed")
	}
}

func TestListPacks(t *testing.T) {
	if len(ListPacks()) < 2 {
		t.Fatal("expected packs")
	}
}
