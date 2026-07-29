package filexfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
)

type stubClient struct {
	sentences []map[string]string
	err       error
}

func (s *stubClient) Run(ctx context.Context, command string, args ...string) (*client.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &client.Result{Sentences: s.sentences}, nil
}

func (s *stubClient) Close() error { return nil }

func TestDownloadAPI(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.rsc")
	c := &stubClient{sentences: []map[string]string{{
		"name":     "x.rsc",
		"contents": "# export\n",
		"size":     "9",
	}}}
	n, err := Download(context.Background(), c, "x.rsc", out, ViaAPI, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Fatalf("n=%d", n)
	}
	b, _ := os.ReadFile(out)
	if string(b) != "# export\n" {
		t.Fatalf("contents=%q", b)
	}
}
