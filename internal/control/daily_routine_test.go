// pattern: Functional Core
package control

import (
	"testing"

	"polis/internal/core"
)

func TestValidateCreateDailyRoutineRequest(t *testing.T) {
	valid := CreateDailyRoutineRequest{
		RoutineID: "routine-1", EmployeeID: "emp-backend", TaskInstruction: "Review the assigned queue.",
		Timezone: "Asia/Shanghai", LocalTime: "09:00", NextLogicalDay: "2026-09-30",
		CatchUpPolicy: string(core.RoutineCatchUpCoalesceLatest), MaxCatchUp: 1, RequestID: "routine-create-1",
	}
	if err := validateCreateDailyRoutineRequest("company-1", "mission-1", valid); err != nil {
		t.Fatalf("valid Routine request rejected: %v", err)
	}
	for name, request := range map[string]CreateDailyRoutineRequest{
		"missing instruction": func() CreateDailyRoutineRequest { item := valid; item.TaskInstruction = "   "; return item }(),
		"invalid policy":      func() CreateDailyRoutineRequest { item := valid; item.CatchUpPolicy = "unbounded"; return item }(),
		"missing request id":  func() CreateDailyRoutineRequest { item := valid; item.RequestID = ""; return item }(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateCreateDailyRoutineRequest("company-1", "mission-1", request); err == nil {
				t.Fatalf("invalid Routine request was accepted: %+v", request)
			}
		})
	}
}
