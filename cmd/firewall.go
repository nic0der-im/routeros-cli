package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Firewall management (filter and NAT rules)",
	}
	cmd.AddCommand(
		newFirewallFilterCmd(),
		newFirewallNATCmd(),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// firewall filter
// ---------------------------------------------------------------------------

func newFirewallFilterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Manage firewall filter rules",
	}
	cmd.AddCommand(
		newFirewallFilterListCmd(),
		newFirewallFilterAddCmd(),
		newFirewallFilterRemoveCmd(),
		newFirewallFilterEnableCmd(),
		newFirewallFilterDisableCmd(),
	)
	return cmd
}

func newFirewallFilterListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
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
	}
}

func newFirewallFilterAddCmd() *cobra.Command {
	var (
		chain      string
		action     string
		protocol   string
		srcAddress string
		dstAddress string
		dstPort    string
		comment    string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a firewall filter rule",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/firewall/filter/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosArgs := buildFirewallArgs(cmd, map[string]string{
					"chain":       chain,
					"action":      action,
					"protocol":    protocol,
					"src-address": srcAddress,
					"dst-address": dstAddress,
					"dst-port":    dstPort,
					"comment":     comment,
				})

				_, err := c.Run(ctx, "/ip/firewall/filter/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("adding filter rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Filter rule added on %s\n", deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&chain, "chain", "", "rule chain (e.g. input, forward, output)")
	cmd.Flags().StringVar(&action, "action", "", "rule action (e.g. accept, drop, reject)")
	cmd.Flags().StringVar(&protocol, "protocol", "", "protocol (e.g. tcp, udp, icmp)")
	cmd.Flags().StringVar(&srcAddress, "src-address", "", "source address or CIDR")
	cmd.Flags().StringVar(&dstAddress, "dst-address", "", "destination address or CIDR")
	cmd.Flags().StringVar(&dstPort, "dst-port", "", "destination port or port range")
	cmd.Flags().StringVar(&comment, "comment", "", "rule comment")
	_ = cmd.MarkFlagRequired("chain")
	_ = cmd.MarkFlagRequired("action")

	return cmd
}

func newFirewallFilterRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a firewall filter rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			if !force {
				fmt.Fprintf(os.Stderr, "Remove firewall rule %s? [y/N] ", id)
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
						fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
						return
					}
				}
			}

			runWithClient(cmd, "/ip/firewall/filter/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/firewall/filter/remove", "=.id="+id)
				if err != nil {
					return fmt.Errorf("removing filter rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Filter rule %s removed from %s\n", id, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

func newFirewallFilterEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a disabled firewall filter rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			runWithClient(cmd, "/ip/firewall/filter/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/firewall/filter/set", "=.id="+id, "=disabled=no")
				if err != nil {
					return fmt.Errorf("enabling filter rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Filter rule %s enabled on %s\n", id, deviceName)
				return nil
			})
		},
	}
}

func newFirewallFilterDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a firewall filter rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			runWithClient(cmd, "/ip/firewall/filter/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/firewall/filter/set", "=.id="+id, "=disabled=yes")
				if err != nil {
					return fmt.Errorf("disabling filter rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Filter rule %s disabled on %s\n", id, deviceName)
				return nil
			})
		},
	}
}

// ---------------------------------------------------------------------------
// firewall nat
// ---------------------------------------------------------------------------

func newFirewallNATCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nat",
		Short: "Manage firewall NAT rules",
	}
	cmd.AddCommand(
		newFirewallNATListCmd(),
		newFirewallNATAddCmd(),
		newFirewallNATRemoveCmd(),
	)
	return cmd
}

func newFirewallNATListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
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
	}
}

func newFirewallNATAddCmd() *cobra.Command {
	var (
		chain       string
		action      string
		protocol    string
		srcAddress  string
		dstAddress  string
		dstPort     string
		toAddresses string
		toPorts     string
		comment     string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a firewall NAT rule",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/firewall/nat/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosArgs := buildFirewallArgs(cmd, map[string]string{
					"chain":        chain,
					"action":       action,
					"protocol":     protocol,
					"src-address":  srcAddress,
					"dst-address":  dstAddress,
					"dst-port":     dstPort,
					"to-addresses": toAddresses,
					"to-ports":     toPorts,
					"comment":      comment,
				})

				_, err := c.Run(ctx, "/ip/firewall/nat/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("adding NAT rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "NAT rule added on %s\n", deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&chain, "chain", "", "rule chain (e.g. srcnat, dstnat)")
	cmd.Flags().StringVar(&action, "action", "", "rule action (e.g. masquerade, dst-nat, src-nat)")
	cmd.Flags().StringVar(&protocol, "protocol", "", "protocol (e.g. tcp, udp)")
	cmd.Flags().StringVar(&srcAddress, "src-address", "", "source address or CIDR")
	cmd.Flags().StringVar(&dstAddress, "dst-address", "", "destination address or CIDR")
	cmd.Flags().StringVar(&dstPort, "dst-port", "", "destination port or port range")
	cmd.Flags().StringVar(&toAddresses, "to-addresses", "", "NAT target address")
	cmd.Flags().StringVar(&toPorts, "to-ports", "", "NAT target port or port range")
	cmd.Flags().StringVar(&comment, "comment", "", "rule comment")
	_ = cmd.MarkFlagRequired("chain")
	_ = cmd.MarkFlagRequired("action")

	return cmd
}

func newFirewallNATRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a firewall NAT rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			if !force {
				fmt.Fprintf(os.Stderr, "Remove firewall rule %s? [y/N] ", id)
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
						fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
						return
					}
				}
			}

			runWithClient(cmd, "/ip/firewall/nat/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/ip/firewall/nat/remove", "=.id="+id)
				if err != nil {
					return fmt.Errorf("removing NAT rule: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "NAT rule %s removed from %s\n", id, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// buildFirewallArgs constructs RouterOS API arguments from a flag-to-value map,
// including only flags that were explicitly set by the user.
func buildFirewallArgs(cmd *cobra.Command, flags map[string]string) []string {
	args := make([]string, 0, len(flags))
	for flagName, value := range flags {
		if cmd.Flags().Changed(flagName) {
			args = append(args, "="+flagName+"="+value)
		}
	}
	return args
}
