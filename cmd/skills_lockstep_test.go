package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestCLICommandSurfaceCoversSkillLockstepRoots ensures skill lockstep allowlist
// stays aligned with the real cobra tree registered in Execute.
func TestCLICommandSurfaceCoversSkillLockstepRoots(t *testing.T) {
	root := &cobra.Command{Use: "ros"}
	root.AddCommand(
		newVersionCmd(),
		newDeviceCmd(),
		newSystemCmd(),
		newInterfaceCmd(),
		newIPCmd(),
		newFirewallCmd(),
		newDNSCmd(),
		newDHCPCmd(),
		newBackupCmd(),
		newFileCmd(),
		newMonitorCmd(),
		newExecCmd(),
		newSchemaCmd(),
		newAuditCmd(),
		newDoctorCmd(),
		newSessionCmd(),
		newPlanCmd(),
		newGetCmd(),
		newCreateCmd(),
		newSetCmd(),
		newDeleteCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newDomainsCmd(),
		newDiagCmd(),
		newSkillsCmd(),
		newNatCmd(),
		newLeaseCmd(),
		newWGCmd(),
		newWifiCmd(),
		newBGPCmd(),
		newOSPFCmd(),
	)

	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}

	// Mirrors internal/skills.productCommandRoots — keep both updated together.
	wantRoots := []string{
		"version", "device", "system", "interface", "ip", "firewall", "dns", "dhcp",
		"backup", "file", "monitor", "exec", "schema", "audit", "doctor", "session",
		"plan", "get", "create", "set", "delete", "enable", "disable", "domains",
		"diag", "skills", "nat", "lease", "wg", "wifi", "bgp", "ospf",
	}
	for _, name := range wantRoots {
		if !have[name] {
			t.Errorf("skill lockstep root %q missing from cobra tree", name)
		}
	}
	for name := range have {
		found := false
		for _, w := range wantRoots {
			if w == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cobra command %q not listed in skill lockstep roots (update lockstep_test.go)", name)
		}
	}

	// Critical nested verbs referenced by skill docs / AGENTS prompts.
	must := [][]string{
		{"session", "begin"},
		{"session", "commit"},
		{"session", "rollback"},
		{"session", "watch"},
		{"session", "status"},
		{"plan", "preview"},
		{"plan", "apply"},
		{"plan", "rollback"},
		{"diag", "log"},
		{"diag", "ping"},
		{"diag", "neighbors"},
		{"diag", "traceroute"},
		{"wg", "peers"},
		{"device", "list"},
		{"device", "auth"},
		{"file", "list"},
		{"file", "get"},
		{"file", "remove"},
		{"backup", "binary"},
		{"backup", "export"},
	}
	for _, path := range must {
		c := root
		for _, part := range path {
			var next *cobra.Command
			for _, sub := range c.Commands() {
				if sub.Name() == part {
					next = sub
					break
				}
			}
			if next == nil {
				t.Errorf("missing command path %v", path)
				break
			}
			c = next
		}
	}
}
