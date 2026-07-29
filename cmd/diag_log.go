package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

const diagLogLongHelp = `Show system log entries (/log/print).

Flags:
  --topics  Comma-separated tokens; keep rows whose topics field contains any
            token (case-insensitive substring match), e.g. firewall,info,error.
  --since   Keep entries with time/timestamp on or after (router_clock − duration).
            Go duration syntax: 15m, 1h, 2h30m.

With neither flag, prints all /log rows (same as before).

Row cap: only the global --limit flag (via App.RowLimit). There is no local
default; omit --limit for unlimited filtered results.

Limitation: RouterOS log time is often "mmm/dd HH:MM:SS" without a year.
Parsing is best-effort relative to /system/clock. Rows that fail to parse are
kept when they match --topics so ambiguous times do not hide errors.`

func newDiagLogCmd() *cobra.Command {
	var topicsFlag, sinceFlag string
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show recent system log entries",
		Long:  diagLogLongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			runDiagLog(cmd, topicsFlag, sinceFlag)
		},
	}
	cmd.Flags().StringVar(&topicsFlag, "topics", "", "comma-separated topic tokens (case-insensitive substring)")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "relative window from router clock (e.g. 15m, 1h, 2h30m)")
	return cmd
}

func runDiagLog(cmd *cobra.Command, topicsFlag, sinceFlag string) {
	topics := parseTopicsList(topicsFlag)
	var since time.Duration
	if sinceFlag != "" {
		d, err := time.ParseDuration(sinceFlag)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: invalid --since %q: %v (use Go duration, e.g. 15m, 1h)\n", sinceFlag, err)
			return
		}
		if d < 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: --since must be non-negative\n")
			return
		}
		since = d
	}

	runWithClient(cmd, "/log/print", func(ctx context.Context, a *App, c client.Client, deviceName string) error {
		var routerNow time.Time
		if since > 0 {
			clockRes, err := c.Run(ctx, "/system/clock/print")
			if err != nil {
				return fmt.Errorf("fetching router clock for --since: %w", err)
			}
			if len(clockRes.Sentences) == 0 {
				return fmt.Errorf("router clock returned no data")
			}
			routerNow, err = parseRouterClock(clockRes.Sentences[0])
			if err != nil {
				return fmt.Errorf("parsing router clock: %w", err)
			}
		}

		result, err := c.Run(ctx, "/log/print")
		if err != nil {
			return err
		}
		if len(topics) > 0 || since > 0 {
			result = &client.Result{Sentences: filterLogEntries(result.Sentences, topics, since, routerNow)}
		}
		return renderGenericResult(a, cmd.OutOrStdout(), result, deviceName, "/log/print")
	})
}

// parseTopicsList splits a comma-separated --topics flag into trimmed tokens.
func parseTopicsList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// logTopicsMatch reports whether topicsField contains any requested token
// as a case-insensitive substring (RouterOS topics are comma/space separated).
func logTopicsMatch(topicsField string, want []string) bool {
	if len(want) == 0 {
		return true
	}
	lower := strings.ToLower(topicsField)
	for _, w := range want {
		wl := strings.ToLower(strings.TrimSpace(w))
		if wl == "" {
			continue
		}
		if strings.Contains(lower, wl) {
			return true
		}
	}
	return false
}

