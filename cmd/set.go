package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [domain|/path] [key=value...]",
		Short: "Update resources (kubectl-style or raw API path)",
		Long: `Update RouterOS resources.

  ros set identity --name "central-hub-buenos-aires"
  ros set /ip/dhcp-server .id=*1 lease-time=1d
  ros set /ip/cloud ddns-enabled=auto update-time=false
  ros set user .id=*2 password=secret
  ros set firewall/nat .id=*1 out-interface=ether1

Singleton menus (no .id) are journaled in safe sessions via a pre-/print snapshot.`,
		Run: runGenericSet,
	}
	cmd.AddCommand(newSetIdentityCmd())
	return cmd
}

func newSetIdentityCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Set system identity",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/system/identity/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable("/system/identity/set"); err != nil {
					return err
				}

				preName := ""
				if res, err := c.Run(ctx, "/system/identity/print"); err == nil && len(res.Sentences) > 0 {
					preName = res.Sentences[0]["name"]
				}

				_, err := c.Run(ctx, "/system/identity/set", "=name="+name)
				if err != nil {
					return fmt.Errorf("setting identity: %w", err)
				}

				if preName != "" {
					_ = a.recordSafeChange(deviceName, session.Change{
						Command:  "/system/identity/set",
						Args:     []string{"=name=" + name},
						Inverse:  []string{"/system/identity/set", "=name=" + preName},
						PreState: map[string]string{"name": preName},
						Note:     "set identity",
					})
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Identity set to %q on %s\n", name, deviceName)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new system identity name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
