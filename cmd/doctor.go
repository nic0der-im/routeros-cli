package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
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
Use -o json for the raw section maps (FINDINGS are human-only).

On success, records LastDoctorAt under ~/.config/ros/state/<device>.doctor
for the prod write doctor-freshness gate.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/audit", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				// Hygiene always skips /tool/profile.
				order, sections, err := collectAudit(ctx, c, "hygiene", false, 0)
				if err != nil {
					return err
				}

				recordDoctorSuccess(deviceName, cmd.ErrOrStderr())

				meta := a.newMeta(deviceName, "doctor", len(sections))

				if a.OutFormat == output.FormatJSON {
					return a.renderRawJSON(cmd.OutOrStdout(), sections, meta)
				}

				return renderAuditHuman(cmd.OutOrStdout(), deviceName, "hygiene", order, sections, false)
			})
		},
	}
}

// recordDoctorSuccess persists LastDoctorAt for the prod write protocol.
// Failures are soft-warned; they must not fail the doctor/hygiene command itself.
func recordDoctorSuccess(deviceName string, errW io.Writer) {
	if err := guardrails.RecordDoctorAt(deviceName, time.Now()); err != nil && errW != nil {
		fmt.Fprintf(errW, "warning: could not record doctor timestamp: %v\n", err)
	}
}
