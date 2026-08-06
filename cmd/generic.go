package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/diff"
	"github.com/nic0der-im/routeros-cli/internal/domains"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/rosapi"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

// resolveResourcePath accepts either a raw /path or friendly domain parts.
func resolveResourcePath(args []string) (path string, rest []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("resource path or domain required")
	}
	if strings.HasPrefix(args[0], "/") {
		return normalizePath(args[0]), args[1:], nil
	}

	for n := len(args); n >= 1; n-- {
		friendly := domains.JoinFriendly(args[:n])
		if p, ok := domains.Resolve(friendly); ok {
			return p, args[n:], nil
		}
	}
	return "", nil, fmt.Errorf("unknown resource %q (use /path or a known domain; see: ros domains)", args[0])
}

func renderGenericResult(a *App, w io.Writer, result *client.Result, deviceName, cmdDisplay string) error {
	if len(result.Sentences) == 0 {
		if a.OutFormat == output.FormatJSON {
			return a.render(w, &rosapi.GenericResults{}, deviceName, cmdDisplay)
		}
		fmt.Fprintln(w, "OK (no data returned)")
		return nil
	}
	items := make([]rosapi.GenericResult, len(result.Sentences))
	for i, s := range result.Sentences {
		items[i] = rosapi.GenericResult{Fields: s}
	}
	gr := &rosapi.GenericResults{Items: items}
	keys := make([]string, 0, len(result.Sentences[0]))
	for k := range result.Sentences[0] {
		keys = append(keys, k)
	}
	gr.SetKeyOrder(keys)
	return a.render(w, gr, deviceName, cmdDisplay)
}

func newDomainsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "domains",
		Short: "List curated domain aliases for get/create/set/delete",
		Run: func(cmd *cobra.Command, args []string) {
			for _, k := range domains.List() {
				fmt.Fprintf(cmd.OutOrStdout(), "%-32s → %s\n", k, domains.Alias[k])
			}
		},
	}
}

func runGenericGet(cmd *cobra.Command, args []string, where []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		return
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return
	}
	rosCmd := pathCommand(path, "print")
	if path == "/ping" {
		rosCmd = "/ping"
	}
	apiArgs := parseAPIArgs(rest)
	filters, err := parseWhereFilters(where)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return
	}
	apiArgs = append(apiArgs, filters...)
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		result, err := c.Run(ctx, rosCmd, apiArgs...)
		if err != nil {
			return err
		}
		display := rosCmd
		if len(apiArgs) > 0 {
			display += " " + strings.Join(apiArgs, " ")
		}
		return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, display)
	})
}

func runGenericCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		_ = cmd.Help()
		return nil
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return nil
	}
	rosCmd := pathCommand(path, "add")
	apiArgs := parseAPIArgs(rest)
	passwordFlag := cmd.Flag(passwordStdinFlag)
	password, err := readMutationPassword("create", path, apiArgs, passwordFlag != nil && passwordFlag.Value.String() == "true")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return err
	}
	runWithClientFn(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		return applyCreateMutationWithPassword(ctx, a, c, cmd, deviceName, path, rosCmd, apiArgs, password)
	})
	return nil
}

// applyCreateMutation creates a resource, or emits a dry-run preview (no write/journal).
// Idempotent: DiffCreate already_exists → action=already_exists without writing.
func applyCreateMutation(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string) error {
	return applyCreateMutationWithPassword(ctx, a, c, cmd, deviceName, path, rosCmd, apiArgs, "")
}

