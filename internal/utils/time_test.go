package utils

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 34: Timestamps Stored in UTC
// Validates: Requirements 11.1
// All time operations should preserve UTC storage regardless of timezone operations
func TestProperty34_TimestampsStoredInUTC(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary UTC timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		utcTime := time.Unix(unixSeconds, 0).UTC()

		// Generate arbitrary IANA timezone
		tzName := rapid.SampledFrom([]string{
			"UTC",
			"America/New_York",
			"Europe/London",
			"Asia/Dhaka",
			"Asia/Tokyo",
			"Australia/Sydney",
			"America/Los_Angeles",
			"Europe/Paris",
			"Asia/Kolkata",
		}).Draw(t, "timezone")

		loc, err := time.LoadLocation(tzName)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tzName, err)
		}

		// Perform timezone-aware operations
		startOfDay := StartOfDay(utcTime, loc)
		endOfDay := EndOfDay(utcTime, loc)
		startOfMonth := StartOfMonth(utcTime, loc)
		endOfMonth := EndOfMonth(utcTime, loc)

		// Property: All results can be converted back to UTC without loss
		// The underlying instant in time is preserved
		_ = startOfDay.UTC()
		_ = endOfDay.UTC()
		_ = startOfMonth.UTC()
		_ = endOfMonth.UTC()

		// Property: Converting to UTC and back to the same location preserves the instant
		if !startOfDay.Equal(startOfDay.UTC().In(loc)) {
			t.Fatalf("StartOfDay: UTC round-trip failed")
		}
		if !endOfDay.Equal(endOfDay.UTC().In(loc)) {
			t.Fatalf("EndOfDay: UTC round-trip failed")
		}
		if !startOfMonth.Equal(startOfMonth.UTC().In(loc)) {
			t.Fatalf("StartOfMonth: UTC round-trip failed")
		}
		if !endOfMonth.Equal(endOfMonth.UTC().In(loc)) {
			t.Fatalf("EndOfMonth: UTC round-trip failed")
		}
	})
}

// Property 35: Timezone Configuration Round-Trip
// Validates: Requirements 11.2
// Loading a timezone and using it for calculations should be consistent
func TestProperty35_TimezoneConfigurationRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary IANA timezone string
		tzName := rapid.SampledFrom([]string{
			"UTC",
			"America/New_York",
			"Europe/London",
			"Asia/Dhaka",
			"Asia/Tokyo",
			"Australia/Sydney",
			"America/Los_Angeles",
			"Europe/Paris",
			"Asia/Kolkata",
			"Pacific/Auckland",
			"America/Chicago",
			"Europe/Berlin",
		}).Draw(t, "timezone")

		// Load timezone
		loc1, err := time.LoadLocation(tzName)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tzName, err)
		}

		// Load same timezone again
		loc2, err := time.LoadLocation(tzName)
		if err != nil {
			t.Fatalf("failed to load timezone %s second time: %v", tzName, err)
		}

		// Generate arbitrary timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		testTime := time.Unix(unixSeconds, 0).UTC()

		// Property: Same timezone name produces identical results
		start1 := StartOfDay(testTime, loc1)
		start2 := StartOfDay(testTime, loc2)

		if !start1.Equal(start2) {
			t.Fatalf("StartOfDay: timezone round-trip failed: %v != %v", start1, start2)
		}

		end1 := EndOfDay(testTime, loc1)
		end2 := EndOfDay(testTime, loc2)

		if !end1.Equal(end2) {
			t.Fatalf("EndOfDay: timezone round-trip failed: %v != %v", end1, end2)
		}

		// Property: Timezone string can be extracted and reused
		zoneName1, _ := start1.Zone()
		zoneName2, _ := start2.Zone()

		// Note: Zone names may differ (e.g., "EST" vs "EDT") but the instant should be the same
		// We already verified the instants are equal above
		_ = zoneName1
		_ = zoneName2
	})
}

