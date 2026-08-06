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
  printf '%s\n' "$ROUTEROS_USER_PASSWORD" | \
    ros -d router-edge set user .id=*2 --password-stdin
  ros set firewall/nat .id=*1 out-interface=ether1
  ros set firewall/filter --comment allow-web disabled=yes
  ros set firewall/mangle --comment mark-conn new-packet-mark=web

Singleton menus (no .id) are journaled in safe sessions via a pre-/print snapshot.
For firewall/filter and firewall/mangle, --comment <exact> resolves to .id
(refuse if 0 or many matches). Use positional comment=value to change the comment field.
Use --password-stdin only for generic set user mutations; it reads one
non-empty password line without placing the secret in caller arguments.
Use --dry-run to preview without writing (the password is redacted; comment
is resolved before preview).`,
		RunE: runGenericSet,
	}
	attachDryRunFlag(cmd)
	cmd.PersistentFlags().Bool(passwordStdinFlag, false, "read the RouterOS user password from stdin (user mutations only)")
	cmd.PersistentPreRunE = func(runCmd *cobra.Command, _ []string) error {
		passwordStdin, _ := cmd.PersistentFlags().GetBool(passwordStdinFlag)
		if passwordStdin && runCmd != cmd {
			return fmt.Errorf("--password-stdin is only supported for generic set user mutations")
		}
		return nil
	}
	attachCommentTargetFlag(cmd)
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
				rosCmd := "/system/identity/set"
				apiArgs := []string{"=name=" + name}

				preName := ""
				pre := map[string]string{}
				if res, err := c.Run(ctx, "/system/identity/print"); err == nil && len(res.Sentences) > 0 {
					preName = res.Sentences[0]["name"]
					pre["name"] = preName
				}

				if isDryRun(cmd) {
					return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
						Verb: "set", Path: "/system/identity", Command: rosCmd, Args: apiArgs, Pre: pre,
					})
				}

				if err := a.ensureWritable(deviceName, rosCmd); err != nil {
					return err
				}

				_, err := c.Run(ctx, rosCmd, apiArgs...)
				if err != nil {
					return fmt.Errorf("setting identity: %w", err)
				}

				if preName != "" {
					_ = a.recordSafeChange(deviceName, session.Change{
						Command:  rosCmd,
						Args:     apiArgs,
						Inverse:  []string{"/system/identity/set", "=name=" + preName},
						PreState: pre,
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
