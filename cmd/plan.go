package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/diff"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/plan"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Preview/apply YAML change plans via dry-run + safe sessions",
		Long: `Compose multi-step mutations from a YAML plan file.

Plans reuse existing dry-run/diff previews and safe-session change journals
(ros session begin --safe). They are not a separate journal format.

  ros plan preview --file plan.yaml
  ros -d home session begin --safe
  ros plan apply --file plan.yaml [--confirm home]
  ros plan rollback   # alias of session rollback`,
	}
	cmd.AddCommand(
		newPlanPreviewCmd(),
		newPlanApplyCmd(),
		newPlanRollbackCmd(),
	)
	return cmd
}

func newPlanPreviewCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Dry-run every plan step (no writes, no session journal)",
		Long: `Load and validate a YAML plan, then dry-run each step against the device.

Prints a human summary (or JSON preview envelope) with per-step diffs and
risk notes (path deny, needs safe session, destructive delete, comment-as-id).

Exits non-zero if any step cannot be previewed (bad path, ambiguous comment, …).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("--file is required")
			}
			doc, err := plan.LoadFile(file)
			if err != nil {
				return err
			}
			validated, err := plan.Validate(doc)
			if err != nil {
				return err
			}

			restore := pushPlanDevice(validated.Device)
			defer restore()

			runWithClient(cmd, "/plan/preview", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return runPlanPreview(ctx, a, c, deviceName, validated, cmd.OutOrStdout())
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to plan YAML file")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newPlanApplyCmd() *cobra.Command {
	var file string
	var confirm string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply plan steps (requires active safe session; fail-fast)",
		Long: `Apply a validated YAML plan to the target device.

Requires an existing safe session (ros session begin --safe) — apply does not
begin one. Successful mutates are journaled via the session change log.

Fail-fast: stops on the first step error. When the plan contains any delete
step, --confirm <exact-inventory-name> is required.

--dry-run is refused; use ros plan preview instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isDryRun(cmd) {
				return fmt.Errorf("plan apply refuses --dry-run; use: ros plan preview --file %s", file)
			}
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("--file is required")
			}
			doc, err := plan.LoadFile(file)
			if err != nil {
				return err
			}
			validated, err := plan.Validate(doc)
			if err != nil {
				return err
			}

			restore := pushPlanDevice(validated.Device)
			defer restore()

			runWithClient(cmd, "/plan/apply", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				return runPlanApply(ctx, a, c, cmd, deviceName, validated, confirm, cmd.OutOrStdout())
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to plan YAML file")
	_ = cmd.MarkFlagRequired("file")
	registerConfirmFlag(cmd, &confirm)
	attachDryRunFlag(cmd)
	return cmd
}

func newPlanRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "Alias of session rollback (inverse safe-session journal)",
		Long: `Alias of ros session rollback for the current device (-d / default).

Rollback applies the inverse journal recorded during a safe session
(including changes made by ros plan apply). Prefer session begin --safe
before apply so every mutate has a recoverable inverse.`,
		Run: runSessionRollback,
	}
}

// pushPlanDevice sets flagDevice from the plan when -d was omitted.
// Returns a restore func.
func pushPlanDevice(planDevice string) func() {
	if flagDevice != "" || strings.TrimSpace(planDevice) == "" {
		return func() {}
	}
	prev := flagDevice
	flagDevice = strings.TrimSpace(planDevice)
	return func() { flagDevice = prev }
}

// planStepPreview is one step in a preview envelope.
type planStepPreview struct {
	Index   int                    `json:"index"`
	Op      string                 `json:"op"`
	Path    string                 `json:"path"`
	PathIn  string                 `json:"path_input,omitempty"`
	ID      string                 `json:"id,omitempty"`
	Comment string                 `json:"comment,omitempty"`
	OK      bool                   `json:"ok"`
	Error   string                 `json:"error,omitempty"`
	Risks   []string               `json:"risks,omitempty"`
	Summary string                 `json:"summary,omitempty"`
	Command string                 `json:"command,omitempty"`
	Args    []string               `json:"args,omitempty"`
	Diff    *diff.Diff             `json:"diff,omitempty"`
	Pre     map[string]string      `json:"pre,omitempty"`
	Props   map[string]string      `json:"props,omitempty"`
}

type planPreviewEnvelope struct {
	Action  string            `json:"action"`
	Summary string            `json:"summary"`
	Device  string            `json:"device"`
	Steps   []planStepPreview `json:"steps"`
	OK      bool              `json:"ok"`
	Errors  int               `json:"errors"`
}

