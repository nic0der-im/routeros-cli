package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

const addressListPath = "/ip/firewall/address-list"

func newFirewallAddressListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address-list",
		Short: "Manage firewall address-list entries",
		Long: `Manage /ip/firewall/address-list entries.

Idempotent key (diff.SemanticKey): list + address.

  ros firewall address-list list [--list blacklist]
  ros firewall address-list add --list blacklist --address 1.2.3.4
  ros firewall address-list remove --list blacklist --address 1.2.3.4
  ros firewall address-list set --id '*1' --comment updated

Mutations support --dry-run and emit write outcomes (created|already_exists|…).`,
	}
	attachDryRunFlag(cmd)
	cmd.AddCommand(
		newFirewallAddressListListCmd(),
		newFirewallAddressListAddCmd(),
		newFirewallAddressListRemoveCmd(),
		newFirewallAddressListSetCmd(),
	)
	return cmd
}

func newFirewallAddressListListCmd() *cobra.Command {
	var listName string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List firewall address-list entries",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, addressListPath+"/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				var apiArgs []string
				if listName != "" {
					apiArgs = append(apiArgs, "?list="+listName)
				}
				result, err := c.Run(ctx, addressListPath+"/print", apiArgs...)
				if err != nil {
					return fmt.Errorf("fetching address-list: %w", err)
				}
				display := addressListPath + "/print"
				if listName != "" {
					display += " ?list=" + listName
				}
				return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, display)
			})
		},
	}
	cmd.Flags().StringVar(&listName, "list", "", "filter by address-list name")
	return cmd
}

func newFirewallAddressListAddCmd() *cobra.Command {
	var (
		listName string
		address  string
		comment  string
		timeout  string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an address-list entry (idempotent on list+address)",
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
	cmd.Flags().StringVar(&timeout, "timeout", "", "optional entry timeout (RouterOS duration)")
	_ = cmd.MarkFlagRequired("list")
	_ = cmd.MarkFlagRequired("address")
	return cmd
}

func newFirewallAddressListRemoveCmd() *cobra.Command {
	var (
		id       string
		listName string
		address  string
	)
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an address-list entry by list+address or --id",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, addressListPath+"/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				resolved := id
				if resolved == "" {
					if listName == "" || address == "" {
						return fmt.Errorf("remove requires --id or both --list and --address")
					}
					var err error
					resolved, err = resolveIDBySemanticKey(ctx, c, addressListPath, map[string]string{
						"list": listName, "address": address,
					})
					if err != nil {
						return err
					}
				}
				apiArgs := []string{"=.id=" + resolved}
				return applyDeleteMutation(ctx, a, c, cmd, deviceName, addressListPath, addressListPath+"/remove", apiArgs, resolved)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id (alternative to list+address)")
	cmd.Flags().StringVar(&listName, "list", "", "address-list name (with --address)")
	cmd.Flags().StringVar(&address, "address", "", "IP address or CIDR (with --list)")
	return cmd
}

func newFirewallAddressListSetCmd() *cobra.Command {
	var (
		id      string
		comment string
		timeout string
		address string
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update an address-list entry by --id",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := addressListPath + "/set"
			apiArgs := []string{"=.id=" + id}
			if cmd.Flags().Changed("comment") {
				apiArgs = append(apiArgs, "=comment="+comment)
			}
			if cmd.Flags().Changed("timeout") {
				apiArgs = append(apiArgs, "=timeout="+timeout)
			}
			if cmd.Flags().Changed("address") {
				apiArgs = append(apiArgs, "=address="+address)
			}
			if len(apiArgs) == 1 {
				fmt.Fprintln(cmd.ErrOrStderr(), "Error: set requires at least one of --comment, --timeout, --address")
				return
			}
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return applySetMutation(ctx, a, c, cmd, deviceName, addressListPath, rosCmd, apiArgs)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id")
	cmd.Flags().StringVar(&comment, "comment", "", "comment")
	cmd.Flags().StringVar(&timeout, "timeout", "", "entry timeout")
	cmd.Flags().StringVar(&address, "address", "", "IP address or CIDR")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
