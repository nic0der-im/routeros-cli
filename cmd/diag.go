package cmd

import (
	"context"
	"strconv"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

func newDiagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diag",
		Short: "Diagnostics: log, ping, neighbors",
	}
	cmd.AddCommand(
		newDiagLogCmd(),
		newDiagPingCmd(),
		newDiagNeighborsCmd(),
	)
	return cmd
}

func newDiagLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Show recent system log entries",
		Run: func(cmd *cobra.Command, args []string) {
			runGenericGet(cmd, []string{"/log"})
		},
	}
}

func newDiagPingCmd() *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "ping <address>",
		Short: "Ping an address from the router",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params := []string{"/ping", "address=" + args[0], "count=" + strconv.Itoa(count)}
			runGenericGet(cmd, params)
		},
	}
	cmd.Flags().IntVar(&count, "count", 4, "number of probes")
	return cmd
}

func newDiagNeighborsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "neighbors",
		Short: "Show MNDP/CDP neighbors discovered by the router",
		Run: func(cmd *cobra.Command, args []string) {
			runGenericGet(cmd, []string{"/ip/neighbor"})
		},
	}
}

func newDeviceDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "List MNDP neighbors from the current device (candidates for inventory)",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/neighbor/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/ip/neighbor/print")
				if err != nil {
					return err
				}
				return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, "/ip/neighbor/print")
			})
		},
	}
}
