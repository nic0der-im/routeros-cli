package cmd

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/filexfer"
	"github.com/spf13/cobra"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Work with files on the router",
	}
	cmd.AddCommand(newFileGetCmd())
	return cmd
}

func newFileGetCmd() *cobra.Command {
	var outputPath string
	var via string

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Download a file from the router to the local disk",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			runWithClient(cmd, "/file/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				out := outputPath
				if out == "" {
					out = filepath.Base(name)
				}
				_, dev, err := a.Inventory.Resolve(deviceName)
				if err != nil {
					// deviceName from connect is already resolved name
					_, dev, err = a.Inventory.Resolve(flagDevice)
					if err != nil {
						return err
					}
				}
				host := hostOnly(dev.Address)
				pass, err := a.Creds.Get(deviceName)
				if err != nil {
					return err
				}
				n, err := filexfer.Download(ctx, c, name, out, filexfer.Via(via), host, dev.Username, pass)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %q → %q (%d bytes) from %q\n", name, out, n, deviceName)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "local output path (default: basename)")
	cmd.Flags().StringVar(&via, "via", "auto", "transfer via: auto|api|ftp")
	return cmd
}

func hostOnly(address string) string {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	} else if i := strings.LastIndex(address, ":"); i > 0 {
		// bare host:port without brackets
		host = address[:i]
	}
	return host
}
