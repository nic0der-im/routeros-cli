package cmd

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/filexfer"
	"github.com/nic0der-im/routeros-cli/internal/publicip"
	"github.com/spf13/cobra"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Work with files on the router",
		Long: `List, download, or remove files on the router.

  ros file list
  ros file get <name> [--output ./local]
  ros file remove <name-or-id>`,
	}
	cmd.AddCommand(
		newFileListCmd(),
		newFileGetCmd(),
		newFileRemoveCmd(),
	)
	return cmd
}

func newFileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List files on the router (/file/print)",
		Long:  `List files via /file/print. Output is table or JSON like other get commands.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/file/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/file/print")
				if err != nil {
					return fmt.Errorf("listing files: %w", err)
				}
				return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, "/file/print")
			})
		},
	}
}

func newFileGetCmd() *cobra.Command {
	var outputPath string
	var via string
	var sourceIP string
	var publicIPURL string
	var ephemeralSSH bool

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Download a file from the router to the local disk",
		Long: `Download a RouterOS file.

Default --via sftp uses ephemeral SSH allowlisting: detect local/public IP,
merge into /ip/service ssh address, SFTP the file, restore previous SSH state.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			runWithClient(cmd, "/file/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				out := outputPath
				if out == "" {
					out = filepath.Base(name)
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
				n, err := filexfer.Download(ctx, name, out, filexfer.Options{
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
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %q → %q (%d bytes) from %q\n", name, out, n, deviceName)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "local output path (default: basename)")
	cmd.Flags().StringVar(&via, "via", "sftp", "transfer via: sftp|auto|api|ftp")
	cmd.Flags().StringVar(&sourceIP, "source-ip", "", "override detected IP for ephemeral SSH allowlist")
	cmd.Flags().StringVar(&publicIPURL, "public-ip-url", publicip.DefaultURL, "HTTPS URL that returns the caller public IP")
	cmd.Flags().BoolVar(&ephemeralSSH, "ephemeral-ssh", true, "temporarily whitelist caller IPs on SSH for SFTP, then restore")
	return cmd
}

func newFileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name-or-id>",
		Short: "Remove a file on the router (/file/remove)",
		Long: `Remove a RouterOS file by name or .id.

  ros file remove stale.backup
  ros file remove '*A'

Names use =numbers=<name> (same as backup cleanup). Values that look like
RouterOS ids (start with *) use =.id=.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := args[0]
			runWithClient(cmd, "/file/remove", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				if err := a.ensureWritable("/file/remove"); err != nil {
					return err
				}
				apiArg := fileRemoveArg(target)
				_, err := c.Run(ctx, "/file/remove", apiArg)
				if err != nil {
					return fmt.Errorf("removing file %q: %w", target, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed file %q on %s\n", target, deviceName)
				return nil
			})
		},
	}
}

// fileRemoveArg builds the RouterOS /file/remove argument for a name or .id.
func fileRemoveArg(nameOrID string) string {
	if strings.HasPrefix(nameOrID, "*") {
		return "=.id=" + nameOrID
	}
	return "=numbers=" + nameOrID
}

func hostOnly(address string) string {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	} else if i := strings.LastIndex(address, ":"); i > 0 && strings.Count(address, ":") == 1 {
		host = address[:i]
	}
	return host
}
