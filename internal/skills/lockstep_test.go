package skills

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/domains"
)

const skillPackVersion = "0.5.0"

// productCommandRoots are top-level ros verbs that skill docs may mention.
// Kept in lockstep with cmd.Execute (see cmd/skills_lockstep_test.go).
var productCommandRoots = map[string]bool{
	"version": true, "device": true, "system": true, "interface": true,
	"ip": true, "firewall": true, "dns": true, "dhcp": true, "backup": true,
	"file": true, "monitor": true, "exec": true, "schema": true, "audit": true,
	"doctor": true, "session": true, "plan": true, "get": true, "create": true,
	"set": true, "delete": true, "enable": true, "disable": true, "domains": true,
	"diag": true, "skills": true, "nat": true, "lease": true, "wg": true,
	"wifi": true, "bgp": true, "ospf": true,
}

// productSubcommands maps parent → allowed child verbs.
var productSubcommands = map[string]map[string]bool{
	"device":      {"list": true, "add": true, "remove": true, "use": true, "test": true, "auth": true, "get": true, "import": true},
	"device auth": {"set": true},
	"session":     {"begin": true, "commit": true, "rollback": true, "watch": true, "status": true},
	"plan":        {"preview": true, "apply": true, "rollback": true},
	"diag":        {"log": true, "ping": true, "neighbors": true, "traceroute": true, "torch": true, "bandwidth-test": true, "discover": true},
	"file":        {"list": true, "get": true, "remove": true},
	"backup":      {"binary": true, "export": true},
	"firewall":    {"filter": true, "nat": true, "address-list": true},
	"dns":         {"static": true},
	"dhcp":        {"lease": true, "server": true, "pool": true},
	"wg":          {"peers": true},
	"wifi":        {"clients": true},
	"bgp":         {"sessions": true},
	"ospf":        {"neighbors": true},
	"skills":      {"install": true, "uninstall": true, "list": true, "path": true},
	"lease":       {"cleanup-waiting": true},
	"system":      {"reboot": true, "identity": true, "resource": true, "clock": true},
	"monitor":     {"traffic": true, "resources": true},
	"interface":   {"wireguard": true, "list": true, "ethernet": true},
	"ip":          {"address": true, "route": true, "dns": true, "cloud": true, "service": true},
	"get":         {"wg": true, "wifi": true, "bgp": true, "ospf": true, "system": true, "interface": true, "ip": true, "firewall": true, "dns": true, "dhcp": true},
	"create":      {"ip": true, "firewall": true, "dns": true},
	"delete":      {"ip": true, "firewall": true, "dns": true, "file": true},
	"set":         {"identity": true},
}

var domainBearingVerbs = map[string]bool{
	"get": true, "create": true, "set": true, "delete": true, "enable": true, "disable": true,
}

var (
	frontmatterVersionRe = regexp.MustCompile(`(?m)^\s*version:\s*"([^"]+)"`)
	fenceRe              = regexp.MustCompile("(?s)```[^\\n]*\\n(.*?)```")
	inlineTickRe         = regexp.MustCompile("`([^`\\n]+)`")
	rosLineRe            = regexp.MustCompile(`(?i)(?:^|[;&|]\s*)ros\b`)
)

