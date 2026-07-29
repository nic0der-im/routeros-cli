package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Safe change sessions with commit/rollback",
	}
	cmd.AddCommand(
		newSessionBeginCmd(),
		newSessionCommitCmd(),
		newSessionRollbackCmd(),
		newSessionStatusCmd(),
	)
	return cmd
}

func newSessionBeginCmd() *cobra.Command {
	safe := true

	cmd := &cobra.Command{
		Use:   "begin",
		Short: "Begin a safe change session for the current device",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name, _, err := a.Inventory.Resolve(flagDevice)
			if err != nil {
				return err
			}

			sess, err := a.Sessions.Begin(name, safe)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Session %s started on %q (safe=%v)\n", sess.ID, name, sess.Safe)
			return nil
		},
	}

	cmd.Flags().BoolVar(&safe, "safe", true, "enable safe journaling (default true)")
	return cmd
}

func newSessionCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit",
		Short: "Commit the active session (keep changes, clear journal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name, _, err := a.Inventory.Resolve(flagDevice)
			if err != nil {
				return err
			}

			sess, err := a.Sessions.Active(name)
			if err != nil {
				return err
			}
			if sess == nil {
				return fmt.Errorf("no active session for device %q", name)
			}

			if err := a.Sessions.Commit(sess); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Session %s committed on %q (%d change(s))\n", sess.ID, name, len(sess.Changes))
			return nil
		},
	}
}

func newSessionRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "Roll back the active session by applying inverses in reverse order",
		Run: func(cmd *cobra.Command, args []string) {
			a, err := loadApp()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(ExitConfError)
			}

			name, _, err := a.Inventory.Resolve(flagDevice)
			if err != nil {
				a.renderError(os.Stderr, "config_error", err.Error(), "")
				os.Exit(ExitConfError)
			}

			sess, err := a.Sessions.Active(name)
			if err != nil {
				a.renderError(os.Stderr, "session_error", err.Error(), name)
				os.Exit(ExitCmdError)
			}
			if sess == nil {
				a.renderError(os.Stderr, "session_error", fmt.Sprintf("no active session for device %q", name), name)
				os.Exit(ExitCmdError)
			}

			runWithClient(cmd, "/session/rollback", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				// Re-load in case Resolve differed; prefer the session device.
				if deviceName != sess.Device {
					deviceName = sess.Device
				}

				for i := len(sess.Changes) - 1; i >= 0; i-- {
					ch := sess.Changes[i]
					if len(ch.Inverse) == 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "skip change %s: no inverse\n", ch.ID)
						continue
					}
					invCmd := ch.Inverse[0]
					invArgs := ch.Inverse[1:]
					if _, err := c.Run(ctx, invCmd, invArgs...); err != nil {
						return fmt.Errorf("rolling back change %s (%s): %w", ch.ID, invCmd, err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "reverted %s via %s\n", ch.ID, invCmd)
				}

				if err := a.Sessions.MarkRolledBack(sess); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Session %s rolled back on %q\n", sess.ID, deviceName)
				return nil
			})
		},
	}
}

func newSessionStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active session for the current/default device",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name, _, err := a.Inventory.Resolve(flagDevice)
			if err != nil {
				return err
			}

			sess, err := a.Sessions.Active(name)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			if sess == nil {
				if a.OutFormat == output.FormatJSON {
					return output.RenderRawJSON(w, map[string]interface{}{
						"active": false,
						"device": name,
					}, output.Meta{
						Device:    name,
						Command:   "session status",
						Timestamp: time.Now().UTC().Format(time.RFC3339),
						Count:     0,
					})
				}
				fmt.Fprintf(w, "No active session for device %q\n", name)
				return nil
			}

			if a.OutFormat == output.FormatJSON {
				return output.RenderRawJSON(w, sess, output.Meta{
					Device:    name,
					Command:   "session status",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Count:     len(sess.Changes),
				})
			}

			fmt.Fprintf(w, "Session %s\n", sess.ID)
			fmt.Fprintf(w, "  Device:     %s\n", sess.Device)
			fmt.Fprintf(w, "  Status:     %s\n", sess.Status)
			fmt.Fprintf(w, "  Safe:       %v\n", sess.Safe)
			fmt.Fprintf(w, "  Started:    %s\n", sess.StartedAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Updated:    %s\n", sess.UpdatedAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Changes:    %d\n", len(sess.Changes))
			for i, ch := range sess.Changes {
				fmt.Fprintf(w, "    %d. %s %v\n", i+1, ch.Command, ch.Args)
			}
			return nil
		},
	}
}
