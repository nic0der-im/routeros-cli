package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestResolveIDByComment_OneMatch(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if !strings.HasSuffix(command, "/print") {
			t.Fatalf("command=%q", command)
		}
		return &client.Result{Sentences: []map[string]string{
			{"comment": "allow-web", "chain": "forward", ".id": "*A"},
			{"comment": "drop-bogons", "chain": "input", ".id": "*B"},
		}}, nil
	}
	id, err := resolveIDByComment(context.Background(), mock, "/ip/firewall/filter", "allow-web")
	if err != nil {
		t.Fatal(err)
	}
	if id != "*A" {
		t.Fatalf("id=%q", id)
	}
}

func TestResolveIDByComment_ZeroMatches(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{
			{"comment": "other", ".id": "*1"},
		}}, nil
	}
	_, err := resolveIDByComment(context.Background(), mock, "/ip/firewall/filter", "missing")
	if err == nil {
		t.Fatal("expected not found")
	}
	kind, ok := apperr.AsKind(err)
	if !ok || kind != apperr.KindNotFound {
		t.Fatalf("kind=%v ok=%v err=%v", kind, ok, err)
	}
}

func TestResolveIDByComment_ManyMatches(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{
			{"comment": "dup", ".id": "*1"},
			{"comment": "dup", ".id": "*2"},
		}}, nil
	}
	_, err := resolveIDByComment(context.Background(), mock, "/ip/firewall/mangle", "dup")
	if err == nil {
		t.Fatal("expected conflict")
	}
	kind, ok := apperr.AsKind(err)
	if !ok || kind != apperr.KindConflict {
		t.Fatalf("kind=%v ok=%v err=%v", kind, ok, err)
	}
	if !strings.Contains(err.Error(), "*1") || !strings.Contains(err.Error(), "*2") {
		t.Fatalf("error should list ids: %v", err)
	}
}

func TestResolveIDByComment_ExactCaseSensitive(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{
			{"comment": "Allow-Web", ".id": "*9"},
		}}, nil
	}
	_, err := resolveIDByComment(context.Background(), mock, "/ip/firewall/filter", "allow-web")
	if err == nil {
		t.Fatal("expected not found for case mismatch")
	}
	id, err := resolveIDByComment(context.Background(), mock, "/ip/firewall/filter", "Allow-Web")
	if err != nil {
		t.Fatal(err)
	}
	if id != "*9" {
		t.Fatalf("id=%q", id)
	}
}

func TestSupportsCommentAsID(t *testing.T) {
	if !supportsCommentAsID("/ip/firewall/filter") || !supportsCommentAsID("ip/firewall/mangle") {
		t.Fatal("filter/mangle should support comment-as-id")
	}
	if supportsCommentAsID("/ip/firewall/nat") || supportsCommentAsID("/ip/firewall/address-list") {
		t.Fatal("nat/address-list must not use comment-as-id in B5")
	}
}

func TestFirewallFilterRemove_ByComment(t *testing.T) {
	a, mock := testApp(t)
	a.OutFormat = output.FormatTable
	var removedID string
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		switch {
		case strings.HasSuffix(command, "/print"):
			return &client.Result{Sentences: []map[string]string{
				{"comment": "allow-web", ".id": "*A", "chain": "forward"},
			}}, nil
		case strings.HasSuffix(command, "/remove"):
			if len(args) < 1 || args[0] != "=.id=*A" {
				t.Fatalf("remove args=%v", args)
			}
			removedID = "*A"
			return &client.Result{}, nil
		default:
			t.Fatalf("unexpected command %s", command)
			return nil, nil
		}
	}

	fw := newFirewallCmd()
	filter := findSub(t, fw, "filter")
	remove := findSub(t, filter, "remove")
	if remove.Flags().Lookup("comment") == nil {
		t.Fatal("missing --comment on filter remove")
	}
	if remove.Flags().Lookup("id") == nil {
		t.Fatal("missing --id on filter remove")
	}
	if remove.Flags().Lookup("dry-run") == nil && filter.PersistentFlags().Lookup("dry-run") == nil {
		t.Fatal("missing --dry-run on filter remove/parent")
	}

	var buf bytes.Buffer
	remove.SetOut(&buf)
	remove.SetErr(&buf)
	if err := remove.Flags().Set("comment", "allow-web"); err != nil {
		t.Fatal(err)
	}
	if err := remove.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}

	// Invoke Run via the command with a mocked client path: call apply path directly
	// through the same resolve + delete helper used by the command.
	err := func() error {
		ctx := context.Background()
		resolved, err := resolveMutateTargetID(ctx, mock, "/ip/firewall/filter", "", "allow-web")
		if err != nil {
			return err
		}
		apiArgs := []string{"=.id=" + resolved}
		return applyDeleteMutation(ctx, a, mock, remove, "lab", "/ip/firewall/filter",
			"/ip/firewall/filter/remove", apiArgs, resolved)
	}()
	if err != nil {
		t.Fatal(err)
	}
	if removedID != "*A" {
		t.Fatalf("removedID=%q", removedID)
	}
}

