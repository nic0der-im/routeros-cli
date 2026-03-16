package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newInterfaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interface",
		Short: "Interface management and monitoring",
	}
	cmd.AddCommand(
		newInterfaceListCmd(),
		newInterfaceEnableCmd(),
		newInterfaceDisableCmd(),
		newInterfaceMonitorCmd(),
	)
	return cmd
}

func newInterfaceListCmd() *cobra.Command {
	var ifType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all interfaces, optionally filtered by type",
		Long: `List all interfaces on the router.

Examples:
  routeros-cli interface list
  routeros-cli interface list --type ether
  routeros-cli interface list --type bridge`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/interface/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				runArgs := []string{}
				if ifType != "" {
					runArgs = append(runArgs, "?type="+ifType)
				}

				result, err := c.Run(ctx, "/interface/print", runArgs...)
				if err != nil {
					return fmt.Errorf("listing interfaces: %w", err)
				}

				interfaces, err := rosapi.MapSentences[rosapi.Interface](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping interfaces: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.Interfaces(interfaces), deviceName, "/interface/print")
			})
		},
	}

	cmd.Flags().StringVar(&ifType, "type", "", "filter by interface type (ether, bridge, vlan, ...)")

	return cmd
}

func newInterfaceEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable an interface by name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ifName := args[0]

			runWithClient(cmd, "/interface/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/interface/set", "=numbers="+ifName, "=disabled=no")
				if err != nil {
					return fmt.Errorf("enabling interface %q: %w", ifName, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Interface %q enabled on %s\n", ifName, deviceName)
				return nil
			})
		},
	}
}

func newInterfaceDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable an interface by name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ifName := args[0]

			runWithClient(cmd, "/interface/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/interface/set", "=numbers="+ifName, "=disabled=yes")
				if err != nil {
					return fmt.Errorf("disabling interface %q: %w", ifName, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Interface %q disabled on %s\n", ifName, deviceName)
				return nil
			})
		},
	}
}

func newInterfaceMonitorCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "monitor <name>",
		Short: "Poll interface traffic stats at a given interval",
		Long: `Monitor real-time traffic statistics for an interface.

The command polls the router at the specified interval and prints
updated rx/tx bit and packet rates. Press Ctrl+C to stop.

Examples:
  routeros-cli interface monitor ether1
  routeros-cli interface monitor ether1 --interval 2s`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ifName := args[0]

			runWithClient(cmd, "/interface/monitor-traffic", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return monitorTraffic(ctx, cmd, a, c, deviceName, ifName, interval)
			})
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", time.Second, "polling interval (e.g. 1s, 500ms)")

	return cmd
}

// monitorResult holds a single monitor-traffic poll result.
type monitorResult struct {
	Interface string
	RxBps     string
	TxBps     string
	RxPps     string
	TxPps     string
}

// monitorResults implements output.Renderable for monitor poll data.
type monitorResults []monitorResult

func (m monitorResults) TableHeaders() []string {
	return []string{"Interface", "Rx bps", "Tx bps", "Rx pps", "Tx pps"}
}

func (m monitorResults) TableRows() [][]string {
	rows := make([][]string, len(m))
	for i, r := range m {
		rows[i] = []string{r.Interface, r.RxBps, r.TxBps, r.RxPps, r.TxPps}
	}
	return rows
}

// monitorTraffic polls interface traffic stats at the given interval until
// interrupted by SIGINT or the context is cancelled.
func monitorTraffic(
	ctx context.Context,
	cmd *cobra.Command,
	a *App,
	c client.Client,
	deviceName, ifName string,
	interval time.Duration,
) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w := cmd.OutOrStdout()

	// Print header once.
	fmt.Fprintf(w, "Monitoring %q on %s every %s (Ctrl+C to stop)\n\n", ifName, deviceName, interval)

	poll := func() error {
		result, err := c.Run(ctx, "/interface/monitor-traffic", "=interface="+ifName, "=once=")
		if err != nil {
			return fmt.Errorf("monitoring interface %q: %w", ifName, err)
		}

		items := make(monitorResults, 0, len(result.Sentences))
		for _, s := range result.Sentences {
			items = append(items, monitorResult{
				Interface: ifName,
				RxBps:     s["rx-bits-per-second"],
				TxBps:     s["tx-bits-per-second"],
				RxPps:     s["rx-packets-per-second"],
				TxPps:     s["tx-packets-per-second"],
			})
		}

		return a.render(w, items, deviceName, "/interface/monitor-traffic")
	}

	// First poll immediately.
	if err := poll(); err != nil {
		return err
	}

	for {
		select {
		case <-sigCh:
			fmt.Fprintln(w, "\nMonitoring stopped.")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}
