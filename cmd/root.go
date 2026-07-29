package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/credential"
	"github.com/nic0der-im/routeros-cli/internal/device"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
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
	// Profile is operator|agent|agent-strict (from ROS_PROFILE / defaults).
	Profile string
	// Force enables break-glass for doctor freshness, maintenance windows, and similar gates.
	Force bool
	// RequestID correlates one CLI invocation in JSON meta and -v logs.
	RequestID string
	// RowLimit is --limit N for list prints; 0 = unlimited.
	RowLimit int
	// MaxOutputBytes is ROS_MAX_OUTPUT_BYTES (default 512_000).
	MaxOutputBytes int
	// AuditDir overrides the write-audit directory when non-empty (tests).
	AuditDir string
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
	flagForce    bool
	flagLimit    int
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
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output (includes request_id)")
	pf.BoolVar(&flagNoColor, "no-color", false, "disable color output")
	pf.BoolVar(&flagReadOnly, "read-only", false, "refuse all write commands (also: ROS_READ_ONLY=1)")
	pf.BoolVar(&flagRawJSON, "raw", false, "include raw RouterOS fields in JSON (secrets unredacted)")
	pf.BoolVar(&flagForce, "skip-doctor-gate", false, "break-glass: skip doctor freshness gate on writes (also: ROS_SKIP_DOCTOR_GATE=1)")
	pf.IntVar(&flagLimit, "limit", 0, "max rows for list output (0 = unlimited)")
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

	if flagLimit < 0 {
		return nil, fmt.Errorf("invalid --limit %d: must be >= 0", flagLimit)
	}

	profile, err := config.ResolveProfileFromEnv()
	if err != nil {
		return nil, err
	}

	sessStore, err := session.NewStore(session.DefaultDir())
	if err != nil {
		return nil, fmt.Errorf("initializing session store: %w", err)
	}

	a := &App{
		Config:         cfg,
		CfgPath:        cfgPath,
		Inventory:      device.NewInventory(cfg, cfgPath),
		Creds:          credential.NewKeyringStore(),
		Sessions:       sessStore,
		OutFormat:      format,
		Timeout:        timeout,
		Verbose:        flagVerbose,
		NoColor:        flagNoColor,
		ReadOnly:       readOnly,
		RawJSON:        flagRawJSON,
		Profile:        profile,
		Force:          flagForce,
		RequestID:      newRequestID(),
		RowLimit:       flagLimit,
		MaxOutputBytes: output.MaxOutputBytesFromEnv(),
	}
	a.verbosef("request_id=%s", a.RequestID)
	return a, nil
}

// newRequestID returns a 16-hex-char id for correlating one CLI invocation.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// verbosef writes to stderr when -v is set (always prefixes request_id).
func (a *App) verbosef(format string, args ...interface{}) {
	if a == nil || !a.Verbose {
		return
	}
	prefix := a.RequestID
	if prefix == "" {
		prefix = "-"
	}
	fmt.Fprintf(os.Stderr, "ros[%s]: "+format+"\n", append([]interface{}{prefix}, args...)...)
}

// renderOpts returns output options for this App.
func (a *App) renderOpts() output.Options {
	max := a.MaxOutputBytes
	if max == 0 {
		max = output.DefaultMaxOutputBytes
	}
	return output.Options{Raw: a.RawJSON, MaxBytes: max}
}

// newMeta builds envelope meta with request_id.
func (a *App) newMeta(deviceName, command string, count int) output.Meta {
	return output.Meta{
		Device:    deviceName,
		Command:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Count:     count,
		RequestID: a.RequestID,
	}
}

// renderRawJSON writes a success envelope with App request_id and byte cap.
func (a *App) renderRawJSON(w io.Writer, data interface{}, meta output.Meta) error {
	if meta.RequestID == "" {
		meta.RequestID = a.RequestID
	}
	return output.RenderRawJSON(w, data, meta, a.renderOpts())
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

	// Connect already wraps read retries; optional read-only sits outside.
	cli := c
	if a.ReadOnly {
		cli = policy.WrapReadOnly(c)
	}

	return cli, name, nil
}

