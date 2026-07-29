package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/filexfer"
	"github.com/nic0der-im/routeros-cli/internal/publicip"
	"github.com/spf13/cobra"
)

const defaultBackupName = "routeros-cli-backup"
const defaultExportName = "ros-export"

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup router configuration",
	}
	cmd.AddCommand(
		newBackupExportCmd(),
		newBackupBinaryCmd(),
	)
	return cmd
}

func newBackupExportCmd() *cobra.Command {
	var filePath string
	var via string
	var sourceIP string
	var publicIPURL string
	var ephemeralSSH bool
	var keepRemote bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export text configuration from the router",
		Long: `Export the router's text configuration.

Many RouterOS versions (including 7.x on RB2011) return an empty body for
/export over the API. ros therefore writes /export file=<name> on the router
and downloads the .rsc (default via SFTP), then removes the remote file.

If --file is omitted, the export is printed to stdout.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/export", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable("/export"); err != nil {
					return err
				}

				// Fast path: some devices still stream export text over the API.
				if result, err := c.Run(ctx, "/export"); err == nil {
					if export := extractExportText(result); export != "" {
						return writeOrPrintExport(cmd, filePath, export, deviceName)
					}
				}

				remoteBase := fmt.Sprintf("%s-%d", defaultExportName, time.Now().Unix())
				remoteName := remoteBase + ".rsc"
				_, err := c.Run(ctx, "/export", "=file="+remoteBase)
				if err != nil {
					return fmt.Errorf("exporting configuration on %q: %w", deviceName, err)
				}

				out := filePath
				tmpLocal := false
				if out == "" {
					tmp, err := os.CreateTemp("", "ros-export-*.rsc")
					if err != nil {
						return err
					}
					out = tmp.Name()
					_ = tmp.Close()
					tmpLocal = true
					defer func() { _ = os.Remove(out) }()
				} else if st, err := os.Stat(out); err == nil && st.IsDir() {
					out = filepath.Join(out, remoteName)
				}

				_, dev, err := a.Inventory.Resolve(flagDevice)
				if err != nil {
					return err
				}
				pass, err := a.Creds.Get(deviceName)
				if err != nil {
					return err
				}
				ephem := ephemeralSSH
				n, err := filexfer.Download(ctx, remoteName, out, filexfer.Options{
					Via:          filexfer.Via(via),
					Host:         hostOnly(dev.Address),
					User:         dev.Username,
					Pass:         pass,
					SourceIP:     sourceIP,
					PublicIPURL:  publicIPURL,
					EphemeralSSH: &ephem,
					Client:       c,
					OnStatus: func(msg string) {
						fmt.Fprintf(cmd.OutOrStdout(), "  · %s\n", msg)
					},
				})
				if !keepRemote {
					if _, rerr := c.Run(context.WithoutCancel(ctx), "/file/remove", "=numbers="+remoteName); rerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove remote %q: %v\n", remoteName, rerr)
					}
				}
				if err != nil {
					return fmt.Errorf("downloading export: %w", err)
				}
				if n == 0 {
					return fmt.Errorf("downloaded empty export from %q", deviceName)
				}

				if tmpLocal {
					data, err := os.ReadFile(out)
					if err != nil {
						return err
					}
					fmt.Fprint(cmd.OutOrStdout(), string(data))
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Configuration exported to %q from %q (%d bytes)\n", out, deviceName, n)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "local file path (or directory) to save the export")
	cmd.Flags().StringVar(&via, "via", "sftp", "download via: sftp|auto|api|ftp")
	cmd.Flags().StringVar(&sourceIP, "source-ip", "", "override detected IP for ephemeral SSH allowlist")
	cmd.Flags().StringVar(&publicIPURL, "public-ip-url", publicip.DefaultURL, "HTTPS URL that returns the caller public IP")
	cmd.Flags().BoolVar(&ephemeralSSH, "ephemeral-ssh", true, "temporarily whitelist caller IPs on SSH for SFTP, then restore")
	cmd.Flags().BoolVar(&keepRemote, "keep-remote", false, "keep the .rsc on the router after download")

	return cmd
}

func writeOrPrintExport(cmd *cobra.Command, filePath, export, deviceName string) error {
	if filePath != "" {
		if err := os.WriteFile(filePath, []byte(export), 0o600); err != nil {
			return fmt.Errorf("writing export to %q: %w", filePath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Configuration exported to %q from %q\n", filePath, deviceName)
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), export)
	return nil
}

func newBackupBinaryCmd() *cobra.Command {
	var backupName string
	var outputPath string
	var via string
	var sourceIP string
	var publicIPURL string
	var ephemeralSSH bool

	cmd := &cobra.Command{
		Use:   "binary",
		Short: "Create a binary backup on the router (optionally download it)",
		Long: `Create a binary backup file on the router via /system/backup/save.

With --output, ros downloads the .backup locally. Default transport is SFTP:
the CLI detects its local/public IP, temporarily merges it into the router's
SSH allowlist, downloads over SFTP, then restores the previous SSH service
state (even if the transfer fails).

Use --via api for small text files with API contents, or --via ftp only when
explicitly required (not recommended).`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/system/backup/save", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable("/system/backup/save"); err != nil {
					return err
				}
				_, err := c.Run(ctx, "/system/backup/save", "=name="+backupName)
				if err != nil {
					return fmt.Errorf("creating binary backup on %q: %w", deviceName, err)
				}

				fileName := backupName + ".backup"
				result, err := c.Run(ctx, "/file/print", "?name="+fileName)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Backup %q created on %q (verification query failed: %v)\n", fileName, deviceName, err)
					return nil
				}

				if len(result.Sentences) > 0 {
					size := result.Sentences[0]["size"]
					fmt.Fprintf(cmd.OutOrStdout(), "Backup %q created on %q (size: %s)\n", fileName, deviceName, size)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Backup command sent to %q (file: %s)\n", deviceName, fileName)
				}

				if outputPath == "" {
					return nil
				}
				out := outputPath
				if st, err := os.Stat(out); err == nil && st.IsDir() {
					out = filepath.Join(out, fileName)
				}
				_, dev, err := a.Inventory.Resolve(flagDevice)
				if err != nil {
					return err
				}
				pass, err := a.Creds.Get(deviceName)
				if err != nil {
					return err
				}
				ephem := ephemeralSSH
				n, err := filexfer.Download(ctx, fileName, out, filexfer.Options{
					Via:          filexfer.Via(via),
					Host:         hostOnly(dev.Address),
					User:         dev.Username,
					Pass:         pass,
					SourceIP:     sourceIP,
					PublicIPURL:  publicIPURL,
					EphemeralSSH: &ephem,
					Client:       c,
					OnStatus: func(msg string) {
						fmt.Fprintf(cmd.OutOrStdout(), "  · %s\n", msg)
					},
				})
				if err != nil {
					return fmt.Errorf("downloading backup: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Downloaded backup to %q (%d bytes)\n", out, n)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&backupName, "file", defaultBackupName, "backup name on the router (without .backup extension)")
	cmd.Flags().StringVar(&outputPath, "output", "", "local path or directory to download the .backup")
	cmd.Flags().StringVar(&via, "via", "sftp", "download via: sftp|auto|api|ftp")
	cmd.Flags().StringVar(&sourceIP, "source-ip", "", "override detected IP for ephemeral SSH allowlist (CIDR or IP)")
	cmd.Flags().StringVar(&publicIPURL, "public-ip-url", publicip.DefaultURL, "HTTPS URL that returns the caller public IP")
	cmd.Flags().BoolVar(&ephemeralSSH, "ephemeral-ssh", true, "temporarily whitelist caller IPs on SSH for SFTP, then restore")

	return cmd
}

// extractExportText pulls the configuration text from a /export result.
func extractExportText(result *client.Result) string {
	var parts []string

	for _, s := range result.Sentences {
		for _, key := range []string{"ret", "message"} {
			if v, ok := s[key]; ok && v != "" {
				parts = append(parts, v)
			}
		}
	}

	if len(parts) > 0 {
		text := strings.Join(parts, "\n")
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text
	}

	for _, s := range result.Sentences {
		for _, v := range s {
			if v != "" {
				parts = append(parts, v)
			}
		}
	}

	if len(parts) > 0 {
		text := strings.Join(parts, "\n")
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text
	}

	return ""
}
