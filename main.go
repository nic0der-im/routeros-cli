package main

import (
	"fmt"
	"os"

	"github.com/nic0der-im/routeros-cli/cmd"
)

// Version info set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		// The root command sets SilenceErrors so that command paths which
		// render structured errors own their output; every other RunE error
		// must still be reported here or it is lost.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
