package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/filexfer"
	"github.com/spf13/cobra"
)

const defaultBackupName = "routeros-cli-backup"

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

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export text configuration from the router",
		Long: `Export the router's text configuration via the RouterOS API.

If --file is specified, the export is written to the given local file path.
Otherwise the export is printed to stdout.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/export", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/export")
				if err != nil {
					return fmt.Errorf("exporting configuration from %q: %w", deviceName, err)
				}

				export := extractExportText(result)
				if export == "" {
					return fmt.Errorf("no configuration data returned from %q", deviceName)
				}

				if filePath != "" {
					if err := os.WriteFile(filePath, []byte(export), 0o644); err != nil {
						return fmt.Errorf("writing export to %q: %w", filePath, err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Configuration exported to %q from %q\n", filePath, deviceName)
					return nil
				}

				fmt.Fprint(cmd.OutOrStdout(), export)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "local file path to save the export")

	return cmd
}

func newBackupBinaryCmd() *cobra.Command {
	var backupName string
	var outputPath string
	var via string

	cmd := &cobra.Command{
		Use:   "binary",
		Short: "Create a binary backup on the router (optionally download it)",
		Long: `Create a binary backup file on the router via /system/backup/save.

The --file flag sets the backup name on the router (without extension).
Use --output to download the .backup to the local disk (API contents or FTP).`,
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
				n, err := filexfer.Download(ctx, c, fileName, out, filexfer.Via(via), hostOnly(dev.Address), dev.Username, pass)
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
	cmd.Flags().StringVar(&via, "via", "auto", "download via: auto|api|ftp")

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
