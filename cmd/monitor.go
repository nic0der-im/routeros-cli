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

// ---------------------------------------------------------------------------
// Renderable types for monitor traffic output
// ---------------------------------------------------------------------------

// trafficMonitorEntry holds a single monitor-traffic poll result with a
// timestamp for each observation.
type trafficMonitorEntry struct {
	Timestamp string
	Interface string
	RxBps     string
	TxBps     string
	RxPps     string
	TxPps     string
}

// trafficMonitorEntries implements output.Renderable for timestamped traffic data.
type trafficMonitorEntries []trafficMonitorEntry

func (t trafficMonitorEntries) TableHeaders() []string {
	return []string{"Timestamp", "Interface", "Rx bps", "Tx bps", "Rx pps", "Tx pps"}
}

func (t trafficMonitorEntries) TableRows() [][]string {
	rows := make([][]string, len(t))
	for i, e := range t {
		rows[i] = []string{e.Timestamp, e.Interface, e.RxBps, e.TxBps, e.RxPps, e.TxPps}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Renderable types for monitor resources output
// ---------------------------------------------------------------------------

// resourceMonitorEntry holds a single resource poll result with a timestamp.
type resourceMonitorEntry struct {
	Timestamp   string
	Uptime      string
	CPULoad     string
	FreeMemory  string
	TotalMemory string
	FreeHDD     string
	TotalHDD    string
}

// resourceMonitorEntries implements output.Renderable for timestamped resource data.
type resourceMonitorEntries []resourceMonitorEntry

func (r resourceMonitorEntries) TableHeaders() []string {
	return []string{"Timestamp", "Uptime", "CPU Load", "Memory Free", "Memory Total", "HDD Free", "HDD Total"}
}

func (r resourceMonitorEntries) TableRows() [][]string {
	rows := make([][]string, len(r))
	for i, e := range r {
		rows[i] = []string{e.Timestamp, e.Uptime, e.CPULoad + "%", e.FreeMemory, e.TotalMemory, e.FreeHDD, e.TotalHDD}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Command constructors
// ---------------------------------------------------------------------------

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Real-time monitoring of traffic and system resources",
	}
	cmd.AddCommand(
		newMonitorTrafficCmd(),
		newMonitorResourcesCmd(),
	)
	return cmd
}

func newMonitorTrafficCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "traffic <interface>",
		Short: "Poll interface traffic stats at a given interval",
		Long: `Monitor real-time traffic statistics for an interface.

The command polls the router at the specified interval and prints
updated rx/tx bit and packet rates. Press Ctrl+C to stop.

Examples:
  routeros-cli monitor traffic ether1
  routeros-cli monitor traffic ether1 --interval 2s
  routeros-cli monitor traffic ether1 --interval 500ms -o json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ifName := args[0]
			runMonitor(cmd, "/interface/monitor-traffic", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return pollTraffic(ctx, cmd, a, c, deviceName, ifName, interval)
			})
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", time.Second, "polling interval (e.g. 1s, 500ms)")

	return cmd
}

func newMonitorResourcesCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Poll system resource usage at a given interval",
		Long: `Monitor real-time system resource usage (CPU, memory, uptime).

The command polls the router at the specified interval and prints
updated resource statistics. Press Ctrl+C to stop.

Examples:
  routeros-cli monitor resources
  routeros-cli monitor resources --interval 5s
  routeros-cli monitor resources -o json --interval 2s`,
		Run: func(cmd *cobra.Command, args []string) {
			runMonitor(cmd, "/system/resource/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return pollResources(ctx, cmd, a, c, deviceName, interval)
			})
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", time.Second, "polling interval (e.g. 1s, 500ms)")

	return cmd
}

// ---------------------------------------------------------------------------
// Monitor execution helper
// ---------------------------------------------------------------------------

// runMonitor is analogous to runWithClient but creates a long-lived context
// based on signal.NotifyContext instead of applying the configured timeout to
// the entire operation. The connection timeout still applies during dial via
// a.connect, but the polling loop runs until SIGINT or an error occurs.
func runMonitor(cmdInstance *cobra.Command, rosCommand string, fn func(ctx context.Context, a *App, c client.Client, deviceName string) error) {
	a, err := loadApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(ExitConfError)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	c, deviceName, err := a.connect(ctx)
	if err != nil {
		a.renderError(os.Stderr, "connection_failed", err.Error(), deviceName)
		os.Exit(ExitConnError)
	}
	defer func() { _ = c.Close() }()

	if err := fn(ctx, a, c, deviceName); err != nil {
		// Context cancellation from SIGINT is not an error worth reporting.
		if ctx.Err() != nil {
			return
		}
		a.renderError(os.Stderr, "command_failed", err.Error(), deviceName)
		os.Exit(ExitCmdError)
	}
}

// ---------------------------------------------------------------------------
// Traffic polling
// ---------------------------------------------------------------------------

// pollTraffic continuously polls /interface/monitor-traffic for the given
// interface and renders each result until the context is cancelled.
func pollTraffic(
	ctx context.Context,
	cmd *cobra.Command,
	a *App,
	c client.Client,
	deviceName, ifName string,
	interval time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Monitoring traffic on %q [%s] every %s (Ctrl+C to stop)\n\n", ifName, deviceName, interval)

	var iterations int

	poll := func() error {
		result, err := c.Run(ctx, "/interface/monitor-traffic", "=interface="+ifName, "=once=")
		if err != nil {
			return fmt.Errorf("monitoring interface %q: %w", ifName, err)
		}

		ts := time.Now().UTC().Format(time.RFC3339)
		entries := make(trafficMonitorEntries, 0, len(result.Sentences))
		for _, s := range result.Sentences {
			entries = append(entries, trafficMonitorEntry{
				Timestamp: ts,
				Interface: ifName,
				RxBps:     s["rx-bits-per-second"],
				TxBps:     s["tx-bits-per-second"],
				RxPps:     s["rx-packets-per-second"],
				TxPps:     s["tx-packets-per-second"],
			})
		}

		iterations++
		return a.render(w, entries, deviceName, "/interface/monitor-traffic")
	}

	// First poll immediately.
	if err := poll(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(w, "\nMonitoring stopped after %d samples.\n", iterations)
			return nil
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Resource polling
// ---------------------------------------------------------------------------

// pollResources continuously polls /system/resource/print and renders each
// result until the context is cancelled.
func pollResources(
	ctx context.Context,
	cmd *cobra.Command,
	a *App,
	c client.Client,
	deviceName string,
	interval time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Monitoring resources on %q every %s (Ctrl+C to stop)\n\n", deviceName, interval)

	var iterations int

	poll := func() error {
		result, err := c.Run(ctx, "/system/resource/print")
		if err != nil {
			return fmt.Errorf("fetching system resources: %w", err)
		}

		resources, err := rosapi.MapSentences[rosapi.SystemResource](result.Sentences)
		if err != nil {
			return fmt.Errorf("mapping system resources: %w", err)
		}

		ts := time.Now().UTC().Format(time.RFC3339)
		entries := make(resourceMonitorEntries, 0, len(resources))
		for _, r := range resources {
			entries = append(entries, resourceMonitorEntry{
				Timestamp:   ts,
				Uptime:      r.Uptime,
				CPULoad:     r.CPULoad,
				FreeMemory:  r.FreeMemory,
				TotalMemory: r.TotalMemory,
				FreeHDD:     r.FreeHDDSpace,
				TotalHDD:    r.TotalHDDSpace,
			})
		}

		iterations++
		return a.render(w, entries, deviceName, "/system/resource/print")
	}

	// First poll immediately.
	if err := poll(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(w, "\nMonitoring stopped after %d samples.\n", iterations)
			return nil
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}
