package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	confirmFlagName  = "confirm"
	confirmFlagUsage = "exact inventory device name (required)"
)

// confirmLongHelp is Long-help boilerplate for destructive commands that use --confirm.
// Pair with registerConfirmFlag. When the command also has --force for [y/N] skip,
// clarify that force does not replace --confirm (see system reboot).
const confirmLongHelp = `Requires --confirm with the exact inventory device name (e.g. --confirm router-edge).
--force does not substitute for --confirm.`

// registerConfirmFlag adds the shared --confirm flag used by destructive commands.
func registerConfirmFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, confirmFlagName, "", confirmFlagUsage)
}

// requireConfirmDevice refuses when --confirm does not exactly match the inventory device name.
func requireConfirmDevice(confirm, deviceName string) error {
	if strings.TrimSpace(confirm) == "" {
		return fmt.Errorf("refusing destructive action on %q: require --confirm %s", deviceName, deviceName)
	}
	if confirm != deviceName {
		return fmt.Errorf("--confirm %q does not match device %q", confirm, deviceName)
	}
	return nil
}