// render outputs data in the configured format.
// Applies --limit row cap (sets meta.truncated) and ROS_MAX_OUTPUT_BYTES.
func (a *App) render(w io.Writer, data output.Renderable, deviceName, command string) error {
	limited, rowTrunc := output.LimitRenderable(data, a.RowLimit)
	meta := a.newMeta(deviceName, command, len(limited.TableRows()))
	meta.Truncated = rowTrunc
	return output.Render(w, a.OutFormat, limited, meta, a.renderOpts())
}

// renderError outputs an error in the configured format.
// Optional suggestedAction (first variadic arg) is included in JSON and human output.
func (a *App) renderError(w io.Writer, code, message, deviceName string, suggestedAction ...string) {
	suggestion := ""
	if len(suggestedAction) > 0 {
		suggestion = suggestedAction[0]
	}
	if a.Verbose {
		a.verbosef("error code=%s device=%s msg=%s", code, deviceName, message)
	}
	if a.OutFormat == output.FormatJSON {
		meta := a.newMeta(deviceName, "", 0)
		_ = output.RenderError(w, code, message, deviceName, meta, suggestion)
		return
	}
	fmt.Fprintf(w, "Error: %s\n", message)
	if suggestion != "" {
		fmt.Fprintf(w, "Suggested action: %s\n", suggestion)
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
		newDNSCmd(),
		newDHCPCmd(),
		newBackupCmd(),
		newFileCmd(),
		newMonitorCmd(),
		newExecCmd(),
		newSchemaCmd(),
		newAuditCmd(),
		newDoctorCmd(),
		newSessionCmd(),
		newPlanCmd(),
		newGetCmd(),
		newCreateCmd(),
		newSetCmd(),
		newDeleteCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newDomainsCmd(),
		newDiagCmd(),
		newSkillsCmd(),
		newNatCmd(),
		newLeaseCmd(),
		newWGCmd(),
		newWifiCmd(),
		newBGPCmd(),
		newOSPFCmd(),
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
		a.renderError(os.Stderr, string(apperr.KindConnection), err.Error(), deviceName)
		os.Exit(ExitConnError)
	}
	defer func() { _ = c.Close() }()

	if err := fn(ctx, a, c, deviceName); err != nil {
		var roErr *policy.ErrReadOnly
		if errors.As(err, &roErr) {
			a.renderError(os.Stderr, string(apperr.KindReadOnly), err.Error(), deviceName)
			os.Exit(ExitReadOnly)
		}
		var sessReq *guardrails.ErrSafeSessionRequired
		var maxCh *guardrails.ErrMaxChanges
		var pathDenied *guardrails.ErrPathDenied
		var doctorStale *guardrails.ErrDoctorStale
		var outsideMaint *guardrails.ErrOutsideMaintenanceWindow
		if errors.As(err, &sessReq) || errors.As(err, &maxCh) || errors.As(err, &pathDenied) || errors.As(err, &doctorStale) || errors.As(err, &outsideMaint) {
			a.renderError(os.Stderr, string(apperr.KindSession), err.Error(), deviceName)
			os.Exit(apperr.ExitCode(apperr.KindSession))
		}
		if kind, ok := apperr.AsKind(err); ok {
			a.renderError(os.Stderr, string(kind), err.Error(), deviceName, apperr.AsSuggestedAction(err))
			os.Exit(apperr.ExitCode(kind))
		}
		a.renderError(os.Stderr, string(apperr.KindAPI), err.Error(), deviceName)
		os.Exit(ExitCmdError)
	}
}

// ensureWritable returns an error if the app is in read-only mode or production
// guardrails block the write. Dry-run callers must skip this (preview only).
func (a *App) ensureWritable(deviceName, action string) error {
	return a.ensureWritableForce(deviceName, action, false)
}

// ensureWritableForce is ensureWritable with an extra break-glass force flag
// (e.g. command-local --force) that also bypasses doctor freshness and
// maintenance window gates.
func (a *App) ensureWritableForce(deviceName, action string, force bool) error {
	if a.ReadOnly {
		return &policy.ErrReadOnly{Command: action}
	}
	return a.enforceGuardrails(deviceName, action, force || a.Force)
}