func TestFirewallFilterEnable_FlagPresence(t *testing.T) {
	fw := newFirewallCmd()
	filter := findSub(t, fw, "filter")
	for _, name := range []string{"enable", "disable", "remove"} {
		sub := findSub(t, filter, name)
		if sub.Flags().Lookup("comment") == nil {
			t.Errorf("%s missing --comment", name)
		}
		if sub.Flags().Lookup("id") == nil {
			t.Errorf("%s missing --id", name)
		}
	}
}

func TestDeleteFirewallFilter_CommentFlag(t *testing.T) {
	del := newDeleteCmd()
	fw := findSub(t, del, "firewall")
	filter := findSub(t, fw, "filter")
	if filter.Flags().Lookup("comment") == nil {
		t.Fatal("delete firewall filter missing --comment")
	}
	mangle := findSub(t, fw, "mangle")
	if mangle.Flags().Lookup("comment") == nil {
		t.Fatal("delete firewall mangle missing --comment")
	}
	nat := findSub(t, fw, "nat")
	if nat.Flags().Lookup("comment") != nil {
		t.Fatal("delete firewall nat must not gain --comment in B5")
	}
}

func TestGenericSetDelete_CommentFlag(t *testing.T) {
	set := newSetCmd()
	if set.Flags().Lookup("comment") == nil {
		t.Fatal("set missing --comment")
	}
	del := newDeleteCmd()
	if del.Flags().Lookup("comment") == nil {
		t.Fatal("delete missing --comment")
	}
	en := newEnableCmd()
	if en.Flags().Lookup("comment") == nil {
		t.Fatal("enable missing --comment")
	}
	dis := newDisableCmd()
	if dis.Flags().Lookup("comment") == nil {
		t.Fatal("disable missing --comment")
	}
}

func TestGenericDelete_ResolveCommentThenDryRun(t *testing.T) {
	a, mock := testApp(t)
	prints := 0
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		if strings.HasSuffix(command, "/remove") {
			t.Fatal("dry-run must not remove")
		}
		if strings.HasSuffix(command, "/print") {
			prints++
			if len(args) == 1 && strings.HasPrefix(args[0], "?.id=") {
				return &client.Result{Sentences: []map[string]string{
					{".id": "*A", "comment": "allow-web", "chain": "forward"},
				}}, nil
			}
			return &client.Result{Sentences: []map[string]string{
				{"comment": "allow-web", ".id": "*A", "chain": "forward"},
			}}, nil
		}
		t.Fatalf("unexpected %s %v", command, args)
		return nil, nil
	}

	cmd := &cobra.Command{Use: "delete"}
	attachDryRunFlag(cmd)
	attachCommentTargetFlag(cmd)
	if err := cmd.PersistentFlags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("comment", "allow-web"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	ctx := context.Background()
	path := "/ip/firewall/filter"
	resolved, err := resolveMutateTargetID(ctx, mock, path, "", getCommentTarget(cmd))
	if err != nil {
		t.Fatal(err)
	}
	apiArgs := ensureIDArg(nil, resolved)
	err = applyDeleteMutation(ctx, a, mock, cmd, "lab", path, path+"/remove", apiArgs, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if prints == 0 {
		t.Fatal("expected print for comment resolve / pre-state")
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Fatalf("output=%q", buf.String())
	}
}

func findSub(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("missing subcommand %q under %s", name, parent.Name())
	return nil
}
