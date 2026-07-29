package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWatchAutoRollback(t *testing.T) {
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

	probes := 0
	rolled := false
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = Watch(ctx, store, sess, WatchConfig{
		Interval:      10 * time.Millisecond,
		FailThreshold: 2,
		Probe: func(ctx context.Context) error {
			probes++
			return errors.New("down")
		},
		Rollback: func(ctx context.Context, s *Session) error {
			rolled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if probes < 2 {
		t.Fatalf("expected >=2 probes, got %d", probes)
	}
	if !rolled {
		t.Fatal("expected rollback")
	}
	got, _ := store.Get(sess.ID)
	if got.Status != StatusRolledBack {
		t.Fatalf("status=%s", got.Status)
	}
}
