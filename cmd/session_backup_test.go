package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
)

func TestSessionBeginNeedsBackup(t *testing.T) {
	tests := []struct {
		name          string
		safe          bool
		env           string
		deviceRequire bool
		force         bool
		want          bool
	}{
		{"prod safe", true, "prod", false, false, true},
		{"prod force", true, "prod", false, true, false},
		{"prod unsafe", false, "prod", false, false, false},
		{"staging", true, "staging", false, false, false},
		{"staging require", true, "staging", true, false, true},
		{"lab", true, "lab", false, false, false},
		{"lab require", true, "lab", true, false, true},
		{"lab require force", true, "lab", true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionBeginNeedsBackup(tt.safe, tt.env, tt.deviceRequire, tt.force)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestSessionBeginForceNoBackupFlag(t *testing.T) {
	cmd := newSessionBeginCmd()
	f := cmd.Flags().Lookup("force-no-backup")
	if f == nil {
		t.Fatal("missing --force-no-backup flag")
	}
	if f.DefValue != "false" {
		t.Fatalf("default=%q", f.DefValue)
	}
	safe := cmd.Flags().Lookup("safe")
	if safe == nil || safe.DefValue != "true" {
		t.Fatalf("safe flag: %+v", safe)
	}
}

func TestPrepareSessionBackupDir(t *testing.T) {
	base := t.TempDir()
	old := backupsBaseDirForTest
	backupsBaseDirForTest = base
	defer func() { backupsBaseDirForTest = old }()

	ts := time.Date(2026, 7, 29, 20, 15, 30, 0, time.UTC)
	dir, err := prepareSessionBackupDir("central hub BA", ts)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "central_hub_BA", "20260729-201530")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		t.Fatalf("dir not created: %v", err)
	}
}

func TestExportTextToLocalAPIStream(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if command == "/export" {
			return &client.Result{Sentences: []map[string]string{{"ret": "# exported\n/ip address\n"}}}, nil
		}
		return nil, fmt.Errorf("unexpected %s", command)
	}
	dir := t.TempDir()
	out, n, err := exportTextToLocal(context.Background(), mock, "lab", exportTextOptions{
		DestPath: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected bytes")
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/ip address") {
		t.Fatalf("content: %q", data)
	}
}

func TestExportTextToLocalAPIFailure(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		if command == "/export" && len(args) == 0 {
			return &client.Result{}, nil // empty stream
		}
		if command == "/export" {
			return nil, errors.New("export denied")
		}
		return &client.Result{}, nil
	}
	_, _, err := exportTextToLocal(context.Background(), mock, "lab", exportTextOptions{
		DestPath: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exporting configuration") {
		t.Fatalf("got %v", err)
	}
}

func TestRunSessionBegin_ProdBackupSuccess(t *testing.T) {
	a, _ := testApp(t)
	dev := a.Config.Devices["lab"]
	dev.EnvClass = config.EnvClassProd
	a.Config.Devices["lab"] = dev

	base := t.TempDir()
	oldBase := backupsBaseDirForTest
	backupsBaseDirForTest = base
	defer func() { backupsBaseDirForTest = oldBase }()

	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, destDir string, _ io.Writer) (string, error) {
		path := filepath.Join(destDir, "ros-export.rsc")
		if err := os.WriteFile(path, []byte("# ok\n"), 0o600); err != nil {
			return "", err
		}
		return path, nil
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	if err := runSessionBegin(context.Background(), a, "lab", true, false, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Pre-session backup saved") {
		t.Fatalf("out=%s", out.String())
	}
	sess, err := a.Sessions.Active("lab")
	if err != nil || sess == nil {
		t.Fatalf("active: %v %v", sess, err)
	}
	if sess.BackupDir == "" {
		t.Fatal("expected backup_dir on session")
	}
	if !strings.HasPrefix(sess.BackupDir, base) {
		t.Fatalf("backup_dir=%q", sess.BackupDir)
	}
	if sess.Note != "" {
		t.Fatalf("note should be empty, got %q", sess.Note)
	}
}

func TestRunSessionBegin_ProdBackupFailureRefuses(t *testing.T) {
	a, _ := testApp(t)
	dev := a.Config.Devices["lab"]
	dev.EnvClass = config.EnvClassProd
	a.Config.Devices["lab"] = dev

	base := t.TempDir()
	oldBase := backupsBaseDirForTest
	backupsBaseDirForTest = base
	defer func() { backupsBaseDirForTest = oldBase }()

	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, _ string, _ io.Writer) (string, error) {
		return "", errors.New("sftp boom")
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	err := runSessionBegin(context.Background(), a, "lab", true, false, &out, &errBuf)
	if err == nil {
		t.Fatal("expected refuse")
	}
	if !strings.Contains(err.Error(), "refusing session begin") || !strings.Contains(err.Error(), "sftp boom") {
		t.Fatalf("got %v", err)
	}
	sess, _ := a.Sessions.Active("lab")
	if sess != nil {
		t.Fatal("session must not start after backup failure")
	}
}

func TestRunSessionBegin_ForceNoBackupWarns(t *testing.T) {
	a, _ := testApp(t)
	dev := a.Config.Devices["lab"]
	dev.EnvClass = config.EnvClassProd
	a.Config.Devices["lab"] = dev

	called := false
	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, _ string, _ io.Writer) (string, error) {
		called = true
		return "", errors.New("should not run")
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	if err := runSessionBegin(context.Background(), a, "lab", true, true, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("backup should be skipped")
	}
	if !strings.Contains(errBuf.String(), "WARNING") || !strings.Contains(errBuf.String(), "force-no-backup") {
		t.Fatalf("stderr=%s", errBuf.String())
	}
	sess, err := a.Sessions.Active("lab")
	if err != nil || sess == nil {
		t.Fatal("expected session")
	}
	if sess.Note != forceNoBackupNote {
		t.Fatalf("note=%q", sess.Note)
	}
	if sess.BackupDir != "" {
		t.Fatalf("backup_dir should be empty")
	}
}

func TestRunSessionBegin_LabSkipsBackup(t *testing.T) {
	a, _ := testApp(t)
	called := false
	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, _ string, _ io.Writer) (string, error) {
		called = true
		return "", nil
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	if err := runSessionBegin(context.Background(), a, "lab", true, false, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("lab should not require backup")
	}
}

func TestRunSessionBegin_StagingRequireFlag(t *testing.T) {
	a, _ := testApp(t)
	dev := a.Config.Devices["lab"]
	dev.EnvClass = config.EnvClassStaging
	dev.RequireBackupBeforeWrite = true
	a.Config.Devices["lab"] = dev

	base := t.TempDir()
	oldBase := backupsBaseDirForTest
	backupsBaseDirForTest = base
	defer func() { backupsBaseDirForTest = oldBase }()

	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, destDir string, _ io.Writer) (string, error) {
		path := filepath.Join(destDir, "x.rsc")
		_ = os.WriteFile(path, []byte("x"), 0o600)
		return path, nil
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	if err := runSessionBegin(context.Background(), a, "lab", true, false, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	sess, _ := a.Sessions.Active("lab")
	if sess == nil || sess.BackupDir == "" {
		t.Fatal("expected backup for staging with require flag")
	}
}

func TestRunSessionBegin_ROSStrictRequiresBackup(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("ROS_STRICT", "1")

	base := t.TempDir()
	oldBase := backupsBaseDirForTest
	backupsBaseDirForTest = base
	defer func() { backupsBaseDirForTest = oldBase }()

	oldFn := preSessionBackupFn
	preSessionBackupFn = func(_ context.Context, _ *App, _ string, destDir string, _ io.Writer) (string, error) {
		path := filepath.Join(destDir, "x.rsc")
		_ = os.WriteFile(path, []byte("x"), 0o600)
		return path, nil
	}
	defer func() { preSessionBackupFn = oldFn }()

	var out, errBuf bytes.Buffer
	if err := runSessionBegin(context.Background(), a, "lab", true, false, &out, &errBuf); err != nil {
		t.Fatal(err)
	}
	sess, _ := a.Sessions.Active("lab")
	if sess == nil || sess.BackupDir == "" {
		t.Fatal("ROS_STRICT should require backup")
	}
}
