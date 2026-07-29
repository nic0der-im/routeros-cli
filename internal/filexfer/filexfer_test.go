package filexfer

import (
	"context"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
)

func TestMergeAddressList(t *testing.T) {
	got := MergeAddressList("192.168.88.0/24", "203.0.113.10", "192.168.88.0/24")
	if got != "192.168.88.0/24,203.0.113.10/32" {
		t.Fatalf("got %q", got)
	}
	got = MergeAddressList("", "10.0.0.5/32")
	if got != "10.0.0.5/32" {
		t.Fatalf("got %q", got)
	}
}

func TestCIDROrIP(t *testing.T) {
	s, err := CIDROrIP("1.2.3.4")
	if err != nil || s != "1.2.3.4/32" {
		t.Fatalf("%q %v", s, err)
	}
}

type stubClient struct {
	calls     [][]string
	sentences []map[string]string
	setErr    error
}

func (s *stubClient) Run(ctx context.Context, command string, args ...string) (*client.Result, error) {
	s.calls = append(s.calls, append([]string{command}, args...))
	if command == "/ip/service/set" && s.setErr != nil {
		return nil, s.setErr
	}
	if command == "/ip/service/print" || command == "/file/print" {
		return &client.Result{Sentences: s.sentences}, nil
	}
	return &client.Result{}, nil
}

func (s *stubClient) Close() error { return nil }

func TestCaptureApplyRestoreSSH(t *testing.T) {
	stub := &stubClient{sentences: []map[string]string{{
		".id":      "*1",
		"name":     "ssh",
		"port":     "22",
		"address":  "192.168.88.0/24",
		"disabled": "false",
	}}}
	st, err := CaptureSSHService(context.Background(), stub)
	if err != nil {
		t.Fatal(err)
	}
	if st.Address != "192.168.88.0/24" || st.Port != "22" {
		t.Fatalf("%+v", st)
	}
	if err := ApplySSHAccess(context.Background(), stub, st, "192.168.88.0/24,203.0.113.1/32"); err != nil {
		t.Fatal(err)
	}
	if err := RestoreSSHService(context.Background(), stub, st); err != nil {
		t.Fatal(err)
	}
	// last call should restore original address
	last := stub.calls[len(stub.calls)-1]
	found := false
	for _, a := range last {
		if a == "=address=192.168.88.0/24" {
			found = true
		}
	}
	if !found {
		t.Fatalf("restore args: %v", last)
	}
}

func TestDownloadAPI(t *testing.T) {
	dir := t.TempDir()
	out := dir + "/x.rsc"
	c := &stubClient{sentences: []map[string]string{{
		"name": "x.rsc", "contents": "# export\n", "size": "9",
	}}}
	n, err := Download(context.Background(), "x.rsc", out, Options{Via: ViaAPI, Client: c})
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Fatalf("n=%d", n)
	}
}

func TestDownloadSFTPEphemeralRestoresOnDialFailure(t *testing.T) {
	stub := &stubClient{sentences: []map[string]string{{
		".id":      "*1",
		"name":     "ssh",
		"port":     "1", // closed port → dial fails fast
		"address":  "192.168.88.0/24",
		"disabled": "false",
	}}}
	ephem := true
	out := t.TempDir() + "/x.backup"
	_, err := Download(context.Background(), "x.backup", out, Options{
		Via:          ViaSFTP,
		Host:         "127.0.0.1",
		User:         "u",
		Pass:         "p",
		SourceIP:     "127.0.0.1",
		EphemeralSSH: &ephem,
		Client:       stub,
	})
	if err == nil {
		t.Fatal("expected dial/sftp error")
	}
	var restored bool
	for _, call := range stub.calls {
		if len(call) == 0 || call[0] != "/ip/service/set" {
			continue
		}
		for _, a := range call {
			if a == "=address=192.168.88.0/24" {
				restored = true
			}
		}
	}
	if !restored {
		t.Fatalf("expected restore of original allowlist; calls=%v", stub.calls)
	}
}

func TestDownloadSFTPRequiresCreds(t *testing.T) {
	_, err := Download(context.Background(), "x", t.TempDir()+"/x", Options{Via: ViaSFTP, Host: "1.2.3.4"})
	if err == nil {
		t.Fatal("expected error")
	}
}
