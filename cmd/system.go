package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/spf13/cobra"
)

func newSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System information and management",
	}
	cmd.AddCommand(
		newSystemInfoCmd(),
		newSystemIdentityCmd(),
		newSystemResourceCmd(),
		newSystemRebootCmd(),
	)
	return cmd
}

func newSystemInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show system identity, version, and resource summary",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/system/resource/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/system/resource/print")
				if err != nil {
					return fmt.Errorf("fetching system resource: %w", err)
				}

				resources, err := rosapi.MapSentences[rosapi.SystemResource](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping system resource: %w", err)
				}

				idResult, err := c.Run(ctx, "/system/identity/print")
				if err != nil {
					return fmt.Errorf("fetching identity: %w", err)
				}

				identities, err := rosapi.MapSentences[rosapi.SystemIdentity](idResult.Sentences)
				if err != nil {
					return fmt.Errorf("mapping identity: %w", err)
				}

				info := make(rosapi.SystemInfoList, 0)
				for _, r := range resources {
					identity := ""
					if len(identities) > 0 {
						identity = identities[0].Name
					}
					info = append(info, rosapi.SystemInfo{
						Identity: identity,
						Resource: r,
					})
				}

				return a.render(cmd.OutOrStdout(), info, deviceName, "/system/resource/print")
			})
		},
	}
}

func newSystemIdentityCmd() *cobra.Command {
	var setName string

	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Get or set system identity",
		Run: func(cmd *cobra.Command, args []string) {
			if setName != "" {
				runWithClient(cmd, "/system/identity/set", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
					_, err := c.Run(ctx, "/system/identity/set", "=name="+setName)
					if err != nil {
						return fmt.Errorf("setting identity: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Identity set to %q on %s\n", setName, deviceName)
					return nil
				})
				return
			}

			runWithClient(cmd, "/system/identity/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/system/identity/print")
				if err != nil {
					return fmt.Errorf("fetching identity: %w", err)
				}

				identities, err := rosapi.MapSentences[rosapi.SystemIdentity](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping identity: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.SystemIdentities(identities), deviceName, "/system/identity/print")
			})
		},
	}

	cmd.Flags().StringVar(&setName, "set", "", "set system identity name")

	return cmd
}

func newSystemResourceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resource",
		Short: "Show system resources (CPU, memory, disk)",
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/system/resource/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/system/resource/print")
				if err != nil {
					return fmt.Errorf("fetching resources: %w", err)
				}

				resources, err := rosapi.MapSentences[rosapi.SystemResource](result.Sentences)
				if err != nil {
					return fmt.Errorf("mapping resources: %w", err)
				}

				return a.render(cmd.OutOrStdout(), rosapi.SystemResources(resources), deviceName, "/system/resource/print")
			})
		},
	}
}

func newSystemRebootCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reboot",
		Short: "Reboot the router",
		Run: func(cmd *cobra.Command, args []string) {
			if !force {
				fmt.Fprint(os.Stderr, "Reboot device? [y/N] ")
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return
				}
			}

			runWithClient(cmd, "/system/reboot", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				_, err := c.Run(ctx, "/system/reboot")
				if err != nil {
					return fmt.Errorf("rebooting: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Reboot command sent to %q\n", deviceName)
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}
