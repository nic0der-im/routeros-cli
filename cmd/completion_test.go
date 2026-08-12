package cmd

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/spf13/cobra"
)

func writeCompletionConfig(t *testing.T) {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	cfg := &config.Config{
		DefaultOutput: "table",
		Devices: map[string]config.DeviceConfig{
			"router-edge": {ID: "edge01", Address: "192.168.88.1", Username: "admin"},
			"branch-lab":  {ID: "lab02", Address: "10.0.0.1", Username: "admin"},
		},
	}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	old := flagConfig
	t.Cleanup(func() { flagConfig = old })
	flagConfig = cfgPath
}

func TestCompleteDeviceNames(t *testing.T) {
	writeCompletionConfig(t)

	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{"all names sorted", "", []string{"branch-lab\t10.0.0.1", "router-edge\t192.168.88.1"}},
		{"name prefix", "rou", []string{"router-edge\t192.168.88.1"}},
		{"id fallback", "lab0", []string{"lab02\tbranch-lab"}},
		{"address fallback", "192.", []string{"192.168.88.1\trouter-edge"}},
		{"no match", "zzz", []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, directive := completeDeviceNames(nil, nil, tc.toComplete)
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive=%v, want NoFileComp", directive)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompleteDeviceNamesArgOnlyFirstPositional(t *testing.T) {
	writeCompletionConfig(t)

	got, directive := completeDeviceNamesArg(nil, []string{"router-edge"}, "")
	if got != nil {
		t.Errorf("second positional suggested %q, want none", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive=%v, want NoFileComp", directive)
	}
}

// The -d flag and every device-name positional must be completable; a missing
// hook silently degrades to filename completion.
func TestDeviceCompletionHooksRegistered(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("device") == nil {
		t.Fatal("missing --device flag")
	}
	if _, ok := rootCmd.GetFlagCompletionFunc("device"); !ok {
		t.Error("--device has no registered completion func")
	}

	for _, c := range newDeviceCmd().Commands() {
		switch c.Name() {
		case "remove", "use", "test", "get":
			if c.ValidArgsFunction == nil {
				t.Errorf("device %s: missing ValidArgsFunction", c.Name())
			}
		case "auth":
			for _, sub := range c.Commands() {
				if sub.Name() == "set" && sub.ValidArgsFunction == nil {
					t.Error("device auth set: missing ValidArgsFunction")
				}
			}
		}
	}
}
