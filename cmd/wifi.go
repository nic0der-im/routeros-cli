package cmd

import (
	"context"
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

func newWifiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "WiFi helpers (clients / registration)",
	}
	cmd.AddCommand(newWifiClientsCmd())
	return cmd
}

func newWifiClientsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "clients",
		Aliases: []string{"registration"},
		Short:   "List WiFi client registrations",
		Long: `List WiFi registrations (/interface/wifi/registration/print).

Shows interface, MAC, signal, and uptime when present.

Also available as:
  ros get wifi clients
  ros get wifi registration
  ros get wifi/registration  (domain alias)`,
		Run: func(cmd *cobra.Command, args []string) {
			runWifiClients(cmd)
		},
	}
}

func runWifiClients(cmd *cobra.Command) {
	rosCmd := "/interface/wifi/registration/print"
	runWithClient(cmd, rosCmd, func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		result, err := c.Run(ctx, rosCmd)
		if err != nil {
			return fmt.Errorf("listing WiFi registrations: %w", err)
		}
		return a.render(cmd.OutOrStdout(), buildWifiClientView(result.Sentences), deviceName, rosCmd)
	})
}

type wifiClientRow struct {
	ID        string
	Interface string
	MAC       string
	Signal    string
	Uptime    string
	SSID      string
}

type wifiClientView []wifiClientRow

func (v wifiClientView) TableHeaders() []string {
	return []string{"Interface", "MAC", "Signal", "Uptime", "SSID"}
}

func (v wifiClientView) TableRows() [][]string {
	rows := make([][]string, len(v))
	for i, r := range v {
		rows[i] = []string{r.Interface, r.MAC, r.Signal, r.Uptime, r.SSID}
	}
	return rows
}

func (v wifiClientView) RawRecords() []map[string]string {
	out := make([]map[string]string, len(v))
	for i, r := range v {
		out[i] = map[string]string{
			".id":         r.ID,
			"interface":   r.Interface,
			"mac-address": r.MAC,
			"signal":      r.Signal,
			"uptime":      r.Uptime,
			"ssid":        r.SSID,
		}
	}
	return out
}

func buildWifiClientView(sentences []map[string]string) wifiClientView {
	out := make(wifiClientView, 0, len(sentences))
	for _, s := range sentences {
		out = append(out, wifiClientRow{
			ID:        firstField(s, ".id"),
			Interface: firstField(s, "interface", "ap"),
			MAC:       firstField(s, "mac-address", "mac"),
			Signal:    firstField(s, "signal", "signal-strength", "rssi"),
			Uptime:    firstField(s, "uptime", "last-activity"),
			SSID:      firstField(s, "ssid"),
		})
	}
	return out
}
