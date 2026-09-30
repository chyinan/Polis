// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProbeFrontendStartingState performs only read-only SQL. It deliberately
// does not call Open, txRecover, or any worker/session lifecycle method.
func ProbeFrontendStartingState(ctx context.Context, dsn string) (FrontendStartingState, error) {
	var state FrontendStartingState
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return state, err
	}
	if cfg.ConnConfig.Database == "" || len(cfg.ConnConfig.Database) < len("polis_r0_") || cfg.ConnConfig.Database[:len("polis_r0_")] != "polis_r0_" {
		return state, errors.New("frontend starting-state probe requires a dedicated polis_r0 database")
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return state, err
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return state, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
SELECT c.id,m.id,bt.id,ft.id
FROM companies c
JOIN missions m ON m.company_id=c.id AND m.state='active'
JOIN tasks bt ON bt.company_id=c.id AND bt.mission_id=m.id AND bt.owner='emp-backend' AND bt.kind='peer_backend'
JOIN tasks ft ON ft.company_id=c.id AND ft.mission_id=m.id AND ft.owner='emp-frontend' AND ft.kind='peer_frontend'
WHERE EXISTS (SELECT 1 FROM messages pm WHERE pm.company_id=c.id AND pm.mission_id=m.id AND pm.recipient='emp-frontend')
ORDER BY c.id,m.id LIMIT 1`).Scan(&state.CompanyID, &state.MissionID, &state.BackendTaskID, &state.FrontendTaskID)
	if err != nil {
		return state, fmt.Errorf("read frontend starting company/tasks: %w", err)
	}
	err = tx.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND id=$2", state.CompanyID, state.FrontendTaskID).Scan(&state.FrontendTaskState)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT epoch FROM employees WHERE company_id=$1 AND id='emp-frontend'", state.CompanyID).Scan(&state.EmployeeEpoch)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT incarnation FROM runtime_control WHERE singleton").Scan(&state.RuntimeIncarnation)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND employee_id='emp-frontend'", state.CompanyID, state.FrontendTaskID).Scan(&state.FrontendSessionCount)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND employee_id='emp-frontend' AND state!='stopped'", state.CompanyID, state.FrontendTaskID).Scan(&state.LiveWriterCount)
	if err != nil {
		return state, err
	}
	if state.LiveWriterCount > 0 {
		_ = tx.QueryRow(ctx, "SELECT id FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND employee_id='emp-frontend' AND state!='stopped' ORDER BY generation DESC,id DESC LIMIT 1", state.CompanyID, state.FrontendTaskID).Scan(&state.CurrentWriterSession)
	}
	err = tx.QueryRow(ctx, `
SELECT m.id,o.id,m.delivery_state,o.state,o.owner,cr.id,cr.state
FROM messages m
JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
JOIN contract_revisions cr ON cr.company_id=m.company_id AND cr.id=m.contract_revision_id
WHERE m.company_id=$1 AND m.mission_id=$2 AND m.recipient='emp-frontend' AND cr.state='accepted'
ORDER BY cr.revision DESC,m.id DESC LIMIT 1`, state.CompanyID, state.MissionID).Scan(&state.MessageID, &state.ObligationID, &state.MessageState, &state.ObligationState, &state.ObligationOwner, &state.ContractRevisionID, &state.ContractState)
	if err != nil {
		return state, fmt.Errorf("read frontend peer responsibility: %w", err)
	}
	err = tx.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND employee_id='emp-backend' ORDER BY generation DESC,id DESC LIMIT 1", state.CompanyID, state.BackendTaskID).Scan(&state.BackendSessionState)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT revision,digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", state.CompanyID, state.FrontendTaskID).Scan(&state.WorkspaceRevision, &state.WorkspaceDigest)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT revision,digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", state.CompanyID, state.BackendTaskID).Scan(&state.BackendWorkspaceRevision, &state.BackendWorkspaceDigest)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, `SELECT id,task_id,author,digest,state,verdict FROM artifacts WHERE company_id=$1 AND task_id=$2 AND author='emp-backend' ORDER BY id DESC LIMIT 1`, state.CompanyID, state.BackendTaskID).Scan(&state.BackendArtifactID, &state.BackendArtifactTaskID, &state.BackendArtifactAuthor, &state.BackendArtifactDigest, &state.BackendArtifactState, &state.BackendArtifactVerdict)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	} else if err != nil {
		return state, err
	}
	if err = tx.QueryRow(ctx, `SELECT cp.id,COALESCE((cp.data->>'workspace_revision')::bigint,0),COALESCE(cp.data->>'workspace_digest',''),COALESCE(cp.data->>'contract_revision_id','') FROM worker_checkpoints cp JOIN worker_sessions s ON s.company_id=cp.company_id AND s.id=cp.session_id WHERE cp.company_id=$1 AND s.task_id=$2 AND cp.data->>'kind'='qualified' ORDER BY cp.id DESC LIMIT 1`, state.CompanyID, state.BackendTaskID).Scan(&state.BackendCheckpointID, &state.CheckpointWorkspaceRevision, &state.CheckpointWorkspaceDigest, &state.CheckpointContractRevisionID); errors.Is(err, pgx.ErrNoRows) {
		err = nil
	} else if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT count(*) FROM messages WHERE company_id=$1 AND (sender='emp-planning' OR recipient='emp-planning')", state.CompanyID).Scan(&state.PlannerRelayCount)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", state.CompanyID).Scan(&state.CompanySequence)
	if err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1", state.CompanyID).Scan(&state.EventCount)
	if err != nil {
		return state, err
	}
	if err = tx.Commit(ctx); err != nil {
		return state, err
	}
	return state, nil
}
