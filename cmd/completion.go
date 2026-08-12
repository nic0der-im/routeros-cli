package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// completeDeviceNames suggests inventory device names (plus their id and
// address aliases, which `-d` also accepts) for shell completion.
//
// Completion must never write to stdout, so every failure degrades to "no
// suggestions" rather than surfacing an error.
func completeDeviceNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	a, err := loadApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	devices := a.Inventory.List()
	seen := make(map[string]struct{}, len(devices)*2)
	out := make([]string, 0, len(devices))

	add := func(candidate, desc string) {
		if candidate == "" || !strings.HasPrefix(candidate, toComplete) {
			return
		}
		if _, dup := seen[candidate]; dup {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate+"\t"+desc)
	}

	for name, dev := range devices {
		desc := dev.Address
		if desc == "" {
			desc = "device"
		}
		add(name, desc)
	}
	// Ids and addresses are only offered when the plain names did not match,
	// so the common case stays a clean list of names.
	if len(out) == 0 {
		for name, dev := range devices {
			add(dev.ID, name)
			add(dev.Address, name)
		}
	}

	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeDeviceNamesArg completes a single device-name positional argument.
func completeDeviceNamesArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeDeviceNames(cmd, args, toComplete)
}
