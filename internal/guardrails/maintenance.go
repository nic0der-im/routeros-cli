package guardrails

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrOutsideMaintenanceWindow is returned when a write is refused because the
// current local time falls outside every configured maintenance window.
type ErrOutsideMaintenanceWindow struct {
	Now        time.Time
	Windows    []string
	NextWindow string // human hint when cheap to compute; may be empty
}

func (e *ErrOutsideMaintenanceWindow) Error() string {
	msg := fmt.Sprintf(
		"write refused: outside maintenance window (now %s local)",
		e.Now.Format("Mon 15:04 MST"),
	)
	if e.NextWindow != "" {
		msg += "; next window: " + e.NextWindow
	} else if len(e.Windows) > 0 {
		msg += "; configured: " + strings.Join(e.Windows, ", ")
	}
	msg += " (use --force / ROS_SKIP_MAINTENANCE_GATE=1 to break-glass)"
	return msg
}

// ROSSkipMaintenanceGate reports whether ROS_SKIP_MAINTENANCE_GATE=1/true is set.
func ROSSkipMaintenanceGate() bool {
	v := os.Getenv("ROS_SKIP_MAINTENANCE_GATE")
	return v == "1" || strings.EqualFold(v, "true")
}

// maintenanceWindow is a parsed window that can answer containment and next start.
type maintenanceWindow interface {
	Contains(now time.Time) bool
	// NextStart returns the next inclusive start at or after now, and a display label.
	NextStart(now time.Time) (start time.Time, label string, ok bool)
	Spec() string
}

// CheckMaintenanceWindow refuses writes outside configured windows unless force
// or ROS_SKIP_MAINTENANCE_GATE is set. An empty windows list means no restriction.
// Weekly specs are interpreted in now's Location (typically local for time.Now()).
func CheckMaintenanceWindow(windows []string, now time.Time, force bool) error {
	if force || ROSSkipMaintenanceGate() {
		return nil
	}
	specs := nonEmptySpecs(windows)
	if len(specs) == 0 {
		return nil
	}
	parsed, err := ParseMaintenanceWindows(specs)
	if err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now()
	}
	for _, w := range parsed {
		if w.Contains(now) {
			return nil
		}
	}
	nextHint := nextWindowHint(parsed, now)
	return &ErrOutsideMaintenanceWindow{
		Now:        now,
		Windows:    specs,
		NextWindow: nextHint,
	}
}

