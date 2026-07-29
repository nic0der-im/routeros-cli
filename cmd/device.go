package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/device"
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
		newDeviceAuthCmd(),
		newDeviceGetCmd(),
		newDeviceDiscoverCmd(),
		newDeviceImportCmd(),
	)
	return cmd
}

func newDeviceAddCmd() *cobra.Command {
	var (
		address       string
		username      string
		passwordStdin bool
		useTLS        bool
		id            string
		tags          []string
		notes         string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a device to the inventory",
		Long: `Add a RouterOS device to the local inventory.

Two modes:

  Interactive (human, TTY):
    ros device add
    ros device add "central-hub-buenos-aires"
    Prompts for name, host, port, username, password, TLS, and optional fields.

  Agentic / scripted (non-interactive):
    echo "$PASS" | ros device add lab \
      --address 192.168.88.1:8728 \
      --username admin \
      --password-stdin

    Requires --address and --password-stdin (or ROS_DEVICE_ADD_NONINTERACTIVE=1).
    Suitable for agents, CI, and secret injection via pipes.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			agentic := nonInteractive ||
				passwordStdin ||
				os.Getenv("ROS_DEVICE_ADD_NONINTERACTIVE") == "1" ||
				!isInteractiveTerminal()

			var password string

			if agentic {
				if name == "" {
					return fmt.Errorf("device name is required in non-interactive mode")
				}
				if address == "" {
					return fmt.Errorf("--address is required in non-interactive mode")
				}
				if !passwordStdin && os.Getenv("ROS_PASSWORD") == "" {
					return fmt.Errorf("--password-stdin (or ROS_PASSWORD) is required in non-interactive mode")
				}
				if passwordStdin {
					password, err = readPassword(true)
				} else {
					password = os.Getenv("ROS_PASSWORD")
				}
				if err != nil {
					return err
				}
				if password == "" {
					return fmt.Errorf("empty password")
				}

				tlsChanged := cmd.Flags().Changed("tls")
				useTLS = device.InferTLS(address, tlsChanged, useTLS)
			} else {
				fmt.Fprintln(os.Stderr, "Add RouterOS device (interactive)")
				fmt.Fprintln(os.Stderr, "Press Enter to accept defaults shown in [brackets].")
				fmt.Fprintln(os.Stderr)

				if name == "" {
					name, err = promptLine(os.Stdin, os.Stderr, "Device name", "")
					if err != nil {
						return err
					}
					if name == "" {
						return fmt.Errorf("device name is required")
					}
				} else {
					fmt.Fprintf(os.Stderr, "Device name: %s\n", name)
				}

				hostDefault := ""
				portDefault := "8728"
				if address != "" {
					h, p, splitErr := net.SplitHostPort(address)
					if splitErr == nil {
						hostDefault, portDefault = h, p
					} else {
						hostDefault = address
					}
				}

				host, err := promptLine(os.Stdin, os.Stderr, "Host / IP", hostDefault)
				if err != nil {
					return err
				}
				port, err := promptLine(os.Stdin, os.Stderr, "API port", portDefault)
				if err != nil {
					return err
				}
				address, err = joinHostPort(host, port)
				if err != nil {
					return err
				}

				userDefault := username
				if userDefault == "" {
					userDefault = "admin"
				}
				username, err = promptLine(os.Stdin, os.Stderr, "Username", userDefault)
				if err != nil {
					return err
				}

				tlsDefault := device.InferTLS(address, false, useTLS)
				useTLS, err = promptYesNo(os.Stdin, os.Stderr, "Use TLS", tlsDefault)
				if err != nil {
					return err
				}

				password, err = promptPassword("Password")
				if err != nil {
					return err
				}

				if id == "" {
					id, err = promptLine(os.Stdin, os.Stderr, "ID / slug (optional)", "")
					if err != nil {
						return err
					}
				}
				if len(tags) == 0 {
					tagStr, err := promptLine(os.Stdin, os.Stderr, "Tags comma-separated (optional)", "")
					if err != nil {
						return err
					}
					if tagStr != "" {
						for _, t := range strings.Split(tagStr, ",") {
							t = strings.TrimSpace(t)
							if t != "" {
								tags = append(tags, t)
							}
						}
					}
				}
				if notes == "" {
					notes, err = promptLine(os.Stdin, os.Stderr, "Notes (optional)", "")
					if err != nil {
						return err
					}
				}
			}

			dev := config.DeviceConfig{
				Address:  address,
				Username: username,
				TLS:      useTLS,
				ID:       id,
				Tags:     tags,
				Notes:    notes,
			}

			if err := a.Inventory.Add(name, dev); err != nil {
				return err
			}
			if err := a.Creds.Set(name, password); err != nil {
				_ = a.Inventory.Remove(name)
				return fmt.Errorf("storing password: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Device %q added (%s, user=%s, tls=%v)\n", name, address, username, useTLS)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: ros device use %q && ros device test\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&address, "address", "", "device address (host:port) — required in non-interactive mode")
	cmd.Flags().StringVar(&username, "username", "admin", "username")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (agentic/non-interactive)")
	cmd.Flags().BoolVar(&useTLS, "tls", true, "use TLS; inferred from port when unset")
	cmd.Flags().StringVar(&id, "id", "", "optional stable id/slug for lookup")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "optional tags")
	cmd.Flags().StringVar(&notes, "notes", "", "optional notes")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "fail instead of prompting (for agents/CI)")

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
					Default:  isDefault,
					Name:     name,
					ID:       dev.ID,
					Address:  dev.Address,
					Username: dev.Username,
					TLS:      fmt.Sprintf("%v", dev.TLS),
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

func newDeviceAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage device credentials",
	}
	cmd.AddCommand(newDeviceAuthSetCmd())
	return cmd
}

func newDeviceAuthSetCmd() *cobra.Command {
	var passwordStdin bool

	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set or rotate the stored password for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			a, err := loadApp()
			if err != nil {
				return err
			}

			resolved, _, err := a.Inventory.Lookup(name)
			if err != nil {
				return err
			}

			var password string
			var errPass error
			if passwordStdin {
				password, errPass = readPassword(true)
			} else if env := os.Getenv("ROS_PASSWORD"); env != "" && !isInteractiveTerminal() {
				password = env
			} else {
				password, errPass = promptPassword("Password")
			}
			if errPass != nil {
				return errPass
			}

			if err := a.Creds.Set(resolved, password); err != nil {
				return fmt.Errorf("storing password: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Password updated for device %q\n", resolved)
			return nil
		},
	}

	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (agentic)")
	return cmd
}

func newDeviceGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name|id|ip>",
		Short: "Show a single device from the inventory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}

			name, dev, err := a.Inventory.Lookup(args[0])
			if err != nil {
				return err
			}

			entry := deviceDetail{
				Name:     name,
				ID:       dev.ID,
				Address:  dev.Address,
				Username: dev.Username,
				TLS:      fmt.Sprintf("%v", dev.TLS),
				Tags:     strings.Join(dev.Tags, ","),
				Notes:    dev.Notes,
				Default:  "",
			}
			if a.Inventory.Default() == name {
				entry.Default = "*"
			}

			if a.OutFormat == output.FormatJSON {
				return output.RenderRawJSON(cmd.OutOrStdout(), entry, output.Meta{
					Device:    name,
					Command:   "device get",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Count:     1,
				})
			}

			dl := deviceList{deviceEntry{
				Default:  entry.Default,
				Name:     entry.Name,
				ID:       entry.ID,
				Address:  entry.Address,
				Username: entry.Username,
				TLS:      entry.TLS,
			}}
			_ = output.Render(cmd.OutOrStdout(), a.OutFormat, dl, output.Meta{Command: "device get", Count: 1})
			if entry.Tags != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Tags:  %s\n", entry.Tags)
			}
			if entry.Notes != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Notes: %s\n", entry.Notes)
			}
			return nil
		},
	}
}

// deviceEntry is used for rendering the device list.
type deviceEntry struct {
	Default  string
	Name     string
	ID       string
	Address  string
	Username string
	TLS      string
}

type deviceList []deviceEntry

func (dl deviceList) TableHeaders() []string {
	return []string{"Default", "Name", "ID", "Address", "Username", "TLS"}
}

func (dl deviceList) TableRows() [][]string {
	rows := make([][]string, len(dl))
	for i, d := range dl {
		rows[i] = []string{d.Default, d.Name, d.ID, d.Address, d.Username, d.TLS}
	}
	return rows
}

type deviceDetail struct {
	Name     string `json:"name"`
	ID       string `json:"id,omitempty"`
	Address  string `json:"address"`
	Username string `json:"username"`
	TLS      string `json:"tls"`
	Tags     string `json:"tags,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Default  string `json:"default,omitempty"`
}
