package guardrails

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultDoctorMaxAge is how fresh a successful doctor/hygiene audit must be
// before prod writes are allowed without break-glass.
const DefaultDoctorMaxAge = 60 * time.Minute

// doctorStateDirForTest overrides ~/.config/ros/state in unit tests.
var doctorStateDirForTest string

// ErrDoctorStale is returned when prod (or strict) writes are refused because
// the last successful doctor/hygiene audit is missing or too old.
type ErrDoctorStale struct {
	DeviceName string
	LastAt     time.Time
	MaxAge     time.Duration
	Missing    bool
}

func (e *ErrDoctorStale) Error() string {
	dev := e.DeviceName
	if dev == "" {
		dev = "<device>"
	}
	hint := "run: ros -d " + dev + " doctor   (or set ROS_SKIP_DOCTOR_GATE=1 / --skip-doctor-gate / --force to break-glass)"
	if e.Missing {
		return fmt.Sprintf("prod write refused: no recent doctor/hygiene for %q; %s", dev, hint)
	}
	age := time.Since(e.LastAt).Round(time.Second)
	return fmt.Sprintf(
		"prod write refused: last doctor/hygiene for %q was %s ago (max %s); %s",
		dev, age, e.MaxAge, hint,
	)
}

// DoctorStateDir returns ~/.config/ros/state (or the test override).
func DoctorStateDir() string {
	if doctorStateDirForTest != "" {
		return doctorStateDirForTest
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".config", "ros", "state")
}

// SetDoctorStateDirForTest redirects the doctor state directory (tests only).
// Pass "" to restore the default.
func SetDoctorStateDirForTest(dir string) {
	doctorStateDirForTest = dir
}

// DoctorStatePath returns the LastDoctorAt file for a device.
func DoctorStatePath(deviceName string) string {
	return filepath.Join(DoctorStateDir(), sanitizeStateDevice(deviceName)+".doctor")
}

// RemoveDoctorState deletes the LastDoctorAt file for a device. It is a no-op
// when the file does not exist, so deleting a device that never ran doctor
// succeeds.
func RemoveDoctorState(deviceName string) error {
	if deviceName == "" {
		return fmt.Errorf("empty device name")
	}
	if err := os.Remove(DoctorStatePath(deviceName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing doctor state: %w", err)
	}
	return nil
}

func sanitizeStateDevice(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "device"
	}
	return string(out)
}

// RecordDoctorAt writes an RFC3339 timestamp for a successful doctor/hygiene run.
func RecordDoctorAt(deviceName string, at time.Time) error {
	if deviceName == "" {
		return fmt.Errorf("empty device name")
	}
	dir := DoctorStateDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating doctor state dir: %w", err)
	}
	path := DoctorStatePath(deviceName)
	payload := []byte(at.UTC().Format(time.RFC3339Nano) + "\n")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return fmt.Errorf("writing doctor state: %w", err)
	}
	return nil
}

// LoadLastDoctorAt reads the LastDoctorAt timestamp for a device.
// ok is false when the file is missing or empty/unparseable.
func LoadLastDoctorAt(deviceName string) (at time.Time, ok bool, err error) {
	path := DoctorStatePath(deviceName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, false, nil
		}
	}
	return parsed, true, nil
}

// ROSSkipDoctorGate reports whether ROS_SKIP_DOCTOR_GATE=1/true is set.
func ROSSkipDoctorGate() bool {
	v := os.Getenv("ROS_SKIP_DOCTOR_GATE")
	return v == "1" || strings.EqualFold(v, "true")
}

// DoctorGateOpts configures EvaluateDoctorGate.
type DoctorGateOpts struct {
	EnvClass   string
	DeviceName string
	LastAt     time.Time
	HasLast    bool
	Now        time.Time
	MaxAge     time.Duration
	Force      bool // --force break-glass
	SkipEnv    bool // ROS_SKIP_DOCTOR_GATE
}

// EvaluateDoctorGate encodes the prod write doctor freshness protocol.
// - lab: no-op
// - staging: soft warning when missing/stale
// - prod: refuse when missing/stale unless Force or SkipEnv (then warn)
// Returns (warning, err). warning may be set alongside a nil err for soft cases.
func EvaluateDoctorGate(opts DoctorGateOpts) (warning string, err error) {
	env := strings.ToLower(strings.TrimSpace(opts.EnvClass))
	switch env {
	case "prod", "production":
		// hard gate below
	case "staging":
		if doctorStale(opts) {
			return doctorWarnMessage(opts, "WARNING"), nil
		}
		return "", nil
	default:
		return "", nil
	}

	if !doctorStale(opts) {
		return "", nil
	}
	if opts.Force || opts.SkipEnv {
		return doctorWarnMessage(opts, "WARNING: doctor gate bypassed"), nil
	}
	return "", &ErrDoctorStale{
		DeviceName: opts.DeviceName,
		LastAt:     opts.LastAt,
		MaxAge:     doctorMaxAge(opts.MaxAge),
		Missing:    !opts.HasLast,
	}
}

func doctorMaxAge(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultDoctorMaxAge
	}
	return d
}

func doctorStale(opts DoctorGateOpts) bool {
	if !opts.HasLast {
		return true
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	return now.Sub(opts.LastAt) > doctorMaxAge(opts.MaxAge)
}

func doctorWarnMessage(opts DoctorGateOpts, prefix string) string {
	dev := opts.DeviceName
	if dev == "" {
		dev = "<device>"
	}
	max := doctorMaxAge(opts.MaxAge)
	if !opts.HasLast {
		return fmt.Sprintf("%s: no recent doctor/hygiene for %q (recommended within %s); run: ros -d %s doctor",
			prefix, dev, max, dev)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	age := now.Sub(opts.LastAt).Round(time.Second)
	return fmt.Sprintf("%s: last doctor/hygiene for %q was %s ago (max %s); run: ros -d %s doctor",
		prefix, dev, age, max, dev)
}

// CheckDoctorGate loads LastDoctorAt and evaluates the gate for envClass.
// Soft warnings are written to warnW when non-nil.
func CheckDoctorGate(envClass, deviceName string, force bool, warnW io.Writer) error {
	last, ok, err := LoadLastDoctorAt(deviceName)
	if err != nil {
		return fmt.Errorf("reading doctor state: %w", err)
	}
	warning, gateErr := EvaluateDoctorGate(DoctorGateOpts{
		EnvClass:   envClass,
		DeviceName: deviceName,
		LastAt:     last,
		HasLast:    ok,
		Now:        time.Now(),
		MaxAge:     DefaultDoctorMaxAge,
		Force:      force,
		SkipEnv:    ROSSkipDoctorGate(),
	})
	if warning != "" && warnW != nil {
		_, _ = fmt.Fprintln(warnW, warning)
	}
	return gateErr
}