func runPlanPreview(ctx context.Context, a *App, c client.Client, deviceName string, v *plan.Validated, w io.Writer) error {
	hasSafe := false
	if a.Sessions != nil {
		if sess, err := a.Sessions.Active(deviceName); err == nil && sess != nil && sess.Safe {
			hasSafe = true
		}
	}
	dev := a.deviceConfig(deviceName)

	steps := make([]planStepPreview, 0, len(v.Steps))
	errCount := 0
	for _, s := range v.Steps {
		sp := previewOneStep(ctx, c, dev, s, hasSafe)
		if !sp.OK {
			errCount++
		}
		steps = append(steps, sp)
	}

	env := planPreviewEnvelope{
		Action:  "plan_preview",
		Summary: fmt.Sprintf("plan preview: %d step(s), %d error(s) on %s", len(steps), errCount, deviceName),
		Device:  deviceName,
		Steps:   steps,
		OK:      errCount == 0,
		Errors:  errCount,
	}

	if a.OutFormat == output.FormatJSON {
		meta := a.newMeta(deviceName, "/plan/preview", len(steps))
		meta.Action = "plan_preview"
		if err := a.renderRawJSON(w, env, meta); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "Plan preview on %q (%d step(s))\n", deviceName, len(steps))
		for _, sp := range steps {
			status := "ok"
			if !sp.OK {
				status = "ERROR"
			}
			fmt.Fprintf(w, "\n[%d] %s %s (%s)\n", sp.Index, sp.Op, sp.Path, status)
			if sp.Summary != "" {
				fmt.Fprintf(w, "  %s\n", sp.Summary)
			}
			if sp.Error != "" {
				fmt.Fprintf(w, "  error: %s\n", sp.Error)
			}
			if sp.Diff != nil {
				for _, ch := range propChangesFromDiff(*sp.Diff) {
					if ch.From == "" {
						fmt.Fprintf(w, "  %s: (unset) → %s\n", ch.Key, ch.To)
					} else {
						fmt.Fprintf(w, "  %s: %s → %s\n", ch.Key, ch.From, ch.To)
					}
				}
				for _, item := range sp.Diff.ToCreate {
					fmt.Fprintf(w, "  create %s\n", item.Key)
				}
				for _, item := range sp.Diff.ToRemove {
					label := item.ID
					if label == "" {
						label = item.Key
					}
					fmt.Fprintf(w, "  remove %s\n", label)
				}
			}
			for _, r := range sp.Risks {
				fmt.Fprintf(w, "  risk: %s\n", r)
			}
		}
		if errCount > 0 {
			fmt.Fprintf(w, "\nPreview failed: %d step(s) could not be previewed\n", errCount)
		} else {
			fmt.Fprintf(w, "\nPreview OK — use `ros plan apply --file …` with an active safe session to write\n")
		}
	}

	if errCount > 0 {
		return fmt.Errorf("plan preview: %d step(s) could not be previewed", errCount)
	}
	return nil
}

func previewOneStep(ctx context.Context, c client.Client, dev config.DeviceConfig, s plan.ValidatedStep, hasSafe bool) planStepPreview {
	sp := planStepPreview{
		Index:   s.Index,
		Op:      s.Op,
		Path:    s.Path,
		PathIn:  s.PathInput,
		ID:      s.ID,
		Comment: s.Comment,
		Props:   s.Props,
		Risks:   planStepRisks(dev, s, hasSafe),
	}

	id := s.ID
	if s.Comment != "" {
		resolved, err := resolveIDByComment(ctx, c, s.Path, s.Comment)
		if err != nil {
			sp.OK = false
			sp.Error = err.Error()
			sp.Summary = fmt.Sprintf("cannot preview %s %s", s.Op, s.Path)
			return sp
		}
		id = resolved
		sp.ID = id
	}

	apiArgs := plan.PropsToAPIArgs(s.Props, id)
	rosCmd := pathCommand(s.Path, plan.APIAction(s.Op))
	sp.Command = rosCmd
	sp.Args = apiArgs

	var pre map[string]string
	switch s.Op {
	case plan.OpSet, plan.OpDelete, plan.OpEnable, plan.OpDisable:
		pre, _ = fetchPreState(ctx, c, s.Path, id)
		sp.Pre = pre
	}

	spec := dryRunSpec{
		Verb:    s.Op,
		Path:    s.Path,
		Command: rosCmd,
		Args:    apiArgs,
		Pre:     pre,
	}
	sem := buildSemanticDiff(spec)
	sp.Diff = &sem
	sp.Summary = dryRunSummary(spec)
	sp.OK = true

	// Soft path-deny risk already recorded; also surface as error when builtin-denied
	// so preview exits non-zero for clearly impossible writes.
	if err := guardrails.CheckWritePath(rosCmd, dev.AllowedWritePaths, dev.DeniedWritePaths); err != nil {
		if _, ok := err.(*guardrails.ErrPathDenied); ok {
			// Keep as risk for allowlist misses; treat builtin deny as hard preview failure.
			if strings.Contains(err.Error(), "builtin deny") {
				sp.OK = false
				sp.Error = err.Error()
			}
		}
	}
	return sp
}