func applyCreateMutationWithPassword(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string, password string) error {
	if password != "" && normalizePath(path) != "/user" {
		return fmt.Errorf("--password-stdin is only supported for generic create user mutations")
	}
	executionArgs := appendPasswordArg(apiArgs, password)
	displayArgs := redactAPIArgs(executionArgs)
	if isDryRun(cmd) {
		return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
			Verb: "create", Path: path, Command: rosCmd, Args: displayArgs,
		})
	}
	if err := a.ensureWritable(deviceName, rosCmd); err != nil {
		return err
	}

	desired := argsToPropMap(apiArgs)
	var createDiff *diff.Diff
	if rows, err := fetchAllRows(ctx, c, path); err == nil {
		action, d, existingID := classifyCreateOutcome(path, rows, desired)
		createDiff = &d
		if action == ActionAlreadyExists {
			return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
				Action:  ActionAlreadyExists,
				Verb:    "create",
				Path:    path,
				Command: rosCmd,
				Args:    displayArgs,
				ID:      existingID,
				Summary: fmt.Sprintf("Already exists: %s on %s", path, deviceName),
				Diff:    &d,
			})
		}
	}

	result, err := c.Run(ctx, rosCmd, executionArgs...)
	if err != nil {
		return apperr.MaybeAmbiguousWrite(redactErrorWithAPIArgs(err, executionArgs))
	}
	if err := recordCreateChange(a, deviceName, rosCmd, executionArgs, result); err != nil {
		return err
	}
	id := extractCreatedID(result)
	summary := fmt.Sprintf("Created %s on %s", path, deviceName)
	if id != "" {
		summary = fmt.Sprintf("Created %s (.id=%s) on %s", path, id, deviceName)
	}
	return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
		Action:  ActionCreated,
		Verb:    "create",
		Path:    path,
		Command: rosCmd,
		Args:    displayArgs,
		ID:      id,
		Summary: summary,
		Diff:    createDiff,
	})
}

func runGenericSet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		_ = cmd.Help()
		return nil
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return nil
	}
	rosCmd := pathCommand(path, "set")
	apiArgs := parseAPIArgs(rest)
	apiArgs, tip, err := normalizeCloudDDNSArgs(path, apiArgs)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return nil
	}
	if tip != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), tip)
	}
	comment := getCommentTarget(cmd)
	if comment != "" {
		if findIDArg(apiArgs) != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), "Error: specify either .id or --comment, not both")
			return nil
		}
		if !supportsCommentAsID(path) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: --comment targeting is only supported for firewall/filter and firewall/mangle, not %s\n", path)
			return nil
		}
	}
	passwordFlag := cmd.Flag(passwordStdinFlag)
	password, err := readMutationPassword("set", path, apiArgs, passwordFlag != nil && passwordFlag.Value.String() == "true")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return err
	}
	runWithClientFn(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		argsForMut := apiArgs
		if comment != "" {
			resolved, err := resolveIDByComment(ctx, c, path, comment)
			if err != nil {
				return err
			}
			argsForMut = ensureIDArg(apiArgs, resolved)
		}
		return applySetMutationWithPassword(ctx, a, c, cmd, deviceName, path, rosCmd, argsForMut, password)
	})
	return nil
}

// applySetMutation updates a resource, or emits a dry-run preview (no write/journal).
// DiffSet with no property changes → action=no_change without writing.
func applySetMutation(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string) error {
	return applySetMutationWithPassword(ctx, a, c, cmd, deviceName, path, rosCmd, apiArgs, "")
}

