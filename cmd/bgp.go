package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

func newBGPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bgp",
		Short: "BGP helpers (sessions)",
	}
	cmd.AddCommand(newBGPSessionsCmd())
	return cmd
}

func newBGPSessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List BGP sessions",
		Long: `List BGP sessions (/routing/bgp/session/print).

Shows name, remote, state, and established when present.

Also available as:
  ros get bgp sessions
  ros get bgp/session  (domain alias)`,
		Run: func(cmd *cobra.Command, args []string) {
			runBGPSessions(cmd)
		},
	}
}

func runBGPSessions(cmd *cobra.Command) {
	rosCmd := "/routing/bgp/session/print"
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		result, err := c.Run(ctx, rosCmd)
		if err != nil {
			return fmt.Errorf("listing BGP sessions: %w", err)
		}
		return a.render(cmd.OutOrStdout(), buildBGPSessionView(result.Sentences), deviceName, rosCmd)
	})
}

type bgpSessionRow struct {
	ID          string
	Name        string
	Remote      string
	State       string
	Established string
}

type bgpSessionView []bgpSessionRow

func (v bgpSessionView) TableHeaders() []string {
	return []string{"Name", "Remote", "State", "Established"}
}

func (v bgpSessionView) TableRows() [][]string {
	rows := make([][]string, len(v))
	for i, r := range v {
		rows[i] = []string{r.Name, r.Remote, r.State, r.Established}
	}
	return rows
}

func (v bgpSessionView) RawRecords() []map[string]string {
	out := make([]map[string]string, len(v))
	for i, r := range v {
		out[i] = map[string]string{
			".id":         r.ID,
			"name":        r.Name,
			"remote":      r.Remote,
			"state":       r.State,
			"established": r.Established,
		}
	}
	return out
}

func buildBGPSessionView(sentences []map[string]string) bgpSessionView {
	out := make(bgpSessionView, 0, len(sentences))
	for _, s := range sentences {
		out = append(out, bgpSessionRow{
			ID:          firstField(s, ".id"),
			Name:        firstField(s, "name"),
			Remote:      firstField(s, "remote.address", "remote", "remote-address"),
			State:       firstField(s, "state", "messages.state", "session-state"),
			Established: firstField(s, "established", "uptime", "messages.established"),
		})
	}
	return out
}
