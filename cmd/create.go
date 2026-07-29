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
  ros create /ip/firewall/address-list list=blacklist address=1.2.3.4
  ros create user name=tech group=read password=...`,
		Run: runGenericCreate,
	}
	cmd.AddCommand(
		newCreateIPCmd(),
		newCreateFirewallCmd(),
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
				if err := a.ensureWritable("/ip/address/add"); err != nil {
					return err
				}

				rosArgs := []string{
					"=address=" + address,
					"=interface=" + iface,
				}
				if comment != "" {
					rosArgs = append(rosArgs, "=comment="+comment)
				}

				result, err := c.Run(ctx, "/ip/address/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("creating IP address: %w", err)
				}

				if err := recordCreateChange(a, deviceName, "/ip/address/add", rosArgs, result); err != nil {
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
				if err := a.ensureWritable("/ip/route/add"); err != nil {
					return err
				}

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

				result, err := c.Run(ctx, "/ip/route/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("creating IP route: %w", err)
				}

				if err := recordCreateChange(a, deviceName, "/ip/route/add", rosArgs, result); err != nil {
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
		Short: "Create firewall filter or nat rules",
	}
	cmd.AddCommand(newCreateFirewallFilterCmd())
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
				if err := a.ensureWritable("/ip/firewall/filter/add"); err != nil {
					return err
				}

				rosArgs := buildFirewallArgs(cmd, map[string]string{
					"chain":       chain,
					"action":      action,
					"protocol":    protocol,
					"src-address": srcAddress,
					"dst-address": dstAddress,
					"dst-port":    dstPort,
					"comment":     comment,
				})

				result, err := c.Run(ctx, "/ip/firewall/filter/add", rosArgs...)
				if err != nil {
					return fmt.Errorf("creating filter rule: %w", err)
				}

				if err := recordCreateChange(a, deviceName, "/ip/firewall/filter/add", rosArgs, result); err != nil {
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
