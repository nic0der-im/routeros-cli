package cmd

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

const (
	passwordCLIHelperEnv  = "ROS_PASSWORD_STDIN_PROCESS_HELPER"
	passwordCLIHelperCase = "ROS_PASSWORD_STDIN_PROCESS_CASE"
)

func TestPasswordStdinProcessExitMatrix(t *testing.T) {
	tests := []struct {
		name        string
		caseName    string
		input       string
		wantExit    int
		wantMessage string
		wantRedact  bool
	}{
		{name: "duplicate positional password", caseName: "duplicate", input: "stdin-value\n", wantExit: 1, wantMessage: "cannot be combined"},
		{name: "empty stdin", caseName: "empty", input: "\n", wantExit: 1, wantMessage: "empty password"},
		{name: "NUL stdin", caseName: "nul", input: "bad\x00value\n", wantExit: 1, wantMessage: "NUL byte"},
		{name: "extra stdin content", caseName: "extra", input: "one\ntwo\n", wantExit: 1, wantMessage: "extra or malformed input"},
		{name: "valid create dry-run", caseName: "create-valid", input: "create-sentinel\n", wantExit: 0, wantRedact: true},
		{name: "valid set dry-run", caseName: "set-valid", input: "set-sentinel\n", wantExit: 0, wantRedact: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := exec.Command(os.Args[0], "-test.run=^TestPasswordStdinProcessHelper$")
			proc.Env = append(os.Environ(), passwordCLIHelperEnv+"=1", passwordCLIHelperCase+"="+tt.caseName)
			proc.Stdin = strings.NewReader(tt.input)
			output, err := proc.CombinedOutput()
			if exit := processExitCode(err); exit != tt.wantExit {
				t.Fatalf("exit=%d want=%d", exit, tt.wantExit)
			}
			if strings.Contains(string(output), "Usage:") || strings.Contains(string(output), "Available Commands:") {
				t.Fatalf("usage output leaked into process result")
			}
			if tt.wantMessage != "" && !strings.Contains(string(output), tt.wantMessage) {
				t.Fatalf("validation message missing")
			}
			for _, sentinel := range []string{"create-sentinel", "set-sentinel", "stdin-value", "argv-value", "bad\x00value", "one", "two"} {
				if strings.Contains(string(output), sentinel) {
					t.Fatalf("sentinel leaked in process output")
				}
			}
			if tt.wantRedact && !strings.Contains(string(output), "***") {
				t.Fatalf("redaction marker missing from valid dry-run output")
			}
		})
	}
}

// TestPasswordStdinProcessHelper is the child process for the exit matrix.
func TestPasswordStdinProcessHelper(t *testing.T) {
	if os.Getenv(passwordCLIHelperEnv) != "1" {
		return
	}

	var cmd *cobra.Command
	switch os.Getenv(passwordCLIHelperCase) {
	case "duplicate":
		cmd = newCreateCmd()
		cmd.SetArgs([]string{"user", "name=tech", "password=argv-value", "--password-stdin"})
	case "empty":
		cmd = newCreateCmd()
		cmd.SetArgs([]string{"user", "name=tech", "--password-stdin", "--dry-run"})
	case "nul", "extra":
		cmd = newSetCmd()
		cmd.SetArgs([]string{"user", ".id=*2", "--password-stdin", "--dry-run"})
	case "create-valid":
		cmd = newCreateCmd()
		cmd.SetArgs([]string{"user", "name=tech", "group=read", "--password-stdin", "--dry-run"})
	case "set-valid":
		cmd = newSetCmd()
		cmd.SetArgs([]string{"user", ".id=*2", "--password-stdin", "--dry-run"})
	default:
		os.Exit(1)
	}
	// Isolate the generic RunE path; curated child dispatch is tested elsewhere.
	cmd.RemoveCommand(cmd.Commands()...)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	runWithClientFn = func(command *cobra.Command, _ string, fn func(context.Context, *App, client.Client, string) error) {
		_ = command
		app, mock := testApp(t)
		if err := fn(context.Background(), app, mock, "lab"); err != nil {
			os.Exit(1)
		}
	}
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
