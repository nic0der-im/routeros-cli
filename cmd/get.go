package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [domain|/path] [params...]",
		Short: "Read resources (kubectl-style or raw API path)",
		Long: `Read RouterOS resources.

Curated:
  ros get system info
  ros get firewall filter
  ros get dhcp lease

Generic (any API path or alias from 'ros domains'):
  ros get /ip/firewall/filter
  ros get user
  ros get radius
  ros get interface/bridge
  ros get log`,
		Run: runGenericGet,
	}
	cmd.AddCommand(
		newGetSystemCmd(),
		newGetInterfaceCmd(),
		newGetIPCmd(),
		newGetFirewallCmd(),
		newGetDHCPCmd(),
	)
	return cmd
}

func newGetSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Get system info, resource, or identity",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "info",
			Short: "Show system identity + resource summary",
			Run:   func(cmd *cobra.Command, args []string) { newSystemInfoCmd().Run(cmd, args) },
		},
		&cobra.Command{
			Use:   "resource",
			Short: "Show system resources",
			Run:   func(cmd *cobra.Command, args []string) { newSystemResourceCmd().Run(cmd, args) },
		},
		&cobra.Command{
			Use:   "identity",
			Short: "Show system identity",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/system/identity/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/system/identity/print")
					if err != nil {
						return fmt.Errorf("fetching identity: %w", err)
					}
					identities, err := rosapi.MapSentences[rosapi.SystemIdentity](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping identity: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.SystemIdentities(identities), deviceName, "/system/identity/print")
				})
			},
		},
	)
	return cmd
}

func newGetInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interface",
		Short: "List interfaces",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/interface/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/interface/print")
				if err != nil {
					return fmt.Errorf("listing interfaces: %w", err)
				}
				ifaces, err := rosapi.MapSentences[rosapi.Interface](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping interfaces: %w", err)
				}
				return a.render(cmd.OutOrStdout(), rosapi.Interfaces(ifaces), deviceName, "/interface/print")
			})
		},
	}
}

func newGetIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "Get IP address, route, or dns",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "address",
			Short: "List IP addresses",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/address/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/address/print")
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
		},
		&cobra.Command{
			Use:   "route",
			Short: "List IP routes",
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
		},
		&cobra.Command{
			Use:   "dns",
			Short: "Show DNS settings",
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
		},
	)
	return cmd
}

func newGetFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Get firewall filter or nat rules",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "filter",
			Short: "List firewall filter rules",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/firewall/filter/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/firewall/filter/print")
					if err != nil {
						return fmt.Errorf("fetching filter rules: %w", err)
					}
					rules, err := rosapi.MapSentences[rosapi.FirewallRule](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping filter rules: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.FirewallRules(rules), deviceName, "/ip/firewall/filter/print")
				})
			},
		},
		&cobra.Command{
			Use:   "nat",
			Short: "List firewall NAT rules",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/firewall/nat/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/firewall/nat/print")
					if err != nil {
						return fmt.Errorf("fetching NAT rules: %w", err)
					}
					rules, err := rosapi.MapSentences[rosapi.FirewallRule](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping NAT rules: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.FirewallRules(rules), deviceName, "/ip/firewall/nat/print")
				})
			},
		},
	)
	return cmd
}

func newGetDHCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "Get DHCP lease, server, or pool",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "lease",
			Short: "List DHCP leases",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/dhcp-server/lease/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/dhcp-server/lease/print")
					if err != nil {
						return fmt.Errorf("fetching DHCP leases: %w", err)
					}
					leases, err := rosapi.MapSentences[rosapi.DHCPLease](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping DHCP leases: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.DHCPLeases(leases), deviceName, "/ip/dhcp-server/lease/print")
				})
			},
		},
		&cobra.Command{
			Use:   "server",
			Short: "List DHCP servers",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/dhcp-server/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/dhcp-server/print")
					if err != nil {
						return fmt.Errorf("fetching DHCP servers: %w", err)
					}
					servers, err := rosapi.MapSentences[rosapi.DHCPServer](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping DHCP servers: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.DHCPServers(servers), deviceName, "/ip/dhcp-server/print")
				})
			},
		},
		&cobra.Command{
			Use:   "pool",
			Short: "List IP pools",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, "/ip/pool/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := c.Run(ctx, "/ip/pool/print")
					if err != nil {
						return fmt.Errorf("fetching IP pools: %w", err)
					}
					pools, err := rosapi.MapSentences[rosapi.DHCPPool](result.Sentences)
					if err != nil {
						return fmt.Errorf("mapping IP pools: %w", err)
					}
					return a.render(cmd.OutOrStdout(), rosapi.DHCPPools(pools), deviceName, "/ip/pool/print")
				})
			},
		},
	)
	return cmd
}
