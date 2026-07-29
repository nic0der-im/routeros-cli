package skills

import (
	"io/fs"
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
	for _, rel := range []string{
		filepath.Join("ros", "references", "commands.md"),
		filepath.Join("ros", "references", "agents.md"),
		filepath.Join("ros", "references", "safety-and-recovery.md"),
		filepath.Join("ros", "references", "routeros-docs.md"),
		filepath.Join("ros-safe-apply", "references", "commands.md"),
		filepath.Join("ros-safe-apply", "references", "agents.md"),
		filepath.Join("ros-safe-apply", "references", "safety-and-recovery.md"),
		filepath.Join("ros-safe-apply", "references", "routeros-docs.md"),
	} {
		p := filepath.Join(root, ".cursor", "skills", rel)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing installed reference %s: %v", rel, err)
		}
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

func TestEmbeddedPackReferences(t *testing.T) {
	required := []string{
		"packs/ros/SKILL.md",
		"packs/ros/references/commands.md",
		"packs/ros/references/agents.md",
		"packs/ros/references/safety-and-recovery.md",
		"packs/ros/references/routeros-docs.md",
		"packs/ros-safe-apply/SKILL.md",
		"packs/ros-safe-apply/references/commands.md",
		"packs/ros-safe-apply/references/agents.md",
		"packs/ros-safe-apply/references/safety-and-recovery.md",
		"packs/ros-safe-apply/references/routeros-docs.md",
	}
	for _, path := range required {
		if _, err := fs.Stat(packsFS, path); err != nil {
			t.Fatalf("missing embedded %s: %v", path, err)
		}
	}
}
