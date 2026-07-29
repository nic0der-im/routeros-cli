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
		Short: "Diagnostics: log, ping, neighbors, traceroute, torch, bandwidth-test",
	}
	cmd.AddCommand(
		newDiagLogCmd(),
		newDiagPingCmd(),
		newDiagNeighborsCmd(),
		newDiagTracerouteCmd(),
		newDiagTorchCmd(),
		newDiagBandwidthTestCmd(),
	)
	return cmd
}

func newDiagPingCmd() *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "ping <address>",
		Short: "Ping an address from the router",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params := []string{"/ping", "address=" + args[0], "count=" + strconv.Itoa(count)}
			runGenericGet(cmd, params, nil)
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
			runGenericGet(cmd, []string{"/ip/neighbor"}, nil)
		},
	}
}

func newDiagTracerouteCmd() *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "traceroute <address>",
		Short: "Traceroute from the router",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params := []string{"/tool/traceroute", "address=" + args[0], "count=" + strconv.Itoa(count)}
			runGenericGet(cmd, params, nil)
		},
	}
	cmd.Flags().IntVar(&count, "count", 3, "probes per hop")
	return cmd
}

func newDiagTorchCmd() *cobra.Command {
	var iface string
	var duration string
	cmd := &cobra.Command{
		Use:   "torch",
		Short: "Short torch summary on an interface",
		Run: func(cmd *cobra.Command, args []string) {
			params := []string{"/tool/torch", "interface=" + iface}
			if duration != "" {
				params = append(params, "duration="+duration)
			}
			runGenericGet(cmd, params, nil)
		},
	}
	cmd.Flags().StringVar(&iface, "interface", "", "interface name")
	cmd.Flags().StringVar(&duration, "duration", "3s", "capture duration")
	_ = cmd.MarkFlagRequired("interface")
	return cmd
}

func newDiagBandwidthTestCmd() *cobra.Command {
	var direction string
	var duration string
	var protocol string
	cmd := &cobra.Command{
		Use:   "bandwidth-test <address>",
		Short: "Run /tool/bandwidth-test toward an address (write; use with care)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := "/tool/bandwidth-test"
			apiArgs := parseAPIArgs([]string{
				"address=" + args[0],
				"direction=" + direction,
				"duration=" + duration,
				"protocol=" + protocol,
			})
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}
				result, err := c.Run(ctx, rosCmd, apiArgs...)
				if err != nil {
					return err
				}
				return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, rosCmd)
			})
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "receive", "receive|transmit|both")
	cmd.Flags().StringVar(&duration, "duration", "5s", "test duration")
	cmd.Flags().StringVar(&protocol, "protocol", "tcp", "tcp|udp")
	return cmd
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
