package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildTLSConfig_WithCA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, []byte("not-a-cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := buildTLSConfig(ConnectConfig{
		CACertPath: path,
	})
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestBuildTLSConfig_NoCA(t *testing.T) {
	cfg, err := buildTLSConfig(ConnectConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected insecure skip verify")
	}
	if cfg.RootCAs != nil {
		t.Error("expected nil RootCAs")
	}
}

func TestBuildTLSConfig_MissingFile(t *testing.T) {
	_, err := buildTLSConfig(ConnectConfig{CACertPath: "/tmp/does-not-exist-ros-ca.pem"})
	if err == nil {
		t.Fatal("expected error")
	}
}
