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

func TestDNSStaticCommandTree(t *testing.T) {
	dns := newDNSCmd()
	var staticCmd *cobra.Command
	for _, c := range dns.Commands() {
		if c.Name() == "static" {
			staticCmd = c
			break
		}
	}
	if staticCmd == nil {
		t.Fatal("missing dns static")
	}
	if staticCmd.PersistentFlags().Lookup("dry-run") == nil {
		t.Fatal("missing --dry-run on dns static")
	}
	subs := map[string]bool{}
	for _, c := range staticCmd.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"list", "add", "set", "remove"} {
		if !subs[want] {
			t.Errorf("missing dns static %s", want)
		}
	}
}

func TestDNSStaticCreate_AlreadyExists(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	writes := 0
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/add") {
			writes++
			t.Fatalf("unexpected add: %s", command)
		}
		// SemanticKey is name+type (default A); address is payload only.
		return &client.Result{Sentences: []map[string]string{
			{"name": "router.lan", "address": "192.168.88.1", "type": "A", ".id": "*3"},
		}}, nil
	}
	cmd := &cobra.Command{Use: "add"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		dnsStaticPath, dnsStaticPath+"/add",
		[]string{"=name=router.lan", "=address=192.168.88.99"})
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

func TestDNSStaticCreate_DryRun(t *testing.T) {
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
		dnsStaticPath, dnsStaticPath+"/add",
		[]string{"=name=router.lan", "=address=192.168.88.1"})
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

func TestDNSStaticCreate_Created(t *testing.T) {
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
	cmd := &cobra.Command{Use: "add"}
	attachDryRunFlag(cmd)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := applyCreateMutation(context.Background(), a, mock, cmd, "lab",
		dnsStaticPath, dnsStaticPath+"/add",
		[]string{"=name=router.lan", "=address=192.168.88.1"})
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
}

func TestDNSStaticList_MockClient(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatJSON
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if command != dnsStaticPath+"/print" {
			t.Fatalf("command=%q", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"name": "router.lan", "address": "192.168.88.1", "type": "A", ".id": "*1"},
		}}, nil
	}
	var buf bytes.Buffer
	result, err := mock.Run(context.Background(), dnsStaticPath+"/print")
	if err != nil {
		t.Fatal(err)
	}
	if err := renderGenericResult(a, &buf, result, "lab", dnsStaticPath+"/print"); err != nil {
		t.Fatal(err)
	}
	var resp output.JSONResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Meta.Count != 1 {
		t.Fatalf("count=%d", resp.Meta.Count)
	}
}

func TestResolveIDBySemanticKey_DNSStatic(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{
			{"name": "router.lan", "type": "A", "address": "192.168.88.1", ".id": "*3"},
			{"name": "router.lan", "type": "AAAA", "address": "::1", ".id": "*4"},
		}}, nil
	}
	id, err := resolveIDBySemanticKey(context.Background(), mock, dnsStaticPath, map[string]string{
		"name": "router.lan",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "*3" {
		t.Fatalf("default type A id=%q", id)
	}
	id, err = resolveIDBySemanticKey(context.Background(), mock, dnsStaticPath, map[string]string{
		"name": "router.lan", "type": "AAAA",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "*4" {
		t.Fatalf("AAAA id=%q", id)
	}
}
