// pattern: Functional Core
package core

import (
	"strings"
	"testing"
	"time"
)

func TestValidateDailyRoutineTaskInstructionIsNonemptyCanonicalAndBounded(t *testing.T) {
	valid := "Check the selected routine inputs and record a next step."
	if err := ValidateDailyRoutineTaskInstruction(valid); err != nil {
		t.Fatalf("valid task instruction rejected: %v", err)
	}
	for name, instruction := range map[string]string{
		"empty":         "",
		"spaces":        "   ",
		"leading space": " " + valid,
		"oversized":     strings.Repeat("x", MaxDailyRoutineInstructionBytes+1),
		"invalid utf8":  string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateDailyRoutineTaskInstruction(instruction); err == nil {
				t.Fatalf("invalid task instruction %q was accepted", name)
			}
		})
	}
}

func TestPlanDailyRoutineOccurrencesAppliesCatchUpPolicy(t *testing.T) {
	now := time.Date(2026, time.September, 30, 2, 0, 0, 0, time.UTC) // 10:00 in Asia/Shanghai
	base := DailyRoutineSchedule{
		RoutineID: "routine-content-daily", TaskInstruction: "Review the assigned recurring responsibility.", Timezone: "Asia/Shanghai", LocalTime: "09:00",
		NextLogicalDay: "2026-09-27", MaxCatchUp: 2,
	}

	t.Run("skip older windows and run latest", func(t *testing.T) {
		plan, err := PlanDailyRoutineOccurrences(withRoutinePolicy(base, RoutineCatchUpSkip), now)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Occurrences) != 1 || plan.Occurrences[0].LogicalDay != "2026-09-30" || plan.Occurrences[0].OccurrenceKey != "routine-content-daily:2026-09-30" {
			t.Fatalf("skip plan occurrences=%+v", plan.Occurrences)
		}
		if plan.SkippedFrom != "2026-09-27" || plan.SkippedThrough != "2026-09-29" || plan.NextLogicalDay != "2026-10-01" {
			t.Fatalf("skip plan window=%+v", plan)
		}
	})

	t.Run("coalesce all overdue windows into latest", func(t *testing.T) {
		plan, err := PlanDailyRoutineOccurrences(withRoutinePolicy(base, RoutineCatchUpCoalesceLatest), now)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Occurrences) != 1 || plan.Occurrences[0].LogicalDay != "2026-09-30" || plan.Occurrences[0].CoalescedFrom != "2026-09-27" {
			t.Fatalf("coalesced plan occurrences=%+v", plan.Occurrences)
		}
		if plan.NextLogicalDay != "2026-10-01" {
			t.Fatalf("coalesced next logical day=%s", plan.NextLogicalDay)
		}
	})

	t.Run("single due window is not labeled as coalesced", func(t *testing.T) {
		schedule := base
		schedule.NextLogicalDay = "2026-09-30"
		schedule.CatchUpPolicy = RoutineCatchUpCoalesceLatest
		plan, err := PlanDailyRoutineOccurrences(schedule, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Occurrences) != 1 || plan.Occurrences[0].CoalescedFrom != "" || plan.Occurrences[0].CoalescedThrough != "" {
			t.Fatalf("single-window plan=%+v", plan.Occurrences)
		}
	})

	t.Run("catch up no more than the configured bound", func(t *testing.T) {
		plan, err := PlanDailyRoutineOccurrences(withRoutinePolicy(base, RoutineCatchUpBounded), now)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Occurrences) != 2 || plan.Occurrences[0].LogicalDay != "2026-09-27" || plan.Occurrences[1].LogicalDay != "2026-09-28" {
			t.Fatalf("bounded catch-up occurrences=%+v", plan.Occurrences)
		}
		if plan.SkippedFrom != "2026-09-29" || plan.SkippedThrough != "2026-09-30" || plan.NextLogicalDay != "2026-10-01" {
			t.Fatalf("bounded catch-up window=%+v", plan)
		}
	})
}

