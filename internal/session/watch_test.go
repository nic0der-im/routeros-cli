package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchAutoRollbackAfterRecover(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.Begin("r1", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.AppendChange(sess, Change{
		Command: "/ip/address/add",
		Inverse: []string{"/ip/address/remove", "=.id=*1"},
	})

	var probes atomic.Int32
	var rolled atomic.Bool
	lost := false

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = Watch(ctx, store, sess, WatchConfig{
		Interval:      20 * time.Millisecond,
		FailThreshold: 2,
		Probe: func(ctx context.Context) error {
			n := probes.Add(1)
			// fail first 3 probes (link down), then recover
			if n <= 3 {
				return errors.New("down")
			}
			return nil
		},
		Rollback: func(ctx context.Context, s *Session) error {
			rolled.Store(true)
			return nil
		},
		OnLinkLost: func(s *Session) { lost = true },
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if probes.Load() < 4 {
		t.Fatalf("expected probes after recover, got %d", probes.Load())
	}
	if !lost {
		t.Fatal("expected OnLinkLost")
	}
	if !rolled.Load() {
		t.Fatal("expected rollback after recover")
	}
	got, _ := store.Get(sess.ID)
	if got.Status != StatusRolledBack {
		t.Fatalf("status=%s", got.Status)
	}
	if got.AutoRollbackPending {
		t.Fatal("expected pending cleared")
	}
}

func TestWatchStaysPendingWhileDown(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.Begin("r1", true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err = Watch(ctx, store, sess, WatchConfig{
		Interval:      20 * time.Millisecond,
		FailThreshold: 1,
		Probe:         func(ctx context.Context) error { return errors.New("down") },
		Rollback: func(ctx context.Context, s *Session) error {
			t.Fatal("rollback should not run while probe fails")
			return nil
		},
	})
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx end, got %v", err)
	}
	got, _ := store.Get(sess.ID)
	if !got.AutoRollbackPending {
		t.Fatal("expected auto_rollback_pending while still down")
	}
	if got.Status != StatusActive {
		t.Fatalf("status=%s", got.Status)
	}
}
