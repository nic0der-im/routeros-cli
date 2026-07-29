package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [domain|/path] .id=*N",
		Short: "Delete resources by .id (kubectl-style or raw API path)",
		Long: `Delete RouterOS resources.

  ros delete ip address --id '*1'
  ros delete firewall filter --id '*A'
  ros delete /ip/dhcp-server/lease .id=*F9
  ros delete user .id=*2`,
		Run: runGenericDelete,
	}
	cmd.AddCommand(
		newDeleteIPCmd(),
		newDeleteFirewallCmd(),
	)
	return cmd
}

func newDeleteIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "Delete IP address or route",
	}
	cmd.AddCommand(
		newDeleteByIDCmd("address", "/ip/address/remove", "IP address"),
		newDeleteByIDCmd("route", "/ip/route/remove", "IP route"),
	)
	return cmd
}

func newDeleteFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Delete firewall filter or nat rules",
	}
	cmd.AddCommand(
		newDeleteByIDCmd("filter", "/ip/firewall/filter/remove", "filter rule"),
		newDeleteByIDCmd("nat", "/ip/firewall/nat/remove", "NAT rule"),
	)
	return cmd
}

func newDeleteByIDCmd(use, rosCmd, label string) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   use,
		Short: "Delete a " + label + " by .id",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable(rosCmd); err != nil {
					return err
				}

				base := strings.TrimSuffix(rosCmd, "/remove")
				pre, _ := fetchPreState(ctx, c, base, id)

				_, err := c.Run(ctx, rosCmd, "=.id="+id)
				if err != nil {
					return fmt.Errorf("deleting %s %s: %w", label, id, err)
				}

				if inv := session.BuildRemoveInverse(rosCmd, pre); len(inv) > 0 {
					_ = a.recordSafeChange(deviceName, session.Change{
						Command:  rosCmd,
						Args:     []string{"=.id=" + id},
						Inverse:  inv,
						PreState: pre,
						Note:     "delete " + id,
					})
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%s %s deleted (%s)\n", label, id, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id of the item to delete")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
