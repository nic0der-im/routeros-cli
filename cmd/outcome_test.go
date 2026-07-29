package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/diff"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestClassifyCreateOutcome_AlreadyExists(t *testing.T) {
	existing := []map[string]string{
		{"list": "blacklist", "address": "1.2.3.4", ".id": "*9"},
	}
	desired := map[string]string{"list": "blacklist", "address": "1.2.3.4"}
	action, d, id := classifyCreateOutcome("/ip/firewall/address-list", existing, desired)
	if action != ActionAlreadyExists {
		t.Fatalf("action=%q", action)
	}
	if id != "*9" {
		t.Fatalf("id=%q", id)
	}
	if !d.Empty() || !diffHasWarning(d, diff.WarnAlreadyExists) {
		t.Fatalf("diff=%+v", d)
	}
}

func TestClassifyCreateOutcome_Created(t *testing.T) {
	action, d, id := classifyCreateOutcome("/ip/firewall/address-list", nil, map[string]string{
		"list": "blacklist", "address": "9.9.9.9",
	})
	if action != ActionCreated || id != "" {
		t.Fatalf("action=%q id=%q", action, id)
	}
	if len(d.ToCreate) != 1 {
		t.Fatalf("to_create=%d", len(d.ToCreate))
	}
}

func TestClassifySetOutcome_NoChange(t *testing.T) {
	pre := map[string]string{"ddns-enabled": "auto", "update-time": "false"}
	desired := map[string]string{"ddns-enabled": "auto", "update-time": "false"}
	action, d := classifySetOutcome("/ip/cloud", pre, desired)
	if action != ActionNoChange || !d.Empty() {
		t.Fatalf("action=%q diff=%+v", action, d)
	}
}

func TestClassifySetOutcome_Updated(t *testing.T) {
	pre := map[string]string{"ddns-enabled": "yes"}
	desired := map[string]string{"ddns-enabled": "auto"}
	action, d := classifySetOutcome("/ip/cloud", pre, desired)
	if action != ActionUpdated || len(d.ToUpdate) != 1 {
		t.Fatalf("action=%q diff=%+v", action, d)
	}
}

func TestClassifySetOutcome_NilPreIsUpdated(t *testing.T) {
	action, _ := classifySetOutcome("/ip/cloud", nil, map[string]string{"ddns-enabled": "auto"})
	if action != ActionUpdated {
		t.Fatalf("action=%q want updated when pre unknown", action)
	}
}

func TestEmitWriteOutcomeJSON(t *testing.T) {
	a, _ := testApp(t)
	a.OutFormat = output.FormatJSON
	var buf bytes.Buffer
	d := diff.DiffSet("/ip/cloud", map[string]string{"ddns-enabled": "yes"}, map[string]string{"ddns-enabled": "auto"})
	err := a.emitWriteOutcome(&buf, "lab", writeOutcomeSpec{
		Action:  ActionUpdated,
		Verb:    "set",
		Path:    "/ip/cloud",
		Command: "/ip/cloud/set",
		Args:    []string{"=ddns-enabled=auto"},
		Summary: "Updated /ip/cloud on lab",
		Diff:    &d,
	})
	if err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Action != ActionUpdated {
		t.Fatalf("meta.action=%q", resp.Meta.Action)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok || data["action"] != ActionUpdated {
		t.Fatalf("data=%v", resp.Data)
	}
}

func TestApplySetMutation_NoChangeSkipsWrite(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	writes := 0
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/set") {
			writes++
			t.Fatalf("unexpected set: %s", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"ddns-enabled": "auto", "update-time": "false"},
		}}, nil
	}
	cmd := &cobra.Command{Use: "set"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applySetMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/cloud", "/ip/cloud/set",
		[]string{"=ddns-enabled=auto", "=update-time=false"})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatalf("writes=%d", writes)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Action != ActionNoChange {
		t.Fatalf("action=%q", resp.Meta.Action)
	}
}

func TestApplyCreateMutation_AlreadyExistsSkipsWrite(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	writes := 0
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/add") {
			writes++
			t.Fatalf("unexpected add: %s", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"list": "blacklist", "address": "1.2.3.4", ".id": "*1"},
		}}, nil
	}
	cmd := &cobra.Command{Use: "create"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/firewall/address-list", "/ip/firewall/address-list/add",
		[]string{"=list=blacklist", "=address=1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatalf("writes=%d", writes)
	}
	if !strings.Contains(buf.String(), "Already exists") {
		t.Fatalf("output=%q", buf.String())
	}
}

func TestApplyCreateMutation_CreatedJSON(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/print") {
			return &client.Result{}, nil
		}
		if strings.HasSuffix(command, "/add") {
			return &client.Result{Sentences: []map[string]string{{"ret": "*42", ".id": "*42"}}}, nil
		}
		t.Fatalf("unexpected: %s", command)
		return nil, nil
	}
	cmd := &cobra.Command{Use: "create"}
	attachDryRunFlag(cmd)
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
	if resp.Meta.Action != ActionCreated {
		t.Fatalf("action=%q", resp.Meta.Action)
	}
	data := resp.Data.(map[string]interface{})
	if data["id"] != "*42" {
		t.Fatalf("id=%v", data["id"])
	}
}

func TestApplyDeleteMutation_RemovedJSON(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/print") {
			return &client.Result{Sentences: []map[string]string{
				{".id": "*1", "address": "10.0.0.1/24"},
			}}, nil
		}
		return &client.Result{}, nil
	}
	cmd := &cobra.Command{Use: "delete"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyDeleteMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/address", "/ip/address/remove",
		[]string{"=.id=*1"}, "*1")
	if err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Action != ActionRemoved {
		t.Fatalf("action=%q", resp.Meta.Action)
	}
}

func TestApplyDeleteMutation_NotFound(t *testing.T) {
	a, mock := testApp(t)
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/remove") {
			t.Fatal("should not remove when missing")
		}
		return &client.Result{}, nil
	}
	cmd := &cobra.Command{Use: "delete"}
	attachDryRunFlag(cmd)
	cmd.SetOut(io.Discard)

	err := applyDeleteMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/address", "/ip/address/remove",
		[]string{"=.id=*missing"}, "*missing")
	k, ok := apperr.AsKind(err)
	if !ok || k != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestApplySetMutation_AmbiguousWrite(t *testing.T) {
	a, mock := testApp(t)
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/print") {
			return &client.Result{Sentences: []map[string]string{
				{"ddns-enabled": "yes"},
			}}, nil
		}
		return nil, errors.New("read tcp: connection reset by peer")
	}
	cmd := &cobra.Command{Use: "set"}
	attachDryRunFlag(cmd)
	cmd.SetOut(io.Discard)

	err := applySetMutation(context.Background(), a, mock, cmd, "lab",
		"/ip/cloud", "/ip/cloud/set",
		[]string{"=ddns-enabled=auto"})
	k, ok := apperr.AsKind(err)
	if !ok || k != apperr.KindTimeout {
		t.Fatalf("kind=%v ok=%v err=%v", k, ok, err)
	}
	if apperr.AsSuggestedAction(err) != apperr.SuggestVerifyBeforeRetry {
		t.Fatalf("suggestion=%q", apperr.AsSuggestedAction(err))
	}
}

func TestOutcomeSummary(t *testing.T) {
	if got := outcomeSummary(ActionCreated, "create", "/ip/address", "*1"); !strings.Contains(got, ".id=*1") {
		t.Fatal(got)
	}
	if got := outcomeSummary(ActionNoChange, "set", "/ip/cloud", ""); got != "no change: /ip/cloud" {
		t.Fatal(got)
	}
}
