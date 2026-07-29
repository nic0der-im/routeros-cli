package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestDryRunFlagExists(t *testing.T) {
	cmds := []*cobra.Command{
		newCreateCmd(),
		newSetCmd(),
		newDeleteCmd(),
		newEnableCmd(),
		newDisableCmd(),
	}
	for _, cmd := range cmds {
		f := cmd.PersistentFlags().Lookup(dryRunFlag)
		if f == nil {
			f = cmd.Flags().Lookup(dryRunFlag)
		}
		if f == nil {
			t.Errorf("%s: missing --%s flag", cmd.Name(), dryRunFlag)
		}
	}
}

func TestDryRunFlagParsing(t *testing.T) {
	cmd := newSetCmd()
	if err := cmd.PersistentFlags().Parse([]string{"--" + dryRunFlag}); err != nil {
		t.Fatal(err)
	}
	if !isDryRun(cmd) {
		t.Fatal("expected isDryRun after parsing --dry-run")
	}

	cmd2 := newSetCmd()
	if isDryRun(cmd2) {
		t.Fatal("expected dry-run false by default")
	}
}

func TestDryRunFlagInheritedByCuratedChild(t *testing.T) {
	setCmd := newSetCmd()
	var identity *cobra.Command
	for _, c := range setCmd.Commands() {
		if c.Name() == "identity" {
			identity = c
			break
		}
	}
	if identity == nil {
		t.Fatal("set identity child not found")
	}
	if setCmd.PersistentFlags().Lookup(dryRunFlag) == nil {
		t.Fatal("set parent missing persistent --dry-run")
	}
	if err := setCmd.PersistentFlags().Set(dryRunFlag, "true"); err != nil {
		t.Fatal(err)
	}
	// AddCommand wires parent; Flag() walks ancestors' persistent flags.
	if identity.Parent() != setCmd {
		t.Fatalf("identity parent=%v want set", identity.Parent())
	}
	if !isDryRun(identity) {
		t.Fatal("expected curated child to inherit parent --dry-run")
	}
}

func TestFormatHumanAPIArgs(t *testing.T) {
	got := formatHumanAPIArgs([]string{"=ddns-enabled=auto", "=.id=*1", "?ignored=1"})
	want := "ddns-enabled=auto .id=*1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPropertyChanges(t *testing.T) {
	pre := map[string]string{"ddns-enabled": "yes", "update-time": "true"}
	got := propertyChanges(pre, []string{"=ddns-enabled=auto", "=update-time=false", "=.id=*1"})
	if len(got) != 2 {
		t.Fatalf("changes=%v", got)
	}
	if got[0].Key != "ddns-enabled" || got[0].From != "yes" || got[0].To != "auto" {
		t.Fatalf("first change: %+v", got[0])
	}
}

func TestEmitDryRunHumanAndJSON(t *testing.T) {
	a, _ := testApp(t)
	a.OutFormat = output.FormatTable
	var buf bytes.Buffer
	err := a.emitDryRun(&buf, "lab", dryRunSpec{
		Verb:    "set",
		Path:    "/ip/cloud",
		Command: "/ip/cloud/set",
		Args:    []string{"=ddns-enabled=auto"},
		Pre:     map[string]string{"ddns-enabled": "yes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "dry-run: would set /ip/cloud ddns-enabled=auto") {
		t.Fatalf("human summary missing: %q", out)
	}
	if !strings.Contains(out, "ddns-enabled: yes → auto") {
		t.Fatalf("property diff missing: %q", out)
	}

	a.OutFormat = output.FormatJSON
	buf.Reset()
	err = a.emitDryRun(&buf, "lab", dryRunSpec{
		Verb:    "set",
		Path:    "/ip/cloud",
		Command: "/ip/cloud/set",
		Args:    []string{"=ddns-enabled=auto"},
		Pre:     map[string]string{"ddns-enabled": "yes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if resp.Meta.Action != dryRunAction {
		t.Fatalf("meta.action=%q", resp.Meta.Action)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type %T", resp.Data)
	}
	if data["action"] != dryRunAction {
		t.Fatalf("data.action=%v", data["action"])
	}
}

func TestApplySetMutationDryRunSkipsWriteAndJournal(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable

	_, err := a.Sessions.Begin("lab", true)
	if err != nil {
		t.Fatal(err)
	}

	writes := 0
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/set") || strings.HasSuffix(command, "/add") ||
			strings.HasSuffix(command, "/remove") || strings.HasSuffix(command, "/enable") ||
			strings.HasSuffix(command, "/disable") {
			writes++
			t.Fatalf("unexpected mutating Run: %s", command)
		}
		// Allow pre-state print.
		return &client.Result{Sentences: []map[string]string{
			{"ddns-enabled": "yes", "update-time": "true"},
		}}, nil
	}

	cmd := &cobra.Command{Use: "set"}
	attachDryRunFlag(cmd)
	if err := cmd.PersistentFlags().Set(dryRunFlag, "true"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = applySetMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/cloud", "/ip/cloud/set",
		[]string{"=ddns-enabled=auto", "=update-time=false"})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatalf("writes=%d", writes)
	}
	if !strings.Contains(buf.String(), "dry-run: would set /ip/cloud") {
		t.Fatalf("output: %q", buf.String())
	}

	sess, err := a.Sessions.Active("lab")
	if err != nil {
		t.Fatal(err)
	}
	if sess == nil {
		t.Fatal("expected active session")
	}
	if len(sess.Changes) != 0 {
		t.Fatalf("dry-run must not journal; changes=%d", len(sess.Changes))
	}
}

func TestApplyCreateMutationDryRunSkipsWrite(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		t.Fatalf("unexpected Run during create dry-run: %s", command)
		return nil, nil
	}
	cmd := &cobra.Command{Use: "create"}
	attachDryRunFlag(cmd)
	_ = cmd.PersistentFlags().Set(dryRunFlag, "true")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/address", "/ip/address/add",
		[]string{"=address=10.0.0.1/24", "=interface=bridge"})
	if err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Action != dryRunAction {
		t.Fatalf("action=%q", resp.Meta.Action)
	}
}

func TestApplyDeleteMutationDryRunSkipsWrite(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/remove") {
			t.Fatalf("unexpected remove: %s", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{".id": "*1", "address": "10.0.0.1/24"},
		}}, nil
	}
	cmd := &cobra.Command{Use: "delete"}
	attachDryRunFlag(cmd)
	_ = cmd.PersistentFlags().Set(dryRunFlag, "true")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyDeleteMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/address", "/ip/address/remove",
		[]string{"=.id=*1"}, "*1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "dry-run: would delete /ip/address .id=*1") {
		t.Fatalf("output: %q", buf.String())
	}
}

func TestApplyEnableDisableMutationDryRunSkipsWrite(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/enable") || strings.HasSuffix(command, "/disable") {
			t.Fatalf("unexpected mutate Run: %s", command)
		}
		// Pre-state print is allowed during dry-run.
		return &client.Result{Sentences: []map[string]string{
			{".id": "*2", "disabled": "true"},
		}}, nil
	}
	cmd := &cobra.Command{Use: "enable"}
	attachDryRunFlag(cmd)
	_ = cmd.PersistentFlags().Set(dryRunFlag, "true")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyEnableDisableMutation(context.Background(), a, mock, cmd, "lab",
		"/interface", "/interface/enable",
		[]string{"=.id=*2"}, "*2", "enable")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "dry-run: would enable /interface .id=*2") {
		t.Fatalf("output: %q", buf.String())
	}
}
