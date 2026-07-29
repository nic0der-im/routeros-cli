package apperr

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestExitCode(t *testing.T) {
	if ExitCode(KindConnection) != 2 {
		t.Fatal()
	}
	if ExitCode(KindAuth) != 2 {
		t.Fatal()
	}
	if ExitCode(KindTimeout) != 2 {
		t.Fatal("timeout should map like connection (exit 2)")
	}
	if ExitCode(KindConfig) != 3 {
		t.Fatal()
	}
	if ExitCode(KindReadOnly) != 4 {
		t.Fatal()
	}
	if ExitCode(KindAPI) != 1 {
		t.Fatal()
	}
	if ExitCode(KindConflict) != 1 {
		t.Fatal()
	}
	if ExitCode(KindBusy) != 1 {
		t.Fatal()
	}
	if ExitCode(KindNotFound) != 1 {
		t.Fatal()
	}
}

func TestAsKind(t *testing.T) {
	err := Wrap(KindSession, "boom", nil)
	k, ok := AsKind(err)
	if !ok || k != KindSession {
		t.Fatalf("%v %v", k, ok)
	}
}

func TestSuggestedAction(t *testing.T) {
	err := New(KindConflict, "duplicate").WithSuggestedAction("use set instead of create")
	if err.SuggestedAction != "use set instead of create" {
		t.Fatalf("%q", err.SuggestedAction)
	}
	if AsSuggestedAction(err) != "use set instead of create" {
		t.Fatal(AsSuggestedAction(err))
	}
	if AsSuggestedAction(errors.New("plain")) != "" {
		t.Fatal("expected empty")
	}
}

func TestIsAmbiguousTransport(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("failure: unknown reply"), false},
		{context.DeadlineExceeded, true},
		{errors.New("running /ip/address/add: i/o timeout"), true},
		{errors.New("EOF"), true},
		{errors.New("read tcp: connection reset by peer"), true},
		{errors.New("write: broken pipe"), true},
		{io.EOF, true},
	}
	for _, tc := range cases {
		if got := IsAmbiguousTransport(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
		}
	}
}

func TestMaybeAmbiguousWrite(t *testing.T) {
	plain := errors.New("no such item")
	if MaybeAmbiguousWrite(plain) != plain {
		t.Fatal("non-ambiguous should pass through")
	}
	if MaybeAmbiguousWrite(nil) != nil {
		t.Fatal()
	}

	wrapped := MaybeAmbiguousWrite(errors.New("connection reset by peer"))
	k, ok := AsKind(wrapped)
	if !ok || k != KindTimeout {
		t.Fatalf("kind=%v ok=%v", k, ok)
	}
	if AsSuggestedAction(wrapped) != SuggestVerifyBeforeRetry {
		t.Fatalf("suggestion=%q", AsSuggestedAction(wrapped))
	}
	var e *Error
	if !errors.As(wrapped, &e) || e.Cause == nil {
		t.Fatal("expected cause")
	}
	if e.Cause.Error() != "connection reset by peer" {
		t.Fatalf("cause=%v", e.Cause)
	}
}

func TestWrapAmbiguousWriteMessage(t *testing.T) {
	err := WrapAmbiguousWrite(io.EOF)
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous write") || !strings.Contains(msg, "EOF") {
		t.Fatalf("message=%q", msg)
	}
	if AsSuggestedAction(err) != SuggestVerifyBeforeRetry {
		t.Fatal()
	}
}
