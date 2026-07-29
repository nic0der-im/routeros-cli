package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

func newOSPFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ospf",
		Short: "OSPF helpers (neighbors)",
	}
	cmd.AddCommand(newOSPFNeighborsCmd())
	return cmd
}

func newOSPFNeighborsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "neighbors",
		Short: "List OSPF neighbors",
		Long: `List OSPF neighbors (/routing/ospf/neighbor/print).

Shows router-id, address, state, interface, and area when present.

Also available as:
  ros get ospf neighbors
  ros get ospf/neighbor  (domain alias)`,
		Run: func(cmd *cobra.Command, args []string) {
			runOSPFNeighbors(cmd)
		},
	}
}

func runOSPFNeighbors(cmd *cobra.Command) {
	rosCmd := "/routing/ospf/neighbor/print"
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		result, err := c.Run(ctx, rosCmd)
		if err != nil {
			return fmt.Errorf("listing OSPF neighbors: %w", err)
		}
		return a.render(cmd.OutOrStdout(), buildOSPFNeighborView(result.Sentences), deviceName, rosCmd)
	})
}

type ospfNeighborRow struct {
	ID       string
	RouterID string
	Address  string
	State    string
	Iface    string
	Area     string
}

type ospfNeighborView []ospfNeighborRow

func (v ospfNeighborView) TableHeaders() []string {
	return []string{"Router-ID", "Address", "State", "Interface", "Area"}
}

func (v ospfNeighborView) TableRows() [][]string {
	rows := make([][]string, len(v))
	for i, r := range v {
		rows[i] = []string{r.RouterID, r.Address, r.State, r.Iface, r.Area}
	}
	return rows
}

func (v ospfNeighborView) RawRecords() []map[string]string {
	out := make([]map[string]string, len(v))
	for i, r := range v {
		out[i] = map[string]string{
			".id":       r.ID,
			"router-id": r.RouterID,
			"address":   r.Address,
			"state":     r.State,
			"interface": r.Iface,
			"area":      r.Area,
		}
	}
	return out
}

func buildOSPFNeighborView(sentences []map[string]string) ospfNeighborView {
	out := make(ospfNeighborView, 0, len(sentences))
	for _, s := range sentences {
		out = append(out, ospfNeighborRow{
			ID:       firstField(s, ".id"),
			RouterID: firstField(s, "router-id", "router-ID"),
			Address:  firstField(s, "address", "neighbor-id"),
			State:    firstField(s, "state"),
			Iface:    firstField(s, "interface", "iface"),
			Area:     firstField(s, "area"),
		})
	}
	return out
}
