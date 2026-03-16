package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage device inventory",
	}
	cmd.AddCommand(
		newDeviceAddCmd(),
		newDeviceRemoveCmd(),
		newDeviceListCmd(),
		newDeviceUseCmd(),
		newDeviceTestCmd(),
	)
	return cmd
}

func newDeviceAddCmd() *cobra.Command {
	var (
		address      string
		username     string
		passwordStdin bool
		useTLS       bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a device to the inventory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			a, err := loadApp()
			if err != nil {
				return err
			}

			dev := config.DeviceConfig{
				Address:  address,
				Username: username,
				TLS:      useTLS,
			}

			if err := a.Inventory.Add(name, dev); err != nil {
				return err
			}

			// Read password from stdin if flag set.
			if passwordStdin {
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					password := strings.TrimSpace(scanner.Text())
					if password != "" {
						if err := a.Creds.Set(name, password); err != nil {
							return fmt.Errorf("storing password: %w", err)
						}
					}
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Device %q added (%s)\n", name, address)
			return nil
		},
	}

	cmd.Flags().StringVar(&address, "address", "", "device address (host:port)")
	cmd.Flags().StringVar(&username, "username", "admin", "username")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin")
	cmd.Flags().BoolVar(&useTLS, "tls", true, "use TLS (port 8729)")
	_ = cmd.MarkFlagRequired("address")

	return cmd
}

func newDeviceRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a device from the inventory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			a, err := loadApp()
			if err != nil {
				return err
			}

			if !force {
				fmt.Fprintf(cmd.OutOrStdout(), "Remove device %q? [y/N] ", name)
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
						fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
						return nil
					}
				}
			}

			if err := a.Inventory.Remove(name); err != nil {
				return err
			}

			// Also remove credentials.
			_ = a.Creds.Delete(name)

			fmt.Fprintf(cmd.OutOrStdout(), "Device %q removed\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

func newDeviceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all devices in the inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			devices := a.Inventory.List()
			defaultDev := a.Inventory.Default()

			dl := make(deviceList, 0, len(devices))
			for name, dev := range devices {
				isDefault := ""
				if name == defaultDev {
					isDefault = "*"
				}
				dl = append(dl, deviceEntry{
					Name:     name,
					Address:  dev.Address,
					Username: dev.Username,
					TLS:      fmt.Sprintf("%v", dev.TLS),
					Default:  isDefault,
				})
			}

			meta := output.Meta{
				Command: "device list",
				Count:   len(dl),
			}
			return output.Render(cmd.OutOrStdout(), a.OutFormat, dl, meta)
		},
	}
}

func newDeviceUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			a, err := loadApp()
			if err != nil {
				return err
			}

			if err := a.Inventory.SetDefault(name); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Default device set to %q\n", name)
			return nil
		},
	}
}

func newDeviceTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [name]",
		Short: "Test connectivity to a device",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				flagDevice = args[0]
			}

			runWithClient(cmd, "/system/identity/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/system/identity/print")
				if err != nil {
					return err
				}
				identity := "unknown"
				if len(result.Sentences) > 0 {
					if name, ok := result.Sentences[0]["name"]; ok {
						identity = name
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Connected to %q (identity: %s)\n", deviceName, identity)
				return nil
			})
		},
	}
}

// deviceEntry is used for rendering the device list.
type deviceEntry struct {
	Name     string
	Address  string
	Username string
	TLS      string
	Default  string
}

type deviceList []deviceEntry

func (dl deviceList) TableHeaders() []string {
	return []string{"Default", "Name", "Address", "Username", "TLS"}
}

func (dl deviceList) TableRows() [][]string {
	rows := make([][]string, len(dl))
	for i, d := range dl {
		rows[i] = []string{d.Default, d.Name, d.Address, d.Username, d.TLS}
	}
	return rows
}
