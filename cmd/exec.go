package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/policy"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute an arbitrary RouterOS command",
		Long: `Execute any RouterOS API command directly.

Examples:
  ros exec /interface/print
  ros exec /ip/address/print =interface=ether1
  ros exec /system/package/print

Commands are filtered by a builtin denylist plus optional per-device
exec_allow / exec_deny globs (defense-in-depth; not a hard security boundary
if the RouterOS user is full-admin).

During a safe session, write commands without a known inverse require --force.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := args[0]
			rosArgs := args[1:]

			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				dev := a.deviceConfig(deviceName)
				if err := guardrails.CheckExec(rosCmd, dev.ExecAllow, dev.ExecDeny); err != nil {
					return err
				}

				if policy.IsWrite(rosCmd) {
					if err := a.ensureWritableForce(deviceName, rosCmd, force); err != nil {
						return err
					}
					if err := a.ensureExecJournalable(deviceName, rosCmd, rosArgs, force); err != nil {
						return err
					}
				}

				result, err := c.Run(ctx, rosCmd, rosArgs...)
				if err != nil {
					return fmt.Errorf("executing %q: %w", rosCmd, err)
				}

				if policy.IsWrite(rosCmd) {
					id := findIDArg(rosArgs)
					if id == "" {
						id = extractCreatedID(result)
					}
					if inv := session.BuildInverse(rosCmd, rosArgs, id); len(inv) > 0 {
						_ = a.recordSafeChange(deviceName, session.Change{
							Command: rosCmd,
							Args:    rosArgs,
							Inverse: inv,
							Note:    "exec",
						})
					}
				}

				if len(result.Sentences) == 0 {
					if a.OutFormat == output.FormatJSON {
						return a.render(cmd.OutOrStdout(), &rosapi.GenericResults{}, deviceName, rosCmd)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "OK (no data returned)")
					return nil
				}

				items := make([]rosapi.GenericResult, len(result.Sentences))
				for i, s := range result.Sentences {
					items[i] = rosapi.GenericResult{Fields: s}
				}
				gr := &rosapi.GenericResults{Items: items}

				if len(items) > 0 {
					keys := make([]string, 0, len(result.Sentences[0]))
					for k := range result.Sentences[0] {
						keys = append(keys, k)
					}
					gr.SetKeyOrder(keys)
				}

				cmdDisplay := rosCmd
				if len(rosArgs) > 0 {
					cmdDisplay = rosCmd + " " + strings.Join(rosArgs, " ")
				}

				return a.render(cmd.OutOrStdout(), gr, deviceName, cmdDisplay)
			})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "allow write exec without journalable inverse during a safe session")
	return cmd
}

func (a *App) ensureExecJournalable(deviceName, rosCmd string, rosArgs []string, force bool) error {
	sess, err := a.Sessions.Active(deviceName)
	if err != nil {
		return err
	}
	if sess == nil || !sess.Safe {
		return nil
	}
	id := findIDArg(rosArgs)
	if inv := session.BuildInverse(rosCmd, rosArgs, id); len(inv) > 0 {
		return nil
	}
	if force {
		return nil
	}
	return fmt.Errorf("safe session active: exec %q has no known inverse; re-run with --force to skip journaling", rosCmd)
}
