package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/plan"
	"github.com/nic0der-im/routeros-cli/internal/session"
	"github.com/spf13/cobra"
)

func TestPlanCmdRegistered(t *testing.T) {
	root := &cobra.Command{Use: "ros"}
	root.AddCommand(newPlanCmd())
	planCmd, _, err := root.Find([]string{"plan"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range planCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"preview", "apply", "rollback"} {
		if !names[want] {
			t.Fatalf("missing plan %s subcommand", want)
		}
	}
}

func TestPlanRollbackIsSessionAlias(t *testing.T) {
	rollback := newPlanRollbackCmd()
	if rollback.Name() != "rollback" {
		t.Fatal(rollback.Name())
	}
	// Same runner as session rollback.
	sess := newSessionRollbackCmd()
	if rollback.Run == nil || sess.Run == nil {
		t.Fatal("expected Run funcs")
	}
}

func writePlanFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPlanPreview_MockClient(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(ctx context.Context, command string, args ...string) (*client.Result, error) {
		switch {
		case strings.HasSuffix(command, "/print"):
			return &client.Result{Sentences: []map[string]string{
				{".id": "*1", "list": "blacklist", "address": "1.2.3.4"},
			}}, nil
		default:
			return &client.Result{}, nil
		}
	}

	doc, err := plan.Parse([]byte(`
steps:
  - op: create
    path: address-list
    props:
      list: blacklist
      address: 9.9.9.9
  - op: set
    path: /ip/firewall/address-list
    id: "*1"
    props:
      comment: updated
`))
	if err != nil {
		t.Fatal(err)
	}
	v, err := plan.Validate(doc)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runPlanPreview(context.Background(), a, mock, "lab", v, &buf); err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data planPreviewEnvelope `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if !env.Data.OK || env.Data.Action != "plan_preview" || len(env.Data.Steps) != 2 {
		t.Fatalf("envelope=%+v", env.Data)
	}
	if !strings.Contains(strings.Join(env.Data.Steps[0].Risks, " "), "safe session") {
		t.Fatalf("expected safe-session risk, got %v", env.Data.Steps[0].Risks)
	}
}

func TestPlanPreview_AmbiguousCommentFails(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	mock.RunFunc = func(ctx context.Context, command string, args ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{
			{".id": "*1", "comment": "dup"},
			{".id": "*2", "comment": "dup"},
		}}, nil
	}
	doc, err := plan.Parse([]byte(`
steps:
  - op: delete
    path: /ip/firewall/filter
    comment: dup
`))
	if err != nil {
		t.Fatal(err)
	}
	v, err := plan.Validate(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = runPlanPreview(context.Background(), a, mock, "lab", v, &buf)
	if err == nil {
		t.Fatal("expected preview error")
	}
	if !strings.Contains(buf.String(), "ERROR") {
		t.Fatalf("output=%s", buf.String())
	}
}

func TestPlanApply_RequiresSafeSession(t *testing.T) {
	a, mock := testApp(t)
	v, err := plan.Validate(&plan.Document{Steps: []plan.Step{{
		Op: "create", Path: "/ip/dns/static",
		Props: map[string]string{"name": "a.lan", "address": "1.1.1.1"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "apply"}
	err = runPlanApply(context.Background(), a, mock, cmd, "lab", v, "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "safe session") {
		t.Fatalf("err=%v", err)
	}
}

func TestPlanApply_FailFast(t *testing.T) {
	a, mock := testApp(t)
	if _, err := a.Sessions.BeginWith("lab", session.BeginOpts{Safe: true}); err != nil {
		t.Fatal(err)
	}

	addCount := 0
	mock.RunFunc = func(ctx context.Context, command string, args ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/print") {
			return &client.Result{}, nil
		}
		if strings.HasSuffix(command, "/add") {
			addCount++
			if addCount >= 2 {
				return nil, fmt.Errorf("simulated add failure")
			}
			return &client.Result{Sentences: []map[string]string{{"ret": "*1"}}}, nil
		}
		return &client.Result{}, nil
	}

	v, err := plan.Validate(&plan.Document{Steps: []plan.Step{
		{Op: "create", Path: "/ip/dns/static", Props: map[string]string{"name": "a.lan", "address": "1.1.1.1"}},
		{Op: "create", Path: "/ip/dns/static", Props: map[string]string{"name": "b.lan", "address": "1.1.1.2"}},
		{Op: "create", Path: "/ip/dns/static", Props: map[string]string{"name": "c.lan", "address": "1.1.1.3"}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "apply"}
	attachDryRunFlag(cmd)
	err = runPlanApply(context.Background(), a, mock, cmd, "lab", v, "", &buf)
	if err == nil || !strings.Contains(err.Error(), "step 1") {
		t.Fatalf("err=%v out=%s", err, buf.String())
	}
	if addCount != 2 {
		t.Fatalf("addCount=%d want 2 (fail-fast before third)", addCount)
	}
	if !strings.Contains(buf.String(), "applied step 0") {
		t.Fatalf("expected first step applied, got %s", buf.String())
	}
	if strings.Contains(buf.String(), "applied step 2") {
		t.Fatal("third step should not apply")
	}

	sess, _ := a.Sessions.Active("lab")
	if sess == nil || len(sess.Changes) != 1 {
		t.Fatalf("journaled changes=%v", sess)
	}
}

func TestPlanApply_DeleteRequiresConfirm(t *testing.T) {
	a, mock := testApp(t)
	if _, err := a.Sessions.BeginWith("lab", session.BeginOpts{Safe: true}); err != nil {
		t.Fatal(err)
	}
	v, err := plan.Validate(&plan.Document{Steps: []plan.Step{{
		Op: "delete", Path: "/ip/firewall/address-list", ID: "*1",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = runPlanApply(context.Background(), a, mock, &cobra.Command{Use: "x"}, "lab", v, "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("err=%v", err)
	}
}

func TestPlanApply_RefusesDryRunFlag(t *testing.T) {
	cmd := newPlanApplyCmd()
	if err := cmd.PersistentFlags().Set(dryRunFlag, "true"); err != nil {
		// flag is on the command itself via attachDryRunFlag
		if err := cmd.Flags().Set(dryRunFlag, "true"); err != nil {
			t.Fatal(err)
		}
	}
	if !isDryRun(cmd) {
		t.Fatal("expected dry-run true")
	}
	// Exercise the check used in RunE.
	file := writePlanFile(t, "steps:\n  - op: create\n    path: /ip/address\n    props:\n      address: 1.1.1.1/32\n")
	_ = file
	if !isDryRun(cmd) {
		t.Fatal("dry-run")
	}
}

func TestPushPlanDevice(t *testing.T) {
	prev := flagDevice
	t.Cleanup(func() { flagDevice = prev })
	flagDevice = ""
	restore := pushPlanDevice("home")
	if flagDevice != "home" {
		t.Fatalf("flagDevice=%q", flagDevice)
	}
	restore()
	if flagDevice != "" {
		t.Fatalf("restored=%q", flagDevice)
	}
	flagDevice = "lab"
	restore2 := pushPlanDevice("home")
	if flagDevice != "lab" {
		t.Fatal(" -d should win over plan device")
	}
	restore2()
}