func nonEmptySpecs(windows []string) []string {
	out := make([]string, 0, len(windows))
	for _, w := range windows {
		w = strings.TrimSpace(w)
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func nextWindowHint(windows []maintenanceWindow, now time.Time) string {
	var best time.Time
	var bestLabel string
	found := false
	for _, w := range windows {
		start, label, ok := w.NextStart(now)
		if !ok {
			continue
		}
		if !found || start.Before(best) {
			best = start
			bestLabel = label
			found = true
		}
	}
	if !found {
		return ""
	}
	return bestLabel
}

// ParseMaintenanceWindows parses every non-empty spec. Fail-closed on invalid input.
func ParseMaintenanceWindows(specs []string) ([]maintenanceWindow, error) {
	out := make([]maintenanceWindow, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		w, err := ParseMaintenanceWindow(spec)
		if err != nil {
			return nil, fmt.Errorf("maintenance_windows %q: %w", spec, err)
		}
		out = append(out, w)
	}
	return out, nil
}

// ParseMaintenanceWindow parses one window spec.
//
// Formats:
//   - "Mon-Fri 22:00-06:00" — weekday range/list + HH:MM-HH:MM (local tz of now)
//   - "weekday=sat,sun;start=00:00;end=23:59" — explicit keys (weekdays= also accepted)
//   - "RFC3339/RFC3339" or "RFC3339..RFC3339" — absolute one-shot range
func ParseMaintenanceWindow(spec string) (maintenanceWindow, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty window spec")
	}

	if w, ok, err := tryParseAbsoluteWindow(spec); err != nil {
		return nil, err
	} else if ok {
		return w, nil
	}

	if strings.Contains(spec, "=") {
		return parseExplicitWindow(spec)
	}

	return parseHumanWeeklyWindow(spec)
}

// --- absolute RFC3339 ---

type absoluteWindow struct {
	spec       string
	start, end time.Time
}

func (w absoluteWindow) Spec() string { return w.spec }

func (w absoluteWindow) Contains(now time.Time) bool {
	return !now.Before(w.start) && now.Before(w.end)
}

func (w absoluteWindow) NextStart(now time.Time) (time.Time, string, bool) {
	if !now.Before(w.end) {
		return time.Time{}, "", false
	}
	start := w.start
	if now.After(w.start) {
		// Currently inside — next "start" is now for hint purposes is not useful;
		// containment already handled. Outside-before-start uses start.
		if w.Contains(now) {
			return time.Time{}, "", false
		}
	}
	if now.Before(w.start) {
		label := fmt.Sprintf("%s → %s", w.start.Format(time.RFC3339), w.end.Format(time.RFC3339))
		return start, label, true
	}
	return time.Time{}, "", false
}

func tryParseAbsoluteWindow(spec string) (absoluteWindow, bool, error) {
	sep := ""
	switch {
	case strings.Contains(spec, ".."):
		sep = ".."
	case strings.Count(spec, "/") == 1 && strings.Contains(spec, "T"):
		sep = "/"
	default:
		return absoluteWindow{}, false, nil
	}
	parts := strings.SplitN(spec, sep, 2)
	if len(parts) != 2 {
		return absoluteWindow{}, false, nil
	}
	start, err1 := parseRFC3339Flexible(strings.TrimSpace(parts[0]))
	end, err2 := parseRFC3339Flexible(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		// Looks like absolute form but failed — fail closed rather than fall through.
		if strings.Contains(spec, "T") {
			if err1 != nil {
				return absoluteWindow{}, false, fmt.Errorf("absolute start: %w", err1)
			}
			return absoluteWindow{}, false, fmt.Errorf("absolute end: %w", err2)
		}
		return absoluteWindow{}, false, nil
	}
	if !end.After(start) {
		return absoluteWindow{}, false, fmt.Errorf("absolute window end must be after start")
	}
	return absoluteWindow{spec: spec, start: start, end: end}, true, nil
}

func parseRFC3339Flexible(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// --- explicit weekday=;start=;end= ---

func parseExplicitWindow(spec string) (maintenanceWindow, error) {
	var daysRaw, startRaw, endRaw string
	for _, part := range strings.Split(spec, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("expected key=value in %q", part)
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		switch key {
		case "weekday", "weekdays", "day", "days":
			daysRaw = val
		case "start":
			startRaw = val
		case "end":
			endRaw = val
		default:
			return nil, fmt.Errorf("unknown key %q (want weekday/start/end)", key)
		}
	}
	if daysRaw == "" || startRaw == "" || endRaw == "" {
		return nil, fmt.Errorf("explicit window requires weekday, start, and end")
	}
	days, err := parseWeekdays(daysRaw)
	if err != nil {
		return nil, err
	}
	startMin, err := parseHHMM(startRaw)
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	endMin, err := parseHHMM(endRaw)
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}
	return weeklyWindow{spec: spec, days: days, startMin: startMin, endMin: endMin}, nil
}

// --- human "Mon-Fri 22:00-06:00" ---

func parseHumanWeeklyWindow(spec string) (maintenanceWindow, error) {
	parts := strings.Fields(spec)
	if len(parts) != 2 {
		return nil, fmt.Errorf("want \"<weekdays> <HH:MM>-<HH:MM>\", got %q", spec)
	}
	days, err := parseWeekdays(parts[0])
	if err != nil {
		return nil, err
	}
	timeParts := strings.Split(parts[1], "-")
	if len(timeParts) != 2 {
		return nil, fmt.Errorf("want HH:MM-HH:MM time range, got %q", parts[1])
	}
	startMin, err := parseHHMM(timeParts[0])
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	endMin, err := parseHHMM(timeParts[1])
	if err != nil {
		return nil, fmt.Errorf("end: %w", err)
	}
	return weeklyWindow{spec: spec, days: days, startMin: startMin, endMin: endMin}, nil
}

type weeklyWindow struct {
	spec     string
	days     map[time.Weekday]bool
	startMin int // minutes from midnight inclusive
	endMin   int // minutes from midnight exclusive-ish: end==start means full day
}

func (w weeklyWindow) Spec() string { return w.spec }

func (w weeklyWindow) Contains(now time.Time) bool {
	loc := now.Location()
	local := now.In(loc)
	day := local.Weekday()
	min := local.Hour()*60 + local.Minute()

	if w.startMin == w.endMin {
		// Degenerate: treat as full day on matching weekdays.
		return w.days[day]
	}

	if w.endMin > w.startMin {
		// Same-day window (inclusive HH:MM on both ends).
		return w.days[day] && min >= w.startMin && min <= w.endMin
	}

	// Overnight: [start, 24:00) on listed day, or [00:00, end] on the following day
	// when the previous weekday is listed.
	if w.days[day] && min >= w.startMin {
		return true
	}
	prev := prevWeekday(day)
	if w.days[prev] && min <= w.endMin {
		return true
	}
	return false
}

func (w weeklyWindow) NextStart(now time.Time) (time.Time, string, bool) {
	loc := now.Location()
	local := now.In(loc)
	// Scan up to 8 days ahead for the next start on a matching weekday.
	for offset := 0; offset < 8; offset++ {
		day := local.AddDate(0, 0, offset)
		wd := day.Weekday()
		if !w.days[wd] {
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), w.startMin/60, w.startMin%60, 0, 0, loc)
		if offset == 0 && !local.Before(start) {
			// Today's start already passed; if we're inside overnight from today
			// containment handles it. Look for a later start.
			continue
		}
		endLabel := formatHHMM(w.endMin)
		label := fmt.Sprintf("%s %s-%s", wd.String()[:3], formatHHMM(w.startMin), endLabel)
		if offset > 0 {
			label = fmt.Sprintf("%s (%s)", label, start.Format("2006-01-02 15:04 MST"))
		} else {
			label = fmt.Sprintf("%s (%s)", label, start.Format("15:04 MST"))
		}
		return start, label, true
	}
	return time.Time{}, "", false
}

