package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/credential"
	"github.com/nic0der-im/routeros-cli/internal/device"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/policy"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

// Exit codes.
const (
	ExitOK        = 0
	ExitCmdError  = 1
	ExitConnError = 2
	ExitConfError = 3
	ExitReadOnly  = 4
)

// App holds shared dependencies injected into all commands.
type App struct {
	Config    *config.Config
	CfgPath   string
	Inventory *device.Inventory
	Creds     credential.Store
	Sessions  *session.Store
	OutFormat output.Format
	Timeout   time.Duration
	Verbose   bool
	NoColor   bool
	ReadOnly  bool
	RawJSON   bool
}

// Global flags.
var (
	flagDevice   string
	flagOutput   string
	flagConfig   string
	flagTimeout  string
	flagVerbose  bool
	flagNoColor  bool
	flagReadOnly bool
	flagRawJSON  bool
)

var rootCmd = &cobra.Command{
	Use:     "ros",
	Aliases: []string{"routeros-cli"},
	Short:   "MikroTik RouterOS CLI for humans and AI agents",
	Long: `ros is a CLI tool for managing MikroTik RouterOS routers with structured
output for humans and AI agents. Supports multi-device inventory, read-only
agent mode, and safe sessions with rollback.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagDevice, "device", "d", "", "device name, id, or IP from inventory")
	pf.StringVarP(&flagOutput, "output", "o", "", "output format: table or json")
	pf.StringVar(&flagConfig, "config", "", "config file path")
	pf.StringVar(&flagTimeout, "timeout", "10s", "connection timeout")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
	pf.BoolVar(&flagNoColor, "no-color", false, "disable color output")
	pf.BoolVar(&flagReadOnly, "read-only", false, "refuse all write commands (also: ROS_READ_ONLY=1)")
	pf.BoolVar(&flagRawJSON, "raw", false, "include raw RouterOS fields in JSON output")
}

// loadApp initializes the App from flags and config.
func loadApp() (*App, error) {
	cfgPath := flagConfig
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	outFmt := cfg.DefaultOutput
	if envOut := os.Getenv("ROS_DEFAULT_OUTPUT"); envOut != "" {
		outFmt = envOut
	}
	if flagOutput != "" {
		outFmt = flagOutput
	}
	format, err := output.ParseFormat(outFmt)
	if err != nil {
		return nil, fmt.Errorf("invalid output format: %w", err)
	}

	timeout, err := time.ParseDuration(flagTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}

	readOnly := flagReadOnly || os.Getenv("ROS_READ_ONLY") == "1" || os.Getenv("ROS_READ_ONLY") == "true"

	sessStore, err := session.NewStore(session.DefaultDir())
	if err != nil {
		return nil, fmt.Errorf("initializing session store: %w", err)
	}

	return &App{
		Config:    cfg,
		CfgPath:   cfgPath,
		Inventory: device.NewInventory(cfg, cfgPath),
		Creds:     credential.NewKeyringStore(),
		Sessions:  sessStore,
		OutFormat: format,
		Timeout:   timeout,
		Verbose:   flagVerbose,
		NoColor:   flagNoColor,
		ReadOnly:  readOnly,
		RawJSON:   flagRawJSON,
	}, nil
}

// connect resolves the target device and establishes a RouterOS connection.
func (a *App) connect(ctx context.Context) (client.Client, string, error) {
	name, dev, err := a.Inventory.Resolve(flagDevice)
	if err != nil {
		return nil, "", err
	}

	password, err := a.Creds.Get(name)
	if err != nil {
		return nil, "", fmt.Errorf("getting credentials for %q: %w", name, err)
	}

	cfg := client.ConnectConfig{
		Address:            dev.Address,
		Username:           dev.Username,
		Password:           password,
		UseTLS:             dev.TLS,
		InsecureSkipVerify: a.Config.TLS.InsecureSkipVerify,
		CACertPath:         a.Config.TLS.CACert,
		Timeout:            a.Timeout,
	}

	c, err := client.Connect(ctx, cfg)
	if err != nil {
		return nil, name, fmt.Errorf("connecting to %q (%s): %w", name, dev.Address, err)
	}

	var cli client.Client = c
	if a.ReadOnly {
		cli = policy.WrapReadOnly(c)
	}

	return cli, name, nil
}

// render outputs data in the configured format.
func (a *App) render(w io.Writer, data output.Renderable, deviceName, command string) error {
	meta := output.Meta{
		Device:    deviceName,
		Command:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Count:     len(data.TableRows()),
	}
	opts := output.Options{Raw: a.RawJSON}
	return output.Render(w, a.OutFormat, data, meta, opts)
}

// renderError outputs an error in the configured format.
func (a *App) renderError(w io.Writer, code, message, deviceName string) {
	if a.OutFormat == output.FormatJSON {
		_ = output.RenderError(w, code, message, deviceName)
	} else {
		fmt.Fprintf(w, "Error: %s\n", message)
	}
}

// Execute runs the root command.
func Execute() error {
	rootCmd.AddCommand(
		newVersionCmd(),
		newDeviceCmd(),
		newSystemCmd(),
		newInterfaceCmd(),
		newIPCmd(),
		newFirewallCmd(),
		newDHCPCmd(),
		newBackupCmd(),
		newMonitorCmd(),
		newExecCmd(),
		newSchemaCmd(),
		newAuditCmd(),
		newSessionCmd(),
		newGetCmd(),
		newCreateCmd(),
		newSetCmd(),
		newDeleteCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newDomainsCmd(),
		newDiagCmd(),
		newSkillsCmd(),
	)
	return rootCmd.Execute()
}

// runWithClient is a helper that loads app, connects, runs fn, and handles errors.
func runWithClient(cmdInstance *cobra.Command, rosCommand string, fn func(ctx context.Context, a *App, c client.Client, deviceName string) error) {
	a, err := loadApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(ExitConfError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.Timeout)
	defer cancel()

	c, deviceName, err := a.connect(ctx)
	if err != nil {
		a.renderError(os.Stderr, "connection_failed", err.Error(), deviceName)
		os.Exit(ExitConnError)
	}
	defer func() { _ = c.Close() }()

	if err := fn(ctx, a, c, deviceName); err != nil {
		var roErr *policy.ErrReadOnly
		if errors.As(err, &roErr) {
			a.renderError(os.Stderr, "read_only_violation", err.Error(), deviceName)
			os.Exit(ExitReadOnly)
		}
		a.renderError(os.Stderr, "command_failed", err.Error(), deviceName)
		os.Exit(ExitCmdError)
	}
}

// ensureWritable returns an error if the app is in read-only mode.
func (a *App) ensureWritable(action string) error {
	if a.ReadOnly {
		return &policy.ErrReadOnly{Command: action}
	}
	return nil
}

// recordSafeChange appends a change to the active safe session if one exists.
func (a *App) recordSafeChange(deviceName string, change session.Change) error {
	sess, err := a.Sessions.Active(deviceName)
	if err != nil {
		return err
	}
	if sess == nil {
		return nil
	}
	return a.Sessions.AppendChange(sess, change)
}
