package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
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

func runGenericGet(cmd *cobra.Command, args []string) {
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

func runGenericCreate(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		return
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return
	}
	rosCmd := pathCommand(path, "add")
	apiArgs := parseAPIArgs(rest)
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		if err := a.ensureWritable(rosCmd); err != nil {
			return err
		}
		result, err := c.Run(ctx, rosCmd, apiArgs...)
		if err != nil {
			return err
		}
		if err := recordCreateChange(a, deviceName, rosCmd, apiArgs, result); err != nil {
			return err
		}
		id := extractCreatedID(result)
		if id != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s (.id=%s) on %s\n", path, id, deviceName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s on %s\n", path, deviceName)
		}
		return nil
	})
}

func runGenericSet(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		return
	}
	path, rest, err := resolveResourcePath(args)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", err)
		return
	}
	rosCmd := pathCommand(path, "set")
	apiArgs := parseAPIArgs(rest)
	id := findIDArg(apiArgs)
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		if err := a.ensureWritable(rosCmd); err != nil {
			return err
		}
		var pre map[string]string
		if id != "" {
			pre, _ = fetchPreState(ctx, c, path, id)
		}
		_, err := c.Run(ctx, rosCmd, apiArgs...)
		if err != nil {
			return err
		}
		if inv := session.BuildSetInverse(rosCmd, id, pre, apiArgs); len(inv) > 0 {
			_ = a.recordSafeChange(deviceName, session.Change{
				Command:  rosCmd,
				Args:     apiArgs,
				Inverse:  inv,
				PreState: pre,
				Note:     "set " + id,
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated %s on %s\n", path, deviceName)
		return nil
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
	if id == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: delete requires .id=*N\n")
		return
	}
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		if err := a.ensureWritable(rosCmd); err != nil {
			return err
		}
		pre, _ := fetchPreState(ctx, c, path, id)
		_, err := c.Run(ctx, rosCmd, apiArgs...)
		if err != nil {
			return err
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
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s %s on %s\n", path, id, deviceName)
		return nil
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
		if id == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s requires .id=*N\n", action)
			return
		}
		runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
			if err := a.ensureWritable(rosCmd); err != nil {
				return err
			}
			_, err := c.Run(ctx, rosCmd, apiArgs...)
			if err != nil {
				return err
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
			fmt.Fprintf(cmd.OutOrStdout(), "%sd %s %s on %s\n", label, path, id, deviceName)
			return nil
		})
	}
}

func newEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <domain|/path> .id=*N",
		Short: "Enable a resource",
		Args:  cobra.MinimumNArgs(1),
		Run:   runGenericEnableDisable("enable"),
	}
}

func newDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <domain|/path> .id=*N",
		Short: "Disable a resource",
		Args:  cobra.MinimumNArgs(1),
		Run:   runGenericEnableDisable("disable"),
	}
}