func planStepRisks(dev config.DeviceConfig, s plan.ValidatedStep, hasSafe bool) []string {
	var risks []string
	if s.IsDestructive() {
		risks = append(risks, "destructive delete (apply requires --confirm)")
	}
	if !hasSafe {
		risks = append(risks, "needs safe session for apply (ros session begin --safe)")
	}
	rosCmd := pathCommand(s.Path, plan.APIAction(s.Op))
	if err := guardrails.CheckWritePath(rosCmd, dev.AllowedWritePaths, dev.DeniedWritePaths); err != nil {
		risks = append(risks, "path deny: "+err.Error())
	}
	if s.NeedsCommentAsID() {
		risks = append(risks, "comment-as-id targeting (filter/mangle only)")
	}
	return risks
}

func runPlanApply(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName string, v *plan.Validated, confirm string, w io.Writer) error {
	sess, err := a.Sessions.Active(deviceName)
	if err != nil {
		return err
	}
	if sess == nil || !sess.Safe {
		return fmt.Errorf("plan apply requires an active safe session on %q; run: ros -d %s session begin --safe", deviceName, deviceName)
	}
	if v.HasDeletes() {
		if err := requireConfirmDevice(confirm, deviceName); err != nil {
			return err
		}
	}

	stepCmd := &cobra.Command{Use: "plan-apply-step"}
	// Ensure isDryRun is false even if parent somehow inherited the flag.
	attachDryRunFlag(stepCmd)
	_ = stepCmd.Flags().Set(dryRunFlag, "false")

	for _, s := range v.Steps {
		if err := applyOnePlanStep(ctx, a, c, stepCmd, deviceName, s); err != nil {
			return fmt.Errorf("plan apply failed at step %d (%s %s): %w", s.Index, s.Op, s.Path, err)
		}
		fmt.Fprintf(w, "applied step %d: %s %s\n", s.Index, s.Op, s.Path)
	}
	fmt.Fprintf(w, "Plan apply complete on %q (%d step(s)); commit or rollback the safe session\n", deviceName, len(v.Steps))
	return nil
}

func applyOnePlanStep(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName string, s plan.ValidatedStep) error {
	id := s.ID
	if s.Comment != "" {
		resolved, err := resolveMutateTargetID(ctx, c, s.Path, s.ID, s.Comment)
		if err != nil {
			return err
		}
		id = resolved
	}
	apiArgs := plan.PropsToAPIArgs(s.Props, id)
	rosCmd := pathCommand(s.Path, plan.APIAction(s.Op))

	// Discard per-step write outcomes; plan apply prints its own progress lines.
	// Write-audit + safe-session journaling still run inside apply*Mutation.
	stepCmd := cmd
	if stepCmd == nil {
		stepCmd = &cobra.Command{Use: "plan-apply-step"}
		attachDryRunFlag(stepCmd)
	}
	stepCmd.SetOut(io.Discard)
	stepCmd.SetErr(io.Discard)

	switch s.Op {
	case plan.OpCreate:
		return applyCreateMutation(ctx, a, c, stepCmd, deviceName, s.Path, rosCmd, apiArgs)
	case plan.OpSet:
		return applySetMutation(ctx, a, c, stepCmd, deviceName, s.Path, rosCmd, apiArgs)
	case plan.OpDelete:
		return applyDeleteMutation(ctx, a, c, stepCmd, deviceName, s.Path, rosCmd, apiArgs, id)
	case plan.OpEnable, plan.OpDisable:
		return applyEnableDisableMutation(ctx, a, c, stepCmd, deviceName, s.Path, rosCmd, apiArgs, id, s.Op)
	default:
		return fmt.Errorf("unknown op %q", s.Op)
	}
}
