package session

import (
	"context"
	"fmt"
	"time"
)

// ProbeFunc dials/probes the device. Returns nil when healthy.
type ProbeFunc func(ctx context.Context) error

// RollbackFunc applies journal inverses for the session.
type RollbackFunc func(ctx context.Context, sess *Session) error

// WatchConfig controls heartbeat behavior.
type WatchConfig struct {
	Interval       time.Duration
	FailThreshold  int
	Probe          ProbeFunc
	Rollback       RollbackFunc
	OnLinkLost     func(sess *Session)
	OnRolledBack   func(sess *Session, err error)
}

// Watch runs until ctx is cancelled. On FailThreshold consecutive probe
// failures it marks auto_rollback_pending and attempts Rollback.
func Watch(ctx context.Context, store *Store, sess *Session, cfg WatchConfig) error {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.FailThreshold <= 0 {
		cfg.FailThreshold = 3
	}
	if cfg.Probe == nil {
		return fmt.Errorf("watch: probe required")
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, cfg.Interval)
			err := cfg.Probe(probeCtx)
			cancel()
			if err == nil {
				failures = 0
				continue
			}
			failures++
			if failures < cfg.FailThreshold {
				continue
			}
			if cfg.OnLinkLost != nil {
				cfg.OnLinkLost(sess)
			}
			_ = store.MarkAutoRollbackPending(sess)
			var rbErr error
			if cfg.Rollback != nil {
				rbErr = cfg.Rollback(ctx, sess)
			}
			if rbErr == nil {
				_ = store.MarkRolledBack(sess)
			}
			if cfg.OnRolledBack != nil {
				cfg.OnRolledBack(sess, rbErr)
			}
			return rbErr
		}
	}
}
