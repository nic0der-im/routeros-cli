package client

import (
	"context"
	"fmt"
)

// MockClient implements Client for testing. Tests can override RunFunc to
// control the result returned by Run.
type MockClient struct {
	// RunFunc allows tests to define custom behavior for Run calls.
	RunFunc func(ctx context.Context, command string, args ...string) (*Result, error)
	// Closed records whether Close has been called.
	Closed bool
}

// NewMockClient returns a MockClient with a default RunFunc that returns an
// empty result.
func NewMockClient() *MockClient {
	return &MockClient{
		RunFunc: func(_ context.Context, _ string, _ ...string) (*Result, error) {
			return &Result{}, nil
		},
	}
}

// Run delegates to RunFunc. It returns an error if RunFunc has not been set.
func (m *MockClient) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if m.RunFunc == nil {
		return nil, fmt.Errorf("MockClient.RunFunc not configured")
	}
	return m.RunFunc(ctx, command, args...)
}

// Close marks the mock client as closed.
func (m *MockClient) Close() error {
	m.Closed = true
	return nil
}
