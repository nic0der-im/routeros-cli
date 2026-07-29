package cmd

import (
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/skills"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install ros agent skills for Cursor, Codex, Claude, OpenCode",
		Long: `Manage bundled LLM skills that teach agents how to use ros safely.

Recommended defaults:
  ros skills install --agent all --scope user

Parameters:
  --agent   cursor|codex|claude|opencode|all   (default: all)
  --scope   user|project                       (default: user)
  --force   overwrite existing skill dirs
  --pack    ros|ros-safe-apply (repeatable; default: both)

Install locations (user scope):
  cursor    ~/.cursor/skills/<pack>
  codex     ~/.codex/skills/<pack>
  claude    ~/.claude/skills/<pack>
  opencode  ~/.agents/skills/<pack>

Project scope uses .cursor/skills, .agents/skills, or .claude/skills under cwd.`,
	}
	cmd.AddCommand(
		newSkillsListCmd(),
		newSkillsPathCmd(),
		newSkillsInstallCmd(),
		newSkillsUninstallCmd(),
	)
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List bundled skill packs",
		Run: func(cmd *cobra.Command, args []string) {
			for _, p := range skills.ListPacks() {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
		},
	}
}

func newSkillsPathCmd() *cobra.Command {
	var (
		agent string
		scope string
	)
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show install directories for agent/scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := skills.ParseAgent(agent)
			if err != nil {
				return err
			}
			s, err := skills.ParseScope(scope)
			if err != nil {
				return err
			}
			for _, ag := range skills.Agents(a) {
				dir, err := skills.DirFor(ag, s, skills.DefaultProjectRoot())
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", ag, dir)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "all", "cursor|codex|claude|opencode|all")
	cmd.Flags().StringVar(&scope, "scope", "user", "user|project")
	return cmd
}

func newSkillsInstallCmd() *cobra.Command {
	var (
		agent string
		scope string
		force bool
		packs []string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install bundled ros skills into agent skill directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := skills.ParseAgent(agent)
			if err != nil {
				return err
			}
			s, err := skills.ParseScope(scope)
			if err != nil {
				return err
			}
			results, err := skills.Install(skills.InstallOptions{
				Agent:   a,
				Scope:   s,
				Project: skills.DefaultProjectRoot(),
				Force:   force,
				Packs:   packs,
			})
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-16s %-8s %s\n", r.Agent, r.Pack, r.Status, r.Target)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nSkills installed. Reload the agent session if skills are cached.")
			fmt.Fprintln(cmd.OutOrStdout(), "Try: \"Audit device router-edge with ros in read-only mode\"")
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "all", "cursor|codex|claude|opencode|all")
	cmd.Flags().StringVar(&scope, "scope", "user", "user|project")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing skill packs")
	cmd.Flags().StringSliceVar(&packs, "pack", nil, "pack name (default: all bundled packs)")
	return cmd
}

func newSkillsUninstallCmd() *cobra.Command {
	var (
		agent string
		scope string
		packs []string
	)
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove installed ros skill packs",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := skills.ParseAgent(agent)
			if err != nil {
				return err
			}
			s, err := skills.ParseScope(scope)
			if err != nil {
				return err
			}
			results, err := skills.Uninstall(skills.InstallOptions{
				Agent:   a,
				Scope:   s,
				Project: skills.DefaultProjectRoot(),
				Packs:   packs,
			})
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-16s %-8s %s\n", r.Agent, r.Pack, r.Status, r.Target)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "all", "cursor|codex|claude|opencode|all")
	cmd.Flags().StringVar(&scope, "scope", "user", "user|project")
	cmd.Flags().StringSliceVar(&packs, "pack", nil, "pack name (default: all bundled packs)")
	return cmd
}
