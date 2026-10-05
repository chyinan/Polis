// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type EmployeeScheduleSnapshot struct {
	CompanyID         string                     `json:"companyId"`
	EmployeeID        string                     `json:"employeeId"`
	State             core.EmployeeScheduleState `json:"state"`
	WorkGeneration    int64                      `json:"workGeneration"`
	CheckedGeneration int64                      `json:"checkedGeneration"`
	PauseReason       string                     `json:"pauseReason,omitempty"`
}

// ProductWorkerDispatchCandidate is the oldest eligible, never-attempted
// product Task currently waiting for admission. It deliberately contains only
// identifiers; the Worker adapter re-reads all Task and authorization facts
// under the existing admission transaction before creating a WorkerSession.
type ProductWorkerDispatchCandidate struct {
	CompanyID string
	MissionID string
	TaskID    string
	ClaimID   string
}

// NextProductWorkerDispatchCandidate returns one eligible product Task after
// the optional Company-ID cursor. Company-ID ordering keeps cursor wraparound
// fair; within a Company, older EmployeeSchedule rows are prioritized. A
// quota-blocked or paused schedule is never eligible;
// any prior WorkerSession remains a durable one-shot fence.
func (k *Kernel) NextProductWorkerDispatchCandidate(ctx context.Context, afterCompanyID string) (ProductWorkerDispatchCandidate, bool, error) {
	return nextProductWorkerDispatchCandidate(ctx, k.pool, afterCompanyID)
}

const productWorkerDispatchClaimTTL = 30 * time.Second

type productWorkerDispatchQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func nextProductWorkerDispatchCandidate(ctx context.Context, queryer productWorkerDispatchQueryer, afterCompanyID string) (ProductWorkerDispatchCandidate, bool, error) {
	var candidate ProductWorkerDispatchCandidate
	err := queryer.QueryRow(ctx, `SELECT t.company_id,t.mission_id,t.id
FROM tasks t
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
JOIN companies c ON c.id=t.company_id
JOIN employee_schedules s ON s.company_id=t.company_id AND s.employee_id=t.owner
WHERE c.state='active' AND m.state='active'
  AND t.owner=$1 AND t.kind=$2 AND t.state='ready'
  AND ($3='' OR t.company_id>$3)
  AND s.state='wake_pending'
  AND EXISTS(SELECT 1 FROM worker_workspaces w WHERE w.company_id=t.company_id AND w.task_id=t.id)
  AND EXISTS(SELECT 1 FROM task_validation_bindings v WHERE v.company_id=t.company_id AND v.task_id=t.id)
  AND NOT EXISTS(SELECT 1 FROM worker_sessions w WHERE w.company_id=t.company_id AND w.task_id=t.id)
  AND NOT EXISTS(SELECT 1 FROM worker_sessions w WHERE w.company_id=t.company_id AND w.employee_id=t.owner AND w.state<>'stopped')
  AND NOT EXISTS(SELECT 1 FROM product_worker_dispatch_claims dispatch_claim
                 WHERE dispatch_claim.company_id=t.company_id AND dispatch_claim.task_id=t.id
                   AND dispatch_claim.expires_at>now())
  AND (SELECT count(*) FROM tasks executable
       WHERE executable.company_id=t.company_id AND executable.mission_id=t.mission_id
         AND executable.owner=$1 AND executable.kind=$2) = 1
ORDER BY t.company_id ASC,s.updated_at ASC,t.mission_id ASC,t.id ASC
LIMIT 1`, core.EmployeeBackendID, core.TaskKindCompat, afterCompanyID).Scan(&candidate.CompanyID, &candidate.MissionID, &candidate.TaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductWorkerDispatchCandidate{}, false, nil
	}
	if err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	return candidate, true, nil
}

