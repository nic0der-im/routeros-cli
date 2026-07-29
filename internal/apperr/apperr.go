// Package apperr defines stable machine-readable error kinds for ros.
package apperr

import (
	"errors"
	"fmt"
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
)

// Error is an application error with a stable Kind.
type Error struct {
	Kind    Kind
	Message string
	Cause   error
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

// ExitCode maps kinds to process exit codes used by ros.
func ExitCode(kind Kind) int {
	switch kind {
	case KindConnection, KindAuth:
		return 2
	case KindConfig:
		return 3
	case KindReadOnly:
		return 4
	default:
		return 1
	}
}
