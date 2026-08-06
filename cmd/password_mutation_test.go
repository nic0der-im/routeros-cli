package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/spf13/cobra"
)

func mutationTestCommand(t *testing.T, dryRun bool) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "mutation"}
	attachDryRunFlag(cmd)
	if dryRun {
		if err := cmd.PersistentFlags().Set(dryRunFlag, "true"); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd, &out
}

func TestGenericPasswordStdinFlags(t *testing.T) {
	for _, cmd := range []*cobra.Command{newCreateCmd(), newSetCmd()} {
		if cmd.PersistentFlags().Lookup(passwordStdinFlag) == nil {
			t.Errorf("%s: missing --%s", cmd.Name(), passwordStdinFlag)
		}
	}
}

func TestReadMutationPasswordValidation(t *testing.T) {
	tests := []struct {
		name  string
		verb  string
		path  string
		args  []string
		input string
		want  string
		err   string
	}{
		{name: "create user", verb: "create", path: "/user", input: "stdin-secret\n", want: "stdin-secret"},
		{name: "set user", verb: "set", path: "/user/", input: "stdin-secret\r\n", want: "stdin-secret"},
		{name: "empty", verb: "create", path: "/user", input: "\n", err: "empty password"},
		{name: "extra line", verb: "set", path: "/user", input: "secret\nextra\n", err: "extra or malformed input"},
		{name: "NUL", verb: "create", path: "/user", input: "secret\x00\n", err: "NUL byte"},
		{name: "duplicate positional", verb: "set", path: "/user", args: []string{"name=tech", "password=argv-secret"}, input: "stdin-secret\n", err: "cannot be combined"},
		{name: "unsupported domain", verb: "create", path: "/user/group", input: "stdin-secret\n", err: "only supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readMutationPasswordFrom(tt.verb, tt.path, parseAPIArgs(tt.args), true, strings.NewReader(tt.input))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error=%v, want %q", err, tt.err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("password accepted=%v err=%v", got != "", err)
			}
		})
	}
}

func TestGenericCreateUserPasswordStdin(t *testing.T) {
	app, mock := testApp(t)
	app.OutFormat = output.FormatJSON
	secret := "create-secret"
	var gotArgs []string
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		switch command {
		case "/user/print":
			return &client.Result{}, nil
		case "/user/add":
			gotArgs = append([]string(nil), args...)
			return &client.Result{Sentences: []map[string]string{{"ret": "*7"}}}, nil
		default:
			return nil, fmt.Errorf("unexpected command %q", command)
		}
	}
	cmd, out := mutationTestCommand(t, false)
	if err := applyCreateMutationWithPassword(context.Background(), app, mock, cmd, "lab", "/user", "/user/add", []string{
		"=name=tech", "=group=read", "=address=192.0.2.10",
	}, secret); err != nil {
		t.Fatal(err)
	}
	if !containsArg(gotArgs, "=password="+secret) {
		t.Fatalf("RouterOS args did not contain the stdin password")
	}
	if strings.Contains(out.String(), secret) || !strings.Contains(out.String(), output.RedactedPlaceholder) {
		t.Fatalf("password was not safely rendered: %q", out.String())
	}
}

func TestGenericSetUserPasswordStdin(t *testing.T) {
	app, mock := testApp(t)
	app.OutFormat = output.FormatJSON
	secret := "set-secret"
	var gotArgs []string
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		switch command {
		case "/user/print":
			return &client.Result{Sentences: []map[string]string{{".id": "*2", "name": "tech", "group": "read"}}}, nil
		case "/user/set":
			gotArgs = append([]string(nil), args...)
			return &client.Result{}, nil
		default:
			return nil, fmt.Errorf("unexpected command %q", command)
		}
	}
	cmd, out := mutationTestCommand(t, false)
	if err := applySetMutationWithPassword(context.Background(), app, mock, cmd, "lab", "/user", "/user/set", []string{"=.id=*2"}, secret); err != nil {
		t.Fatal(err)
	}
	if !containsArg(gotArgs, "=password="+secret) {
		t.Fatalf("RouterOS args did not contain the stdin password")
	}
	if strings.Contains(out.String(), secret) || !strings.Contains(out.String(), output.RedactedPlaceholder) {
		t.Fatalf("password was not safely rendered: %q", out.String())
	}
}

func TestPasswordMutationDryRunRedacts(t *testing.T) {
	app, mock := testApp(t)
	app.OutFormat = output.FormatJSON
	secret := "dry-run-secret"
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if command == "/user/print" {
			return &client.Result{Sentences: []map[string]string{{".id": "*2", "name": "tech"}}}, nil
		}
		return nil, fmt.Errorf("unexpected mutation during dry-run")
	}
	cmd, out := mutationTestCommand(t, true)
	if err := applySetMutationWithPassword(context.Background(), app, mock, cmd, "lab", "/user", "/user/set", []string{"=.id=*2"}, secret); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), secret) || !strings.Contains(out.String(), output.RedactedPlaceholder) {
		t.Fatalf("dry-run leaked password: %q", out.String())
	}
	var response output.JSONResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordMutationSessionAndRollbackEvidenceRedact(t *testing.T) {
	app, mock := testApp(t)
	app.OutFormat = output.FormatTable
	secret := "session-secret"
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		switch command {
		case "/user/print":
			return &client.Result{Sentences: []map[string]string{{".id": "*2", "name": "tech"}}}, nil
		case "/user/set":
			return &client.Result{}, nil
		default:
			return nil, fmt.Errorf("unexpected command %q", command)
		}
	}
	if _, err := app.Sessions.Begin("lab", true); err != nil {
		t.Fatal(err)
	}
	cmd, out := mutationTestCommand(t, false)
	if err := applySetMutationWithPassword(context.Background(), app, mock, cmd, "lab", "/user", "/user/set", []string{"=.id=*2"}, secret); err != nil {
		t.Fatal(err)
	}
	sess, err := app.Sessions.Active("lab")
	if err != nil || sess == nil || len(sess.Changes) != 1 {
		t.Fatalf("session changes=%v err=%v", sess, err)
	}
	encoded, err := json.Marshal(sess)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || !strings.Contains(string(encoded), output.RedactedPlaceholder) {
		t.Fatalf("session journal leaked password: %s", encoded)
	}
	if len(sess.Changes[0].Inverse) != 0 {
		t.Fatalf("password-only change unexpectedly has rollback inverse: %v", sess.Changes[0].Inverse)
	}
	var rollback bytes.Buffer
	if err := applySessionRollback(context.Background(), app, mock, sess, &rollback); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rollback.String(), secret) {
		t.Fatalf("rollback evidence leaked password: %q", rollback.String())
	}
	if strings.Contains(out.String(), secret) {
		t.Fatalf("mutation output leaked password: %q", out.String())
	}
}

func TestPasswordMutationErrorRedacts(t *testing.T) {
	app, mock := testApp(t)
	secret := "error-secret"
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		if command == "/user/print" {
			return &client.Result{}, nil
		}
		return nil, fmt.Errorf("router rejected password %s", secret)
	}
	cmd, _ := mutationTestCommand(t, false)
	err := applyCreateMutationWithPassword(context.Background(), app, mock, cmd, "lab", "/user", "/user/add", []string{"=name=tech"}, secret)
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), output.RedactedPlaceholder) {
		t.Fatalf("error was not redacted: %v", err)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
