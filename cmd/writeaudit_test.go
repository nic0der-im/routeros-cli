package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/audit"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestEmitWriteOutcome_WritesAuditLine(t *testing.T) {
	t.Setenv("ROS_AUDIT", "")
	t.Setenv("ROS_NO_AUDIT", "")

	a, _ := testApp(t)
	a.RequestID = "audit-req-1"
	a.OutFormat = output.FormatJSON
	dir := a.AuditDir

	var buf bytes.Buffer
	err := a.emitWriteOutcome(&buf, "lab", writeOutcomeSpec{
		Action:  ActionUpdated,
		Verb:    "set",
		Path:    "/interface/wireguard",
		Command: "/interface/wireguard/set",
		Args:    []string{"=.id=*1", "=private-key=SUPERSECRET", "=comment=x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "writes-*.ndjson"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("audit files=%v err=%v", matches, err)
	}
	line := readFirstAuditLine(t, matches[0])
	if !strings.Contains(line, `"request_id":"audit-req-1"`) {
		t.Fatalf("missing request_id: %s", line)
	}
	if strings.Contains(line, "SUPERSECRET") {
		t.Fatalf("secret leaked: %s", line)
	}
	if !strings.Contains(line, output.RedactedPlaceholder) {
		t.Fatalf("expected redaction: %s", line)
	}
	var ev audit.Event
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Outcome != ActionUpdated || ev.Verb != "set" || ev.Device != "lab" {
		t.Fatalf("ev=%+v", ev)
	}
	if ev.Props["private-key"] != output.RedactedPlaceholder {
		t.Fatalf("props=%v", ev.Props)
	}
}

func TestEmitWriteOutcome_AuditDisabled(t *testing.T) {
	a, _ := testApp(t)
	dir := a.AuditDir
	t.Setenv("ROS_AUDIT", "0")

	a.RequestID = "no-audit"
	_ = a.emitWriteOutcome(ioDiscard{}, "lab", writeOutcomeSpec{
		Action: ActionNoChange, Verb: "set", Path: "/ip/cloud", Command: "/ip/cloud/set",
	})
	matches, _ := filepath.Glob(filepath.Join(dir, "writes-*.ndjson"))
	if len(matches) != 0 {
		t.Fatalf("expected no audit file when disabled, got %v", matches)
	}
}

func TestApplySetMutation_WritesAuditWithRequestID(t *testing.T) {
	t.Setenv("ROS_AUDIT", "")
	t.Setenv("ROS_NO_AUDIT", "")

	a, mock := testApp(t)
	dir := a.AuditDir
	a.RequestID = "mut-req-9"
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/print") {
			return &client.Result{Sentences: []map[string]string{{"ddns-enabled": "yes"}}}, nil
		}
		if strings.HasSuffix(command, "/set") {
			return &client.Result{}, nil
		}
		return &client.Result{}, nil
	}

	cmd := &cobra.Command{Use: "set"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := applySetMutation(context.Background(), a, mock, cmd, "lab", "/ip/cloud", "/ip/cloud/set",
		[]string{"=ddns-enabled=auto", "=password=SECRETPW"})
	if err != nil {
		t.Fatal(err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "writes-*.ndjson"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("audit files=%v err=%v", matches, err)
	}
	line := readFirstAuditLine(t, matches[0])
	if !strings.Contains(line, "mut-req-9") {
		t.Fatalf("missing request_id: %s", line)
	}
	if strings.Contains(line, "SECRETPW") {
		t.Fatalf("password leaked: %s", line)
	}
	if !strings.Contains(line, `"outcome":"updated"`) && !strings.Contains(line, `"action":"updated"`) {
		t.Fatalf("expected updated outcome: %s", line)
	}
}

func TestEmitDryRun_WritesAuditDryRun(t *testing.T) {
	t.Setenv("ROS_NO_AUDIT", "")
	t.Setenv("ROS_AUDIT", "")

	a, _ := testApp(t)
	dir := a.AuditDir
	a.RequestID = "dry-1"
	_ = a.emitDryRun(ioDiscard{}, "lab", dryRunSpec{
		Verb: "create", Path: "/ip/address", Command: "/ip/address/add",
		Args: []string{"=address=10.0.0.1/24", "=password=x"},
	})
	matches, _ := filepath.Glob(filepath.Join(dir, "writes-*.ndjson"))
	if len(matches) != 1 {
		t.Fatalf("files=%v", matches)
	}
	line := readFirstAuditLine(t, matches[0])
	if !strings.Contains(line, `"dry_run":true`) {
		t.Fatalf("dry_run missing: %s", line)
	}
	if strings.Contains(line, `"password":"x"`) || strings.Contains(line, "=password=x") {
		t.Fatalf("leaked: %s", line)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func readFirstAuditLine(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("empty audit file")
	}
	return sc.Text()
}
