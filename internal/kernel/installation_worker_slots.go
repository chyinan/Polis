// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
)

const (
	workerSlotClassOrdinary  = "ordinary"
	workerSlotClassProtected = "protected"
)

// InstallationWorkerSlotPolicy is an installation-wide view. A policy may be
// unconfigured; in that state Worker admission is closed and the action field
// tells the installation owner how to configure it.
type InstallationWorkerSlotPolicy struct {
	Status               string `json:"status"`
	Configured           bool   `json:"configured"`
	MaxActiveSlots       *int64 `json:"maxActiveSlots,omitempty"`
	ProtectedSlots       *int64 `json:"protectedSlots,omitempty"`
	Revision             int64  `json:"revision"`
	ActiveSlots          int64  `json:"activeSlots"`
	OrdinaryActiveSlots  int64  `json:"ordinaryActiveSlots"`
	ProtectedActiveSlots int64  `json:"protectedActiveSlots"`
	Action               string `json:"action,omitempty"`
}

type InstallationWorkerSlotPolicyInput struct {
	RequestID        string
	ExpectedRevision int64
	MaxActiveSlots   int64
	ProtectedSlots   int64
}

func (k *Kernel) GetInstallationWorkerSlotPolicy(ctx context.Context, scope InstallationOwnerScope) (InstallationWorkerSlotPolicy, error) {
	if k == nil || k.pool == nil || ctx == nil || scope.incarnation == "" || scope.incarnation != k.incarnation {
		return InstallationWorkerSlotPolicy{}, core.Denied
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	policy, err := installationWorkerSlotPolicyTX(ctx, tx)
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	return policy, nil
}

// TXSetInstallationWorkerSlotPolicy is called only from an installation-owner
// authenticated, CSRF-checked route. The expected revision prevents a stale
// owner view from silently replacing a newer capacity decision.
func (k *Kernel) TXSetInstallationWorkerSlotPolicy(ctx context.Context, scope InstallationOwnerScope, input InstallationWorkerSlotPolicyInput) (InstallationWorkerSlotPolicy, error) {
	if k == nil || k.pool == nil || ctx == nil || scope.incarnation == "" || scope.incarnation != k.incarnation ||
		!core.ValidID(input.RequestID) || input.ExpectedRevision < 0 || input.MaxActiveSlots <= 0 || input.ProtectedSlots < 0 || input.ProtectedSlots > input.MaxActiveSlots {
		return InstallationWorkerSlotPolicy{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	var currentMax, currentProtected pgtype.Int8
	var revision int64
	err = tx.QueryRow(ctx, `SELECT max_active_slots,protected_slots,revision
FROM installation_worker_slot_policy WHERE singleton FOR UPDATE`).Scan(&currentMax, &currentProtected, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return InstallationWorkerSlotPolicy{}, core.Integrity
	}
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}

	var priorRevision, priorMax, priorProtected int64
	err = tx.QueryRow(ctx, `SELECT revision,max_active_slots,protected_slots
FROM installation_worker_slot_policy_events WHERE request_id=$1`, input.RequestID).Scan(&priorRevision, &priorMax, &priorProtected)
	if err == nil {
		if priorMax != input.MaxActiveSlots || priorProtected != input.ProtectedSlots {
			return InstallationWorkerSlotPolicy{}, core.Conflict
		}
		policy, projectionErr := installationWorkerSlotPolicyTX(ctx, tx)
		if projectionErr != nil {
			return InstallationWorkerSlotPolicy{}, projectionErr
		}
		if err = tx.Commit(ctx); err != nil {
			return InstallationWorkerSlotPolicy{}, err
		}
		return policy, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return InstallationWorkerSlotPolicy{}, err
	}
	if revision != input.ExpectedRevision {
		return InstallationWorkerSlotPolicy{}, core.ConflictError{
			Reason:       "installation Worker slot policy revision changed; refresh the owner policy view before submitting a new request",
			CurrentState: fmt.Sprintf("revision:%d", revision),
		}
	}
	if currentMax.Valid && currentProtected.Valid && currentMax.Int64 == input.MaxActiveSlots && currentProtected.Int64 == input.ProtectedSlots {
		policy, projectionErr := installationWorkerSlotPolicyTX(ctx, tx)
		if projectionErr != nil {
			return InstallationWorkerSlotPolicy{}, projectionErr
		}
		if err = tx.Commit(ctx); err != nil {
			return InstallationWorkerSlotPolicy{}, err
		}
		return policy, nil
	}
	if revision == int64(^uint64(0)>>1) {
		return InstallationWorkerSlotPolicy{}, core.Integrity
	}
	nextRevision := revision + 1
	if _, err = tx.Exec(ctx, `UPDATE installation_worker_slot_policy
SET max_active_slots=$1,protected_slots=$2,revision=$3,updated_at=clock_timestamp()
WHERE singleton`, input.MaxActiveSlots, input.ProtectedSlots, nextRevision); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO installation_worker_slot_policy_events(revision,request_id,max_active_slots,protected_slots)
VALUES($1,$2,$3,$4)`, nextRevision, input.RequestID, input.MaxActiveSlots, input.ProtectedSlots); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	policy, err := installationWorkerSlotPolicyTX(ctx, tx)
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	return policy, nil
}

func installationWorkerSlotPolicyTX(ctx context.Context, tx pgx.Tx) (InstallationWorkerSlotPolicy, error) {
	policy := InstallationWorkerSlotPolicy{}
	var maxActiveSlots, protectedSlots pgtype.Int8
	err := tx.QueryRow(ctx, `SELECT max_active_slots,protected_slots,revision
FROM installation_worker_slot_policy WHERE singleton`).Scan(&maxActiveSlots, &protectedSlots, &policy.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return InstallationWorkerSlotPolicy{}, core.Integrity
	}
	if err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	if maxActiveSlots.Valid != protectedSlots.Valid {
		return InstallationWorkerSlotPolicy{}, core.Integrity
	}
	if maxActiveSlots.Valid {
		policy.Configured = true
		policy.Status = "configured"
		policy.MaxActiveSlots = int64Pointer(maxActiveSlots.Int64)
		policy.ProtectedSlots = int64Pointer(protectedSlots.Int64)
	} else {
		policy.Status = "unconfigured"
		policy.Action = "An installation owner must set maxActiveSlots and protectedSlots at /api/workbench/installation/worker-slots before WorkerSessions can be admitted."
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE s.state<>'stopped'),
count(*) FILTER (WHERE s.state<>'stopped' AND COALESCE(r.slot_class,'ordinary')='ordinary'),
count(*) FILTER (WHERE s.state<>'stopped' AND r.slot_class='protected')
FROM worker_sessions s
LEFT JOIN installation_worker_slot_reservations r ON r.company_id=s.company_id AND r.worker_session_id=s.id`).Scan(
		&policy.ActiveSlots, &policy.OrdinaryActiveSlots, &policy.ProtectedActiveSlots,
	); err != nil {
		return InstallationWorkerSlotPolicy{}, err
	}
	if policy.Configured && policy.ActiveSlots > *policy.MaxActiveSlots {
		policy.Status = "over_capacity"
		policy.Action = "Existing WorkerSessions exceed the configured cap; new admissions stay closed until sessions stop or the installation owner revises the policy."
	}
	return policy, nil
}

// checkInstallationWorkerSlotCapacityTX serializes every new admission across
// Companies on the singleton policy row. Callers keep the lock until the same
// transaction inserts both WorkerSession and its reservation record.
func checkInstallationWorkerSlotCapacityTX(ctx context.Context, tx pgx.Tx, taskKind string) (string, error) {
	var maxActiveSlots, protectedSlots pgtype.Int8
	err := tx.QueryRow(ctx, `SELECT max_active_slots,protected_slots
FROM installation_worker_slot_policy WHERE singleton FOR UPDATE`).Scan(&maxActiveSlots, &protectedSlots)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", core.Integrity
	}
	if err != nil {
		return "", err
	}
	if !maxActiveSlots.Valid && !protectedSlots.Valid {
		return "", fmt.Errorf("%w: an installation owner must set maxActiveSlots and protectedSlots at /api/workbench/installation/worker-slots before WorkerSessions can be admitted", core.WorkerSlotsUnconfigured)
	}
	if !maxActiveSlots.Valid || !protectedSlots.Valid || maxActiveSlots.Int64 <= 0 || protectedSlots.Int64 < 0 || protectedSlots.Int64 > maxActiveSlots.Int64 {
		return "", core.Integrity
	}
	var activeSlots, ordinaryActiveSlots int64
	err = tx.QueryRow(ctx, `SELECT count(*),
count(*) FILTER (WHERE COALESCE(r.slot_class,'ordinary')='ordinary')
FROM worker_sessions s
LEFT JOIN installation_worker_slot_reservations r ON r.company_id=s.company_id AND r.worker_session_id=s.id
WHERE s.state<>'stopped'`).Scan(&activeSlots, &ordinaryActiveSlots)
	if err != nil {
		return "", err
	}
	if activeSlots >= maxActiveSlots.Int64 {
		return "", fmt.Errorf("%w: installation Worker slots are full (%d of %d); wait for a session to stop or ask the installation owner to revise the policy", core.WorkerSlotsFull, activeSlots, maxActiveSlots.Int64)
	}
	if isProblemClosingTaskKind(taskKind) {
		return workerSlotClassProtected, nil
	}
	ordinaryLimit := maxActiveSlots.Int64 - protectedSlots.Int64
	if ordinaryActiveSlots >= ordinaryLimit {
		return "", fmt.Errorf("%w: ordinary Worker capacity is reserved for recovery and verification (%d of %d ordinary slots); wait for a slot or ask the installation owner to revise the policy", core.WorkerSlotsFull, ordinaryActiveSlots, ordinaryLimit)
	}
	return workerSlotClassOrdinary, nil
}

func recordInstallationWorkerSlotReservationTX(ctx context.Context, tx pgx.Tx, companyID, sessionID, slotClass string) error {
	if (slotClass != workerSlotClassOrdinary && slotClass != workerSlotClassProtected) || !core.ValidID(companyID) || !core.ValidID(sessionID) {
		return core.Malformed
	}
	_, err := tx.Exec(ctx, `INSERT INTO installation_worker_slot_reservations(company_id,worker_session_id,slot_class)
VALUES($1,$2,$3)`, companyID, sessionID, slotClass)
	return err
}
