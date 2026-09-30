// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
)

const (
	employeeScheduleReconcileInterval = 30 * time.Second
	employeeScheduleScanPageSize      = 128
	employeeScheduleListenerChannel   = "polis_employee_wake"
)

type employeeScheduleCursor struct {
	companyID  string
	employeeID string
}

// StartEmployeeScheduleReconciler subscribes before its initial scan, then
// reconciles wake hints and one bounded database page per interval. It never
// starts a Worker or performs provider work.
func (k *Kernel) StartEmployeeScheduleReconciler(ctx context.Context, onError func(error)) (func(), error) {
	return k.startEmployeeScheduleReconciler(ctx, employeeScheduleReconcileInterval, onError)
}

func (k *Kernel) startEmployeeScheduleReconciler(ctx context.Context, interval time.Duration, onError func(error)) (func(), error) {
	if ctx == nil || interval < time.Millisecond || interval > 24*time.Hour {
		return nil, core.Malformed
	}
	listenerConfig := k.pool.Config().Copy()
	listenerConfig.MaxConns = 1
	listenerConfig.MinConns = 0
	if listenerConfig.ConnConfig.RuntimeParams == nil {
		listenerConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	listenerConfig.ConnConfig.RuntimeParams["application_name"] = "polis_employee_schedule_listener"
	listenerPool, err := pgxpool.NewWithConfig(ctx, listenerConfig)
	if err != nil {
		return nil, err
	}
	setupCtx, setupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer setupCancel()
	connection, err := listenerPool.Acquire(setupCtx)
	if err != nil {
		listenerPool.Close()
		return nil, err
	}
	if _, err = connection.Exec(setupCtx, "LISTEN "+employeeScheduleListenerChannel); err != nil {
		connection.Release()
		listenerPool.Close()
		return nil, err
	}
	if err = k.reconcileEmployeeSchedulesOnStartup(setupCtx); err != nil {
		connection.Release()
		listenerPool.Close()
		return nil, err
	}
	routineCursor, dueErr := k.materializeDueDailyRoutinePage(setupCtx, time.Now().UTC(), routineIDKey{})
	if dueErr != nil {
		reportScheduleReconcileError(onError, dueErr)
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer listenerPool.Close()
		periodicDone := make(chan struct{})
		go func() {
			defer close(periodicDone)
			k.runEmployeeSchedulePeriodicScan(runCtx, interval, routineCursor, onError)
		}()
		k.runEmployeeScheduleNotificationLoop(runCtx, listenerPool, connection, onError)
		cancel()
		<-periodicDone
	}()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(cancel)
		<-done
	}
	return stop, nil
}

func (k *Kernel) runEmployeeScheduleNotificationLoop(ctx context.Context, pool *pgxpool.Pool, connection *pgxpool.Conn, onError func(error)) {
	defer func() {
		if connection != nil {
			connection.Release()
		}
	}()
	backoff := time.Second
	for ctx.Err() == nil {
		if connection == nil {
			if !waitEmployeeScheduleBackoff(ctx, backoff) {
				return
			}
			var err error
			connection, err = pool.Acquire(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				reportScheduleReconcileError(onError, err)
				backoff = nextEmployeeScheduleBackoff(backoff)
				continue
			}
			if _, err = connection.Exec(ctx, "LISTEN "+employeeScheduleListenerChannel); err == nil {
				err = k.reconcileEmployeeSchedulesOnStartup(ctx)
			}
			if err != nil {
				connection.Release()
				connection = nil
				reportScheduleReconcileError(onError, err)
				backoff = nextEmployeeScheduleBackoff(backoff)
				continue
			}
			backoff = time.Second
		}
		notification, err := connection.Conn().WaitForNotification(ctx)
		if err == nil {
			if notification.Channel != employeeScheduleListenerChannel {
				continue
			}
			err = k.reconcileEmployeeScheduleHint(ctx, notification.Payload)
			if err != nil {
				reportScheduleReconcileError(onError, err)
			}
			continue
		}
		connection.Release()
		connection = nil
		if ctx.Err() != nil {
			return
		}
		reportScheduleReconcileError(onError, err)
		backoff = nextEmployeeScheduleBackoff(backoff)
	}
}

