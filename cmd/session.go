package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
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

const forceNoBackupNote = "force-no-backup"

// backupsBaseDirForTest overrides ~/.config/ros/backups in unit tests.
var backupsBaseDirForTest string

// preSessionBackupFn is the dial+export hook for session begin (overridable in tests).
var preSessionBackupFn = takePreSessionBackup

func newSessionBeginCmd() *cobra.Command {
	safe := true
	forceNoBackup := false

	cmd := &cobra.Command{
		Use:   "begin",
		Short: "Begin a safe change session for the current device",
		Long: `Begin a change session for the current device.

On env_class=prod (or ROS_STRICT=1), --safe sessions take a local text backup
first under ~/.config/ros/backups/<device>/<timestamp>/. Failure refuses begin.
Use --force-no-backup only as break-glass (prints a strong warning and notes
the session). Staging/lab skip the backup unless require_backup_before_write=true.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name, _, err := a.Inventory.Resolve(flagDevice)
			if err != nil {
				return err
			}

			return runSessionBegin(cmd.Context(), a, name, safe, forceNoBackup, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().BoolVar(&safe, "safe", true, "enable safe journaling (default true)")
	cmd.Flags().BoolVar(&forceNoBackup, "force-no-backup", false, "skip mandatory pre-session text backup (break-glass; prod/require_backup_before_write)")
	return cmd
}

func sessionBeginNeedsBackup(safe bool, envClass string, deviceRequire, forceNoBackup bool) bool {
	if !safe || forceNoBackup {
		return false
	}
	return guardrails.RequireBackupBeforeSafeSession(envClass, deviceRequire)
}

// warnDoctorFreshness prints a loud warning when doctor/hygiene is missing or stale.
// Never refuses session begin (hard gate lives in ensureWritable).
func warnDoctorFreshness(envClass, deviceName string, force bool, errW io.Writer) {
	last, ok, err := guardrails.LoadLastDoctorAt(deviceName)
	if err != nil {
		fmt.Fprintf(errW, "WARNING: could not read doctor state: %v\n", err)
		return
	}
	warning, gateErr := guardrails.EvaluateDoctorGate(guardrails.DoctorGateOpts{
		EnvClass:   envClass,
		DeviceName: deviceName,
		LastAt:     last,
		HasLast:    ok,
		Now:        time.Now(),
		MaxAge:     guardrails.DefaultDoctorMaxAge,
		Force:      force,
		SkipEnv:    guardrails.ROSSkipDoctorGate(),
	})
	if warning != "" {
		fmt.Fprintln(errW, warning)
		return
	}
	if gateErr != nil {
		fmt.Fprintf(errW, "WARNING: %s\n", gateErr.Error())
	}
}

func runSessionBegin(ctx context.Context, a *App, name string, safe, forceNoBackup bool, out, errW io.Writer) error {
	dev := a.deviceConfig(name)
	envClass := config.EffectiveEnvClass(dev, a.effectiveStrict())
	needBackup := sessionBeginNeedsBackup(safe, envClass, dev.RequireBackupBeforeWrite, forceNoBackup)

	var (
		backupDir string
		note      string
	)

	if safe && forceNoBackup && guardrails.RequireBackupBeforeSafeSession(envClass, dev.RequireBackupBeforeWrite) {
		note = forceNoBackupNote
		fmt.Fprintf(errW, "WARNING: skipping mandatory pre-session backup on %s device %q (--force-no-backup). Proceed only if you already have a verified local export.\n", envClass, name)
	} else if needBackup {
		dir, err := prepareSessionBackupDir(name, time.Now())
		if err != nil {
			return err
		}
		saved, err := preSessionBackupFn(ctx, a, name, dir, out)
		if err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("refusing session begin: pre-session backup failed: %w\nHint: fix connectivity/export, or use --force-no-backup only as break-glass", err)
		}
		backupDir = dir
		fmt.Fprintf(out, "Pre-session backup saved to %q\n", saved)
	}

	// Soft doctor freshness warning for prod/staging before the first mutate.
	// Session begin never hard-refuses on doctor age; ensureWritable does.
	if safe {
		warnDoctorFreshness(envClass, name, a.Force, errW)
	}

	sess, err := a.Sessions.BeginWith(name, session.BeginOpts{
		Safe:      safe,
		Note:      note,
		BackupDir: backupDir,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Session %s started on %q (safe=%v)\n", sess.ID, name, sess.Safe)
	if sess.BackupDir != "" {
		fmt.Fprintf(out, "  Backup:     %s\n", sess.BackupDir)
	}
	if sess.Note != "" {
		fmt.Fprintf(out, "  Note:       %s\n", sess.Note)
	}
	fmt.Fprintf(out, "Tip: run `ros -d %s session watch` in another terminal for link-loss auto-rollback\n", name)
	return nil
}

func defaultBackupsDir() string {
	if backupsBaseDirForTest != "" {
		return backupsBaseDirForTest
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".config", "ros", "backups")
}

func sanitizeBackupDevice(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	s := string(out)
	if s == "" {
		return "device"
	}
	return s
}

func prepareSessionBackupDir(device string, t time.Time) (string, error) {
	dir := filepath.Join(defaultBackupsDir(), sanitizeBackupDevice(device), t.UTC().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating backup directory %q: %w", dir, err)
	}
	return dir, nil
}

// takePreSessionBackup connects and writes a text export into destDir.
// Skips ensureWritable so prod can backup before the safe session exists.
func takePreSessionBackup(ctx context.Context, a *App, deviceName, destDir string, w io.Writer) (string, error) {
	c, name, err := a.connect(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = c.Close() }()
	if name != "" {
		deviceName = name
	}

	_, dev, err := a.Inventory.Resolve(flagDevice)
	if err != nil {
		return "", err
	}
	pass, err := a.Creds.Get(deviceName)
	if err != nil {
		return "", err
	}

	out, n, err := exportTextToLocal(ctx, c, deviceName, exportTextOptions{
		DestPath:     destDir,
		Via:          "sftp",
		EphemeralSSH: true,
		Host:         hostOnly(dev.Address),
		User:         dev.Username,
		Pass:         pass,
		Status:       statusWriter(w),
	})
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", fmt.Errorf("empty export from %q", deviceName)
	}
	return out, nil
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
		Run:   runSessionRollback,
	}
}

// runSessionRollback applies inverse journal entries for the active session.
// Shared by ros session rollback and ros plan rollback.
func runSessionRollback(cmd *cobra.Command, args []string) {
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
					// Reload inventory each probe so address/VPN changes are picked up.
					fresh, err := loadApp()
					if err != nil {
						return err
					}
					c, _, err := fresh.connect(pctx)
					if err != nil {
						return err
					}
					defer func() { _ = c.Close() }()
					_, err = c.Run(pctx, "/system/identity/print")
					return err
				},
				Rollback: func(rctx context.Context, s *session.Session) error {
					fresh, err := loadApp()
					if err != nil {
						return err
					}
					c, _, err := fresh.connect(rctx)
					if err != nil {
						return err
					}
					defer func() { _ = c.Close() }()
					return applySessionRollback(rctx, fresh, c, s, cmd.OutOrStdout())
				},
				OnLinkLost: func(s *session.Session) {
					fmt.Fprintf(cmd.OutOrStdout(), "Link lost — auto-rollback pending for session %s (will retry when reachable)\n", s.ID)
				},
				OnRolledBack: func(s *session.Session, rbErr error) {
					if rbErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Auto-rollback attempt failed (will retry): %v\n", rbErr)
						return
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Auto-rolled back session %s\n", s.ID)
				},
			})
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
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
					return a.renderRawJSON(w, map[string]interface{}{
						"active": false,
						"device": name,
					}, a.newMeta(name, "session status", 0))
				}
				fmt.Fprintf(w, "No active session for device %q\n", name)
				return nil
			}

			if sess.AutoRollbackPending {
				fmt.Fprintf(w, "WARNING: auto_rollback_pending — run: ros -d %s session rollback\n", name)
			}

			safeSess := sanitizeSession(sess)
			if a.OutFormat == output.FormatJSON {
				return a.renderRawJSON(w, safeSess, a.newMeta(name, "session status", len(safeSess.Changes)))
			}

			fmt.Fprintf(w, "Session %s\n", safeSess.ID)
			fmt.Fprintf(w, "  Device:     %s\n", safeSess.Device)
			fmt.Fprintf(w, "  Status:     %s\n", safeSess.Status)
			fmt.Fprintf(w, "  Safe:       %v\n", safeSess.Safe)
			fmt.Fprintf(w, "  Pending RB: %v\n", safeSess.AutoRollbackPending)
			if safeSess.BackupDir != "" {
				fmt.Fprintf(w, "  Backup:     %s\n", safeSess.BackupDir)
			}
			if safeSess.Note != "" {
				fmt.Fprintf(w, "  Note:       %s\n", safeSess.Note)
			}
			fmt.Fprintf(w, "  Started:    %s\n", safeSess.StartedAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Updated:    %s\n", safeSess.UpdatedAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Changes:    %d\n", len(safeSess.Changes))
			for i, ch := range safeSess.Changes {
				fmt.Fprintf(w, "    %d. %s %v\n", i+1, ch.Command, ch.Args)
			}
			return nil
		},
	}
}
