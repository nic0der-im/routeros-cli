package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

const dnsStaticPath = "/ip/dns/static"

func newDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS management (static entries)",
	}
	cmd.AddCommand(newDNSStaticCmd())
	return cmd
}

func newDNSStaticCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "static",
		Short: "Manage DNS static entries",
		Long: `Manage /ip/dns/static entries.

Idempotent key (diff.SemanticKey): name + type (type defaults to A).

  ros dns static list
  ros dns static add --name router.lan --address 192.168.88.1
  ros dns static set --name router.lan --address 192.168.88.2
  ros dns static remove --name router.lan

Mutations support --dry-run and emit write outcomes (created|already_exists|…).`,
	}
	attachDryRunFlag(cmd)
	cmd.AddCommand(
		newDNSStaticListCmd(),
		newDNSStaticAddCmd(),
		newDNSStaticSetCmd(),
		newDNSStaticRemoveCmd(),
	)
	return cmd
}

func newDNSStaticListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List DNS static entries",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, dnsStaticPath+"/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, dnsStaticPath+"/print")
				if err != nil {
					return fmt.Errorf("fetching DNS static: %w", err)
				}
				return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, dnsStaticPath+"/print")
			})
		},
	}
}

func newDNSStaticAddCmd() *cobra.Command {
	var (
		name    string
		address string
		typ     string
		comment string
		ttl     string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a DNS static entry (idempotent on name+type)",
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
	cmd.Flags().StringVar(&typ, "type", "", "record type (default A on RouterOS / SemanticKey)")
	cmd.Flags().StringVar(&comment, "comment", "", "optional comment")
	cmd.Flags().StringVar(&ttl, "ttl", "", "optional TTL")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("address")
	return cmd
}

func newDNSStaticSetCmd() *cobra.Command {
	var (
		id      string
		name    string
		typ     string
		address string
		comment string
		ttl     string
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update a DNS static entry by --id or --name[+--type]",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, dnsStaticPath+"/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				resolved := id
				if resolved == "" {
					if name == "" {
						return fmt.Errorf("set requires --id or --name")
					}
					desired := map[string]string{"name": name}
					if typ != "" {
						desired["type"] = typ
					}
					var err error
					resolved, err = resolveIDBySemanticKey(ctx, c, dnsStaticPath, desired)
					if err != nil {
						return err
					}
				}
				apiArgs := []string{"=.id=" + resolved}
				if cmd.Flags().Changed("address") {
					apiArgs = append(apiArgs, "=address="+address)
				}
				if cmd.Flags().Changed("comment") {
					apiArgs = append(apiArgs, "=comment="+comment)
				}
				if cmd.Flags().Changed("ttl") {
					apiArgs = append(apiArgs, "=ttl="+ttl)
				}
				if cmd.Flags().Changed("type") && id != "" {
					apiArgs = append(apiArgs, "=type="+typ)
				}
				if len(apiArgs) == 1 {
					return fmt.Errorf("set requires at least one of --address, --comment, --ttl")
				}
				return applySetMutation(ctx, a, c, cmd, deviceName, dnsStaticPath, dnsStaticPath+"/set", apiArgs)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id (alternative to --name)")
	cmd.Flags().StringVar(&name, "name", "", "DNS name (with optional --type)")
	cmd.Flags().StringVar(&typ, "type", "", "record type when resolving by name (default A)")
	cmd.Flags().StringVar(&address, "address", "", "IP address")
	cmd.Flags().StringVar(&comment, "comment", "", "comment")
	cmd.Flags().StringVar(&ttl, "ttl", "", "TTL")
	return cmd
}

func newDNSStaticRemoveCmd() *cobra.Command {
	var (
		id   string
		name string
		typ  string
	)
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a DNS static entry by --name[+--type] or --id",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, dnsStaticPath+"/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				resolved := id
				if resolved == "" {
					if name == "" {
						return fmt.Errorf("remove requires --id or --name")
					}
					desired := map[string]string{"name": name}
					if typ != "" {
						desired["type"] = typ
					}
					var err error
					resolved, err = resolveIDBySemanticKey(ctx, c, dnsStaticPath, desired)
					if err != nil {
						return err
					}
				}
				apiArgs := []string{"=.id=" + resolved}
				return applyDeleteMutation(ctx, a, c, cmd, deviceName, dnsStaticPath, dnsStaticPath+"/remove", apiArgs, resolved)
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "RouterOS .id (alternative to --name)")
	cmd.Flags().StringVar(&name, "name", "", "DNS name")
	cmd.Flags().StringVar(&typ, "type", "", "record type when resolving by name (default A)")
	return cmd
}
