package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var where []string
	cmd := &cobra.Command{
		Use:   "get [domain|/path] [params...]",
		Short: "Read resources (kubectl-style or raw API path)",
		Long: `Read RouterOS resources.

Curated:
  ros get system info
  ros get firewall filter
  ros get dhcp lease
  ros get wg peers [--stale-after 5m]
  ros get wifi clients
  ros get bgp sessions
  ros get ospf neighbors

Generic (any API path or alias from 'ros domains'):
  ros get /ip/firewall/filter
  ros get user
  ros get radius
  ros get interface/bridge
  ros get log
  ros get wg/peers
  ros get bgp/session

Filters (--where works on curated and generic get):
  ros get interface --where name=ether1
  ros get firewall/filter --where chain=forward --where disabled=false
  ros get /interface --where name=ether1`,
		Run: func(cmd *cobra.Command, args []string) {
			runGenericGet(cmd, args, where)
		},
	}
	cmd.PersistentFlags().StringArrayVar(&where, "where", nil, "RouterOS query filter key=value (repeatable; becomes ?key=value)")
	cmd.AddCommand(
		newGetSystemCmd(),
		newGetInterfaceCmd(),
		newGetIPCmd(),
		newGetFirewallCmd(),
		newGetDNSCmd(),
		newGetDHCPCmd(),
		newGetWGCmd(),
		newGetWifiCmd(),
		newGetBGPCmd(),
		newGetOSPFCmd(),
	)
	return cmd
}

func newGetWGCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wg",
		Short: "Get WireGuard peers",
	}
	cmd.AddCommand(newWGPeersCmd())
	return cmd
}

func newGetWifiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "Get WiFi clients / registration",
	}
	cmd.AddCommand(newWifiClientsCmd())
	return cmd
}

func newGetBGPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bgp",
		Short: "Get BGP sessions",
	}
	cmd.AddCommand(newBGPSessionsCmd())
	return cmd
}

func newGetOSPFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ospf",
		Short: "Get OSPF neighbors",
	}
	cmd.AddCommand(newOSPFNeighborsCmd())
	return cmd
}

// whereQueryArgs reads inherited --where flags for curated get subcommands.
func whereQueryArgs(cmd *cobra.Command) ([]string, error) {
	wheres, err := cmd.Flags().GetStringArray("where")
	if err != nil {
		return nil, err
	}
	return parseWhereFilters(wheres)
}

func runPrint(ctx context.Context, c client.Client, path string, cmd *cobra.Command) (*client.Result, error) {
	filters, err := whereQueryArgs(cmd)
	if err != nil {
		return nil, err
	}
	return c.Run(ctx, path, filters...)
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
				result, err := runPrint(ctx, c, "/interface/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/address/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/route/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/dns/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/firewall/filter/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/firewall/nat/print", cmd)
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
		&cobra.Command{
			Use:   "address-list",
			Short: "List firewall address-list entries",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, addressListPath+"/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := runPrint(ctx, c, addressListPath+"/print", cmd)
					if err != nil {
						return fmt.Errorf("fetching address-list: %w", err)
					}
					return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, addressListPath+"/print")
				})
			},
		},
	)
	return cmd
}

func newGetDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Get DNS static entries",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "static",
			Short: "List DNS static entries",
			Run: func(cmd *cobra.Command, args []string) {
				runWithClient(cmd, dnsStaticPath+"/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					result, err := runPrint(ctx, c, dnsStaticPath+"/print", cmd)
					if err != nil {
						return fmt.Errorf("fetching DNS static: %w", err)
					}
					return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, dnsStaticPath+"/print")
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
					result, err := runPrint(ctx, c, "/ip/dhcp-server/lease/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/dhcp-server/print", cmd)
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
					result, err := runPrint(ctx, c, "/ip/pool/print", cmd)
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
