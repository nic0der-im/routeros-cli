// Package skills embeds and installs agent skill packs for ros.
package skills

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

//go:embed all:packs
var packsFS embed.FS

// Agent identifies a target agent runtime.
type Agent string

const (
	AgentCursor   Agent = "cursor"
	AgentCodex    Agent = "codex"
	AgentClaude   Agent = "claude"
	AgentOpenCode Agent = "opencode"
	AgentAll      Agent = "all"
)

// Scope is user-global vs project-local install.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// Pack names shipped with ros.
var PackNames = []string{"ros", "ros-safe-apply"}

// ParseAgent validates an agent flag.
func ParseAgent(s string) (Agent, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "cursor":
		return AgentCursor, nil
	case "codex":
		return AgentCodex, nil
	case "claude":
		return AgentClaude, nil
	case "opencode", "oc":
		return AgentOpenCode, nil
	case "all":
		return AgentAll, nil
	default:
		return "", fmt.Errorf("unknown agent %q (cursor|codex|claude|opencode|all)", s)
	}
}

// ParseScope validates scope flag.
func ParseScope(s string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user", "":
		return ScopeUser, nil
	case "project", "repo", "local":
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("unknown scope %q (user|project)", s)
	}
}

// Agents expands "all" into concrete agents.
func Agents(a Agent) []Agent {
	if a == AgentAll {
		return []Agent{AgentCursor, AgentCodex, AgentClaude, AgentOpenCode}
	}
	return []Agent{a}
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return os.Getenv("HOME")
	}
	return h
}

// DirFor returns the skills directory for agent+scope.
// projectRoot is used when scope=project (usually cwd).
func DirFor(agent Agent, scope Scope, projectRoot string) (string, error) {
	home := homeDir()
	switch scope {
	case ScopeProject:
		if projectRoot == "" {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			projectRoot = wd
		}
		switch agent {
		case AgentCursor:
			return filepath.Join(projectRoot, ".cursor", "skills"), nil
		case AgentCodex, AgentOpenCode:
			return filepath.Join(projectRoot, ".agents", "skills"), nil
		case AgentClaude:
			return filepath.Join(projectRoot, ".claude", "skills"), nil
		default:
			return "", fmt.Errorf("unsupported agent %q", agent)
		}
	case ScopeUser:
		switch agent {
		case AgentCursor:
			return filepath.Join(home, ".cursor", "skills"), nil
		case AgentCodex:
			return filepath.Join(home, ".codex", "skills"), nil
		case AgentOpenCode:
			// OpenCode commonly shares ~/.agents/skills or project .agents/skills
			return filepath.Join(home, ".agents", "skills"), nil
		case AgentClaude:
			return filepath.Join(home, ".claude", "skills"), nil
		default:
			return "", fmt.Errorf("unsupported agent %q", agent)
		}
	default:
		return "", fmt.Errorf("unsupported scope %q", scope)
	}
}

// ListPacks returns embedded pack names.
func ListPacks() []string {
	out := append([]string{}, PackNames...)
	sort.Strings(out)
	return out
}

// InstallOptions controls install behavior.
type InstallOptions struct {
	Agent   Agent
	Scope   Scope
	Project string // project root for ScopeProject
	Force   bool
	Packs   []string // empty = all PackNames
}

// InstallResult describes one pack install.
type InstallResult struct {
	Agent  Agent
	Pack   string
	Target string
	Status string // installed|updated|skipped
}

// Install copies embedded skill packs into agent skill dirs.
func Install(opts InstallOptions) ([]InstallResult, error) {
	packs := opts.Packs
	if len(packs) == 0 {
		packs = PackNames
	}
	var results []InstallResult
	for _, agent := range Agents(opts.Agent) {
		base, err := DirFor(agent, opts.Scope, opts.Project)
		if err != nil {
			return results, err
		}
		if err := os.MkdirAll(base, 0o755); err != nil {
			return results, err
		}
		for _, pack := range packs {
			target := filepath.Join(base, pack)
			status, err := installPack(pack, target, opts.Force)
			results = append(results, InstallResult{Agent: agent, Pack: pack, Target: target, Status: status})
			if err != nil {
				return results, fmt.Errorf("%s/%s: %w", agent, pack, err)
			}
		}
	}
	return results, nil
}

func installPack(pack, target string, force bool) (string, error) {
	srcRoot := filepath.ToSlash(filepath.Join("packs", pack))
	_, err := fs.Stat(packsFS, srcRoot)
	if err != nil {
		return "", fmt.Errorf("embedded pack %q not found", pack)
	}

	if fi, err := os.Stat(target); err == nil && fi.IsDir() && !force {
		return "skipped", nil
	}
	status := "installed"
	if _, err := os.Stat(target); err == nil {
		status = "updated"
		if err := os.RemoveAll(target); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}

	err = fs.WalkDir(packsFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		return copyEmbeddedFile(path, dest)
	})
	if err != nil {
		return status, err
	}
	return status, nil
}

func copyEmbeddedFile(src, dest string) error {
	in, err := packsFS.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// Uninstall removes installed packs for agent+scope.
func Uninstall(opts InstallOptions) ([]InstallResult, error) {
	packs := opts.Packs
	if len(packs) == 0 {
		packs = PackNames
	}
	var results []InstallResult
	for _, agent := range Agents(opts.Agent) {
		base, err := DirFor(agent, opts.Scope, opts.Project)
		if err != nil {
			return results, err
		}
		for _, pack := range packs {
			target := filepath.Join(base, pack)
			status := "missing"
			if _, err := os.Stat(target); err == nil {
				if err := os.RemoveAll(target); err != nil {
					return results, err
				}
				status = "removed"
			}
			results = append(results, InstallResult{Agent: agent, Pack: pack, Target: target, Status: status})
		}
	}
	return results, nil
}

// DefaultProjectRoot returns cwd.
func DefaultProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// GOOS exposes runtime for tests/docs.
func GOOS() string { return runtime.GOOS }
