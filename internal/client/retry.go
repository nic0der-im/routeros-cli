package client

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
)

const (
	// DefaultReadRetries is the default max attempts for read-only API calls
	// (ROS_READ_RETRIES). Includes the first try; 3 ⇒ up to 2 retries.
	DefaultReadRetries = 3

	// DefaultReadRetryBackoff is the base exponential backoff
	// (ROS_READ_RETRY_BACKOFF). Jitter is applied per sleep.
	DefaultReadRetryBackoff = 75 * time.Millisecond

	// DefaultReadRetryMaxBackoff caps exponential backoff (~1s).
	DefaultReadRetryMaxBackoff = time.Second

	// softDeviceFailWindow adds a small delay when the same address failed
	// recently (in-memory soft per-device backoff).
	softDeviceFailWindow = 500 * time.Millisecond
)

// deviceLastFail tracks recent transient failures per ConnectConfig.Address.
var (
	deviceFailMu   sync.Mutex
	deviceLastFail = map[string]time.Time{}
)

// ReadRetryConfig controls automatic retries for non-mutating API calls.
type ReadRetryConfig struct {
	// MaxAttempts is total tries including the first (default 3). Values ≤1
	// disable retries. Overridden by ROS_READ_RETRIES when using FromEnv.
	MaxAttempts int
	// BaseBackoff is the exponential base delay (default 75ms).
	// Overridden by ROS_READ_RETRY_BACKOFF when using FromEnv.
	BaseBackoff time.Duration
	// MaxBackoff caps backoff (default 1s).
	MaxBackoff time.Duration
	// Address is ConnectConfig.Address for soft per-device backoff.
	Address string

	// sleep is overridable in tests; nil uses context-aware time.Timer sleep.
	sleep func(ctx context.Context, d time.Duration) error
}

// ReadRetryConfigFromEnv builds config from ROS_READ_RETRIES and
// ROS_READ_RETRY_BACKOFF, attaching address for soft per-device backoff.
func ReadRetryConfigFromEnv(address string) ReadRetryConfig {
	cfg := ReadRetryConfig{
		MaxAttempts: DefaultReadRetries,
		BaseBackoff: DefaultReadRetryBackoff,
		MaxBackoff:  DefaultReadRetryMaxBackoff,
		Address:     address,
	}
	if v := strings.TrimSpace(os.Getenv("ROS_READ_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			if n <= 1 {
				cfg.MaxAttempts = 1
			} else {
				cfg.MaxAttempts = n
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("ROS_READ_RETRY_BACKOFF")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.BaseBackoff = d
		}
	}
	return cfg
}

// WithReadRetries wraps inner so transient failures on read-only commands are
// retried with exponential backoff + jitter. Mutating commands are never
// retried (ambiguous-write policy A5).
func WithReadRetries(inner Client, cfg ReadRetryConfig) Client {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultReadRetries
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = DefaultReadRetryBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = DefaultReadRetryMaxBackoff
	}
	if cfg.sleep == nil {
		cfg.sleep = sleepContext
	}
	return &readRetryClient{inner: inner, cfg: cfg}
}

type readRetryClient struct {
	inner Client
	cfg   ReadRetryConfig
}

var _ Client = (*readRetryClient)(nil)

func (c *readRetryClient) Close() error {
	return c.inner.Close()
}

func (c *readRetryClient) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	// Writes: single attempt only — never auto-retry after dispatch (A5).
	if isMutatingCommand(command) {
		return c.inner.Run(ctx, command, args...)
	}

	max := c.cfg.MaxAttempts
	if max <= 1 {
		return c.inner.Run(ctx, command, args...)
	}

	var lastErr error
	for attempt := 0; attempt < max; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}

		if attempt > 0 {
			delay := retryBackoff(attempt-1, c.cfg.BaseBackoff, c.cfg.MaxBackoff)
			if soft := softDeviceBackoff(c.cfg.Address); soft > delay {
				delay = soft
			}
			if err := c.cfg.sleep(ctx, delay); err != nil {
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, err
			}
		}

		res, err := c.inner.Run(ctx, command, args...)
		if err == nil {
			return res, nil
		}
		lastErr = err

		if !isTransientReadError(err) || attempt == max-1 {
			noteDeviceFailure(c.cfg.Address)
			return nil, err
		}
	}
	noteDeviceFailure(c.cfg.Address)
	return nil, lastErr
}

// isTransientReadError reports whether err is safe to retry on a read.
// Context cancellation is not transient. Ambiguous-write apperrs are never
// retried (defense in depth for A5).
func isTransientReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	var ae *apperr.Error
	if errors.As(err, &ae) {
		if ae.Kind == apperr.KindTimeout && ae.SuggestedAction == apperr.SuggestVerifyBeforeRetry {
			return false
		}
		if ae.Kind == apperr.KindBusy || ae.Kind == apperr.KindTimeout || ae.Kind == apperr.KindConnection {
			return true
		}
	}

	if apperr.IsAmbiguousTransport(err) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		type temporary interface{ Temporary() bool }
		if t, ok := any(netErr).(temporary); ok && t.Temporary() {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	for _, n := range []string{
		"busy",
		"temporary",
		"try again",
		"resource temporarily",
		"connection refused",
	} {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func retryBackoff(failureIndex int, base, max time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	// failureIndex 0 → base, 1 → 2*base, …
	mult := time.Duration(1 << failureIndex)
	d := base * mult
	if d > max || d < base { // overflow or cap
		d = max
	}
	// Full jitter in [0, d].
	if d <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func noteDeviceFailure(addr string) {
	if addr == "" {
		return
	}
	deviceFailMu.Lock()
	deviceLastFail[addr] = time.Now()
	deviceFailMu.Unlock()
}

func softDeviceBackoff(addr string) time.Duration {
	if addr == "" {
		return 0
	}
	deviceFailMu.Lock()
	last, ok := deviceLastFail[addr]
	deviceFailMu.Unlock()
	if !ok {
		return 0
	}
	elapsed := time.Since(last)
	if elapsed >= softDeviceFailWindow {
		return 0
	}
	return softDeviceFailWindow - elapsed
}