// filterLogEntries applies client-side --topics and --since filters.
// When since > 0, rows whose time cannot be parsed are kept (prefer not hiding errors).
func filterLogEntries(rows []map[string]string, topics []string, since time.Duration, routerNow time.Time) []map[string]string {
	useSince := since > 0
	var cutoff time.Time
	if useSince {
		cutoff = routerNow.Add(-since)
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if !logTopicsMatch(row["topics"], topics) {
			continue
		}
		if useSince {
			t, ok := parseLogEntryTime(row, routerNow)
			if ok && t.Before(cutoff) {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

// parseRouterClock reads /system/clock/print fields into a wall-clock time.
func parseRouterClock(sentence map[string]string) (time.Time, error) {
	date := strings.TrimSpace(sentence["date"])
	tim := strings.TrimSpace(sentence["time"])
	if date != "" && tim != "" {
		combined := date + " " + tim
		if t, err := parseROSDateTime(combined, rosFullDateTimeLayouts); err == nil {
			return t, nil
		}
	}
	if tim != "" {
		if t, err := parseROSDateTime(tim, rosFullDateTimeLayouts); err == nil {
			return t, nil
		}
	}
	if date != "" {
		if t, err := parseROSDateTime(date+" 00:00:00", rosFullDateTimeLayouts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized clock fields date=%q time=%q", date, tim)
}

// parseLogEntryTime best-effort parses a log row's time/timestamp relative to routerNow.
// ok is false when parsing fails (caller should keep the row for --since).
func parseLogEntryTime(row map[string]string, routerNow time.Time) (time.Time, bool) {
	if ts := strings.TrimSpace(row["timestamp"]); ts != "" {
		if n, err := strconv.ParseInt(ts, 10, 64); err == nil {
			switch {
			case n > 1e14: // nanoseconds
				return time.Unix(0, n), true
			case n > 1e11: // milliseconds
				return time.UnixMilli(n), true
			default:
				return time.Unix(n, 0), true
			}
		}
		if t, err := parseROSDateTime(ts, rosFullDateTimeLayouts); err == nil {
			return t, true
		}
	}

	raw := strings.TrimSpace(row["time"])
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := parseROSDateTime(raw, rosFullDateTimeLayouts); err == nil {
		return t, true
	}
	if t, err := parseROSDateTime(raw, rosMonthDayTimeLayouts); err == nil {
		return alignLogTimeToClock(t, routerNow), true
	}
	if t, err := time.Parse("15:04:05", raw); err == nil {
		return time.Date(routerNow.Year(), routerNow.Month(), routerNow.Day(),
			t.Hour(), t.Minute(), t.Second(), 0, time.Local), true
	}
	if t, err := time.Parse("15:04", raw); err == nil {
		return time.Date(routerNow.Year(), routerNow.Month(), routerNow.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local), true
	}
	return time.Time{}, false
}

// alignLogTimeToClock assigns the router year (and adjusts across year boundary).
func alignLogTimeToClock(parsed, routerNow time.Time) time.Time {
	t := time.Date(routerNow.Year(), parsed.Month(), parsed.Day(),
		parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), time.Local)
	// If more than a day ahead of the router clock, assume previous year (e.g. dec→jan).
	if t.After(routerNow.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t
}

var rosFullDateTimeLayouts = []string{
	"Jan/02/2006 15:04:05",
	"Jan/2/2006 15:04:05",
	"2006-01-02 15:04:05",
	"Jan/02/2006 15:04",
	"2006-01-02 15:04",
	"Jan/02/2006",
	"2006-01-02",
}

var rosMonthDayTimeLayouts = []string{
	"Jan/02 15:04:05",
	"Jan/2 15:04:05",
	"Jan/02 15:04",
	"01/02 15:04:05",
}

func parseROSDateTime(s string, layouts []string) (time.Time, error) {
	s = strings.TrimSpace(s)
	candidates := []string{s, capitalizeROSMonth(s)}
	var lastErr error
	for _, c := range candidates {
		for _, layout := range layouts {
			t, err := time.ParseInLocation(layout, c, time.Local)
			if err == nil {
				return t, nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no layout matched")
	}
	return time.Time{}, lastErr
}

// capitalizeROSMonth uppercases the first letter of a leading month token
// (RouterOS prints "jul/29/2026"; Go layouts expect "Jul").
func capitalizeROSMonth(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
			return string(runes)
		}
		if r == '/' || unicode.IsDigit(r) {
			break
		}
	}
	return s
}
