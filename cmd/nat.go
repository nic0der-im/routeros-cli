package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func newNatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nat",
		Short: "NAT helpers (curated capa linda)",
	}
	cmd.AddCommand(newNatSetOutInterfaceCmd())
	return cmd
}

func newNatSetOutInterfaceCmd() *cobra.Command {
	var id string
	var iface string

	cmd := &cobra.Command{
		Use:   "set-out-interface",
		Short: "Set out-interface on a NAT rule (e.g. masquerade → ether1)",
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := "/ip/firewall/nat/set"
			apiArgs := []string{"=.id=" + id, "=out-interface=" + iface}
			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable(rosCmd); err != nil {
					return err
				}
				pre, _ := fetchPreState(ctx, c, "/ip/firewall/nat", id)
				_, err := c.Run(ctx, rosCmd, apiArgs...)
				if err != nil {
					return err
				}
				if inv := session.BuildSetInverse(rosCmd, id, pre, apiArgs); len(inv) > 0 {
					_ = a.recordSafeChange(deviceName, session.Change{
						Command:  rosCmd,
						Args:     apiArgs,
						Inverse:  inv,
						PreState: pre,
						Note:     "nat set-out-interface",
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "NAT %s out-interface=%s on %s\n", id, iface, deviceName)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "NAT rule .id")
	cmd.Flags().StringVar(&iface, "interface", "", "out-interface name")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("interface")
	return cmd
}
