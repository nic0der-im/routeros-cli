package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newDHCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "DHCP server management",
	}
	cmd.AddCommand(
		newDHCPLeaseCmd(),
		newDHCPServerCmd(),
		newDHCPPoolCmd(),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// dhcp lease
// ---------------------------------------------------------------------------

func newDHCPLeaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lease",
		Short: "Manage DHCP leases",
	}
	cmd.AddCommand(newDHCPLeaseListCmd())
	return cmd
}

func newDHCPLeaseListCmd() *cobra.Command {
	var active bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List DHCP leases",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := "/ip/dhcp-server/lease/print"

			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				var cmdArgs []string
				if active {
					cmdArgs = append(cmdArgs, "?status=bound")
				}

				result, err := c.Run(ctx, rosCmd, cmdArgs...)
				if err != nil {
					return fmt.Errorf("fetching DHCP leases: %w", err)
				}

				leases, err := rosapi.MapSentences[rosapi.DHCPLease](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping DHCP leases: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.DHCPLeases(leases), deviceName, rosCmd)
			})
		},
	}

	cmd.Flags().BoolVar(&active, "active", false, "show only active (bound) leases")

	return cmd
}

// ---------------------------------------------------------------------------
// dhcp server
// ---------------------------------------------------------------------------

func newDHCPServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage DHCP servers",
	}
	cmd.AddCommand(newDHCPServerListCmd())
	return cmd
}

func newDHCPServerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List DHCP servers",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := "/ip/dhcp-server/print"

			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, rosCmd)
				if err != nil {
					return fmt.Errorf("fetching DHCP servers: %w", err)
				}

				servers, err := rosapi.MapSentences[rosapi.DHCPServer](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping DHCP servers: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.DHCPServers(servers), deviceName, rosCmd)
			})
		},
	}
}

// ---------------------------------------------------------------------------
// dhcp pool
// ---------------------------------------------------------------------------

func newDHCPPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage IP pools",
	}
	cmd.AddCommand(newDHCPPoolListCmd())
	return cmd
}

func newDHCPPoolListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List IP pools",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := "/ip/pool/print"

			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, rosCmd)
				if err != nil {
					return fmt.Errorf("fetching IP pools: %w", err)
				}

				pools, err := rosapi.MapSentences[rosapi.DHCPPool](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping IP pools: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.DHCPPools(pools), deviceName, rosCmd)
			})
		},
	}
}
