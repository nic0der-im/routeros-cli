//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/policy"
)

// Integration tests against a real RouterOS device.
//
// Required env:
//   ROS_TEST_ADDRESS  e.g. 192.168.88.1:8728
//   ROS_TEST_USER     e.g. admin
//   ROS_TEST_PASSWORD
//   ROS_TEST_TLS      true|false (optional)
//
// Run:
//   go test -tags=integration ./test/integration/...

func testConnect(t *testing.T) (client.Client, func()) {
	t.Helper()
	addr := os.Getenv("ROS_TEST_ADDRESS")
	user := os.Getenv("ROS_TEST_USER")
	pass := os.Getenv("ROS_TEST_PASSWORD")
	if addr == "" || pass == "" {
		t.Skip("ROS_TEST_ADDRESS / ROS_TEST_PASSWORD not set")
	}
	if user == "" {
		user = "admin"
	}
	useTLS := strings.EqualFold(os.Getenv("ROS_TEST_TLS"), "true")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	c, err := client.Connect(ctx, client.ConnectConfig{
		Address:  addr,
		Username: user,
		Password: pass,
		UseTLS:   useTLS,
		Timeout:  15 * time.Second,
	})
	if err != nil {
		cancel()
		t.Fatalf("connect: %v", err)
	}
	return c, func() {
		_ = c.Close()
		cancel()
	}
}

func TestIdentityPrint(t *testing.T) {
	c, cleanup := testConnect(t)
	defer cleanup()

	result, err := c.Run(context.Background(), "/system/identity/print")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sentences) == 0 {
		t.Fatal("empty identity response")
	}
	t.Logf("identity=%v", result.Sentences[0]["name"])
}

func TestReadOnlyBlocksWrite(t *testing.T) {
	c, cleanup := testConnect(t)
	defer cleanup()

	ro := policy.WrapReadOnly(c)
	_, err := ro.Run(context.Background(), "/system/identity/set", "=name=should-not-apply")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if _, ok := err.(*policy.ErrReadOnly); !ok {
		t.Fatalf("got %T: %v", err, err)
	}
}
