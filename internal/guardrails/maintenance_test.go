package guardrails

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseMaintenanceWindow_HumanWeekly(t *testing.T) {
	w, err := ParseMaintenanceWindow("Mon-Fri 22:00-06:00")
	if err != nil {
		t.Fatal(err)
	}
	ww, ok := w.(weeklyWindow)
	if !ok {
		t.Fatalf("got %T", w)
	}
	if !ww.days[time.Monday] || !ww.days[time.Friday] || ww.days[time.Saturday] {
		t.Fatalf("unexpected days: %v", ww.days)
	}
	if ww.startMin != 22*60 || ww.endMin != 6*60 {
		t.Fatalf("times start=%d end=%d", ww.startMin, ww.endMin)
	}
}

func TestParseMaintenanceWindow_HumanList(t *testing.T) {
	w, err := ParseMaintenanceWindow("Sat,Sun 00:00-23:59")
	if err != nil {
		t.Fatal(err)
	}
	ww, ok := w.(weeklyWindow)
	if !ok {
		t.Fatalf("got %T", w)
	}
	if !ww.days[time.Saturday] || !ww.days[time.Sunday] || ww.days[time.Monday] {
		t.Fatalf("days: %v", ww.days)
	}
}

func TestParseMaintenanceWindow_Explicit(t *testing.T) {
	w, err := ParseMaintenanceWindow("weekday=sat,sun;start=00:00;end=23:59")
	if err != nil {
		t.Fatal(err)
	}
	ww, ok := w.(weeklyWindow)
	if !ok {
		t.Fatalf("got %T", w)
	}
	if !ww.days[time.Saturday] || !ww.days[time.Sunday] {
		t.Fatalf("days: %v", ww.days)
	}
	if ww.startMin != 0 || ww.endMin != 23*60+59 {
		t.Fatalf("times %d-%d", ww.startMin, ww.endMin)
	}
}

func TestParseMaintenanceWindow_AbsoluteRFC3339(t *testing.T) {
	w, err := ParseMaintenanceWindow("2026-07-29T22:00:00-03:00/2026-07-30T06:00:00-03:00")
	if err != nil {
		t.Fatal(err)
	}
	aw, ok := w.(absoluteWindow)
	if !ok {
		t.Fatalf("got %T", w)
	}
	if aw.start.Hour() != 22 || aw.end.Hour() != 6 {
		t.Fatalf("got start=%v end=%v", aw.start, aw.end)
	}

	w2, err := ParseMaintenanceWindow("2026-07-29T22:00:00Z..2026-07-30T06:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w2.(absoluteWindow); !ok {
		t.Fatalf("got %T", w2)
	}
}

