package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/audit"
	"github.com/nic0der-im/routeros-cli/internal/diff"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

const dryRunFlag = "dry-run"
const dryRunAction = "dry_run"

// attachDryRunFlag registers a persistent --dry-run so curated subcommands inherit it.
func attachDryRunFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(dryRunFlag, false, "preview the mutation without writing to RouterOS")
}

// isDryRun reports whether --dry-run is set on cmd or an ancestor.
func isDryRun(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flag(dryRunFlag); f != nil {
		return f.Value.String() == "true"
	}
	for c := cmd.Parent(); c != nil; c = c.Parent() {
		if f := c.PersistentFlags().Lookup(dryRunFlag); f != nil {
			return f.Value.String() == "true"
		}
	}
	return false
}

// dryRunSpec describes a mutation preview.
type dryRunSpec struct {
	Verb    string            // create|set|delete|enable|disable
	Path    string            // resource path, e.g. /ip/cloud
	Command string            // full API command, e.g. /ip/cloud/set
	Args    []string          // RouterOS API args (=key=value)
	Pre     map[string]string // optional pre-state for property diffs
}

// formatHumanAPIArgs turns =key=value into key=value for human summaries.
func formatHumanAPIArgs(apiArgs []string) string {
	apiArgs = redactAPIArgs(apiArgs)
	parts := make([]string, 0, len(apiArgs))
	for _, a := range apiArgs {
		a = strings.TrimSpace(a)
		if a == "" || strings.HasPrefix(a, "?") {
			continue
		}
		if strings.HasPrefix(a, "=") {
			parts = append(parts, strings.TrimPrefix(a, "="))
			continue
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// argsToPropMap converts =key=value API args into a property map.
func argsToPropMap(apiArgs []string) map[string]string {
	out := make(map[string]string, len(apiArgs))
	for _, a := range apiArgs {
		if !strings.HasPrefix(a, "=") {
			continue
		}
		rest := strings.TrimPrefix(a, "=")
		i := strings.IndexByte(rest, '=')
		if i <= 0 {
			continue
		}
		out[rest[:i]] = rest[i+1:]
	}
	return out
}

// propertyChanges builds before→after previews from set-style args and optional pre-state.
// Prefer buildSemanticDiff when emitting dry-run output.
func propertyChanges(pre map[string]string, apiArgs []string) []propChange {
	out := make([]propChange, 0, len(apiArgs))
	for _, a := range apiArgs {
		if !strings.HasPrefix(a, "=") {
			continue
		}
		rest := strings.TrimPrefix(a, "=")
		i := strings.IndexByte(rest, '=')
		if i <= 0 {
			continue
		}
		key, val := rest[:i], rest[i+1:]
		if key == ".id" || key == "id" {
			continue
		}
		from := ""
		if pre != nil {
			from = pre[key]
		}
		if from == val {
			continue
		}
		out = append(out, propChange{Key: key, From: from, To: val})
	}
	return out
}

type propChange struct {
	Key  string `json:"key"`
	From string `json:"from,omitempty"`
	To   string `json:"to"`
}

func dryRunSummary(spec dryRunSpec) string {
	human := formatHumanAPIArgs(spec.Args)
	s := fmt.Sprintf("dry-run: would %s %s", spec.Verb, spec.Path)
	if human != "" {
		s += " " + human
	}
	return s
}

// buildSemanticDiff uses internal/diff for set/create/delete/enable/disable previews.
func buildSemanticDiff(spec dryRunSpec) diff.Diff {
	desired := argsToPropMap(spec.Args)
	switch spec.Verb {
	case "set":
		return diff.DiffSet(spec.Path, spec.Pre, desired)
	case "create":
		return diff.DiffCreate(spec.Path, nil, desired)
	case "delete":
		id := findIDArg(spec.Args)
		var rows []map[string]string
		if spec.Pre != nil {
			rows = []map[string]string{spec.Pre}
		}
		return diff.DiffDelete(spec.Path, rows, id)
	case "enable":
		return diff.DiffSet(spec.Path, spec.Pre, map[string]string{"disabled": "false"})
	case "disable":
		return diff.DiffSet(spec.Path, spec.Pre, map[string]string{"disabled": "true"})
	default:
		return diff.Diff{}
	}
}

func redactSemanticKey(key string, secrets ...string) string {
	for _, secret := range secrets {
		key = strings.ReplaceAll(key, secret, output.RedactedPlaceholder)
	}
	parts := strings.Split(key, "|")
	for i, part := range parts {
		idx := strings.IndexByte(part, '=')
		if idx <= 0 {
			continue
		}
		parts[i] = part[:idx+1] + output.RedactValue(part[:idx], part[idx+1:])
	}
	return strings.Join(parts, "|")
}

func redactDiff(d diff.Diff, args ...[]string) diff.Diff {
	var secrets []string
	if len(args) > 0 {
		secrets = secretValuesFromAPIArgs(args[0])
	}
	redactItems := func(items []diff.Item) []diff.Item {
		if items == nil {
			return nil
		}
		out := make([]diff.Item, len(items))
		for i, item := range items {
			item.Key = redactSemanticKey(item.Key, secrets...)
			item.Before = output.RedactRecord(item.Before)
			item.After = output.RedactRecord(item.After)
			out[i] = item
		}
		return out
	}
	d.ToCreate = redactItems(d.ToCreate)
	d.ToUpdate = redactItems(d.ToUpdate)
	d.ToRemove = redactItems(d.ToRemove)
	return d
}

func propChangesFromDiff(d diff.Diff) []propChange {
	var out []propChange
	for _, item := range d.ToUpdate {
		for k, to := range item.After {
			from := ""
			if item.Before != nil {
				from = item.Before[k]
			}
			out = append(out, propChange{Key: k, From: from, To: to})
		}
	}
	return out
}

// emitDryRun prints a human summary or JSON envelope with action=dry_run.
// Does not journal session changes.
func (a *App) emitDryRun(w io.Writer, deviceName string, spec dryRunSpec) error {
	a.appendWriteAudit(deviceName, audit.Event{
		Verb:    spec.Verb,
		Action:  dryRunAction,
		Outcome: dryRunAction,
		Command: spec.Command,
		Path:    spec.Path,
		Args:    spec.Args,
		DryRun:  true,
	})

	safeSpec := spec
	safeSpec.Args = redactAPIArgs(spec.Args)
	safeSpec.Pre = output.RedactRecord(spec.Pre)
	summary := dryRunSummary(safeSpec)
	sem := redactDiff(buildSemanticDiff(safeSpec))
	changes := propChangesFromDiff(sem)
	if len(changes) == 0 && (spec.Verb == "set" || spec.Verb == "enable" || spec.Verb == "disable") {
		changes = propertyChanges(safeSpec.Pre, safeSpec.Args)
	}
	displayCmd := spec.Command
	if human := formatHumanAPIArgs(safeSpec.Args); human != "" {
		displayCmd += " " + human
	}

	if a.OutFormat == output.FormatJSON {
		payload := map[string]interface{}{
			"action":  dryRunAction,
			"summary": summary,
			"verb":    spec.Verb,
			"path":    spec.Path,
			"command": spec.Command,
			"args":    safeSpec.Args,
			"diff":    sem,
		}
		if len(changes) > 0 {
			payload["changes"] = changes
		}
		if safeSpec.Pre != nil {
			payload["pre"] = safeSpec.Pre
		}
		meta := a.newMeta(deviceName, displayCmd, len(sem.ToCreate)+len(sem.ToUpdate)+len(sem.ToRemove))
		meta.Action = dryRunAction
		return a.renderRawJSON(w, payload, meta)
	}

	fmt.Fprintln(w, summary)
	for _, ch := range changes {
		if ch.From == "" {
			fmt.Fprintf(w, "  %s: (unset) → %s\n", ch.Key, ch.To)
			continue
		}
		fmt.Fprintf(w, "  %s: %s → %s\n", ch.Key, ch.From, ch.To)
	}
	for _, item := range sem.ToCreate {
		fmt.Fprintf(w, "  create %s\n", item.Key)
	}
	for _, item := range sem.ToRemove {
		label := item.ID
		if label == "" {
			label = item.Key
		}
		fmt.Fprintf(w, "  remove %s\n", label)
	}
	for _, wmsg := range sem.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", wmsg)
	}
	return nil
}