func TestSkillPackVersionMetadata(t *testing.T) {
	for _, path := range []string{
		"packs/ros/SKILL.md",
		"packs/ros-safe-apply/SKILL.md",
	} {
		data, err := fs.ReadFile(packsFS, path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		m := frontmatterVersionRe.FindSubmatch(data)
		if m == nil {
			t.Fatalf("%s: missing version metadata", path)
		}
		if got := string(m[1]); got != skillPackVersion {
			t.Fatalf("%s: version=%q want %q", path, got, skillPackVersion)
		}
	}
}

func TestSkillDocsLockstep(t *testing.T) {
	root := moduleRoot(t)
	srcTree := filepath.Join(root, "skills")
	packTree := filepath.Join(root, "internal", "skills", "packs")
	assertTreesEqual(t, srcTree, packTree)

	aliases := domains.Alias
	var unknowns []string
	for _, tree := range []string{srcTree, packTree} {
		err := filepath.WalkDir(tree, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if filepath.Base(path) == "SKILL.md" {
				m := frontmatterVersionRe.FindSubmatch(data)
				if m == nil || string(m[1]) != skillPackVersion {
					t.Errorf("%s: skill metadata version want %s", rel, skillPackVersion)
				}
			}
			body := stripFrontmatter(string(data))
			for _, mention := range extractCommandMentions(body) {
				if err := validateMention(mention, aliases); err != nil {
					unknowns = append(unknowns, rel+": "+err.Error())
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(unknowns) > 0 {
		t.Fatalf("skill docs mention commands/domains not in product surface:\n  %s", strings.Join(unknowns, "\n  "))
	}
}

func TestSkillDocsRejectFakeCommand(t *testing.T) {
	if err := validateMention([]string{"mcp", "serve"}, domains.Alias); err == nil {
		t.Fatal("expected fake ros mcp to fail validation")
	}
	md := "```\nros mcp serve\n```\n"
	ments := extractCommandMentions(md)
	if len(ments) != 1 || ments[0][0] != "mcp" {
		t.Fatalf("extract=%v", ments)
	}
	if err := validateMention(ments[0], domains.Alias); err == nil {
		t.Fatal("expected extracted ros mcp to fail")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func stripFrontmatter(md string) string {
	if !strings.HasPrefix(md, "---") {
		return md
	}
	rest := md[3:]
	if i := strings.Index(rest, "\n---"); i >= 0 {
		return rest[i+4:]
	}
	return md
}

func assertTreesEqual(t *testing.T, a, b string) {
	t.Helper()
	filesA := map[string][]byte{}
	filesB := map[string][]byte{}
	collect := func(root string, into map[string][]byte) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			into[filepath.ToSlash(rel)] = data
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	collect(a, filesA)
	collect(b, filesB)
	if len(filesA) != len(filesB) {
		t.Fatalf("skills tree file count %d vs packs %d", len(filesA), len(filesB))
	}
	for rel, data := range filesA {
		other, ok := filesB[rel]
		if !ok {
			t.Errorf("missing in packs: %s", rel)
			continue
		}
		if !bytes.Equal(data, other) {
			t.Errorf("skills/ vs packs/ drift: %s", rel)
		}
	}
	for rel := range filesB {
		if _, ok := filesA[rel]; !ok {
			t.Errorf("extra in packs (missing under skills/): %s", rel)
		}
	}
}

func extractCommandMentions(md string) [][]string {
	seen := map[string]bool{}
	var out [][]string
	add := func(toks []string) {
		if len(toks) == 0 {
			return
		}
		key := strings.Join(toks, " ")
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, toks)
	}

	for _, m := range fenceRe.FindAllStringSubmatch(md, -1) {
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if rosLineRe.MatchString(line) {
				for _, alt := range expandPipeAlts(stripRosPrefix(line)) {
					add(tokenizeCommand(alt))
				}
			}
		}
	}

	for _, m := range inlineTickRe.FindAllStringSubmatch(md, -1) {
		tick := strings.TrimSpace(m[1])
		if tick == "" || tick == "ros" || tick == "routeros-cli" {
			continue
		}
		for _, alt := range expandPipeAlts(stripRosPrefix(tick)) {
			toks := tokenizeCommand(alt)
			if len(toks) == 0 {
				continue
			}
			if !productCommandRoots[toks[0]] {
				continue // not a command mention (flag, path, prose fragment)
			}
			add(toks)
		}
	}
	return out
}

func stripRosPrefix(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `\|`, "|")
	fields := strings.Fields(s)
	start := 0
	for i, f := range fields {
		if strings.EqualFold(f, "ros") {
			start = i + 1
			break
		}
	}
	if start == 0 && len(fields) > 0 && !strings.EqualFold(fields[0], "ros") {
		// No leading/mid-line ros token — treat whole string as a bare command.
		start = 0
	} else if start == 0 && len(fields) > 0 && strings.EqualFold(fields[0], "ros") {
		start = 1
	}
	s = strings.Join(fields[start:], " ")
	// Drop leading flags / -d DEV repeatedly.
	for {
		fields := strings.Fields(s)
		if len(fields) == 0 {
			return ""
		}
		f := fields[0]
		if strings.HasPrefix(f, "-") {
			if f == "-d" || f == "--device" {
				if len(fields) < 3 {
					return ""
				}
				s = strings.Join(fields[2:], " ")
				continue
			}
			s = strings.Join(fields[1:], " ")
			continue
		}
		break
	}
	return s
}

func expandPipeAlts(s string) []string {
	// Only expand simple verb alts like ping|neighbors|traceroute inside a token.
	if !strings.Contains(s, "|") {
		return []string{s}
	}
	fields := strings.Fields(s)
	var out []string
	var expand func(idx int, cur []string)
	expand = func(idx int, cur []string) {
		if idx >= len(fields) {
			out = append(out, strings.Join(cur, " "))
			return
		}
		f := fields[idx]
		if strings.Contains(f, "|") && !strings.Contains(f, "/") && !strings.Contains(f, "=") {
			for _, p := range strings.Split(f, "|") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				expand(idx+1, append(append([]string{}, cur...), p))
			}
			return
		}
		expand(idx+1, append(append([]string{}, cur...), f))
	}
	expand(0, nil)
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

func tokenizeCommand(raw string) []string {
	raw = strings.ReplaceAll(raw, "…", " ")
	raw = strings.ReplaceAll(raw, "...", " ")
	var toks []string
	for _, part := range strings.Fields(raw) {
		part = strings.Trim(part, "`*,;()[]")
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "-") {
			break
		}
		if strings.Contains(part, "=") {
			break
		}
		if strings.HasPrefix(part, ".") {
			break
		}
		if part == "DEV" || part == "NAME" || strings.HasPrefix(part, "./") {
			break
		}
		if strings.ContainsAny(part, "<>$") {
			break
		}
		// create|set|delete meta — skip pipe-joined verb lists as a single token
		if strings.Contains(part, "|") {
			break
		}
		toks = append(toks, strings.ToLower(part))
		if len(toks) >= 4 {
			break
		}
	}
	return toks
}

func validateMention(toks []string, aliases map[string]string) error {
	if len(toks) == 0 {
		return nil
	}
	root := toks[0]
	if !productCommandRoots[root] {
		return &mentionError{toks: toks, reason: "unknown top-level command"}
	}
	if len(toks) == 1 {
		return nil
	}
	second := toks[1]

	if domainBearingVerbs[root] {
		if strings.HasPrefix(second, "/") {
			return nil
		}
		if _, ok := aliases[second]; ok {
			return nil
		}
		if kids, ok := productSubcommands[root]; ok && kids[second] {
			return validateNested(toks, aliases)
		}
		if looksLikeDomain(second) {
			return &mentionError{toks: toks, reason: "unknown domain alias " + second}
		}
		return &mentionError{toks: toks, reason: "unknown domain or subcommand after " + root}
	}

	return validateNested(toks, aliases)
}

func validateNested(toks []string, aliases map[string]string) error {
	if len(toks) < 2 {
		return nil
	}
	root := toks[0]
	second := toks[1]
	kids, ok := productSubcommands[root]
	if !ok {
		return nil
	}
	if !kids[second] {
		compound := root + " " + second
		if ckids, ok := productSubcommands[compound]; ok {
			if len(toks) >= 3 && !ckids[toks[2]] {
				return &mentionError{toks: toks, reason: "unknown subcommand under " + compound}
			}
			return nil
		}
		if domainBearingVerbs[root] {
			if strings.HasPrefix(second, "/") {
				return nil
			}
			if _, ok := aliases[second]; ok {
				return nil
			}
			return &mentionError{toks: toks, reason: "unknown domain " + second}
		}
		return &mentionError{toks: toks, reason: "unknown subcommand " + root + " " + second}
	}
	if len(toks) >= 3 {
		third := toks[2]
		compound := root + " " + second
		if ckids, ok := productSubcommands[compound]; ok && !ckids[third] {
			key := second + "/" + third
			if _, ok := aliases[key]; ok {
				return nil
			}
			if _, ok := aliases[third]; ok {
				return nil
			}
			return &mentionError{toks: toks, reason: "unknown subcommand under " + compound}
		}
	}
	return nil
}

func looksLikeDomain(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "/") {
		return true
	}
	switch s {
	case "user", "radius", "file", "netwatch", "arp", "ospf", "wg", "service", "container", "certificate", "log", "ping":
		return true
	}
	return false
}

type mentionError struct {
	toks   []string
	reason string
}

func (e *mentionError) Error() string {
	return e.reason + ": " + strings.Join(e.toks, " ")
}
