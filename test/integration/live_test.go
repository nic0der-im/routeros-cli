//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"testing"
)

// Opt-in live RouterOS tests.
//
//	ROS_INTEGRATION_DEVICE=home go test -tags=integration ./test/integration/...
func TestLiveAudit(t *testing.T) {
	dev := os.Getenv("ROS_INTEGRATION_DEVICE")
	if dev == "" {
		t.Skip("set ROS_INTEGRATION_DEVICE to run")
	}
	cmd := exec.Command("ros", "-d", dev, "--read-only", "audit", "--profile", "network", "-o", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ros audit: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Fatal("empty audit output")
	}
}

func TestLiveSystemInfo(t *testing.T) {
	dev := os.Getenv("ROS_INTEGRATION_DEVICE")
	if dev == "" {
		t.Skip("set ROS_INTEGRATION_DEVICE to run")
	}
	cmd := exec.Command("ros", "-d", dev, "get", "system", "info")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ros get system info: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}