func prevWeekday(d time.Weekday) time.Weekday {
	if d == time.Sunday {
		return time.Saturday
	}
	return d - 1
}

func formatHHMM(min int) string {
	if min < 0 {
		min = 0
	}
	min = min % (24 * 60)
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time %q (want HH:MM)", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h*60 + m, nil
}

func parseWeekdays(raw string) (map[time.Weekday]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty weekday list")
	}
	days := map[time.Weekday]bool{}
	// Support ranges and comma lists: Mon-Fri, Sat,Sun, Mon-Wed,Fri
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '|'
	})
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.Contains(tok, "-") {
			ends := strings.SplitN(tok, "-", 2)
			if len(ends) != 2 {
				return nil, fmt.Errorf("invalid weekday range %q", tok)
			}
			start, err := parseWeekdayName(ends[0])
			if err != nil {
				return nil, err
			}
			end, err := parseWeekdayName(ends[1])
			if err != nil {
				return nil, err
			}
			for d := start; ; d = (d + 1) % 7 {
				days[d] = true
				if d == end {
					break
				}
			}
			continue
		}
		d, err := parseWeekdayName(tok)
		if err != nil {
			return nil, err
		}
		days[d] = true
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("no weekdays parsed from %q", raw)
	}
	return days, nil
}

func parseWeekdayName(s string) (time.Weekday, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "sun", "sunday":
		return time.Sunday, nil
	case "mon", "monday":
		return time.Monday, nil
	case "tue", "tues", "tuesday":
		return time.Tuesday, nil
	case "wed", "wednesday":
		return time.Wednesday, nil
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday, nil
	case "fri", "friday":
		return time.Friday, nil
	case "sat", "saturday":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday %q", s)
	}
}
