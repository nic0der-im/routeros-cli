package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/device"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/winbox"
	"github.com/spf13/cobra"
)

func newDeviceImportCmd() *cobra.Command {
	var (
		from           string
		file           string
		withPasswords  bool
		dryRun         bool
		force          bool
		apiPort        string
		keepWinboxPort bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import devices from Winbox address book (or light .rsc)",
		Long: `Import RouterOS devices into the local inventory.

Sources:
  --from winbox   Winbox 3 (.WBX) or Winbox 4 (Addresses.cdb)
  --from rsc      Optional light RouterOS export (.rsc); identity + hints only

When --file is omitted for winbox, ros auto-detects the default address book
for the current OS (CDB first, then WBX).

By default every imported address uses --api-port (8728). Winbox stores the
GUI/Winbox port (8291/…), which is not the RouterOS API — so those ports are
replaced unless you pass --keep-winbox-port.

Passwords are never printed. With --with-passwords, Winbox cleartext secrets
are moved into the OS keyring. Without it, set secrets later via:
  ros device auth set <name>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			from = strings.ToLower(strings.TrimSpace(from))
			switch from {
			case "winbox":
				return runWinboxImport(cmd, file, withPasswords, dryRun, force, apiPort, keepWinboxPort)
			case "rsc":
				if file == "" {
					return fmt.Errorf("--file is required for --from rsc")
				}
				return runRSCImport(cmd, file, dryRun)
			default:
				return fmt.Errorf("unsupported --from %q (want winbox or rsc)", from)
			}
		},
	}

	cmd.Flags().StringVar(&from, "from", "winbox", "source: winbox | rsc")
	cmd.Flags().StringVar(&file, "file", "", "path to Addresses.cdb / addresses.WBX / export.rsc")
	cmd.Flags().BoolVar(&withPasswords, "with-passwords", false, "import passwords into the OS keyring (winbox only)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print entries without writing inventory")
	cmd.Flags().BoolVar(&force, "force", false, "update address/username when device name already exists")
	cmd.Flags().StringVar(&apiPort, "api-port", winbox.DefaultAPIPort, "RouterOS API port applied to every imported host")
	cmd.Flags().BoolVar(&keepWinboxPort, "keep-winbox-port", false, "keep the port stored in Winbox (GUI port; usually wrong for API)")

	return cmd
}

type importRow struct {
	Action   string
	Name     string
	Address  string
	Username string
	Group    string
	Comment  string
	Password string // always empty in rendered output
}

type importList []importRow

func (il importList) TableHeaders() []string {
	return []string{"Action", "Name", "Address", "Username", "Group", "Comment"}
}

func (il importList) TableRows() [][]string {
	rows := make([][]string, len(il))
	for i, r := range il {
		rows[i] = []string{r.Action, r.Name, r.Address, r.Username, r.Group, r.Comment}
	}
	return rows
}

func runWinboxImport(cmd *cobra.Command, file string, withPasswords, dryRun, force bool, apiPort string, keepWinboxPort bool) error {
	a, err := loadApp()
	if err != nil {
		return err
	}

	path := file
	if path == "" {
		path, err = winbox.FindDefaultFile()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Using Winbox file: %s\n", path)
	}

	entries, err := winbox.ParseFile(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no entries found in %s", path)
	}

	if apiPort == "" {
		apiPort = winbox.DefaultAPIPort
	}
	forceAPIPort := !keepWinboxPort

	if withPasswords {
		fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: Winbox stores secrets in cleartext; imported passwords will be moved into the OS keyring.")
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "Note: passwords not imported; set them with: ros device auth set <name>")
	}
	if forceAPIPort {
		fmt.Fprintf(cmd.ErrOrStderr(), "Note: applying API port %s to every host (Winbox GUI ports ignored; use --keep-winbox-port to keep them)\n", apiPort)
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "Note: keeping Winbox ports as stored (may not be RouterOS API)")
	}

	taken := func(name string) bool {
		_, err := a.Inventory.Get(name)
		return err == nil
	}
	// Also reserve names we assign in this batch.
	batch := map[string]bool{}
	takenOrBatch := func(name string) bool {
		return taken(name) || batch[name]
	}

	rows := make(importList, 0, len(entries))
	added, updated, skipped := 0, 0, 0

	for _, e := range entries {
		addr := winbox.NormalizeAddressForAPI(e.Address, apiPort, forceAPIPort)
		if addr == "" {
			skipped++
			continue
		}
		user := e.Username
		if user == "" {
			user = "admin"
		}

		base := winbox.PreferredName(e)
		var name string
		var action string

		if force && taken(base) {
			name = base
			action = "update"
		} else if taken(base) && !force {
			// Skip existing names unless --force; still try unique only for brand-new hosts.
			name = base
			action = "skip"
		} else {
			name = winbox.UniqueName(base, takenOrBatch)
			action = "add"
		}
		batch[name] = true

		row := importRow{
			Action:   action,
			Name:     name,
			Address:  addr,
			Username: user,
			Group:    e.Group,
			Comment:  e.Comment,
		}
		rows = append(rows, row)

		if dryRun || action == "skip" {
			if action == "skip" {
				skipped++
			}
			continue
		}

		notes := strings.TrimSpace(strings.Join(filterEmpty(e.Comment, e.Group), "; "))
		dev := config.DeviceConfig{
			Address:  addr,
			Username: user,
			TLS:      device.InferTLS(addr, false, false),
			Notes:    notes,
			Tags:     tagFromGroup(e.Group),
		}

		switch action {
		case "add":
			if err := a.Inventory.Add(name, dev); err != nil {
				return err
			}
			added++
		case "update":
			existing, _ := a.Inventory.Get(name)
			dev.ID = existing.ID
			if len(existing.Tags) > 0 && len(dev.Tags) == 0 {
				dev.Tags = existing.Tags
			}
			if existing.Notes != "" && dev.Notes == "" {
				dev.Notes = existing.Notes
			}
			if err := a.Inventory.Update(name, dev); err != nil {
				return err
			}
			updated++
		}

		if withPasswords && e.Password != "" {
			if err := a.Creds.Set(name, e.Password); err != nil {
				return fmt.Errorf("storing password for %q: %w", name, err)
			}
		}
	}

	meta := a.newMeta("", "device import", len(rows))
	if err := output.Render(cmd.OutOrStdout(), a.OutFormat, rows, meta, a.renderOpts()); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Import from %s: added=%d updated=%d skipped=%d (dry-run=%v with-passwords=%v api-port=%s)\n",
		filepath.Base(path), added, updated, skipped, dryRun, withPasswords, apiPort)
	return nil
}

func tagFromGroup(group string) []string {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil
	}
	return []string{winbox.SanitizeName(group)}
}

func filterEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// runRSCImport is a light optional importer: extracts /system identity name
// and simple dhcp/gateway hints when easy; otherwise reports skip.
func runRSCImport(cmd *cobra.Command, file string, dryRun bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("empty rsc file: %s", file)
	}

	text := string(data)
	identity := rscFindIdentity(text)
	gateway := rscFindGatewayHint(text)

	if identity == "" && gateway == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "rsc import: no identity or gateway hints found; skipping (use winbox import for address books)")
		return nil
	}

	name := winbox.SanitizeName(identity)
	if name == "device" && gateway != "" {
		name = winbox.SanitizeName(gateway)
	}
	addr := ""
	if gateway != "" {
		addr = winbox.NormalizeAddress(gateway)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "rsc hints: identity=%q gateway=%q -> name=%q address=%q\n", identity, gateway, name, addr)
	if addr == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "rsc import: found identity but no address hint; not writing inventory")
		return nil
	}
	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "dry-run: no changes written")
		return nil
	}

	a, err := loadApp()
	if err != nil {
		return err
	}
	if _, err := a.Inventory.Get(name); err == nil {
		return fmt.Errorf("device %q already exists (use winbox import --force or pick another name)", name)
	}
	dev := config.DeviceConfig{
		Address:  addr,
		Username: "admin",
		TLS:      false,
		Notes:    "imported from rsc",
	}
	if err := a.Inventory.Add(name, dev); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Device %q added from rsc (%s). Set password: ros device auth set %q\n", name, addr, name)
	return nil
}

func rscFindIdentity(text string) string {
	// Matches: set name="FOO" or name=FOO near /system identity
	lines := strings.Split(text, "\n")
	inIdentity := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.Contains(lower, "/system identity") || strings.HasPrefix(lower, "/system/identity") {
			inIdentity = true
		}
		if inIdentity || strings.Contains(lower, "name=") {
			if name := rscExtractProp(trim, "name"); name != "" {
				if inIdentity || strings.Contains(lower, "identity") {
					return name
				}
			}
		}
		if inIdentity && (strings.HasPrefix(trim, "/") && !strings.Contains(lower, "identity")) {
			inIdentity = false
		}
	}
	return ""
}

func rscFindGatewayHint(text string) string {
	// Prefer dhcp-client gateway, then first /ip route gateway=.
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.Contains(lower, "gateway=") {
			if gw := rscExtractProp(trim, "gateway"); gw != "" && !strings.EqualFold(gw, "0.0.0.0") {
				return gw
			}
		}
	}
	return ""
}

func rscExtractProp(line, key string) string {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, strings.ToLower(key)+"=")
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(key)+1:]
	if rest == "" {
		return ""
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return ""
		}
		return rest[1 : 1+end]
	}
	// unquoted until space
	end := len(rest)
	for i, c := range rest {
		if c == ' ' || c == '\t' || c == ';' {
			end = i
			break
		}
	}
	return rest[:end]
}