func TestParseMaintenanceWindow_Invalid(t *testing.T) {
	cases := []string{
		"",
		"not-a-window",
		"Mon-Fri",
		"Mon-Fri 25:00-06:00",
		"weekday=sat;start=00:00", // missing end
		"2026-07-29T22:00:00Z/2026-07-28T06:00:00Z", // end before start
	}
	for _, c := range cases {
		if _, err := ParseMaintenanceWindow(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestWeeklyWindow_ContainsSameDay(t *testing.T) {
	w, err := ParseMaintenanceWindow("Mon-Fri 09:00-17:00")
	if err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("TEST", -3*3600)
	// Monday 2026-07-27 is a Monday.
	inside := time.Date(2026, 7, 27, 12, 0, 0, 0, loc)
	outsideEarly := time.Date(2026, 7, 27, 8, 0, 0, 0, loc)
	outsideLate := time.Date(2026, 7, 27, 17, 1, 0, 0, loc)
	weekend := time.Date(2026, 7, 25, 12, 0, 0, 0, loc) // Saturday

	if !w.Contains(inside) {
		t.Fatal("expected inside")
	}
	if w.Contains(outsideEarly) || w.Contains(outsideLate) {
		t.Fatal("expected outside on weekday edges")
	}
	if w.Contains(weekend) {
		t.Fatal("expected outside on weekend")
	}
}

func TestWeeklyWindow_ContainsOvernight(t *testing.T) {
	w, err := ParseMaintenanceWindow("Mon-Fri 22:00-06:00")
	if err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("TEST", 0)
	// Mon 23:00 inside; Tue 03:00 inside (from Mon overnight); Tue 07:00 outside;
	// Sat 03:00 inside (from Fri overnight); Sat 23:00 outside.
	monLate := time.Date(2026, 7, 27, 23, 0, 0, 0, loc)
	tueEarly := time.Date(2026, 7, 28, 3, 0, 0, 0, loc)
	tueLate := time.Date(2026, 7, 28, 7, 0, 0, 0, loc)
	satEarly := time.Date(2026, 7, 25, 3, 0, 0, 0, loc) // Fri overnight into Sat
	satLate := time.Date(2026, 7, 25, 23, 0, 0, 0, loc)

	if !w.Contains(monLate) {
		t.Fatal("Mon 23:00 should be inside")
	}
	if !w.Contains(tueEarly) {
		t.Fatal("Tue 03:00 should be inside (Mon overnight)")
	}
	if w.Contains(tueLate) {
		t.Fatal("Tue 07:00 should be outside")
	}
	if !w.Contains(satEarly) {
		t.Fatal("Sat 03:00 should be inside (Fri overnight)")
	}
	if w.Contains(satLate) {
		t.Fatal("Sat 23:00 should be outside")
	}
}

func TestCheckMaintenanceWindow_EmptyAllows(t *testing.T) {
	if err := CheckMaintenanceWindow(nil, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := CheckMaintenanceWindow([]string{"", "  "}, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func TestCheckMaintenanceWindow_InsideOutsideForce(t *testing.T) {
	t.Setenv("ROS_SKIP_MAINTENANCE_GATE", "")
	windows := []string{"Mon-Fri 22:00-06:00"}
	loc := time.FixedZone("TEST", 0)
	inside := time.Date(2026, 7, 27, 23, 0, 0, 0, loc)  // Mon
	outside := time.Date(2026, 7, 27, 12, 0, 0, 0, loc) // Mon noon

	if err := CheckMaintenanceWindow(windows, inside, false); err != nil {
		t.Fatalf("inside: %v", err)
	}
	err := CheckMaintenanceWindow(windows, outside, false)
	if err == nil {
		t.Fatal("expected outside error")
	}
	var out *ErrOutsideMaintenanceWindow
	if !errors.As(err, &out) {
		t.Fatalf("got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "outside maintenance window") {
		t.Errorf("message: %v", err)
	}
	if out.NextWindow == "" {
		t.Error("expected next window hint")
	}

	if err := CheckMaintenanceWindow(windows, outside, true); err != nil {
		t.Fatalf("force should bypass: %v", err)
	}
}

func TestCheckMaintenanceWindow_SkipEnv(t *testing.T) {
	t.Setenv("ROS_SKIP_MAINTENANCE_GATE", "1")
	windows := []string{"Mon-Fri 22:00-06:00"}
	outside := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	if err := CheckMaintenanceWindow(windows, outside, false); err != nil {
		t.Fatalf("skip env should bypass: %v", err)
	}
}

func TestCheckMaintenanceWindow_Absolute(t *testing.T) {
	t.Setenv("ROS_SKIP_MAINTENANCE_GATE", "")
	windows := []string{"2026-07-29T22:00:00Z/2026-07-30T06:00:00Z"}
	inside := time.Date(2026, 7, 30, 1, 0, 0, 0, time.UTC)
	before := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	if err := CheckMaintenanceWindow(windows, inside, false); err != nil {
		t.Fatal(err)
	}
	if err := CheckMaintenanceWindow(windows, before, false); err == nil {
		t.Fatal("expected refuse before window")
	}
	if err := CheckMaintenanceWindow(windows, after, false); err == nil {
		t.Fatal("expected refuse after window")
	}
}

func TestCheckMaintenanceWindow_ExplicitWeekend(t *testing.T) {
	t.Setenv("ROS_SKIP_MAINTENANCE_GATE", "")
	windows := []string{"weekday=sat,sun;start=00:00;end=23:59"}
	loc := time.UTC
	sat := time.Date(2026, 7, 25, 15, 0, 0, 0, loc)
	mon := time.Date(2026, 7, 27, 15, 0, 0, 0, loc)
	if err := CheckMaintenanceWindow(windows, sat, false); err != nil {
		t.Fatal(err)
	}
	if err := CheckMaintenanceWindow(windows, mon, false); err == nil {
		t.Fatal("monday should be refused")
	}
}