// ClaimNextProductWorkerDispatchCandidate durably advances a shared Company
// cursor and creates a short-lived claim in one transaction. The claim keeps
// another dispatcher instance from selecting the same Task before Worker
// admission creates its authoritative WorkerSession. A crash releases the
// claim by expiry.
func (k *Kernel) ClaimNextProductWorkerDispatchCandidate(ctx context.Context) (ProductWorkerDispatchCandidate, bool, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM product_worker_dispatch_claims WHERE expires_at<=now()`); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	var cursor string
	if err = tx.QueryRow(ctx, `SELECT cursor_company_id FROM product_worker_dispatch_state WHERE id=1 FOR UPDATE`).Scan(&cursor); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	candidate, found, err := nextProductWorkerDispatchCandidate(ctx, tx, cursor)
	if err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	if !found && cursor != "" {
		candidate, found, err = nextProductWorkerDispatchCandidate(ctx, tx, "")
		if err != nil {
			return ProductWorkerDispatchCandidate{}, false, err
		}
	}
	if !found {
		if err = tx.Commit(ctx); err != nil {
			return ProductWorkerDispatchCandidate{}, false, err
		}
		return ProductWorkerDispatchCandidate{}, false, nil
	}
	var claimIDBytes [16]byte
	if _, err = rand.Read(claimIDBytes[:]); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	candidate.ClaimID = hex.EncodeToString(claimIDBytes[:])
	if _, err = tx.Exec(ctx, `INSERT INTO product_worker_dispatch_claims(claim_id,company_id,task_id,expires_at)
VALUES($1,$2,$3,now()+$4::interval)`, candidate.ClaimID, candidate.CompanyID, candidate.TaskID, productWorkerDispatchClaimTTL.String()); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE product_worker_dispatch_state SET cursor_company_id=$1,updated_at=now() WHERE id=1`, candidate.CompanyID); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ProductWorkerDispatchCandidate{}, false, err
	}
	return candidate, true, nil
}

// ReleaseProductWorkerDispatchClaim is idempotent so a successful Worker
// admission can always retire its dispatcher coordination claim.
func (k *Kernel) ReleaseProductWorkerDispatchClaim(ctx context.Context, claimID string) error {
	if len(claimID) != 32 {
		return core.Malformed
	}
	claimIDBytes, err := hex.DecodeString(claimID)
	if err != nil {
		return core.Malformed
	}
	if hex.EncodeToString(claimIDBytes) != claimID {
		return core.Malformed
	}
	_, err = k.pool.Exec(ctx, `DELETE FROM product_worker_dispatch_claims WHERE claim_id=$1`, claimID)
	return err
}

func (k *Kernel) EmployeeSchedule(ctx context.Context, scope Scope, employeeID string) (EmployeeScheduleSnapshot, error) {
	if !core.ValidID(scope.company) || !core.ValidID(employeeID) {
		return EmployeeScheduleSnapshot{}, core.Malformed
	}
	var schedule EmployeeScheduleSnapshot
	schedule.CompanyID, schedule.EmployeeID = scope.company, employeeID
	err := k.pool.QueryRow(ctx, `SELECT state,work_generation,checked_generation,COALESCE(pause_reason,'')
FROM employee_schedules WHERE company_id=$1 AND employee_id=$2`, scope.company, employeeID).Scan(
		&schedule.State, &schedule.WorkGeneration, &schedule.CheckedGeneration, &schedule.PauseReason,
	)
	if err == pgx.ErrNoRows {
		return EmployeeScheduleSnapshot{}, core.OutOfScope
	}
	return schedule, err
}

// TXReconcileEmployeeSchedule checks authoritative pending obligations and
// live WorkerSessions while holding the per-employee schedule row lock. Signal
// writers acquire that same row lock before committing their generation bump.
func (k *Kernel) TXReconcileEmployeeSchedule(ctx context.Context, scope Scope, employeeID, key string) (EmployeeScheduleSnapshot, error) {
	if !core.ValidID(scope.company) || !core.ValidID(employeeID) || !core.ValidID(key) {
		return EmployeeScheduleSnapshot{}, core.Malformed
	}
	_, err := k.TXWrite(ctx, scope, nil, key, "employee.schedule.reconcile", struct {
		EmployeeID string `json:"employeeId"`
	}{employeeID}, func(tx pgx.Tx) (Receipt, error) {
		schedule, err := reconcileEmployeeScheduleTX(ctx, tx, scope, employeeID)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: employeeID, Status: string(schedule.State), Revision: schedule.WorkGeneration}, nil
	})
	if err != nil {
		return EmployeeScheduleSnapshot{}, err
	}
	return k.EmployeeSchedule(ctx, scope, employeeID)
}

func ensureEmployeeScheduleTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string) error {
	tag, err := tx.Exec(ctx, `INSERT INTO employee_schedules(company_id,employee_id,state)
SELECT company_id,id,'sleeping' FROM employees WHERE company_id=$1 AND id=$2
ON CONFLICT(company_id,employee_id) DO NOTHING`, scope.company, employeeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)`, scope.company, employeeID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return core.OutOfScope
		}
	}
	return nil
}

func lockEmployeeScheduleTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string) (core.EmployeeSchedule, string, error) {
	if err := ensureEmployeeScheduleTX(ctx, tx, scope, employeeID); err != nil {
		return core.EmployeeSchedule{}, "", err
	}
	var schedule core.EmployeeSchedule
	var pauseReason string
	err := tx.QueryRow(ctx, `SELECT state,work_generation,checked_generation,COALESCE(pause_reason,'')
FROM employee_schedules WHERE company_id=$1 AND employee_id=$2 FOR UPDATE`, scope.company, employeeID).Scan(
		&schedule.State, &schedule.WorkGeneration, &schedule.CheckedGeneration, &pauseReason,
	)
	return schedule, pauseReason, err
}

func persistEmployeeScheduleTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string, schedule core.EmployeeSchedule, pauseReason string) error {
	_, err := tx.Exec(ctx, `UPDATE employee_schedules
SET state=$3,work_generation=$4,checked_generation=$5,pause_reason=NULLIF($6,''),updated_at=now()
WHERE company_id=$1 AND employee_id=$2`, scope.company, employeeID, string(schedule.State), schedule.WorkGeneration, schedule.CheckedGeneration, pauseReason)
	return err
}

func signalEmployeeScheduleTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string) error {
	schedule, pauseReason, err := lockEmployeeScheduleTX(ctx, tx, scope, employeeID)
	if err != nil {
		return err
	}
	schedule = core.RecordEmployeeWorkSignal(schedule, true)
	if err = persistEmployeeScheduleTX(ctx, tx, scope, employeeID, schedule, pauseReason); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_notify('polis_employee_wake',$1)`, scope.company+":"+employeeID)
	return err
}

func reconcileEmployeeScheduleTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string) (EmployeeScheduleSnapshot, error) {
	schedule, pauseReason, err := lockEmployeeScheduleTX(ctx, tx, scope, employeeID)
	if err != nil {
		return EmployeeScheduleSnapshot{}, err
	}
	previousSchedule, previousPauseReason := schedule, pauseReason
	var actionableWork, admittedWorker, activeWorker, missionPaused bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM peer_work_signals s JOIN obligations o
  ON o.company_id=s.company_id AND o.id=s.obligation_id
 JOIN tasks source_task ON source_task.company_id=o.company_id AND source_task.id=o.task_id
 JOIN missions source_mission ON source_mission.company_id=source_task.company_id AND source_mission.id=source_task.mission_id
 WHERE s.company_id=$1 AND s.recipient=$2 AND s.state IN ('pending','observed','acknowledged','applied') AND o.state IN ('pending','observed','applied')
  AND source_mission.state='active'
) OR EXISTS(
 SELECT 1 FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
 WHERE t.company_id=$1 AND t.owner=$2 AND t.state='ready' AND m.state='active'
) OR EXISTS(
 SELECT 1 FROM routine_occurrences o JOIN routines r ON r.company_id=o.company_id AND r.id=o.routine_id
 JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
 WHERE o.company_id=$1 AND r.employee_id=$2 AND o.state='pending' AND m.state='active'
)`, scope.company, employeeID).Scan(&actionableWork); err != nil {
		return EmployeeScheduleSnapshot{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND employee_id=$2 AND state NOT IN ('stopped','restoring','validating','activation_pending_environment')),
 EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND employee_id=$2 AND state IN ('restoring','validating','activation_pending_environment'))`, scope.company, employeeID).Scan(&activeWorker, &admittedWorker); err != nil {
		return EmployeeScheduleSnapshot{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
 WHERE t.company_id=$1 AND t.owner=$2 AND m.state='paused'
)`, scope.company, employeeID).Scan(&missionPaused); err != nil {
		return EmployeeScheduleSnapshot{}, err
	}
	schedule = core.ReconcileEmployeeSleep(schedule, schedule.WorkGeneration, actionableWork, admittedWorker, activeWorker, missionPaused)
	if missionPaused {
		pauseReason = "mission_paused"
	} else if schedule.State != core.EmployeeSchedulePaused {
		pauseReason = ""
	}
	if schedule != previousSchedule || pauseReason != previousPauseReason {
		if err = persistEmployeeScheduleTX(ctx, tx, scope, employeeID, schedule, pauseReason); err != nil {
			return EmployeeScheduleSnapshot{}, err
		}
	}
	return EmployeeScheduleSnapshot{
		CompanyID: scope.company, EmployeeID: employeeID, State: schedule.State,
		WorkGeneration: schedule.WorkGeneration, CheckedGeneration: schedule.CheckedGeneration, PauseReason: pauseReason,
	}, nil
}

func (k *Kernel) reconcileEmployeeSchedulesOnStartup(ctx context.Context) error {
	const pageSize = 128
	lastCompany, lastEmployee := "", ""
	for {
		rows, err := k.pool.Query(ctx, `SELECT s.company_id,s.employee_id
FROM employee_schedules s JOIN companies c ON c.id=s.company_id
WHERE c.state='active' AND (s.company_id,s.employee_id)>($1,$2)
ORDER BY s.company_id,s.employee_id LIMIT $3`, lastCompany, lastEmployee, pageSize)
		if err != nil {
			return err
		}
		type employeeKey struct{ companyID, employeeID string }
		page := make([]employeeKey, 0, pageSize)
		for rows.Next() {
			var key employeeKey
			if err = rows.Scan(&key.companyID, &key.employeeID); err != nil {
				rows.Close()
				return err
			}
			page = append(page, key)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		for _, key := range page {
			tx, beginErr := k.pool.Begin(ctx)
			if beginErr != nil {
				return beginErr
			}
			scope := Scope{company: key.companyID}
			if err = k.guard(ctx, tx, scope, nil); err == nil {
				var companyState string
				err = tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", key.companyID).Scan(&companyState)
				if err == nil && companyState == "active" {
					_, err = reconcileEmployeeScheduleTX(ctx, tx, scope, key.employeeID)
				}
			}
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
			if err = tx.Commit(ctx); err != nil {
				return err
			}
			lastCompany, lastEmployee = key.companyID, key.employeeID
		}
		if len(page) < pageSize {
			return nil
		}
	}
}

func setMissionEmployeeSchedulesTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID string, paused bool) error {
	if _, err := tx.Exec(ctx, `INSERT INTO employee_schedules(company_id,employee_id,state)
SELECT DISTINCT owners.company_id,owners.employee_id,'sleeping' FROM (
 SELECT t.company_id,t.owner AS employee_id FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
 UNION
 SELECT r.company_id,r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
) owners
ON CONFLICT(company_id,employee_id) DO NOTHING`, scope.company, missionID); err != nil {
		return err
	}
	if paused {
		_, err := tx.Exec(ctx, `UPDATE employee_schedules s
	SET state='paused',pause_reason='mission_paused',next_due_at=NULL,updated_at=now()
WHERE s.company_id=$1 AND s.employee_id IN (
 SELECT t.owner FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
 UNION
 SELECT r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
)`, scope.company, missionID)
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE employee_schedules s
SET state=CASE
 WHEN EXISTS(SELECT 1 FROM worker_sessions w WHERE w.company_id=s.company_id AND w.employee_id=s.employee_id AND w.state NOT IN ('stopped','restoring','validating','activation_pending_environment')) THEN 'working'
 WHEN EXISTS(SELECT 1 FROM worker_sessions w WHERE w.company_id=s.company_id AND w.employee_id=s.employee_id AND w.state IN ('restoring','validating','activation_pending_environment')) THEN 'admitted'
 WHEN EXISTS(SELECT 1 FROM tasks t WHERE t.company_id=s.company_id AND t.mission_id=$2 AND t.owner=s.employee_id AND t.state='ready') THEN 'wake_pending'
 WHEN EXISTS(SELECT 1 FROM routine_occurrences o JOIN routines r ON r.company_id=o.company_id AND r.id=o.routine_id
  JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
  WHERE o.company_id=s.company_id AND r.mission_id=$2 AND r.employee_id=s.employee_id AND o.state='pending' AND m.state='active') THEN 'wake_pending'
 WHEN EXISTS(SELECT 1 FROM peer_work_signals p JOIN obligations o ON o.company_id=p.company_id AND o.id=p.obligation_id
  JOIN tasks source_task ON source_task.company_id=o.company_id AND source_task.id=o.task_id
  JOIN missions source_mission ON source_mission.company_id=source_task.company_id AND source_mission.id=source_task.mission_id
  WHERE p.company_id=s.company_id AND p.recipient=s.employee_id AND p.state IN ('pending','observed','acknowledged','applied') AND o.state IN ('pending','observed','applied')
   AND source_mission.state='active') THEN 'wake_pending'
 ELSE 'sleeping' END,
 checked_generation=work_generation,pause_reason=NULL,updated_at=now()
WHERE s.company_id=$1 AND s.state='paused'
 AND s.employee_id IN (
  SELECT t.owner FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2
  UNION
  SELECT r.employee_id FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2
 )`, scope.company, missionID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT employee_id FROM routines WHERE company_id=$1 AND mission_id=$2`, scope.company, missionID)
	if err != nil {
		return err
	}
	employees := make([]string, 0, 4)
	for rows.Next() {
		var employeeID string
		if err = rows.Scan(&employeeID); err != nil {
			rows.Close()
			return err
		}
		employees = append(employees, employeeID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, employeeID := range employees {
		if err = refreshEmployeeNextDueTX(ctx, tx, scope, employeeID); err != nil {
			return err
		}
	}
	return nil
}
