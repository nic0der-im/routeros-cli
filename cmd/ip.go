package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "IP configuration and management",
	}
	cmd.AddCommand(
		newIPAddressCmd(),
		newIPRouteCmd(),
		newIPDNSCmd(),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// ip address
// ---------------------------------------------------------------------------

func newIPAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Manage IP addresses",
	}
	cmd.AddCommand(
		newIPAddressListCmd(),
		newIPAddressAddCmd(),
		newIPAddressRemoveCmd(),
	)
	return cmd
}

func newIPAddressListCmd() *cobra.Command {
	var iface string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List IP addresses",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/address/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosArgs := make([]string, 0)
				if iface != "" {
					rosArgs = append(rosArgs, "?interface="+iface)
				}

				result, err := c.Run(ctx, "/ip/address/print", rosArgs...)
				if err != nil {
					return fmt.Errorf("listing IP addresses: %w", err)
				}

				addresses, err := rosapi.MapSentences[rosapi.IPAddress](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping IP addresses: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.IPAddresses(addresses), deviceName, "/ip/address/print")
			})
		},
	}

	cmd.Flags().StringVar(&iface, "interface", "", "filter by interface name")

	return cmd
}

func newIPAddressAddCmd() *cobra.Command {
	var (
		address string
		iface   string
		comment string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an IP address",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/address/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosArgs := []string{
					"=address=" + address,
					"=interface=" + iface,
				}
				if comment != "" {
					rosArgs = append(rosArgs, "=comment="+comment)
				}

				_, err := c.Run(ctx, "/ip/address/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("adding IP address: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "IP address %s added on %s (%s)\n", address, iface, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&address, "address", "", "IP address in CIDR notation (e.g. 192.168.1.1/24)")
	cmd.Flags().StringVar(&iface, "interface", "", "interface name")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	_ = cmd.MarkFlagRequired("address")
	_ = cmd.MarkFlagRequired("interface")

	return cmd
}

func newIPAddressRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an IP address by .id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			runWithClient(cmd, "/ip/address/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/address/remove", "=.id="+id)
				if err != nil {
					return fmt.Errorf("removing IP address %s: %w", id, err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "IP address %s removed (%s)\n", id, deviceName)
				return nil
			})
		},
	}
}

// ---------------------------------------------------------------------------
// ip route
// ---------------------------------------------------------------------------

func newIPRouteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage IP routes",
	}
	cmd.AddCommand(
		newIPRouteListCmd(),
		newIPRouteAddCmd(),
		newIPRouteRemoveCmd(),
	)
	return cmd
}

func newIPRouteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all IP routes",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/route/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/ip/route/print")
				if err != nil {
					return fmt.Errorf("listing IP routes: %w", err)
				}

				routes, err := rosapi.MapSentences[rosapi.IPRoute](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping IP routes: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.IPRoutes(routes), deviceName, "/ip/route/print")
			})
		},
	}
}

func newIPRouteAddCmd() *cobra.Command {
	var (
		dstAddress string
		gateway    string
		distance   string
		comment    string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an IP route",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/route/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosArgs := []string{
					"=dst-address=" + dstAddress,
					"=gateway=" + gateway,
				}
				if distance != "" {
					rosArgs = append(rosArgs, "=distance="+distance)
				}
				if comment != "" {
					rosArgs = append(rosArgs, "=comment="+comment)
				}

				_, err := c.Run(ctx, "/ip/route/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("adding IP route: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Route %s via %s added (%s)\n", dstAddress, gateway, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&dstAddress, "dst-address", "", "destination address in CIDR notation")
	cmd.Flags().StringVar(&gateway, "gateway", "", "gateway IP address")
	cmd.Flags().StringVar(&distance, "distance", "", "route distance (metric)")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	_ = cmd.MarkFlagRequired("dst-address")
	_ = cmd.MarkFlagRequired("gateway")

	return cmd
}

func newIPRouteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an IP route by .id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			runWithClient(cmd, "/ip/route/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/route/remove", "=.id="+id)
				if err != nil {
					return fmt.Errorf("removing IP route %s: %w", id, err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Route %s removed (%s)\n", id, deviceName)
				return nil
			})
		},
	}
}

// ---------------------------------------------------------------------------
// ip dns
// ---------------------------------------------------------------------------

func newIPDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS settings",
	}
	cmd.AddCommand(
		newIPDNSGetCmd(),
		newIPDNSSetCmd(),
	)
	return cmd
}

func newIPDNSGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current DNS settings",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/dns/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/ip/dns/print")
				if err != nil {
					return fmt.Errorf("fetching DNS settings: %w", err)
				}

				settings, err := rosapi.MapSentences[rosapi.DNSSettings](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping DNS settings: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.DNSSettingsList(settings), deviceName, "/ip/dns/print")
			})
		},
	}
}

func newIPDNSSetCmd() *cobra.Command {
	var servers string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set DNS servers",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/dns/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/dns/set", "=servers="+servers)
				if err != nil {
					return fmt.Errorf("setting DNS servers: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "DNS servers set to %s (%s)\n", servers, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&servers, "servers", "", "comma-separated list of DNS server IPs")
	_ = cmd.MarkFlagRequired("servers")

	return cmd
}
