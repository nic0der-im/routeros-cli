package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/output"
)

func TestEnabled(t *testing.T) {
	t.Setenv("ROS_AUDIT", "")
	t.Setenv("ROS_NO_AUDIT", "")
	if !Enabled() {
		t.Fatal("default should be enabled")
	}

	t.Setenv("ROS_AUDIT", "0")
	if Enabled() {
		t.Fatal("ROS_AUDIT=0 should disable")
	}

	t.Setenv("ROS_AUDIT", "")
	t.Setenv("ROS_NO_AUDIT", "1")
	if Enabled() {
		t.Fatal("ROS_NO_AUDIT=1 should disable")
	}

	t.Setenv("ROS_NO_AUDIT", "")
	t.Setenv("ROS_AUDIT", "false")
	if Enabled() {
		t.Fatal("ROS_AUDIT=false should disable")
	}
}

func TestRedactAPIArgs(t *testing.T) {
	got := RedactAPIArgs([]string{
		"=name=wg0",
		"=private-key=SUPERSECRET",
		"=password=hunter2",
		"?address=1.2.3.4",
	})
	if got[0] != "=name=wg0" {
		t.Fatalf("name: %q", got[0])
	}
	if got[1] != "=private-key="+output.RedactedPlaceholder {
		t.Fatalf("private-key: %q", got[1])
	}
	if got[2] != "=password="+output.RedactedPlaceholder {
		t.Fatalf("password: %q", got[2])
	}
	if got[3] != "?address=1.2.3.4" {
		t.Fatalf("filter: %q", got[3])
	}
}

func TestPrepare_RedactsPropsAndFillsOutcome(t *testing.T) {
	ev := Prepare(Event{
		Action: "created",
		Args:   []string{"=password=hunter2", "=comment=ok"},
		Props:  map[string]string{"private-key": "AAAA", "name": "wg0"},
	})
	if ev.Outcome != "created" || ev.Action != "created" {
		t.Fatalf("action/outcome: %q/%q", ev.Action, ev.Outcome)
	}
	if ev.TS == "" {
		t.Fatal("ts empty")
	}
	if ev.Props["private-key"] != output.RedactedPlaceholder {
		t.Fatalf("props: %#v", ev.Props)
	}
	if ev.Args[0] != "=password="+output.RedactedPlaceholder {
		t.Fatalf("args: %#v", ev.Args)
	}
	if strings.Contains(mustJSON(t, ev), "hunter2") || strings.Contains(mustJSON(t, ev), "AAAA") {
		t.Fatalf("leaked secret: %s", mustJSON(t, ev))
	}
}

func TestAppend_WritesNDJSONWithPerms(t *testing.T) {
	dir := t.TempDir()
	SetDirForTest(dir)
	t.Cleanup(func() { SetDirForTest("") })

	ev := Event{
		RequestID: "req-abc",
		Device:    "lab",
		Profile:   "operator",
		EnvClass:  "lab",
		Verb:      "set",
		Action:    "updated",
		Command:   "/interface/wireguard/set",
		Path:      "/interface/wireguard",
		Args:      []string{"=.id=*1", "=private-key=LEAKME"},
		DryRun:    false,
	}
	if err := Append(dir, ev); err != nil {
		t.Fatal(err)
	}

	path := PathFor(dir, time.Now().UTC())
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode=%04o want 0600", fi.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode=%04o want 0700", dirInfo.Mode().Perm())
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("expected one line")
	}
	line := sc.Text()
	if strings.Contains(line, "LEAKME") {
		t.Fatalf("secret leaked: %s", line)
	}
	var got Event
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	if got.RequestID != "req-abc" || got.Outcome != "updated" {
		t.Fatalf("got=%+v", got)
	}
	if got.Props["private-key"] != output.RedactedPlaceholder {
		t.Fatalf("props=%v", got.Props)
	}
	if sc.Scan() {
		t.Fatal("expected exactly one line")
	}
}

func TestAppend_Concurrent(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	const n = 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = Append(dir, Event{
				RequestID: "c",
				Action:    "created",
				Verb:      "create",
				Args:      []string{"=comment=n"},
			})
		}(i)
	}
	wg.Wait()

	path := PathFor(dir, time.Now().UTC())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != n {
		t.Fatalf("lines=%d want %d", len(lines), n)
	}
	for _, line := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
	}
}

func TestFileName(t *testing.T) {
	ts := time.Date(2026, 7, 29, 15, 0, 0, 0, time.FixedZone("ART", -3*3600))
	got := FileName(ts)
	want := "writes-2026-07-29.ndjson"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if PathFor("/tmp/a", ts) != filepath.Join("/tmp/a", want) {
		t.Fatal(PathFor("/tmp/a", ts))
	}
}

func mustJSON(t *testing.T, ev Event) string {
	t.Helper()
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
