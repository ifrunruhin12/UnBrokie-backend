package utils

import "time"

// ymd extracts year, month, day in the given location.
func ymd(t time.Time, loc *time.Location) (int, time.Month, int) {
	t = t.In(loc)
	y, m, d := t.Date()
	return y, m, d
}

// StartOfDay returns midnight of the given day in the provided timezone.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := ymd(t, loc)
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// NextDay returns the start of the next calendar day in loc.
// DST-safe (no Add(24h)).
func NextDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := ymd(t, loc)
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// EndOfDay returns the last nanosecond of the current day.
func EndOfDay(t time.Time, loc *time.Location) time.Time {
	return NextDay(t, loc).Add(-time.Nanosecond)
}

// StartOfMonth returns the first instant of the month in the given timezone.
func StartOfMonth(t time.Time, loc *time.Location) time.Time {
	y, m, _ := ymd(t, loc)
	return time.Date(y, m, 1, 0, 0, 0, 0, loc)
}

// StartOfNextMonth returns the first instant of the next month.
func StartOfNextMonth(t time.Time, loc *time.Location) time.Time {
	y, m, _ := ymd(t, loc)
	return time.Date(y, m+1, 1, 0, 0, 0, 0, loc)
}

// EndOfMonth returns the last nanosecond of the current month.
func EndOfMonth(t time.Time, loc *time.Location) time.Time {
	return StartOfNextMonth(t, loc).Add(-time.Nanosecond)
}

// DaysLeftInMonth returns remaining days after today.
func DaysLeftInMonth(t time.Time, loc *time.Location) int {
	now := t.In(loc)

	_, _, day := now.Date()
	last := EndOfMonth(t, loc)

	_, _, lastDay := last.Date()

	if lastDay < day {
		return 0
	}
	return lastDay - day
}