func TestPlanDailyRoutineOccurrencesWaitsBeforeLocalTriggerTime(t *testing.T) {
	schedule := DailyRoutineSchedule{
		RoutineID: "routine-content-daily", TaskInstruction: "Review the assigned recurring responsibility.", Timezone: "Asia/Shanghai", LocalTime: "09:00",
		NextLogicalDay: "2026-09-30", CatchUpPolicy: RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	plan, err := PlanDailyRoutineOccurrences(schedule, time.Date(2026, time.September, 30, 0, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Occurrences) != 0 || plan.NextLogicalDay != schedule.NextLogicalDay {
		t.Fatalf("pre-trigger plan=%+v, want no occurrence and unchanged cursor", plan)
	}
}

func TestPlanDailyRoutineOccurrencesResolvesDSTTransitionsDeterministically(t *testing.T) {
	base := DailyRoutineSchedule{
		RoutineID: "routine-content-daily", TaskInstruction: "Review the assigned recurring responsibility.", Timezone: "America/New_York", LocalTime: "02:30",
		NextLogicalDay: "2026-03-08", CatchUpPolicy: RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	now := time.Date(2026, time.March, 8, 8, 0, 0, 0, time.UTC)
	gapPlan, err := PlanDailyRoutineOccurrences(base, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(gapPlan.Occurrences) != 1 || !gapPlan.Occurrences[0].ScheduledAt.Equal(time.Date(2026, time.March, 8, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("DST gap plan=%+v, want first valid local minute 03:00 EDT", gapPlan)
	}
	foldSchedule := base
	foldSchedule.LocalTime = "01:30"
	foldSchedule.NextLogicalDay = "2026-11-01"
	foldPlan, err := PlanDailyRoutineOccurrences(foldSchedule, time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(foldPlan.Occurrences) != 1 || !foldPlan.Occurrences[0].ScheduledAt.Equal(time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)) {
		t.Fatalf("DST fold plan=%+v, want first 01:30 occurrence", foldPlan)
	}
	if _, err := PlanDailyRoutineOccurrences(DailyRoutineSchedule{RoutineID: base.RoutineID, TaskInstruction: base.TaskInstruction, Timezone: "Unknown/Zone", LocalTime: base.LocalTime, NextLogicalDay: base.NextLogicalDay, CatchUpPolicy: RoutineCatchUpCoalesceLatest, MaxCatchUp: 1}, now); err == nil {
		t.Fatal("unknown IANA time zone was accepted")
	}
	if _, err := PlanDailyRoutineOccurrences(DailyRoutineSchedule{RoutineID: base.RoutineID, TaskInstruction: base.TaskInstruction, Timezone: "Local", LocalTime: base.LocalTime, NextLogicalDay: base.NextLogicalDay, CatchUpPolicy: RoutineCatchUpCoalesceLatest, MaxCatchUp: 1}, now); err == nil {
		t.Fatal("host-local time zone was accepted")
	}
	if _, err := PlanDailyRoutineOccurrences(DailyRoutineSchedule{RoutineID: base.RoutineID, TaskInstruction: base.TaskInstruction, Timezone: base.Timezone, LocalTime: "9:00", NextLogicalDay: base.NextLogicalDay, CatchUpPolicy: RoutineCatchUpCoalesceLatest, MaxCatchUp: 1}, now); err == nil {
		t.Fatal("noncanonical local time was accepted")
	}
	if _, err := PlanDailyRoutineOccurrences(DailyRoutineSchedule{RoutineID: base.RoutineID, TaskInstruction: base.TaskInstruction, Timezone: base.Timezone, LocalTime: base.LocalTime, NextLogicalDay: base.NextLogicalDay, CatchUpPolicy: RoutineCatchUpBounded, MaxCatchUp: MaxDailyRoutineCatchUp + 1}, now); err == nil {
		t.Fatal("catch-up above the finite cap was accepted")
	}
}

func withRoutinePolicy(schedule DailyRoutineSchedule, policy RoutineCatchUpPolicy) DailyRoutineSchedule {
	schedule.CatchUpPolicy = policy
	return schedule
}
