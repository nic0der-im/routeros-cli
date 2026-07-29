package winbox

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "device"},
		{"central hub BA", "central-hub-BA"},
		{"lab/router", "lab-router"},
		{"@@@", "device"},
		{"  office-1  ", "office-1"},
	}
	for _, tt := range tests {
		if got := SanitizeName(tt.in); got != tt.want {
			t.Errorf("SanitizeName(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"192.168.88.1", "192.168.88.1:8728"},
		{"192.168.88.1:8291", "192.168.88.1:8291"},
		{"aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff"},
		{"[fe80::1]", "[fe80::1]:8728"},
		{"fe80::1", "[fe80::1]:8728"},
	}
	for _, tt := range tests {
		got := NormalizeAddress(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeAddress(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAddressForAPI(t *testing.T) {
	got := NormalizeAddressForAPI("192.168.88.1:8291", "8728", true)
	if got != "192.168.88.1:8728" {
		t.Fatalf("force API port: got %q", got)
	}
	got = NormalizeAddressForAPI("192.168.88.1:8291", "8728", false)
	if got != "192.168.88.1:8291" {
		t.Fatalf("keep winbox port: got %q", got)
	}
	got = NormalizeAddressForAPI("10.0.0.1", "8729", true)
	if got != "10.0.0.1:8729" {
		t.Fatalf("custom api port: got %q", got)
	}
}

func TestUniqueName(t *testing.T) {
	taken := map[string]bool{"lab": true, "lab-2": true}
	got := UniqueName("lab", func(s string) bool { return taken[s] })
	if got != "lab-3" {
		t.Fatalf("got %q", got)
	}
}

func TestParseWBX_Empty(t *testing.T) {
	_, err := ParseWBX(nil)
	if err == nil {
		t.Fatal("expected error for empty")
	}
	_, err = ParseWBX([]byte{})
	if err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestParseWBX_Synthetic(t *testing.T) {
	// Build L2 TLV record (same layout as YATV WBX-tools).
	rec := buildWBXL2([][2]string{
		{"group", "LAB"},
		{"host", "10.0.0.1"},
		{"login", "admin"},
		{"pwd", "s3cret"},
		{"note", "core-sw"},
	})
	data := append(append([]byte{0x0f, 0x10, 0xc0, 0xbe}, rec...), 0x00, 0x00)

	entries, err := ParseWBX(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
	e := entries[0]
	if e.Address != "10.0.0.1:8728" {
		t.Errorf("address=%q", e.Address)
	}
	if e.Username != "admin" {
		t.Errorf("user=%q", e.Username)
	}
	if e.Password != "s3cret" {
		t.Errorf("pwd=%q", e.Password)
	}
	if e.Comment != "core-sw" {
		t.Errorf("comment=%q", e.Comment)
	}
	if e.Group != "LAB" {
		t.Errorf("group=%q", e.Group)
	}
}

func buildWBXL2(fields [][2]string) []byte {
	var out []byte
	for _, fv := range fields {
		k := []byte(fv[0])
		v := []byte(fv[1])
		total := 1 + len(k) + len(v)
		out = append(out, byte(total&0xff), byte((total>>8)&0xff), byte(len(k)))
		out = append(out, k...)
		out = append(out, v...)
	}
	return out
}

func TestParseCDB_Empty(t *testing.T) {
	_, err := ParseCDB(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = ParseCDB([]byte{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCDB_SyntheticV4(t *testing.T) {
	body := []byte{'M', '2', 0x05, 0x00, 0x00, 0x00}
	body = append(body, cdbStringFieldImplicit(0, "r1")...)
	body = append(body, cdbStringField(1, "192.168.88.1")...)
	body = append(body, cdbStringField(2, "admin")...)
	body = append(body, cdbStringField(3, "pw")...)
	body = append(body, cdbStringField(4, "edge")...)
	body = append(body, cdbStringField(8, "site-a")...)

	data := []byte{0x0d, 0xf0, 0x1d, 0xc0}
	size := uint32(len(body))
	data = append(data, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	data = append(data, body...)

	entries, err := ParseCDB(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
	e := entries[0]
	if e.Address != "192.168.88.1:8728" {
		t.Errorf("address=%q", e.Address)
	}
	if e.Username != "admin" || e.Password != "pw" {
		t.Errorf("creds user=%q pwd=%q", e.Username, e.Password)
	}
	if e.Comment != "edge" || e.Group != "site-a" || e.Name != "r1" {
		t.Errorf("meta name=%q comment=%q group=%q", e.Name, e.Comment, e.Group)
	}
}

func cdbStringField(fid int, s string) []byte {
	b := []byte(s)
	out := []byte{byte(fid), 0x00, 0x00, 0x21, byte(len(b))}
	return append(out, b...)
}

func cdbStringFieldImplicit(_ int, s string) []byte {
	b := []byte(s)
	out := []byte{0x09, 0x00, 0xfe, 0x21, byte(len(b))}
	return append(out, b...)
}

func TestParseFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Addresses.cdb")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected empty-file error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err=%v", err)
	}
}

func TestDefaultPaths_ContainOSHints(t *testing.T) {
	dirs := DefaultWinboxDirs()
	if len(dirs) == 0 {
		t.Fatal("expected at least one dir")
	}
	joined := strings.Join(dirs, "\n")
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(joined, "Application Support") {
			t.Fatalf("macOS paths missing Application Support: %v", dirs)
		}
	case "linux":
		if !strings.Contains(joined, ".local/share") && !strings.Contains(joined, ".local"+string(os.PathSeparator)+"share") {
			t.Fatalf("linux paths missing .local/share: %v", dirs)
		}
	case "windows":
		// APPDATA may be empty in CI; still should return something or empty ok.
	}
	cdbs := DefaultCDBPaths()
	if len(cdbs) == 0 {
		t.Fatal("expected CDB candidates")
	}
	if !strings.HasSuffix(strings.ToLower(cdbs[0]), "addresses.cdb") {
		t.Fatalf("first cdb=%q", cdbs[0])
	}
	wbx := DefaultWBXPaths()
	if len(wbx) == 0 {
		t.Fatal("expected WBX candidates")
	}
}

func TestFindDefaultFile_None(t *testing.T) {
	// Without planting files in real home dirs, FindDefaultFile may or may not
	// find something. Just ensure it returns a sensible error when forced via
	// empty search by checking the error message shape if err != nil.
	_, err := FindDefaultFile()
	if err != nil && !strings.Contains(err.Error(), "no Winbox") && !strings.Contains(err.Error(), "--file") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestPreferredName(t *testing.T) {
	e := Entry{Comment: "Core SW", Address: "1.2.3.4:8728", Group: "g"}
	if got := PreferredName(e); got != "Core-SW" {
		t.Fatalf("got %q", got)
	}
	e2 := Entry{Group: "LAB", Address: "1.2.3.4:8728"}
	if got := PreferredName(e2); got != "LAB" {
		t.Fatalf("got %q", got)
	}
}
