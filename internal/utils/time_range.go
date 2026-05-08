package utils

import "time"

// TimeRange represents a closed time window [Start, End].
// It is used by services for aggregation and projections.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// DayRange returns the full day window for a given time.
func DayRange(t time.Time, loc *time.Location) TimeRange {
	return TimeRange{
		Start: StartOfDay(t, loc),
		End:   EndOfDay(t, loc),
	}
}

// NextDayRange returns the full next calendar day window.
func NextDayRange(t time.Time, loc *time.Location) TimeRange {
	start := NextDay(t, loc)
	return TimeRange{
		Start: start,
		End:   EndOfDay(start, loc),
	}
}

// MonthRange returns the full month window.
func MonthRange(t time.Time, loc *time.Location) TimeRange {
	return TimeRange{
		Start: StartOfMonth(t, loc),
		End:   EndOfMonth(t, loc),
	}
}

// NextMonthRange returns the full next month window.
func NextMonthRange(t time.Time, loc *time.Location) TimeRange {
	start := StartOfNextMonth(t, loc)
	return TimeRange{
		Start: start,
		End:   EndOfMonth(start, loc),
	}
}

// RemainingMonthRange returns the window [tomorrow, EndOfMonth].
// This is the range the projection engine uses for "future planned expenses".
func RemainingMonthRange(t time.Time, loc *time.Location) TimeRange {
	return TimeRange{
		Start: NextDay(t, loc),
		End:   EndOfMonth(t, loc),
	}
}

// Contains checks if a timestamp is inside the range.
func (r TimeRange) Contains(t time.Time) bool {
	return !t.Before(r.Start) && !t.After(r.End)
}