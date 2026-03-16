package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nic0der-im/routeros-cli/pkg/schema"
	"github.com/spf13/cobra"
)

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [name]",
		Short: "Print JSON Schema for command output types",
		Long: `Print the JSON Schema definition for a command's output data type.
This is useful for AI agents to understand the structure of JSON output.

Without arguments, lists all available schema names.
With a name argument, prints the full JSON Schema for that type.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				names := schema.List()
				fmt.Fprintln(cmd.OutOrStdout(), "Available schemas:")
				for _, name := range names {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nUse: routeros-cli schema <name>\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Special: routeros-cli schema envelope | error\n")
				return nil
			}

			name := strings.ToLower(args[0])

			var s schema.Schema
			var found bool

			switch name {
			case "envelope":
				s = schema.EnvelopeSchema()
				found = true
			case "error":
				s = schema.ErrorSchema()
				found = true
			default:
				s, found = schema.Get(name)
			}

			if !found {
				fmt.Fprintf(os.Stderr, "Unknown schema: %q\nRun 'routeros-cli schema' to list available schemas.\n", name)
				os.Exit(1)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(s)
		},
	}

	return cmd
}