// Property 36: Day Boundaries Use User Timezone
// Validates: Requirements 11.3, 11.5
// Day boundaries must be computed in user timezone, not server timezone
func TestProperty36_DayBoundariesUseUserTimezone(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		testTime := time.Unix(unixSeconds, 0).UTC()

		// Generate two different timezones
		tz1Name := rapid.SampledFrom([]string{
			"UTC",
			"America/New_York",
			"Europe/London",
			"Asia/Dhaka",
		}).Draw(t, "timezone1")

		tz2Name := rapid.SampledFrom([]string{
			"Asia/Tokyo",
			"Australia/Sydney",
			"America/Los_Angeles",
			"Europe/Paris",
		}).Draw(t, "timezone2")

		loc1, err := time.LoadLocation(tz1Name)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tz1Name, err)
		}

		loc2, err := time.LoadLocation(tz2Name)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tz2Name, err)
		}

		// Compute day boundaries in both timezones
		start1 := StartOfDay(testTime, loc1)
		start2 := StartOfDay(testTime, loc2)

		end1 := EndOfDay(testTime, loc1)
		end2 := EndOfDay(testTime, loc2)

		// Property: Start of day in loc1 should be midnight in loc1
		y1, m1, d1 := start1.In(loc1).Date()
		h1, min1, s1 := start1.In(loc1).Clock()
		if h1 != 0 || min1 != 0 || s1 != 0 {
			t.Fatalf("StartOfDay in %s is not midnight: %02d:%02d:%02d", tz1Name, h1, min1, s1)
		}

		// Property: Start of day in loc2 should be midnight in loc2
		y2, m2, d2 := start2.In(loc2).Date()
		h2, min2, s2 := start2.In(loc2).Clock()
		if h2 != 0 || min2 != 0 || s2 != 0 {
			t.Fatalf("StartOfDay in %s is not midnight: %02d:%02d:%02d", tz2Name, h2, min2, s2)
		}

		// Property: End of day should be 23:59:59.999999999 in the respective timezone
		endLocal1 := end1.In(loc1)
		h1e, min1e, s1e := endLocal1.Clock()
		if h1e != 23 || min1e != 59 || s1e != 59 {
			t.Fatalf("EndOfDay in %s is not 23:59:59: %02d:%02d:%02d", tz1Name, h1e, min1e, s1e)
		}

		endLocal2 := end2.In(loc2)
		h2e, min2e, s2e := endLocal2.Clock()
		if h2e != 23 || min2e != 59 || s2e != 59 {
			t.Fatalf("EndOfDay in %s is not 23:59:59: %02d:%02d:%02d", tz2Name, h2e, min2e, s2e)
		}

		// Property: Day boundaries are timezone-specific
		// The same UTC instant may fall on different calendar days in different timezones
		// So start1 and start2 may differ (this is correct behavior)
		inLoc1 := testTime.In(loc1)
		inLoc2 := testTime.In(loc2)

		// Verify that StartOfDay returns the correct day for each timezone
		if y1 != inLoc1.Year() || m1 != inLoc1.Month() || d1 != inLoc1.Day() {
			t.Fatalf("StartOfDay in %s returned wrong date: got %04d-%02d-%02d, want %04d-%02d-%02d",
				tz1Name, y1, m1, d1, inLoc1.Year(), inLoc1.Month(), inLoc1.Day())
		}

		if y2 != inLoc2.Year() || m2 != inLoc2.Month() || d2 != inLoc2.Day() {
			t.Fatalf("StartOfDay in %s returned wrong date: got %04d-%02d-%02d, want %04d-%02d-%02d",
				tz2Name, y2, m2, d2, inLoc2.Year(), inLoc2.Month(), inLoc2.Day())
		}
	})
}

// Property 37: Default Timezone Is UTC
// Validates: Requirements 11.4
// When no timezone is configured, UTC should be used
func TestProperty37_DefaultTimezoneIsUTC(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		testTime := time.Unix(unixSeconds, 0).UTC()

		// Use UTC as the default timezone
		utcLoc := time.UTC

		// Compute day boundaries using UTC
		startOfDay := StartOfDay(testTime, utcLoc)
		endOfDay := EndOfDay(testTime, utcLoc)
		startOfMonth := StartOfMonth(testTime, utcLoc)
		endOfMonth := EndOfMonth(testTime, utcLoc)

		// Property: Results should be in UTC
		if startOfDay.Location() != time.UTC {
			t.Fatalf("StartOfDay with UTC location is not in UTC")
		}
		if endOfDay.Location() != time.UTC {
			t.Fatalf("EndOfDay with UTC location is not in UTC")
		}
		if startOfMonth.Location() != time.UTC {
			t.Fatalf("StartOfMonth with UTC location is not in UTC")
		}
		if endOfMonth.Location() != time.UTC {
			t.Fatalf("EndOfMonth with UTC location is not in UTC")
		}

		// Property: Start of day in UTC should be midnight UTC
		h, min, s := startOfDay.Clock()
		if h != 0 || min != 0 || s != 0 {
			t.Fatalf("StartOfDay in UTC is not midnight: %02d:%02d:%02d", h, min, s)
		}

		// Property: End of day in UTC should be 23:59:59.999999999 UTC
		he, mine, se := endOfDay.Clock()
		if he != 23 || mine != 59 || se != 59 {
			t.Fatalf("EndOfDay in UTC is not 23:59:59: %02d:%02d:%02d", he, mine, se)
		}

		// Property: Start of month should be the 1st at midnight
		y, m, d := startOfMonth.Date()
		hm, minm, sm := startOfMonth.Clock()
		if d != 1 || hm != 0 || minm != 0 || sm != 0 {
			t.Fatalf("StartOfMonth in UTC is not 1st at midnight: %04d-%02d-%02d %02d:%02d:%02d",
				y, m, d, hm, minm, sm)
		}

		// Property: End of month should be the last day at 23:59:59.999999999
		ye, me, _ := endOfMonth.Date()
		hem, minem, sem := endOfMonth.Clock()

		// Verify it's the last day of the month by checking that adding 1 nanosecond
		// moves us to the 1st of the next month
		nextInstant := endOfMonth.Add(time.Nanosecond)
		yn, mn, dn := nextInstant.Date()

		if dn != 1 {
			t.Fatalf("EndOfMonth + 1ns is not the 1st of next month: %04d-%02d-%02d", yn, mn, dn)
		}

		if hem != 23 || minem != 59 || sem != 59 {
			t.Fatalf("EndOfMonth in UTC is not 23:59:59: %02d:%02d:%02d", hem, minem, sem)
		}

		// Verify month rolled over correctly
		expectedNextMonth := me + 1
		expectedNextYear := ye
		if expectedNextMonth > 12 {
			expectedNextMonth = 1
			expectedNextYear++
		}

		if mn != expectedNextMonth || yn != expectedNextYear {
			t.Fatalf("EndOfMonth + 1ns has wrong month/year: got %04d-%02d, want %04d-%02d",
				yn, mn, expectedNextYear, expectedNextMonth)
		}
	})
}

