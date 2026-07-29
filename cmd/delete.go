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
  ros delete firewall filter --comment allow-web
  ros delete firewall mangle --comment mark-conn
  ros delete firewall address-list --id '*1'
  ros delete dns static --id '*1'
  ros delete /ip/dhcp-server/lease .id=*F9
  ros delete user .id=*2

Requires exactly one .id=*N (mass / non-.id selectors are refused), except
firewall/filter and firewall/mangle which also accept --comment <exact>.
Single-row .id deletes do not require --confirm; use --confirm on reboot,
file remove, device remove, and lease cleanup-waiting.

Use --dry-run to preview without writing (comment is resolved before preview).`,
		Run: runGenericDelete,
	}
	attachDryRunFlag(cmd)
	attachCommentTargetFlag(cmd)
	cmd.AddCommand(
		newDeleteIPCmd(),
		newDeleteFirewallCmd(),
		newDeleteDNSCmd(),
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
		Short: "Delete firewall filter, nat, mangle, or address-list entries",
	}
	cmd.AddCommand(
		newDeleteByIDOrCommentCmd("filter", "/ip/firewall/filter", "filter rule"),
		newDeleteByIDCmd("nat", "/ip/firewall/nat/remove", "NAT rule"),
		newDeleteByIDOrCommentCmd("mangle", "/ip/firewall/mangle", "mangle rule"),
		newDeleteByIDCmd("address-list", addressListPath+"/remove", "address-list entry"),
	)
	return cmd
}

func newDeleteDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Delete DNS static entries",
	}
	cmd.AddCommand(newDeleteByIDCmd("static", dnsStaticPath+"/remove", "DNS static entry"))
	return cmd
}

func newDeleteByIDCmd(use, rosCmd, label string) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   use,
		Short: "Delete a " + label + " by .id",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				base := strings.TrimSuffix(rosCmd, "/remove")
				apiArgs := []string{"=.id=" + id}
				pre, _ := fetchPreState(ctx, c, base, id)

				if isDryRun(cmd) {
					return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
						Verb: "delete", Path: base, Command: rosCmd, Args: apiArgs, Pre: pre,
					})
				}

				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}

				_, err := c.Run(ctx, rosCmd, apiArgs...)
				if err != nil {
					return fmt.Errorf("deleting %s %s: %w", label, id, err)
				}

				if inv := session.BuildRemoveInverse(rosCmd, pre); len(inv) > 0 {
					_ = a.recordSafeChange(deviceName, session.Change{
						Command:  rosCmd,
						Args:     apiArgs,
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

// newDeleteByIDOrCommentCmd deletes filter/mangle by --id or exact --comment.
func newDeleteByIDOrCommentCmd(use, basePath, label string) *cobra.Command {
	var (
		id      string
		comment string
	)
	rosCmd := basePath + "/remove"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Delete a " + label + " by --id or --comment",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				resolved, err := resolveMutateTargetID(ctx, c, basePath, id, comment)
				if err != nil {
					return err
				}
				apiArgs := []string{"=.id=" + resolved}
				return applyDeleteMutation(ctx, a, c, cmd, deviceName, basePath, rosCmd, apiArgs, resolved)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id (alternative to --comment)")
	cmd.Flags().StringVar(&comment, "comment", "", "exact rule comment (alternative to --id)")
	return cmd
}
