package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/audit"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/diff"
	"github.com/nic0der-im/routeros-cli/internal/output"
)

// Stable JSON write-outcome action tokens for agents.
const (
	ActionCreated       = "created"
	ActionUpdated       = "updated"
	ActionRemoved       = "removed"
	ActionAlreadyExists = "already_exists"
	ActionNoChange      = "no_change"
	// ActionDryRun aliases dryRunAction for outcome helpers.
	ActionDryRun = dryRunAction
)

// writeOutcomeSpec describes a successful (or idempotent) mutation result.
type writeOutcomeSpec struct {
	Action  string // created|updated|removed|already_exists|no_change|dry_run
	Verb    string
	Path    string
	Command string
	Args    []string
	ID      string
	Summary string
	Diff    *diff.Diff
	Pre     map[string]string
}

func outcomeSummary(action, verb, path, id string) string {
	switch action {
	case ActionAlreadyExists:
		return fmt.Sprintf("already exists: %s", path)
	case ActionNoChange:
		return fmt.Sprintf("no change: %s", path)
	case ActionCreated:
		if id != "" {
			return fmt.Sprintf("created %s (.id=%s)", path, id)
		}
		return fmt.Sprintf("created %s", path)
	case ActionUpdated:
		if id != "" {
			return fmt.Sprintf("updated %s (.id=%s)", path, id)
		}
		return fmt.Sprintf("updated %s", path)
	case ActionRemoved:
		if id != "" {
			return fmt.Sprintf("removed %s %s", path, id)
		}
		return fmt.Sprintf("removed %s", path)
	case ActionDryRun:
		return fmt.Sprintf("dry-run: would %s %s", verb, path)
	default:
		return fmt.Sprintf("%s %s", action, path)
	}
}

func diffHasWarning(d diff.Diff, want string) bool {
	for _, w := range d.Warnings {
		if w == want {
			return true
		}
	}
	return false
}

// classifyCreateOutcome uses DiffCreate against existing rows.
// Returns already_exists when the semantic key is present; otherwise created
// (caller still performs the write when action is created).
func classifyCreateOutcome(path string, existing []map[string]string, desired map[string]string) (action string, d diff.Diff, existingID string) {
	d = diff.DiffCreate(path, existing, desired)
	if d.Empty() && diffHasWarning(d, diff.WarnAlreadyExists) {
		return ActionAlreadyExists, d, findMatchingRowID(path, existing, desired)
	}
	return ActionCreated, d, ""
}

// classifySetOutcome uses DiffSet. When pre is available and no properties
// change, returns no_change; otherwise updated.
func classifySetOutcome(path string, pre, desired map[string]string) (action string, d diff.Diff) {
	d = diff.DiffSet(path, pre, desired)
	if pre != nil && d.Empty() {
		return ActionNoChange, d
	}
	return ActionUpdated, d
}

func findMatchingRowID(path string, existing []map[string]string, desired map[string]string) string {
	want := diff.SemanticKey(path, desired)
	for _, row := range existing {
		if diff.SemanticKey(path, row) == want {
			if id := row[".id"]; id != "" {
				return id
			}
			for k, v := range row {
				if strings.EqualFold(k, ".id") && v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// resolveIDBySemanticKey prints the path and returns .id for the row matching desired props.
func resolveIDBySemanticKey(ctx context.Context, c client.Client, path string, desired map[string]string) (string, error) {
	rows, err := fetchAllRows(ctx, c, path)
	if err != nil {
		return "", err
	}
	id := findMatchingRowID(path, rows, desired)
	if id == "" {
		return "", apperr.New(apperr.KindNotFound, fmt.Sprintf("%s not found (%s)", path, diff.SemanticKey(path, desired)))
	}
	return id, nil
}

// fetchAllRows prints a resource table for idempotency checks.
func fetchAllRows(ctx context.Context, c client.Client, basePath string) ([]map[string]string, error) {
	printCmd := normalizePath(basePath) + "/print"
	result, err := c.Run(ctx, printCmd)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Sentences) == 0 {
		return nil, nil
	}
	out := make([]map[string]string, len(result.Sentences))
	for i, s := range result.Sentences {
		row := make(map[string]string, len(s))
		for k, v := range s {
			row[k] = v
		}
		out[i] = row
	}
	return out, nil
}

// emitWriteOutcome prints a human summary or JSON envelope with meta.action.
func (a *App) emitWriteOutcome(w io.Writer, deviceName string, spec writeOutcomeSpec) error {
	a.appendWriteAudit(deviceName, audit.Event{
		Verb:    spec.Verb,
		Action:  spec.Action,
		Outcome: spec.Action,
		Command: spec.Command,
		Path:    spec.Path,
		Args:    spec.Args,
		DryRun:  spec.Action == ActionDryRun,
	})

	safeArgs := redactAPIArgs(spec.Args)
	safePre := output.RedactRecord(spec.Pre)
	safeDiff := (*diff.Diff)(nil)
	if spec.Diff != nil {
		d := redactDiff(*spec.Diff, spec.Args)
		safeDiff = &d
	}
	summary := spec.Summary
	if summary == "" {
		summary = outcomeSummary(spec.Action, spec.Verb, spec.Path, spec.ID)
	}
	displayCmd := spec.Command
	if human := formatHumanAPIArgs(safeArgs); human != "" {
		displayCmd += " " + human
	}

	if a.OutFormat == output.FormatJSON {
		payload := map[string]interface{}{
			"action":  spec.Action,
			"summary": summary,
			"verb":    spec.Verb,
			"path":    spec.Path,
			"command": spec.Command,
			"args":    safeArgs,
		}
		if spec.ID != "" {
			payload["id"] = spec.ID
		}
		if safeDiff != nil {
			payload["diff"] = *safeDiff
		}
		if safePre != nil {
			payload["pre"] = safePre
		}
		count := 1
		if spec.Action == ActionNoChange || spec.Action == ActionAlreadyExists {
			count = 0
		}
		if safeDiff != nil {
			count = len(safeDiff.ToCreate) + len(safeDiff.ToUpdate) + len(safeDiff.ToRemove)
		}
		meta := a.newMeta(deviceName, displayCmd, count)
		meta.Action = spec.Action
		return a.renderRawJSON(w, payload, meta)
	}

	fmt.Fprintln(w, summary)
	return nil
}
