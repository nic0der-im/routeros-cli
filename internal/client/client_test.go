package client

import (
	"context"
	"fmt"
	"testing"
)

// Compile-time check: MockClient must satisfy Client.
var _ Client = (*MockClient)(nil)

func TestMockClient_RunReturnsConfiguredResult(t *testing.T) {
	mock := NewMockClient()

	expected := &Result{
		Sentences: []map[string]string{
			{"uptime": "3d12h", "version": "7.14"},
		},
	}

	mock.RunFunc = func(_ context.Context, command string, args ...string) (*Result, error) {
		if command != "/system/resource/print" {
			t.Errorf("expected command %q, got %q", "/system/resource/print", command)
		}
		return expected, nil
	}

	result, err := mock.Run(context.Background(), "/system/resource/print")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(result.Sentences) != 1 {
		t.Fatalf("expected 1 sentence, got %d", len(result.Sentences))
	}
	if result.Sentences[0]["uptime"] != "3d12h" {
		t.Errorf("expected uptime %q, got %q", "3d12h", result.Sentences[0]["uptime"])
	}
	if result.Sentences[0]["version"] != "7.14" {
		t.Errorf("expected version %q, got %q", "7.14", result.Sentences[0]["version"])
	}
}

func TestMockClient_RunReturnsError(t *testing.T) {
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*Result, error) {
		return nil, fmt.Errorf("connection refused")
	}

	_, err := mock.Run(context.Background(), "/system/identity/print")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection refused" {
		t.Errorf("expected error %q, got %q", "connection refused", err.Error())
	}
}

func TestMockClient_RunPassesArgs(t *testing.T) {
	mock := NewMockClient()

	var gotArgs []string
	mock.RunFunc = func(_ context.Context, _ string, args ...string) (*Result, error) {
		gotArgs = args
		return &Result{}, nil
	}

	_, err := mock.Run(context.Background(), "/ip/address/print", "=interface=ether1", "?disabled=false")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(gotArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(gotArgs))
	}
	if gotArgs[0] != "=interface=ether1" {
		t.Errorf("expected first arg %q, got %q", "=interface=ether1", gotArgs[0])
	}
	if gotArgs[1] != "?disabled=false" {
		t.Errorf("expected second arg %q, got %q", "?disabled=false", gotArgs[1])
	}
}

func TestMockClient_DefaultRunReturnsEmptyResult(t *testing.T) {
	mock := NewMockClient()

	result, err := mock.Run(context.Background(), "/system/identity/print")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Sentences) != 0 {
		t.Errorf("expected 0 sentences from default RunFunc, got %d", len(result.Sentences))
	}
}

func TestMockClient_CloseSetsFlag(t *testing.T) {
	mock := NewMockClient()

	if mock.Closed {
		t.Fatal("expected Closed to be false before Close()")
	}

	if err := mock.Close(); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}

	if !mock.Closed {
		t.Error("expected Closed to be true after Close()")
	}
}
