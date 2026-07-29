package cmd

import (
	"context"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Read-only hygiene audit with FINDINGS (alias of audit --profile hygiene)",
		Long: `Read-only hygiene audit with FINDINGS (alias of audit --profile hygiene).

Reuses the same collect+render path as audit --profile hygiene. Findings are
informational hints only — exit code stays 0 even when warnings appear.
Use -o json for the raw section maps (FINDINGS are human-only).`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/audit", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				// Hygiene always skips /tool/profile.
				order, sections, err := collectAudit(ctx, c, "hygiene", false, 0)
				if err != nil {
					return err
				}

				meta := output.Meta{
					Device:    deviceName,
					Command:   "doctor",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Count:     len(sections),
				}

				if a.OutFormat == output.FormatJSON {
					return output.RenderRawJSON(cmd.OutOrStdout(), sections, meta)
				}

				return renderAuditHuman(cmd.OutOrStdout(), deviceName, "hygiene", order, sections, false)
			})
		},
	}
}