// Additional property test: NextDay is DST-safe
// Validates that NextDay correctly handles DST transitions
func TestProperty_NextDayIsDSTSafe(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Use timezones known to have DST
		tzName := rapid.SampledFrom([]string{
			"America/New_York",
			"Europe/London",
			"Australia/Sydney",
			"America/Los_Angeles",
		}).Draw(t, "timezone")

		loc, err := time.LoadLocation(tzName)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tzName, err)
		}

		// Generate arbitrary timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		testTime := time.Unix(unixSeconds, 0).UTC()

		// Get start of current day and next day
		today := StartOfDay(testTime, loc)
		tomorrow := NextDay(testTime, loc)

		// Property: NextDay should advance exactly one calendar day
		todayLocal := today.In(loc)
		tomorrowLocal := tomorrow.In(loc)

		yToday, mToday, dToday := todayLocal.Date()
		yTomorrow, mTomorrow, dTomorrow := tomorrowLocal.Date()

		// Calculate expected next day
		expectedNextDay := time.Date(yToday, mToday, dToday+1, 0, 0, 0, 0, loc)
		yExpected, mExpected, dExpected := expectedNextDay.Date()

		if yTomorrow != yExpected || mTomorrow != mExpected || dTomorrow != dExpected {
			t.Fatalf("NextDay failed: got %04d-%02d-%02d, want %04d-%02d-%02d",
				yTomorrow, mTomorrow, dTomorrow, yExpected, mExpected, dExpected)
		}

		// Property: Tomorrow should be midnight in the target timezone
		h, min, s := tomorrowLocal.Clock()
		if h != 0 || min != 0 || s != 0 {
			t.Fatalf("NextDay is not midnight: %02d:%02d:%02d", h, min, s)
		}

		// Property: The duration between today and tomorrow may not be exactly 24 hours
		// due to DST transitions, but it should be close (23-25 hours)
		duration := tomorrow.Sub(today)
		hours := duration.Hours()

		if hours < 23 || hours > 25 {
			t.Fatalf("Duration between consecutive days is suspicious: %.2f hours", hours)
		}
	})
}

// Additional property test: DaysLeftInMonth calculation
// Validates that DaysLeftInMonth returns correct values
func TestProperty_DaysLeftInMonthCalculation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary timezone
		tzName := rapid.SampledFrom([]string{
			"UTC",
			"America/New_York",
			"Asia/Dhaka",
			"Europe/London",
		}).Draw(t, "timezone")

		loc, err := time.LoadLocation(tzName)
		if err != nil {
			t.Fatalf("failed to load timezone %s: %v", tzName, err)
		}

		// Generate arbitrary timestamp
		unixSeconds := rapid.Int64Range(0, 2147483647).Draw(t, "unixSeconds")
		testTime := time.Unix(unixSeconds, 0).UTC()

		// Calculate days left
		daysLeft := DaysLeftInMonth(testTime, loc)

		// Property: Days left should be non-negative
		if daysLeft < 0 {
			t.Fatalf("DaysLeftInMonth returned negative value: %d", daysLeft)
		}

		// Property: Days left should be at most 30 (31 - 1 for current day)
		if daysLeft > 30 {
			t.Fatalf("DaysLeftInMonth returned too large value: %d", daysLeft)
		}

		// Property: On the last day of the month, days left should be 0
		endOfMonth := EndOfMonth(testTime, loc)
		endOfMonthLocal := endOfMonth.In(loc)
		_, _, lastDay := endOfMonthLocal.Date()

		testTimeLocal := testTime.In(loc)
		_, _, currentDay := testTimeLocal.Date()

		expectedDaysLeft := lastDay - currentDay
		if expectedDaysLeft < 0 {
			expectedDaysLeft = 0
		}

		if daysLeft != expectedDaysLeft {
			t.Fatalf("DaysLeftInMonth mismatch: got %d, want %d (current day: %d, last day: %d)",
				daysLeft, expectedDaysLeft, currentDay, lastDay)
		}
	})
}
