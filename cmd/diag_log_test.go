package cmd

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDiagLogFlagsRegistered(t *testing.T) {
	cmd := newDiagLogCmd()
	topics := cmd.Flags().Lookup("topics")
	if topics == nil {
		t.Fatal("missing --topics")
	}
	if topics.DefValue != "" {
		t.Fatalf("topics default: %q", topics.DefValue)
	}
	since := cmd.Flags().Lookup("since")
	if since == nil {
		t.Fatal("missing --since")
	}
	if since.DefValue != "" {
		t.Fatalf("since default: %q", since.DefValue)
	}
	for _, needle := range []string{"--topics", "--since", "Limitation", "--limit"} {
		if !strings.Contains(cmd.Long, needle) {
			t.Fatalf("Long help missing %q:\n%s", needle, cmd.Long)
		}
	}
}

func TestParseTopicsList(t *testing.T) {
	if got := parseTopicsList(""); got != nil {
		t.Fatalf("empty: %v", got)
	}
	got := parseTopicsList(" firewall , Info, ERROR ,, ")
	want := []string{"firewall", "Info", "ERROR"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestLogTopicsMatch(t *testing.T) {
	field := "firewall,info,account"
	if !logTopicsMatch(field, []string{"FIREWALL"}) {
		t.Fatal("expected firewall match")
	}
	if !logTopicsMatch(field, []string{"error", "info"}) {
		t.Fatal("expected info match among tokens")
	}
	if logTopicsMatch(field, []string{"error", "critical"}) {
		t.Fatal("expected no match")
	}
	if !logTopicsMatch(field, nil) {
		t.Fatal("empty want matches all")
	}
	if !logTopicsMatch(field, []string{}) {
		t.Fatal("empty slice matches all")
	}
	if !logTopicsMatch("system,error", []string{"err"}) {
		t.Fatal("expected substring match")
	}
}

func TestFilterLogEntries_TopicsAndSince(t *testing.T) {
	routerNow := time.Date(2026, 7, 29, 18, 0, 0, 0, time.Local)
	rows := []map[string]string{
		{"topics": "firewall,info", "time": "jul/29 17:50:00", "message": "drop"},
		{"topics": "system,info", "time": "jul/29 16:00:00", "message": "old"},
		{"topics": "error", "time": "jul/29 17:55:00", "message": "fail"},
		{"topics": "error", "time": "not-a-time", "message": "ambiguous"},
		{"topics": "dhcp,info", "time": "jul/29 17:58:00", "message": "lease"},
	}

	got := filterLogEntries(rows, []string{"error"}, 0, time.Time{})
	if len(got) != 2 {
		t.Fatalf("topics-only: got %d want 2", len(got))
	}

	// since 30m from 18:00 → cutoff 17:30; keep firewall, error@17:55, ambiguous, dhcp; drop system@16:00
	got = filterLogEntries(rows, nil, 30*time.Minute, routerNow)
	if len(got) != 4 {
		t.Fatalf("since-only: got %d want 4 (%v)", len(got), logMessages(got))
	}
	for _, r := range got {
		if r["message"] == "old" {
			t.Fatal("old entry should be filtered by --since")
		}
	}

	got = filterLogEntries(rows, []string{"error"}, 30*time.Minute, routerNow)
	if len(got) != 2 {
		t.Fatalf("topics+since: got %d want 2 (%v)", len(got), logMessages(got))
	}
}

func logMessages(rows []map[string]string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r["message"]
	}
	return out
}

func TestParseLogEntryTime_RelativeToClock(t *testing.T) {
	clock := time.Date(2026, 7, 29, 18, 0, 0, 0, time.Local)

	t1, ok := parseLogEntryTime(map[string]string{"time": "jul/29 17:45:01"}, clock)
	if !ok {
		t.Fatal("expected parse")
	}
	if t1.Year() != 2026 || t1.Month() != time.July || t1.Day() != 29 || t1.Hour() != 17 || t1.Minute() != 45 {
		t.Fatalf("unexpected %v", t1)
	}

	jan := time.Date(2026, 1, 5, 12, 0, 0, 0, time.Local)
	t2, ok := parseLogEntryTime(map[string]string{"time": "dec/31 23:00:00"}, jan)
	if !ok {
		t.Fatal("expected dec parse")
	}
	if t2.Year() != 2025 {
		t.Fatalf("expected 2025, got %v", t2)
	}

	unix := clock.Unix()
	t3, ok := parseLogEntryTime(map[string]string{"timestamp": strconv.FormatInt(unix, 10)}, clock)
	if !ok || t3.Unix() != unix {
		t.Fatalf("unix timestamp: ok=%v t=%v", ok, t3)
	}

	_, ok = parseLogEntryTime(map[string]string{"time": "garbage"}, clock)
	if ok {
		t.Fatal("garbage should fail parse")
	}
}

func TestParseRouterClock(t *testing.T) {
	got, err := parseRouterClock(map[string]string{
		"date": "jul/29/2026",
		"time": "18:00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2026 || got.Month() != time.July || got.Day() != 29 || got.Hour() != 18 {
		t.Fatalf("got %v", got)
	}

	if _, err := parseRouterClock(map[string]string{}); err == nil {
		t.Fatal("expected error on empty clock")
	}
}
