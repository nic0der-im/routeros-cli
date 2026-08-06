package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [domain|/path] [key=value...]",
		Short: "Create resources (kubectl-style or raw API path)",
		Long: `Create RouterOS resources.

  ros create ip address --address 10.0.0.1/24 --interface bridge
  ros create firewall filter --chain forward --action accept
  ros create firewall address-list --list blacklist --address 1.2.3.4
  ros create dns static --name router.lan --address 192.168.88.1
  ros create /ip/firewall/address-list list=blacklist address=1.2.3.4
  printf '%s\n' "$ROUTEROS_USER_PASSWORD" | \
    ros -d router-edge create user name=tech group=read address=192.0.2.10 --password-stdin

--password-stdin is supported only by generic create user mutations. It reads
one non-empty password line without placing the secret in caller arguments;
use --dry-run to preview with the password redacted.`,
		RunE: runGenericCreate,
	}
	attachDryRunFlag(cmd)
	cmd.PersistentFlags().Bool(passwordStdinFlag, false, "read the RouterOS user password from stdin (user mutations only)")
	cmd.PersistentPreRunE = func(runCmd *cobra.Command, _ []string) error {
		passwordStdin, _ := cmd.PersistentFlags().GetBool(passwordStdinFlag)
		if passwordStdin && runCmd != cmd {
			return fmt.Errorf("--password-stdin is only supported for generic create user mutations")
		}
		return nil
	}
	cmd.AddCommand(
		newCreateIPCmd(),
		newCreateFirewallCmd(),
		newCreateDNSCmd(),
	)
	return cmd
}

func newCreateIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "Create IP address or route",
	}
	cmd.AddCommand(
		newCreateIPAddressCmd(),
		newCreateIPRouteCmd(),
	)
	return cmd
}

func newCreateIPAddressCmd() *cobra.Command {
	var (
		address string
		iface   string
		comment string
	)

	cmd := &cobra.Command{
		Use:   "address",
		Short: "Create an IP address",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/address/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosCmd := "/ip/address/add"
				rosArgs := []string{
					"=address=" + address,
					"=interface=" + iface,
				}
				if comment != "" {
					rosArgs = append(rosArgs, "=comment="+comment)
				}
				if isDryRun(cmd) {
					return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
						Verb: "create", Path: "/ip/address", Command: rosCmd, Args: rosArgs,
					})
				}
				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}

				result, err := c.Run(ctx, rosCmd, rosArgs...)
				if err != nil {
					return fmt.Errorf("creating IP address: %w", err)
				}

				if err := recordCreateChange(a, deviceName, rosCmd, rosArgs, result); err != nil {
					return err
				}

				id := extractCreatedID(result)
				if id != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "IP address %s created on %s (.id=%s, device=%s)\n", address, iface, id, deviceName)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "IP address %s created on %s (%s)\n", address, iface, deviceName)
				}
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&address, "address", "", "IP address in CIDR notation")
	cmd.Flags().StringVar(&iface, "interface", "", "interface name")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	_ = cmd.MarkFlagRequired("address")
	_ = cmd.MarkFlagRequired("interface")
	return cmd
}

func newCreateIPRouteCmd() *cobra.Command {
	var (
		dstAddress string
		gateway    string
		distance   string
		comment    string
	)

	cmd := &cobra.Command{
		Use:   "route",
		Short: "Create an IP route",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/route/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosCmd := "/ip/route/add"
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
				if isDryRun(cmd) {
					return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
						Verb: "create", Path: "/ip/route", Command: rosCmd, Args: rosArgs,
					})
				}
				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}

				result, err := c.Run(ctx, rosCmd, rosArgs...)
				if err != nil {
					return fmt.Errorf("creating IP route: %w", err)
				}

				if err := recordCreateChange(a, deviceName, rosCmd, rosArgs, result); err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Route %s via %s created (%s)\n", dstAddress, gateway, deviceName)
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

func newCreateFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Create firewall filter or address-list entries",
	}
	cmd.AddCommand(
		newCreateFirewallFilterCmd(),
		newCreateFirewallAddressListCmd(),
	)
	return cmd
}

func newCreateFirewallAddressListCmd() *cobra.Command {
	var (
		listName string
		address  string
		comment  string
		timeout  string
	)
	cmd := &cobra.Command{
		Use:   "address-list",
		Short: "Create a firewall address-list entry (idempotent on list+address)",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := addressListPath + "/add"
			apiArgs := []string{"=list=" + listName, "=address=" + address}
			if comment != "" {
				apiArgs = append(apiArgs, "=comment="+comment)
			}
			if timeout != "" {
				apiArgs = append(apiArgs, "=timeout="+timeout)
			}
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return applyCreateMutation(ctx, a, c, cmd, deviceName, addressListPath, rosCmd, apiArgs)
			})
		},
	}
	cmd.Flags().StringVar(&listName, "list", "", "address-list name")
	cmd.Flags().StringVar(&address, "address", "", "IP address or CIDR")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	cmd.Flags().StringVar(&timeout, "timeout", "", "optional entry timeout")
	_ = cmd.MarkFlagRequired("list")
	_ = cmd.MarkFlagRequired("address")
	return cmd
}

func newCreateDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Create DNS static entries",
	}
	cmd.AddCommand(newCreateDNSStaticCmd())
	return cmd
}

func newCreateDNSStaticCmd() *cobra.Command {
	var (
		name    string
		address string
		typ     string
		comment string
		ttl     string
	)
	cmd := &cobra.Command{
		Use:   "static",
		Short: "Create a DNS static entry (idempotent on name+type)",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := dnsStaticPath + "/add"
			apiArgs := []string{"=name=" + name, "=address=" + address}
			if typ != "" {
				apiArgs = append(apiArgs, "=type="+typ)
			}
			if comment != "" {
				apiArgs = append(apiArgs, "=comment="+comment)
			}
			if ttl != "" {
				apiArgs = append(apiArgs, "=ttl="+ttl)
			}
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return applyCreateMutation(ctx, a, c, cmd, deviceName, dnsStaticPath, rosCmd, apiArgs)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "DNS name")
	cmd.Flags().StringVar(&address, "address", "", "IP address (A/AAAA)")
	cmd.Flags().StringVar(&typ, "type", "", "record type (default A)")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	cmd.Flags().StringVar(&ttl, "ttl", "", "optional TTL")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("address")
	return cmd
}

func newCreateFirewallFilterCmd() *cobra.Command {
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
		Use:   "filter",
		Short: "Create a firewall filter rule",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/firewall/filter/add", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				rosCmd := "/ip/firewall/filter/add"
				rosArgs := buildFirewallArgs(cmd, map[string]string{
					"chain":       chain,
					"action":      action,
					"protocol":    protocol,
					"src-address": srcAddress,
					"dst-address": dstAddress,
					"dst-port":    dstPort,
					"comment":     comment,
				})
				if isDryRun(cmd) {
					return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
						Verb: "create", Path: "/ip/firewall/filter", Command: rosCmd, Args: rosArgs,
					})
				}
				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}

				result, err := c.Run(ctx, rosCmd, rosArgs...)
				if err != nil {
					return fmt.Errorf("creating filter rule: %w", err)
				}

				if err := recordCreateChange(a, deviceName, rosCmd, rosArgs, result); err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Filter rule created on %s\n", deviceName)
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
