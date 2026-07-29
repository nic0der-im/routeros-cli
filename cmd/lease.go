package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func newLeaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lease",
		Short: "DHCP lease helpers (curated capa linda)",
	}
	cmd.AddCommand(newLeaseCleanupWaitingCmd())
	return cmd
}

func newLeaseCleanupWaitingCmd() *cobra.Command {
	var (
		dryRun  bool
		confirm string
	)
	cmd := &cobra.Command{
		Use:   "cleanup-waiting",
		Short: "Remove DHCP leases stuck in status=waiting",
		Long: `Remove DHCP leases stuck in status=waiting (may delete many rows).

` + confirmLongHelp + `

--dry-run lists matches without deleting and does not require --confirm.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/ip/dhcp-server/lease/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				result, err := c.Run(ctx, "/ip/dhcp-server/lease/print", "?status=waiting")
				if err != nil {
					return err
				}
				if len(result.Sentences) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No waiting leases")
					return nil
				}
				if !dryRun {
					if err := requireConfirmDevice(confirm, deviceName); err != nil {
						return err
					}
				}
				removed := 0
				for _, s := range result.Sentences {
					id := s[".id"]
					if id == "" {
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "waiting lease %s address=%s mac=%s comment=%s\n",
						id, s["address"], s["mac-address"], s["comment"])
					if dryRun {
						continue
					}
					rosCmd := "/ip/dhcp-server/lease/remove"
					if err := a.ensureWritable(deviceName, rosCmd); err != nil {
						return err
					}
					pre := map[string]string{}
					for k, v := range s {
						pre[k] = v
					}
					if _, err := c.Run(ctx, rosCmd, "=.id="+id); err != nil {
						return fmt.Errorf("removing lease %s: %w", id, err)
					}
					if inv := session.BuildRemoveInverse(rosCmd, pre); len(inv) > 0 {
						_ = a.recordSafeChange(deviceName, session.Change{
							Command:  rosCmd,
							Args:     []string{"=.id=" + id},
							Inverse:  inv,
							PreState: pre,
							Note:     "lease cleanup-waiting",
						})
					}
					removed++
				}
				if dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: %d waiting lease(s)\n", len(result.Sentences))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed %d waiting lease(s) on %s\n", removed, deviceName)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list waiting leases without deleting")
	registerConfirmFlag(cmd, &confirm)
	return cmd
}
