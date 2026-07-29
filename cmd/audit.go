package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAuditCmd() *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Read-only multi-domain snapshot for humans and agents",
		Long: `Collect a read-only configuration/status snapshot from the router.

Profiles:
  full     system, interfaces, ip, routes, dns, firewall, dhcp (default)
  network  interfaces, ip addresses, routes, dns, dhcp
  security firewall filter/nat, system identity`,
		Run: func(cmd *cobra.Command, args []string) {
			runWithClient(cmd, "/audit", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
				order, sections, err := collectAudit(ctx, c, profile)
				if err != nil {
					return err
				}

				meta := output.Meta{
					Device:    deviceName,
					Command:   "audit --profile " + profile,
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Count:     len(sections),
				}

				if a.OutFormat == output.FormatJSON {
					return output.RenderRawJSON(cmd.OutOrStdout(), sections, meta)
				}

				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "Audit of %q (profile=%s)\n", deviceName, profile)
				for _, name := range order {
					data := sections[name]
					n := 0
					if rows, ok := data.([]map[string]string); ok {
						n = len(rows)
					}
					fmt.Fprintf(w, "  %-20s %d item(s)\n", name+":", n)
				}
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "full", "audit profile: full, network, or security")
	return cmd
}

func collectAudit(ctx context.Context, c client.Client, profile string) ([]string, map[string]interface{}, error) {
	type sectionSpec struct {
		key string
		cmd string
	}

	var specs []sectionSpec
	switch profile {
	case "full", "":
		specs = []sectionSpec{
			{"system_resource", "/system/resource/print"},
			{"system_identity", "/system/identity/print"},
			{"interfaces", "/interface/print"},
			{"ip_addresses", "/ip/address/print"},
			{"ip_routes", "/ip/route/print"},
			{"dns", "/ip/dns/print"},
			{"firewall_filter", "/ip/firewall/filter/print"},
			{"firewall_nat", "/ip/firewall/nat/print"},
			{"dhcp_leases", "/ip/dhcp-server/lease/print"},
			{"dhcp_servers", "/ip/dhcp-server/print"},
		}
	case "network":
		specs = []sectionSpec{
			{"interfaces", "/interface/print"},
			{"ip_addresses", "/ip/address/print"},
			{"ip_routes", "/ip/route/print"},
			{"dns", "/ip/dns/print"},
			{"dhcp_leases", "/ip/dhcp-server/lease/print"},
			{"dhcp_servers", "/ip/dhcp-server/print"},
		}
	case "security":
		specs = []sectionSpec{
			{"firewall_filter", "/ip/firewall/filter/print"},
			{"firewall_nat", "/ip/firewall/nat/print"},
			{"system_identity", "/system/identity/print"},
		}
	default:
		return nil, nil, fmt.Errorf("unknown audit profile %q (valid: full, network, security)", profile)
	}

	order := make([]string, 0, len(specs))
	out := make(map[string]interface{}, len(specs))
	for _, s := range specs {
		result, err := c.Run(ctx, s.cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("audit section %s (%s): %w", s.key, s.cmd, err)
		}
		order = append(order, s.key)
		out[s.key] = result.Sentences
	}
	return order, out, nil
}
