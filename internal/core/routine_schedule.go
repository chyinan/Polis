// pattern: Functional Core
package core

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type RoutineCatchUpPolicy string

const (
	RoutineCatchUpSkip              RoutineCatchUpPolicy = "skip"
	RoutineCatchUpCoalesceLatest    RoutineCatchUpPolicy = "coalesce_latest"
	RoutineCatchUpBounded           RoutineCatchUpPolicy = "catch_up"
	MaxDailyRoutineCatchUp                               = 10
	MaxDailyRoutineInstructionBytes                      = 4096
)

// DailyRoutineSchedule is one bounded calendar profile. NextLogicalDay is the
// first local date whose occurrence has not been materialized.
type DailyRoutineSchedule struct {
	RoutineID       string
	TaskInstruction string
	Timezone        string
	LocalTime       string
	NextLogicalDay  string
	CatchUpPolicy   RoutineCatchUpPolicy
	MaxCatchUp      int
}

type DailyRoutineOccurrence struct {
	OccurrenceKey    string
	LogicalDay       string
	ScheduledAt      time.Time
	CoalescedFrom    string
	CoalescedThrough string
	State            string
	TaskID           string
}

type DailyRoutinePlan struct {
	Occurrences    []DailyRoutineOccurrence
	SkippedFrom    string
	SkippedThrough string
	NextLogicalDay string
}

// ValidateDailyRoutineTaskInstruction bounds the user-authored work text that
// will be frozen into each occurrence and delivered as its Task plan.
func ValidateDailyRoutineTaskInstruction(instruction string) error {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" || trimmed != instruction || len(instruction) > MaxDailyRoutineInstructionBytes || !utf8.ValidString(instruction) {
		return Malformed
	}
	return nil
}

// PlanDailyRoutineOccurrences creates deterministic occurrence keys from a
// routine and its local logical day. Skip preserves only the latest due window;
// coalesce_latest records one latest occurrence covering the missed range; and
// catch_up materializes the oldest windows up to MaxDailyRoutineCatchUp, then
// explicitly skips the remainder.
func PlanDailyRoutineOccurrences(schedule DailyRoutineSchedule, now time.Time) (DailyRoutinePlan, error) {
	location, err := validateDailyRoutineSchedule(schedule)
	if err != nil || now.IsZero() {
		return DailyRoutinePlan{}, Malformed
	}
	nextDay, _ := parseRoutineLogicalDay(schedule.NextLogicalDay)
	currentLocal := now.In(location)
	latestDueDay := currentLocal.Format(time.DateOnly)
	if currentLocal.Format("15:04") < schedule.LocalTime {
		latestDueDay = routineLogicalDayOffset(latestDueDay, -1)
	}
	latestDay, err := parseRoutineLogicalDay(latestDueDay)
	if err != nil {
		return DailyRoutinePlan{}, err
	}
	plan := DailyRoutinePlan{Occurrences: make([]DailyRoutineOccurrence, 0), NextLogicalDay: schedule.NextLogicalDay}
	if nextDay.After(latestDay) {
		return plan, nil
	}
	dueCount := int(latestDay.Sub(nextDay).Hours()/24) + 1
	if dueCount < 1 {
		return DailyRoutinePlan{}, fmt.Errorf("routine logical day ordering is invalid")
	}
	firstDay := schedule.NextLogicalDay
	addOccurrence := func(day, coalescedFrom, coalescedThrough string) error {
		dueAt, dueErr := routineLocalOccurrenceTime(day, schedule.LocalTime, location)
		if dueErr != nil {
			return dueErr
		}
		plan.Occurrences = append(plan.Occurrences, DailyRoutineOccurrence{
			OccurrenceKey: schedule.RoutineID + ":" + day, LogicalDay: day,
			ScheduledAt: dueAt, CoalescedFrom: coalescedFrom, CoalescedThrough: coalescedThrough,
		})
		return nil
	}
	switch schedule.CatchUpPolicy {
	case RoutineCatchUpSkip:
		if dueCount > 1 {
			plan.SkippedFrom = firstDay
			plan.SkippedThrough = routineLogicalDayOffset(latestDueDay, -1)
		}
		if err = addOccurrence(latestDueDay, "", ""); err != nil {
			return DailyRoutinePlan{}, err
		}
	case RoutineCatchUpCoalesceLatest:
		coalescedFrom := ""
		coalescedThrough := ""
		if dueCount > 1 {
			coalescedFrom = firstDay
			coalescedThrough = latestDueDay
		}
		if err = addOccurrence(latestDueDay, coalescedFrom, coalescedThrough); err != nil {
			return DailyRoutinePlan{}, err
		}
	case RoutineCatchUpBounded:
		materialized := dueCount
		if materialized > schedule.MaxCatchUp {
			materialized = schedule.MaxCatchUp
		}
		for index := 0; index < materialized; index++ {
			if err = addOccurrence(routineLogicalDayOffset(firstDay, index), "", ""); err != nil {
				return DailyRoutinePlan{}, err
			}
		}
		if dueCount > materialized {
			plan.SkippedFrom = routineLogicalDayOffset(firstDay, materialized)
			plan.SkippedThrough = latestDueDay
		}
	}
	plan.NextLogicalDay = routineLogicalDayOffset(latestDueDay, 1)
	return plan, nil
}

