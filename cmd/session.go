package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/session"
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
		newSessionWatchCmd(),
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
			fmt.Fprintf(cmd.OutOrStdout(), "Tip: run `ros -d %s session watch` in another terminal for link-loss auto-rollback\n", name)
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

func applySessionRollback(ctx context.Context, a *App, c client.Client, sess *session.Session, w io.Writer) error {
	for i := len(sess.Changes) - 1; i >= 0; i-- {
		ch := sess.Changes[i]
		if len(ch.Inverse) == 0 {
			if w != nil {
				fmt.Fprintf(w, "skip change %s: no inverse\n", ch.ID)
			}
			continue
		}
		invCmd := ch.Inverse[0]
		invArgs := ch.Inverse[1:]
		if _, err := c.Run(ctx, invCmd, invArgs...); err != nil {
			return fmt.Errorf("rolling back change %s (%s): %w", ch.ID, invCmd, err)
		}
		if w != nil {
			fmt.Fprintf(w, "reverted %s via %s\n", ch.ID, invCmd)
		}
	}
	if err := a.Sessions.MarkRolledBack(sess); err != nil {
		return err
	}
	return nil
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
				if err := applySessionRollback(ctx, a, c, sess, cmd.OutOrStdout()); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Session %s rolled back on %q\n", sess.ID, deviceName)
				return nil
			})
		},
	}
}

func newSessionWatchCmd() *cobra.Command {
	var interval time.Duration
	var fails int

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Heartbeat the device and auto-rollback the safe session on link loss",
		Long: `Probes the device periodically. After consecutive failures, marks the
session auto_rollback_pending and attempts rollback when the API is reachable
again. Best-effort client-side analogue of RouterOS Safe Mode (not available
over the binary API).`,
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
			if err != nil || sess == nil {
				a.renderError(os.Stderr, "session_error", fmt.Sprintf("no active session for device %q", name), name)
				os.Exit(ExitCmdError)
			}
			if !sess.Safe {
				a.renderError(os.Stderr, "session_error", "session watch requires a safe session", name)
				os.Exit(ExitCmdError)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Watching session %s on %q (interval=%s, fails=%d)\n", sess.ID, name, interval, fails)
			ctx := cmd.Context()
			err = session.Watch(ctx, a.Sessions, sess, session.WatchConfig{
				Interval:      interval,
				FailThreshold: fails,
				Probe: func(pctx context.Context) error {
					c, _, err := a.connect(pctx)
					if err != nil {
						return err
					}
					defer c.Close()
					_, err = c.Run(pctx, "/system/identity/print")
					return err
				},
				Rollback: func(rctx context.Context, s *session.Session) error {
					c, _, err := a.connect(rctx)
					if err != nil {
						return err
					}
					defer c.Close()
					return applySessionRollback(rctx, a, c, s, cmd.OutOrStdout())
				},
				OnLinkLost: func(s *session.Session) {
					fmt.Fprintf(cmd.OutOrStdout(), "Link lost — auto-rollback pending for session %s\n", s.ID)
				},
				OnRolledBack: func(s *session.Session, rbErr error) {
					if rbErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Auto-rollback failed: %v\n", rbErr)
						return
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Auto-rolled back session %s\n", s.ID)
				},
			})
			if err != nil && err != context.Canceled {
				a.renderError(os.Stderr, "session_error", err.Error(), name)
				os.Exit(ExitCmdError)
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "probe interval")
	cmd.Flags().IntVar(&fails, "fails", 3, "consecutive probe failures before auto-rollback")
	return cmd
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

			if sess.AutoRollbackPending {
				fmt.Fprintf(w, "WARNING: auto_rollback_pending — run: ros -d %s session rollback\n", name)
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
			fmt.Fprintf(w, "  Pending RB: %v\n", sess.AutoRollbackPending)
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
