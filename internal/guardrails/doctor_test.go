package guardrails

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordAndLoadDoctorAt(t *testing.T) {
	dir := t.TempDir()
	doctorStateDirForTest = dir
	t.Cleanup(func() { doctorStateDirForTest = "" })

	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	if err := RecordDoctorAt("edge-core", now); err != nil {
		t.Fatal(err)
	}
	path := DoctorStatePath("edge-core")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	got, ok, err := LoadLastDoctorAt("edge-core")
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if !got.Equal(now) {
		t.Fatalf("got %v want %v", got, now)
	}

	// sanitized name for spaces
	if err := RecordDoctorAt("central hub", now); err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(dir, "central_hub.doctor")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("sanitized path: %v", err)
	}
}

func TestLoadLastDoctorAt_Missing(t *testing.T) {
	doctorStateDirForTest = t.TempDir()
	t.Cleanup(func() { doctorStateDirForTest = "" })

	_, ok, err := LoadLastDoctorAt("ghost")
	if err != nil || ok {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
}

func TestEvaluateDoctorGate(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	fresh := now.Add(-10 * time.Minute)
	stale := now.Add(-2 * time.Hour)

	tests := []struct {
		name     string
		opts     DoctorGateOpts
		wantWarn bool
		wantErr  bool
	}{
		{
			name: "lab missing ok",
			opts: DoctorGateOpts{EnvClass: "lab", DeviceName: "r1", Now: now},
		},
		{
			name:     "staging missing warns",
			opts:     DoctorGateOpts{EnvClass: "staging", DeviceName: "r1", Now: now},
			wantWarn: true,
		},
		{
			name:     "staging stale warns",
			opts:     DoctorGateOpts{EnvClass: "staging", DeviceName: "r1", LastAt: stale, HasLast: true, Now: now},
			wantWarn: true,
		},
		{
			name: "staging fresh silent",
			opts: DoctorGateOpts{EnvClass: "staging", DeviceName: "r1", LastAt: fresh, HasLast: true, Now: now},
		},
		{
			name:    "prod missing refuses",
			opts:    DoctorGateOpts{EnvClass: "prod", DeviceName: "edge", Now: now},
			wantErr: true,
		},
		{
			name:    "prod stale refuses",
			opts:    DoctorGateOpts{EnvClass: "prod", DeviceName: "edge", LastAt: stale, HasLast: true, Now: now},
			wantErr: true,
		},
		{
			name: "prod fresh ok",
			opts: DoctorGateOpts{EnvClass: "prod", DeviceName: "edge", LastAt: fresh, HasLast: true, Now: now},
		},
		{
			name:     "prod stale force bypass warns",
			opts:     DoctorGateOpts{EnvClass: "prod", DeviceName: "edge", LastAt: stale, HasLast: true, Now: now, Force: true},
			wantWarn: true,
		},
		{
			name:     "prod missing skip env bypass warns",
			opts:     DoctorGateOpts{EnvClass: "prod", DeviceName: "edge", Now: now, SkipEnv: true},
			wantWarn: true,
		},
		{
			name: "production alias refuses",
			opts: DoctorGateOpts{EnvClass: "production", DeviceName: "edge", Now: now},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, err := EvaluateDoctorGate(tt.opts)
			if tt.wantWarn && warn == "" {
				t.Fatal("expected warning")
			}
			if !tt.wantWarn && warn != "" {
				t.Fatalf("unexpected warning: %s", warn)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				var staleErr *ErrDoctorStale
				if !errors.As(err, &staleErr) {
					t.Fatalf("got %T: %v", err, err)
				}
				if !strings.Contains(err.Error(), "doctor") {
					t.Errorf("error should mention doctor: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}

func TestCheckDoctorGate_ProdRefuseAndBypass(t *testing.T) {
	doctorStateDirForTest = t.TempDir()
	t.Cleanup(func() { doctorStateDirForTest = "" })
	t.Setenv("ROS_SKIP_DOCTOR_GATE", "")

	var buf bytes.Buffer
	err := CheckDoctorGate("prod", "edge", false, &buf)
	if err == nil {
		t.Fatal("expected refuse")
	}
	var stale *ErrDoctorStale
	if !errors.As(err, &stale) {
		t.Fatalf("got %T", err)
	}

	buf.Reset()
	if err := CheckDoctorGate("prod", "edge", true, &buf); err != nil {
		t.Fatalf("force should bypass: %v", err)
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Fatalf("expected bypass warning, got %q", buf.String())
	}

	if err := RecordDoctorAt("edge", time.Now()); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := CheckDoctorGate("prod", "edge", false, &buf); err != nil {
		t.Fatalf("fresh doctor should allow: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected warn: %q", buf.String())
	}
}

func TestCheckDoctorGate_SkipEnv(t *testing.T) {
	doctorStateDirForTest = t.TempDir()
	t.Cleanup(func() { doctorStateDirForTest = "" })
	t.Setenv("ROS_SKIP_DOCTOR_GATE", "1")

	var buf bytes.Buffer
	if err := CheckDoctorGate("prod", "edge", false, &buf); err != nil {
		t.Fatalf("skip env: %v", err)
	}
	if !strings.Contains(buf.String(), "bypassed") {
		t.Fatalf("got %q", buf.String())
	}
}
