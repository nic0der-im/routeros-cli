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

func TestFirewallAddressListCommandTree(t *testing.T) {
	fw := newFirewallCmd()
	var addrList *cobra.Command
	for _, c := range fw.Commands() {
		if c.Name() == "address-list" {
			addrList = c
			break
		}
	}
	if addrList == nil {
		t.Fatal("missing firewall address-list")
	}
	if addrList.PersistentFlags().Lookup("dry-run") == nil {
		t.Fatal("missing --dry-run on address-list")
	}
	subs := map[string]bool{}
	for _, c := range addrList.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"list", "add", "remove", "set"} {
		if !subs[want] {
			t.Errorf("missing address-list %s", want)
		}
	}
}

func TestAddressListCreate_AlreadyExists(t *testing.T) {
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
	cmd := &cobra.Command{Use: "add"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		addressListPath, addressListPath+"/add",
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

func TestAddressListCreate_DryRun(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		t.Fatalf("unexpected API call during dry-run: %s", command)
		return nil, nil
	}
	cmd := &cobra.Command{Use: "add"}
	attachDryRunFlag(cmd)
	if err := cmd.PersistentFlags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		addressListPath, addressListPath+"/add",
		[]string{"=list=blacklist", "=address=1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Action != ActionDryRun {
		t.Fatalf("action=%q", resp.Meta.Action)
	}
}

func TestAddressListList_MockClient(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		if command != addressListPath+"/print" {
			t.Fatalf("command=%q", command)
		}
		if len(args) != 1 || args[0] != "?list=blacklist" {
			t.Fatalf("args=%v", args)
		}
		return &client.Result{Sentences: []map[string]string{
			{"list": "blacklist", "address": "1.2.3.4", ".id": "*1"},
			{"list": "blacklist", "address": "5.6.7.8", ".id": "*2"},
		}}, nil
	}
	var buf bytes.Buffer
	result, err := mock.Run(context.Background(), addressListPath+"/print", "?list=blacklist")
	if err != nil {
		t.Fatal(err)
	}
	if err := renderGenericResult(a, &buf, result, "lab", addressListPath+"/print"); err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Count != 2 {
		t.Fatalf("count=%d", resp.Meta.Count)
	}
}

func TestResolveIDBySemanticKey_AddressList(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if !strings.HasSuffix(command, "/print") {
			t.Fatalf("command=%q", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"list": "allow", "address": "10.0.0.1", ".id": "*7"},
			{"list": "blacklist", "address": "1.2.3.4", ".id": "*9"},
		}}, nil
	}
	id, err := resolveIDBySemanticKey(context.Background(), mock, addressListPath, map[string]string{
		"list": "blacklist", "address": "1.2.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "*9" {
		t.Fatalf("id=%q", id)
	}
	_, err = resolveIDBySemanticKey(context.Background(), mock, addressListPath, map[string]string{
		"list": "missing", "address": "9.9.9.9",
	})
	if err == nil {
		t.Fatal("expected not found")
	}
}