// ValidateDailyRoutineSchedule checks the fixed daily profile without depending
// on a wall clock, so the same validation is used when schedules are persisted.
func ValidateDailyRoutineSchedule(schedule DailyRoutineSchedule) error {
	_, err := validateDailyRoutineSchedule(schedule)
	return err
}

func validateDailyRoutineSchedule(schedule DailyRoutineSchedule) (*time.Location, error) {
	if !ValidID(schedule.RoutineID) || schedule.MaxCatchUp < 1 || schedule.MaxCatchUp > MaxDailyRoutineCatchUp {
		return nil, Malformed
	}
	if schedule.Timezone == "Local" {
		return nil, Malformed
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return nil, Malformed
	}
	localClock, err := time.Parse("15:04", schedule.LocalTime)
	if err != nil || localClock.Format("15:04") != schedule.LocalTime {
		return nil, Malformed
	}
	if _, err = parseRoutineLogicalDay(schedule.NextLogicalDay); err != nil {
		return nil, Malformed
	}
	if schedule.CatchUpPolicy != RoutineCatchUpSkip && schedule.CatchUpPolicy != RoutineCatchUpCoalesceLatest && schedule.CatchUpPolicy != RoutineCatchUpBounded {
		return nil, Malformed
	}
	return location, nil
}

// ResolveDailyRoutineTime returns the deterministic UTC instant for one local
// logical day using the daily Routine time-zone rules.
func ResolveDailyRoutineTime(logicalDay, localTime, timezone string) (time.Time, error) {
	if timezone == "Local" {
		return time.Time{}, Malformed
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, Malformed
	}
	return routineLocalOccurrenceTime(logicalDay, localTime, location)
}

func parseRoutineLogicalDay(value string) (time.Time, error) {
	day, err := time.Parse(time.DateOnly, value)
	if err != nil || day.Format(time.DateOnly) != value {
		return time.Time{}, Malformed
	}
	return day.UTC(), nil
}

func routineLogicalDayOffset(value string, days int) string {
	day, _ := time.Parse(time.DateOnly, value)
	return day.AddDate(0, 0, days).Format(time.DateOnly)
}

func routineLocalOccurrenceTime(logicalDay, localTime string, location *time.Location) (time.Time, error) {
	day, err := parseRoutineLogicalDay(logicalDay)
	if err != nil {
		return time.Time{}, err
	}
	clock, err := time.Parse("15:04", localTime)
	if err != nil || clock.Format("15:04") != localTime {
		return time.Time{}, Malformed
	}
	// Search the bounded UTC neighborhood of this wall-clock timestamp. This
	// handles ordinary offsets, DST gaps, and DST folds without relying on
	// time.Date's implementation-specific choice for ambiguous/nonexistent times.
	wantedMinute := clock.Hour()*60 + clock.Minute()
	wallClockAsUTC := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, time.UTC)
	start := wallClockAsUTC.Add(-18 * time.Hour)
	end := wallClockAsUTC.Add(18 * time.Hour)
	var exact, nextValid time.Time
	nextValidMinute := 24 * 60
	for candidate := start; !candidate.After(end); candidate = candidate.Add(time.Minute) {
		local := candidate.In(location)
		if local.Format(time.DateOnly) != logicalDay {
			continue
		}
		minute := local.Hour()*60 + local.Minute()
		if minute == wantedMinute {
			if exact.IsZero() || candidate.Before(exact) {
				exact = candidate
			}
			continue
		}
		if minute >= wantedMinute && minute < nextValidMinute {
			nextValid = candidate
			nextValidMinute = minute
		}
	}
	if !exact.IsZero() {
		return exact.UTC(), nil
	}
	if !nextValid.IsZero() {
		return nextValid.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("routine logical day or local time does not exist in timezone")
}
