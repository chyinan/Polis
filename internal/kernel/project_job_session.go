// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

func (k *Kernel) txCreateProjectJobSession(ctx context.Context, tx pgx.Tx, companyID, taskID, targetProfile, requestID string) (string, error) {
	var taskState, taskKind, taskOwner, missionState string
	var employeeEpoch int64
	err := tx.QueryRow(ctx, `SELECT t.state,t.kind,t.owner,m.state,e.epoch
FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner
WHERE t.company_id=$1 AND t.id=$2 FOR UPDATE OF t,m,e`, companyID, taskID).Scan(&taskState, &taskKind, &taskOwner, &missionState, &employeeEpoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", core.OutOfScope
	}
	if err != nil {
		return "", err
	}
	if taskState != "working" || taskKind != string(core.TaskKindCompat) || taskOwner != core.EmployeeBackendID || missionState != "active" {
		return "", core.Denied
	}
	var activeSession bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND employee_id=$2 AND state<>'stopped')`, companyID, taskOwner).Scan(&activeSession); err != nil {
		return "", err
	}
	if activeSession {
		return "", core.Denied
	}
	slotClass, err := checkInstallationWorkerSlotCapacityTX(ctx, tx, taskKind)
	if err != nil {
		return "", err
	}
	var generation int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(generation),0)+1 FROM worker_sessions WHERE company_id=$1 AND task_id=$2`, companyID, taskID).Scan(&generation); err != nil {
		return "", err
	}
	sessionID := stableCapabilityID("project-job-session", companyID, requestID)
	if _, err = tx.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,execution_mode)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active','project_job')`, companyID, sessionID, taskOwner, taskID, generation, employeeEpoch, k.incarnation, "project-job:"+targetProfile); err != nil {
		if isUniqueViolation(err) {
			return "", core.Conflict
		}
		return "", err
	}
	if err = recordInstallationWorkerSlotReservationTX(ctx, tx, companyID, sessionID, slotClass); err != nil {
		return "", err
	}
	return sessionID, nil
}
