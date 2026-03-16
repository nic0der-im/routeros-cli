package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute an arbitrary RouterOS command",
		Long: `Execute any RouterOS API command directly.

Examples:
  routeros-cli exec /interface/print
  routeros-cli exec /ip/address/print =interface=ether1
  routeros-cli exec /system/package/print`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rosCmd := args[0]
			rosArgs := args[1:]

			runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, rosCmd, rosArgs...)
				if err != nil {
					return fmt.Errorf("executing %q: %w", rosCmd, err)
				}

				if len(result.Sentences) == 0 {
					if a.OutFormat == "json" {
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

				// Build a stable key order from the first sentence.
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
}
