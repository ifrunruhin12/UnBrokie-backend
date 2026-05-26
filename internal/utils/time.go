package utils

import "time"

func ymd(t time.Time, loc *time.Location) (int, time.Month, int) {
	t = t.In(loc)
	y, m, d := t.Date()
	return y, m, d
}

func StartOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := ymd(t, loc)
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func NextDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := ymd(t, loc)
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

func EndOfDay(t time.Time, loc *time.Location) time.Time {
	return NextDay(t, loc).Add(-time.Nanosecond)
}

func StartOfMonth(t time.Time, loc *time.Location) time.Time {
	y, m, _ := ymd(t, loc)
	return time.Date(y, m, 1, 0, 0, 0, 0, loc)
}

func StartOfNextMonth(t time.Time, loc *time.Location) time.Time {
	y, m, _ := ymd(t, loc)
	return time.Date(y, m+1, 1, 0, 0, 0, 0, loc)
}

func EndOfMonth(t time.Time, loc *time.Location) time.Time {
	return StartOfNextMonth(t, loc).Add(-time.Nanosecond)
}

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