func applySetMutationWithPassword(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string, password string) error {
	if password != "" && normalizePath(path) != "/user" {
		return fmt.Errorf("--password-stdin is only supported for generic set user mutations")
	}
	executionArgs := appendPasswordArg(apiArgs, password)
	displayArgs := redactAPIArgs(executionArgs)
	id := findIDArg(apiArgs)
	// Optional read for property diffs / journaling context.
	pre, _ := fetchPreState(ctx, c, path, id)
	if isDryRun(cmd) {
		return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
			Verb: "set", Path: path, Command: rosCmd, Args: displayArgs, Pre: pre,
		})
	}
	if err := a.ensureWritable(deviceName, rosCmd); err != nil {
		return err
	}

	desired := argsToPropMap(executionArgs)
	action, d := classifySetOutcome(path, pre, desired)
	if action == ActionNoChange {
		return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
			Action:  ActionNoChange,
			Verb:    "set",
			Path:    path,
			Command: rosCmd,
			Args:    displayArgs,
			ID:      id,
			Summary: fmt.Sprintf("No change: %s on %s", path, deviceName),
			Diff:    &d,
			Pre:     pre,
		})
	}

	_, err := c.Run(ctx, rosCmd, executionArgs...)
	if err != nil {
		return apperr.MaybeAmbiguousWrite(redactErrorWithAPIArgs(err, executionArgs))
	}
	if inv := session.BuildSetInverse(rosCmd, id, pre, executionArgs); len(inv) > 0 || hasPasswordArg(executionArgs) {
		note := "set singleton"
		if id != "" {
			note = "set " + id
		}
		if hasPasswordArg(executionArgs) {
			note += " (password not auto-rolled back)"
		}
		_ = a.recordSafeChange(deviceName, session.Change{
			Command:  rosCmd,
			Args:     executionArgs,
			Inverse:  inv,
			PreState: pre,
			Note:     note,
		})
	}
	return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
		Action:  ActionUpdated,
		Verb:    "set",
		Path:    path,
		Command: rosCmd,
		Args:    displayArgs,
		ID:      id,
		Summary: fmt.Sprintf("Updated %s on %s", path, deviceName),
		Diff:    &d,
		Pre:     pre,
	})
}

func runGenericDelete(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		return
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return
	}
	rosCmd := pathCommand(path, "remove")
	apiArgs := parseAPIArgs(rest)
	id := findIDArg(apiArgs)
	comment := getCommentTarget(cmd)
	if id != "" && comment != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error: specify either .id or --comment, not both")
		return
	}
	if id == "" && comment == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error: delete requires .id=*N (or --comment for firewall/filter|mangle)")
		return
	}
	if comment != "" && !supportsCommentAsID(path) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: --comment targeting is only supported for firewall/filter and firewall/mangle, not %s\n", path)
		return
	}
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		resolved := id
		argsForMut := apiArgs
		if comment != "" {
			var err error
			resolved, err = resolveIDByComment(ctx, c, path, comment)
			if err != nil {
				return err
			}
			argsForMut = ensureIDArg(apiArgs, resolved)
		}
		return applyDeleteMutation(ctx, a, c, cmd, deviceName, path, rosCmd, argsForMut, resolved)
	})
}

// applyDeleteMutation removes a resource, or emits a dry-run preview (no write/journal).
func applyDeleteMutation(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string, id string) error {
	pre, preErr := fetchPreState(ctx, c, path, id)
	if isDryRun(cmd) {
		return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
			Verb: "delete", Path: path, Command: rosCmd, Args: apiArgs, Pre: pre,
		})
	}
	if err := a.ensureWritable(deviceName, rosCmd); err != nil {
		return err
	}
	if pre == nil && preErr == nil {
		return apperr.New(apperr.KindNotFound, fmt.Sprintf("%s %s not found", path, id))
	}
	_, err := c.Run(ctx, rosCmd, apiArgs...)
	if err != nil {
		return apperr.MaybeAmbiguousWrite(err)
	}
	if inv := session.BuildRemoveInverse(rosCmd, pre); len(inv) > 0 {
		_ = a.recordSafeChange(deviceName, session.Change{
			Command:  rosCmd,
			Args:     apiArgs,
			Inverse:  inv,
			PreState: pre,
			Note:     "delete " + id,
		})
	}
	return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
		Action:  ActionRemoved,
		Verb:    "delete",
		Path:    path,
		Command: rosCmd,
		Args:    apiArgs,
		ID:      id,
		Summary: fmt.Sprintf("Deleted %s %s on %s", path, id, deviceName),
		Pre:     pre,
	})
}

