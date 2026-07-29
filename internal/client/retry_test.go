package client

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
)

func TestWithReadRetries_ReadFailsTwiceThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*Result, error) {
		if !strings.HasSuffix(command, "/print") {
			t.Fatalf("unexpected command %q", command)
		}
		n := attempts.Add(1)
		if n < 3 {
			return nil, errors.New("i/o timeout")
		}
		return &Result{Sentences: []map[string]string{{"ok": "1"}}}, nil
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
		sleep: func(ctx context.Context, d time.Duration) error {
			return nil // skip real waits in tests
		},
	})

	res, err := cli.Run(context.Background(), "/system/resource/print")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts=%d want 3", attempts.Load())
	}
	if len(res.Sentences) != 1 || res.Sentences[0]["ok"] != "1" {
		t.Fatalf("result=%v", res)
	}
}

func TestWithReadRetries_WriteFailsOnceNoRetry(t *testing.T) {
	var attempts atomic.Int32
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*Result, error) {
		attempts.Add(1)
		if !strings.HasSuffix(command, "/set") {
			t.Fatalf("unexpected command %q", command)
		}
		return nil, errors.New("i/o timeout")
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 5,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
		sleep: func(context.Context, time.Duration) error {
			t.Fatal("sleep must not be called for writes")
			return nil
		},
	})

	_, err := cli.Run(context.Background(), "/ip/address/set", "=.id=*1", "=comment=x")
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts=%d want 1 (no write retry)", attempts.Load())
	}
	if !apperr.IsAmbiguousTransport(err) && !strings.Contains(err.Error(), "timeout") {
		// Mock returns plain error; wrapper must not retry regardless.
		t.Logf("err=%v", err)
	}
}

func TestWithReadRetries_ContextCancelStopsRetries(t *testing.T) {
	var attempts atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*Result, error) {
		attempts.Add(1)
		return nil, errors.New("connection reset by peer")
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 5,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
		sleep: func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		},
	})

	_, err := cli.Run(ctx, "/interface/print")
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts=%d want 1 (stopped before retry Run)", attempts.Load())
	}
	// Last transport error is preferred over cancel when sleep aborts.
	if !strings.Contains(err.Error(), "connection reset") && !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestWithReadRetries_ContextCanceledBeforeAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var attempts atomic.Int32
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*Result, error) {
		attempts.Add(1)
		return nil, errors.New("should not run")
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
	})

	_, err := cli.Run(ctx, "/interface/print")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if attempts.Load() != 0 {
		t.Fatalf("attempts=%d want 0", attempts.Load())
	}
}

func TestWithReadRetries_NonTransientNoRetry(t *testing.T) {
	var attempts atomic.Int32
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*Result, error) {
		attempts.Add(1)
		return nil, errors.New("no such item (!trap)")
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 4,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
		sleep: func(context.Context, time.Duration) error {
			t.Fatal("must not sleep on non-transient")
			return nil
		},
	})

	_, err := cli.Run(context.Background(), "/ip/address/print")
	if err == nil || !strings.Contains(err.Error(), "no such item") {
		t.Fatalf("err=%v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
}

func TestWithReadRetries_AmbiguousWriteNeverRetried(t *testing.T) {
	var attempts atomic.Int32
	mock := NewMockClient()
	mock.RunFunc = func(_ context.Context, _ string, _ ...string) (*Result, error) {
		attempts.Add(1)
		return nil, apperr.WrapAmbiguousWrite(io.EOF)
	}

	cli := WithReadRetries(mock, ReadRetryConfig{
		MaxAttempts: 4,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  time.Millisecond,
		sleep: func(context.Context, time.Duration) error {
			t.Fatal("must not retry ambiguous write")
			return nil
		},
	})

	// Even if classification somehow treated this as a read path, A5 holds.
	_, err := cli.Run(context.Background(), "/system/resource/print")
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
	if apperr.AsSuggestedAction(err) != apperr.SuggestVerifyBeforeRetry {
		t.Fatalf("suggestion=%q", apperr.AsSuggestedAction(err))
	}
}

func TestIsTransientReadError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.Canceled, false},
		{errors.New("no such item"), false},
		{errors.New("i/o timeout"), true},
		{io.EOF, true},
		{errors.New("connection reset by peer"), true},
		{errors.New("device busy"), true},
		{apperr.New(apperr.KindBusy, "busy"), true},
		{apperr.WrapAmbiguousWrite(io.EOF), false},
	}
	for _, tc := range cases {
		if got := isTransientReadError(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
		}
	}
}

func TestIsMutatingCommand(t *testing.T) {
	if !isMutatingCommand("/ip/address/add") {
		t.Fatal("add")
	}
	if !isMutatingCommand("/ip/address/set") {
		t.Fatal("set")
	}
	if isMutatingCommand("/ip/address/print") {
		t.Fatal("print must not be mutating")
	}
	if isMutatingCommand("/interface/listen") {
		t.Fatal("listen must not be mutating")
	}
}

func TestReadRetryConfigFromEnv(t *testing.T) {
	t.Setenv("ROS_READ_RETRIES", "5")
	t.Setenv("ROS_READ_RETRY_BACKOFF", "100ms")
	cfg := ReadRetryConfigFromEnv("192.0.2.1:8728")
	if cfg.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts=%d", cfg.MaxAttempts)
	}
	if cfg.BaseBackoff != 100*time.Millisecond {
		t.Fatalf("BaseBackoff=%v", cfg.BaseBackoff)
	}
	if cfg.Address != "192.0.2.1:8728" {
		t.Fatalf("Address=%q", cfg.Address)
	}

	t.Setenv("ROS_READ_RETRIES", "0")
	cfg = ReadRetryConfigFromEnv("")
	if cfg.MaxAttempts != 1 {
		t.Fatalf("disabled MaxAttempts=%d want 1", cfg.MaxAttempts)
	}
}