// effectiveStrict reports whether all devices should use prod-class gates.
func (a *App) effectiveStrict() bool {
	return config.EffectiveStrict(a.Profile)
}

// enforceGuardrails applies env_class safe-session, blast-radius, path,
// maintenance-window, and doctor gates.
func (a *App) enforceGuardrails(deviceName, action string, force bool) error {
	dev := a.deviceConfig(deviceName)
	envClass := config.EffectiveEnvClass(dev, a.effectiveStrict())

	var (
		sess *session.Session
		err  error
	)
	if a.Sessions != nil {
		sess, err = a.Sessions.Active(deviceName)
		if err != nil {
			return err
		}
	}
	hasSafe := sess != nil && sess.Safe

	// agent / agent-strict: refuse writes without a safe session (even on lab).
	if config.ProfileRequiresSafeSession(a.Profile) {
		if !hasSafe {
			return &guardrails.ErrSafeSessionRequired{EnvClass: a.Profile, DeviceName: deviceName}
		}
	} else if err := guardrails.CheckSafeSession(envClass, deviceName, hasSafe); err != nil {
		return err
	}

	if hasSafe {
		max := config.EffectiveMaxSessionChanges(dev, envClass)
		if err := guardrails.CheckMaxChanges(len(sess.Changes), max); err != nil {
			return err
		}
	}

	if err := guardrails.CheckWritePath(action, dev.AllowedWritePaths, dev.DeniedWritePaths); err != nil {
		return err
	}

	// Maintenance windows (when configured): enforce for all env classes.
	// Break-glass: command --force / App.Force (--skip-doctor-gate) / ROS_SKIP_MAINTENANCE_GATE.
	// Dry-run callers skip ensureWritable entirely, so they are not blocked here.
	if err := guardrails.CheckMaintenanceWindow(dev.MaintenanceWindows, time.Now(), force); err != nil {
		return err
	}

	// Prod doctor freshness (staging soft-warns). Break-glass: --force / --skip-doctor-gate / ROS_SKIP_DOCTOR_GATE.
	if err := guardrails.CheckDoctorGate(envClass, deviceName, force, os.Stderr); err != nil {
		return err
	}
	return nil
}

func (a *App) deviceConfig(deviceName string) config.DeviceConfig {
	if a.Inventory == nil || deviceName == "" {
		return config.DeviceConfig{}
	}
	dev, err := a.Inventory.Get(deviceName)
	if err != nil {
		return config.DeviceConfig{}
	}
	return dev
}

// recordSafeChange appends a change to the active safe session if one exists.
// No-op when there is no session or the session was started with --safe=false.
func (a *App) recordSafeChange(deviceName string, change session.Change) error {
	sess, err := a.Sessions.Active(deviceName)
	if err != nil {
		return err
	}
	if sess == nil || !sess.Safe {
		return nil
	}
	dev := a.deviceConfig(deviceName)
	envClass := config.EffectiveEnvClass(dev, a.effectiveStrict())
	max := config.EffectiveMaxSessionChanges(dev, envClass)
	if err := guardrails.CheckMaxChanges(len(sess.Changes), max); err != nil {
		return err
	}
	if err := guardrails.CheckWritePath(change.Command, dev.AllowedWritePaths, dev.DeniedWritePaths); err != nil {
		return err
	}
	return a.Sessions.AppendChange(sess, change)
}

// fetchPreState prints a row for journaling.
// When id is set, filters with ?.id=. When id is empty (singleton menus),
// prints the parent path and uses the first sentence.
func fetchPreState(ctx context.Context, c client.Client, basePath, id string) (map[string]string, error) {
	printCmd := normalizePath(basePath) + "/print"
	var (
		result *client.Result
		err    error
	)
	if id == "" {
		result, err = c.Run(ctx, printCmd)
	} else {
		result, err = c.Run(ctx, printCmd, "?.id="+id)
	}
	if err != nil {
		return nil, err
	}
	if len(result.Sentences) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(result.Sentences[0]))
	for k, v := range result.Sentences[0] {
		out[k] = v
	}
	return out, nil
}