func (k *Kernel) reconcileEmployeeScheduleHint(ctx context.Context, payload string) error {
	companyID, employeeID, ok := strings.Cut(payload, ":")
	if !ok || !core.ValidID(companyID) || !core.ValidID(employeeID) {
		return nil
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	scope := Scope{company: companyID}
	if err = k.guard(ctx, tx, scope, nil); err != nil {
		return err
	}
	var companyState string
	if err = tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&companyState); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if companyState == "active" {
		if _, err = reconcileEmployeeScheduleTX(ctx, tx, scope, employeeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (k *Kernel) runEmployeeSchedulePeriodicScan(ctx context.Context, interval time.Duration, routineCursor routineIDKey, onError func(error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	cursor := employeeScheduleCursor{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var err error
			routineCursor, err = k.materializeDueDailyRoutinePage(ctx, time.Now().UTC(), routineCursor)
			if err != nil {
				reportScheduleReconcileError(onError, err)
			}
			next, count, err := k.reconcileEmployeeSchedulesPage(ctx, cursor)
			if err != nil {
				reportScheduleReconcileError(onError, err)
				continue
			}
			cursor = next
			if count < employeeScheduleScanPageSize {
				cursor = employeeScheduleCursor{}
			}
		}
	}
}

func (k *Kernel) materializeDueDailyRoutinePage(ctx context.Context, now time.Time, cursor routineIDKey) (routineIDKey, error) {
	const (
		backfillPageSize = 32
		duePageSize      = 96
	)
	backfillCursor, firstErr := k.reconcileDueRoutineTimesPage(ctx, backfillPageSize, cursor)
	rows, err := k.pool.Query(ctx, `SELECT r.company_id,r.id
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
JOIN companies c ON c.id=r.company_id
WHERE c.state='active' AND m.state='active' AND r.scheduling_active AND r.next_due_at<=$1
ORDER BY r.next_due_at,r.company_id,r.id LIMIT $2`, now.UTC(), duePageSize)
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
		return backfillCursor, firstErr
	}
	dueRoutines := make([]routineIDKey, 0, duePageSize)
	for rows.Next() {
		var routine routineIDKey
		if err = rows.Scan(&routine.companyID, &routine.routineID); err != nil {
			rows.Close()
			if firstErr == nil {
				firstErr = err
			}
			return backfillCursor, firstErr
		}
		dueRoutines = append(dueRoutines, routine)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		return backfillCursor, firstErr
	}
	for _, dueRoutine := range dueRoutines {
		requestID := "routine-due-" + newID()
		if _, err = k.TXMaterializeDailyRoutine(ctx, Scope{company: dueRoutine.companyID}, dueRoutine.routineID, now, requestID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return backfillCursor, firstErr
}

type routineIDKey struct {
	companyID string
	routineID string
}

func (k *Kernel) reconcileDueRoutineTimesPage(ctx context.Context, limit int, cursor routineIDKey) (routineIDKey, error) {
	rows, err := k.pool.Query(ctx, `SELECT company_id,id FROM routines
WHERE scheduling_active AND next_due_at IS NULL AND (company_id,id)>($1,$2)
ORDER BY company_id,id LIMIT $3`, cursor.companyID, cursor.routineID, limit)
	if err != nil {
		return cursor, err
	}
	missing := make([]routineIDKey, 0, limit)
	for rows.Next() {
		var key routineIDKey
		if err = rows.Scan(&key.companyID, &key.routineID); err != nil {
			rows.Close()
			return cursor, err
		}
		missing = append(missing, key)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return cursor, err
	}
	var firstErr error
	for _, key := range missing {
		if err = k.reconcileDailyRoutineDueTime(ctx, key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(missing) == 0 || len(missing) < limit {
		return routineIDKey{}, firstErr
	}
	return missing[len(missing)-1], firstErr
}

func (k *Kernel) reconcileDailyRoutineDueTime(ctx context.Context, key routineIDKey) error {
	scope := Scope{company: key.companyID}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, scope, nil); err != nil {
		return err
	}
	var employeeID, timezone, localTime, logicalDay, missionState, companyState string
	var schedulingActive bool
	err = tx.QueryRow(ctx, `SELECT r.employee_id,r.timezone,r.local_time,to_char(r.next_logical_day,'YYYY-MM-DD'),m.state,c.state,r.scheduling_active
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
JOIN companies c ON c.id=r.company_id
WHERE r.company_id=$1 AND r.id=$2 AND r.next_due_at IS NULL FOR UPDATE OF r`, key.companyID, key.routineID).Scan(
		&employeeID, &timezone, &localTime, &logicalDay, &missionState, &companyState, &schedulingActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	dueAt, err := core.ResolveDailyRoutineTime(logicalDay, localTime, timezone)
	if err != nil {
		return core.Integrity
	}
	if _, err = tx.Exec(ctx, `UPDATE routines SET next_due_at=$3,updated_at=now()
WHERE company_id=$1 AND id=$2 AND next_due_at IS NULL`, key.companyID, key.routineID, dueAt); err != nil {
		return err
	}
	if !schedulingActive {
		return tx.Commit(ctx)
	}
	if missionState == "active" && companyState == "active" {
		if err = refreshEmployeeNextDueTX(ctx, tx, scope, employeeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (k *Kernel) reconcileEmployeeSchedulesPage(ctx context.Context, cursor employeeScheduleCursor) (employeeScheduleCursor, int, error) {
	rows, err := k.pool.Query(ctx, `SELECT s.company_id,s.employee_id
FROM employee_schedules s JOIN companies c ON c.id=s.company_id
WHERE c.state='active' AND (s.company_id,s.employee_id)>($1,$2)
ORDER BY s.company_id,s.employee_id LIMIT $3`, cursor.companyID, cursor.employeeID, employeeScheduleScanPageSize)
	if err != nil {
		return cursor, 0, err
	}
	page := make([]employeeScheduleCursor, 0, employeeScheduleScanPageSize)
	for rows.Next() {
		var key employeeScheduleCursor
		if err = rows.Scan(&key.companyID, &key.employeeID); err != nil {
			rows.Close()
			return cursor, 0, err
		}
		page = append(page, key)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return cursor, 0, err
	}
	for _, key := range page {
		tx, beginErr := k.pool.Begin(ctx)
		if beginErr != nil {
			return cursor, 0, beginErr
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
			return cursor, 0, err
		}
		if err = tx.Commit(ctx); err != nil {
			return cursor, 0, err
		}
	}
	if len(page) == 0 {
		return employeeScheduleCursor{}, 0, nil
	}
	return page[len(page)-1], len(page), nil
}

func waitEmployeeScheduleBackoff(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextEmployeeScheduleBackoff(current time.Duration) time.Duration {
	if current >= 30*time.Second {
		return 30 * time.Second
	}
	return current * 2
}

func reportScheduleReconcileError(onError func(error), err error) {
	if onError != nil && err != nil && !errors.Is(err, context.Canceled) {
		onError(err)
	}
}
