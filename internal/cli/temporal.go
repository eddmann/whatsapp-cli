package cli

import (
	"fmt"
	"time"
)

// TimeframePreset represents a named time range.
type TimeframePreset string

const (
	TimeframeLastHour  TimeframePreset = "last_hour"
	TimeframeToday     TimeframePreset = "today"
	TimeframeYesterday TimeframePreset = "yesterday"
	TimeframeLast3Days TimeframePreset = "last_3_days"
	TimeframeThisWeek  TimeframePreset = "this_week"
	TimeframeLastWeek  TimeframePreset = "last_week"
	TimeframeThisMonth TimeframePreset = "this_month"
)

// ParseTimeframe converts a timeframe preset string into after/before timestamps.
func ParseTimeframe(timeframe string) (after string, before string, err error) {
	if timeframe == "" {
		return "", "", nil
	}

	now := time.Now()
	var afterTime, beforeTime time.Time

	switch TimeframePreset(timeframe) {
	case TimeframeLastHour:
		afterTime = now.Add(-1 * time.Hour)
		beforeTime = now

	case TimeframeToday:
		afterTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		beforeTime = now

	case TimeframeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		afterTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		beforeTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	case TimeframeLast3Days:
		afterTime = now.AddDate(0, 0, -3)
		beforeTime = now

	case TimeframeThisWeek:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		daysToMonday := weekday - 1
		monday := now.AddDate(0, 0, -daysToMonday)
		afterTime = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
		beforeTime = now

	case TimeframeLastWeek:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		daysToLastMonday := weekday + 6
		lastMonday := now.AddDate(0, 0, -daysToLastMonday)
		afterTime = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, now.Location())
		lastSunday := lastMonday.AddDate(0, 0, 6)
		beforeTime = time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 23, 59, 59, 0, now.Location())

	case TimeframeThisMonth:
		afterTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		beforeTime = now

	default:
		return "", "", fmt.Errorf("invalid timeframe: %s (valid: last_hour, today, yesterday, last_3_days, this_week, last_week, this_month)", timeframe)
	}

	return afterTime.Format(time.RFC3339), beforeTime.Format(time.RFC3339), nil
}
