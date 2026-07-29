// Package apperr defines stable machine-readable error kinds for ros.
package apperr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

// Kind is a stable error classification for agents and exit-code mapping.
type Kind string

const (
	KindConnection Kind = "connection"
	KindAuth       Kind = "auth"
	KindConfig     Kind = "config"
	KindReadOnly   Kind = "read_only"
	KindSession    Kind = "session"
	KindAPI        Kind = "api"
	KindNotFound   Kind = "not_found"
	KindInternal   Kind = "internal"
	KindConflict   Kind = "conflict"
	KindTimeout    Kind = "timeout"
	KindBusy       Kind = "busy"
)

// SuggestVerifyBeforeRetry is the standard recovery hint after an ambiguous write.
const SuggestVerifyBeforeRetry = "verify with read-only get before retry; do not blindly re-run the write"

// Error is an application error with a stable Kind.
type Error struct {
	Kind            Kind
	Message         string
	Cause           error
	SuggestedAction string
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

// Code returns the JSON error.code string (alias of Kind for envelope stability).
func (e *Error) Code() string { return string(e.Kind) }

// WithSuggestedAction sets SuggestedAction and returns e for chaining.
func (e *Error) WithSuggestedAction(action string) *Error {
	if e == nil {
		return nil
	}
	e.SuggestedAction = action
	return e
}

// New builds an apperr.Error.
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// Wrap builds an apperr.Error with cause.
func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

// AsKind extracts Kind from err if present.
func AsKind(err error) (Kind, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind, true
	}
	return "", false
}

// AsSuggestedAction extracts SuggestedAction from an apperr.Error in the chain.
func AsSuggestedAction(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.SuggestedAction
	}
	return ""
}

// ExitCode maps kinds to process exit codes used by ros.
// Existing kind→code mappings are preserved for agents.
func ExitCode(kind Kind) int {
	switch kind {
	case KindConnection, KindAuth, KindTimeout:
		return 2
	case KindConfig:
		return 3
	case KindReadOnly:
		return 4
	default:
		// KindAPI, KindSession, KindNotFound, KindInternal, KindConflict, KindBusy, …
		return 1
	}
}

// IsAmbiguousTransport reports whether err looks like a timeout, EOF, or
// connection reset where a write may or may not have been applied.
func IsAmbiguousTransport(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"timeout",
		"i/o timeout",
		"deadline exceeded",
		"eof",
		"connection reset",
		"broken pipe",
		"use of closed network connection",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

// WrapAmbiguousWrite wraps a transport failure that occurred after a mutation
// was sent. Callers must not auto-retry the write.
func WrapAmbiguousWrite(cause error) *Error {
	return Wrap(KindTimeout, "ambiguous write result: connection failed after mutation was sent", cause).
		WithSuggestedAction(SuggestVerifyBeforeRetry)
}

// MaybeAmbiguousWrite returns WrapAmbiguousWrite(err) when err is an ambiguous
// transport failure; otherwise returns err unchanged. Idempotent if err is
// already an ambiguous-write apperr.
func MaybeAmbiguousWrite(err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) && e.Kind == KindTimeout && e.SuggestedAction == SuggestVerifyBeforeRetry {
		return err
	}
	if !IsAmbiguousTransport(err) {
		return err
	}
	return WrapAmbiguousWrite(err)
}
