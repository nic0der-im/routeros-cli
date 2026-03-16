package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"
)

// SetVersionInfo sets the version information from ldflags.
func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "routeros-cli %s\n  commit: %s\n  built:  %s\n", versionStr, commitStr, dateStr)
		},
	}
}