func runGenericEnableDisable(action string) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		path, rest, err := resolveResourcePath(args)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
			return
		}
		rosCmd := pathCommand(path, action)
		apiArgs := parseAPIArgs(rest)
		id := findIDArg(apiArgs)
		comment := getCommentTarget(cmd)
		if id != "" && comment != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), "Error: specify either .id or --comment, not both")
			return
		}
		if id == "" && comment == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s requires .id=*N (or --comment for firewall/filter|mangle)\n", action)
			return
		}
		if comment != "" && !supportsCommentAsID(path) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: --comment targeting is only supported for firewall/filter and firewall/mangle, not %s\n", path)
			return
		}
		runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
			resolved := id
			argsForMut := apiArgs
			if comment != "" {
				var err error
				resolved, err = resolveIDByComment(ctx, c, path, comment)
				if err != nil {
					return err
				}
				argsForMut = ensureIDArg(apiArgs, resolved)
			}
			return applyEnableDisableMutation(ctx, a, c, cmd, deviceName, path, rosCmd, argsForMut, resolved, action)
		})
	}
}

// applyEnableDisableMutation enables/disables a resource, or emits a dry-run preview.
// When pre-state shows disabled already matches the target, returns no_change.
func applyEnableDisableMutation(ctx context.Context, a *App, c client.Client, cmd *cobra.Command, deviceName, path, rosCmd string, apiArgs []string, id, action string) error {
	pre, _ := fetchPreState(ctx, c, path, id)
	if isDryRun(cmd) {
		return a.emitDryRun(cmd.OutOrStdout(), deviceName, dryRunSpec{
			Verb: action, Path: path, Command: rosCmd, Args: apiArgs, Pre: pre,
		})
	}
	if err := a.ensureWritable(deviceName, rosCmd); err != nil {
		return err
	}

	desiredDisabled := "false"
	if action == "disable" {
		desiredDisabled = "true"
	}
	desired := map[string]string{"disabled": desiredDisabled}
	setAction, d := classifySetOutcome(path, pre, desired)
	if setAction == ActionNoChange {
		return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
			Action:  ActionNoChange,
			Verb:    action,
			Path:    path,
			Command: rosCmd,
			Args:    apiArgs,
			ID:      id,
			Summary: fmt.Sprintf("No change: %s %s already %sd on %s", path, id, action, deviceName),
			Diff:    &d,
			Pre:     pre,
		})
	}

	_, err := c.Run(ctx, rosCmd, apiArgs...)
	if err != nil {
		return apperr.MaybeAmbiguousWrite(err)
	}
	invAction := "disable"
	if action == "disable" {
		invAction = "enable"
	}
	_ = recordIDChange(a, deviceName, rosCmd, apiArgs,
		[]string{pathCommand(path, invAction), "=.id=" + id},
		action+" "+id)
	label := action
	if len(label) > 0 {
		label = strings.ToUpper(label[:1]) + label[1:]
	}
	return a.emitWriteOutcome(cmd.OutOrStdout(), deviceName, writeOutcomeSpec{
		Action:  ActionUpdated,
		Verb:    action,
		Path:    path,
		Command: rosCmd,
		Args:    apiArgs,
		ID:      id,
		Summary: fmt.Sprintf("%sd %s %s on %s", label, path, id, deviceName),
		Diff:    &d,
		Pre:     pre,
	})
}

func newEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <domain|/path> .id=*N",
		Short: "Enable a resource",
		Long: `Enable a resource by .id, or by --comment for firewall/filter|mangle.

  ros enable interface .id=*E
  ros enable firewall/filter --comment allow-web
  ros enable firewall/mangle --comment mark-conn`,
		Args: cobra.MinimumNArgs(1),
		Run:  runGenericEnableDisable("enable"),
	}
	attachDryRunFlag(cmd)
	attachCommentTargetFlag(cmd)
	return cmd
}

func newDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <domain|/path> .id=*N",
		Short: "Disable a resource",
		Long: `Disable a resource by .id, or by --comment for firewall/filter|mangle.

  ros disable interface/wireguard .id=*E
  ros disable firewall/filter --comment allow-web
  ros disable firewall/mangle --comment mark-conn`,
		Args: cobra.MinimumNArgs(1),
		Run:  runGenericEnableDisable("disable"),
	}
	attachDryRunFlag(cmd)
	attachCommentTargetFlag(cmd)
	return cmd
}
