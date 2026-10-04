// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/runner"
)

type WorkerProcessRecoveryCandidate struct {
	CompanyID   string
	SessionID   string
	TaskID      string
	EmployeeID  string
	State       string
	PID         int
	Epoch       int64
	Incarnation string
	Containment runner.ProcessContainmentMetadata
}

type WorkerHostReconciliationSummary struct {
	Candidates int `json:"candidates"`
	Stopped    int `json:"stopped"`
	Unresolved int `json:"unresolved"`
}

type windowsWorkerHostStopObservation struct {
	HostOS      string `json:"host_os"`
	Profile     string `json:"profile"`
	PID         int    `json:"process_pid,omitempty"`
	TreeStopped bool   `json:"tree_stopped"`
}

func windowsWorkerHostStopReceiptMatches(state, stopReceipt string, processPID int, metadata runner.ProcessContainmentMetadata, observation windowsWorkerHostStopObservation) bool {
	return state == "stopped" && processPID >= 0 && stopReceipt == fmt.Sprintf("windows-worker-job-zero:%d", processPID) &&
		metadata.HostOS == "windows" && metadata.Profile == "windows_worker_job_object@1" &&
		observation.HostOS == "windows" && observation.Profile == metadata.Profile && observation.PID == processPID && observation.TreeStopped
}

func (k *Kernel) windowsWorkerHostStopAlreadyConfirmed(ctx context.Context, companyID, sessionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(sessionID) {
		return false, core.Malformed
	}
	var state, stopReceipt string
	var processPID int64
	var containmentRaw, hostRaw []byte
	err := k.pool.QueryRow(ctx, `SELECT s.state,COALESCE(s.stop_receipt,''),COALESCE(s.process_pid,0),containment.data,host_observation.data
FROM worker_sessions s
JOIN LATERAL (
 SELECT data FROM worker_observations
 WHERE company_id=s.company_id AND session_id=s.id AND reason IN ('process_containment','process_containment_bound')
 ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1
) containment ON true
JOIN LATERAL (
 SELECT data FROM worker_observations
 WHERE company_id=s.company_id AND session_id=s.id AND reason='host_reconciliation'
 ORDER BY id DESC LIMIT 1
) host_observation ON true
WHERE s.company_id=$1 AND s.id=$2`, companyID, sessionID).Scan(&state, &stopReceipt, &processPID, &containmentRaw, &hostRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var metadata runner.ProcessContainmentMetadata
	var observation windowsWorkerHostStopObservation
	if json.Unmarshal(containmentRaw, &metadata) != nil || json.Unmarshal(hostRaw, &observation) != nil {
		return false, core.Integrity
	}
	return windowsWorkerHostStopReceiptMatches(state, stopReceipt, int(processPID), metadata, observation), nil
}

func txEnsureWorkerProcessContainment(ctx context.Context, tx pgx.Tx, scope Scope, sessionID string, metadata runner.ProcessContainmentMetadata) error {
	if !core.ValidID(sessionID) || !metadata.Valid() {
		return core.Malformed
	}
	var existing []byte
	err := tx.QueryRow(ctx, `SELECT data FROM worker_observations WHERE company_id=$1 AND session_id=$2 AND reason IN ('process_containment','process_containment_bound') ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1`, scope.company, sessionID).Scan(&existing)
	if err == nil {
		var persisted runner.ProcessContainmentMetadata
		if json.Unmarshal(existing, &persisted) != nil || persisted != metadata {
			return core.Conflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'process_containment',$4)`, scope.company, newID(), sessionID, data)
	return err
}

func txBindWorkerProcessContainment(ctx context.Context, tx pgx.Tx, scope Scope, sessionID string, metadata runner.ProcessContainmentMetadata) error {
	if !core.ValidID(sessionID) || !metadata.Valid() {
		return core.Malformed
	}
	var reason string
	var existing []byte
	err := tx.QueryRow(ctx, `SELECT reason,data FROM worker_observations WHERE company_id=$1 AND session_id=$2 AND reason IN ('process_containment','process_containment_bound') ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1`, scope.company, sessionID).Scan(&reason, &existing)
	if err != nil {
		return err
	}
	var persisted runner.ProcessContainmentMetadata
	if json.Unmarshal(existing, &persisted) != nil || !persisted.Valid() {
		return core.Integrity
	}
	if persisted == metadata {
		return nil
	}
	if reason != "process_containment" || persisted.HostOS != "linux" || persisted.Profile != "linux_process_group@1" ||
		metadata.HostOS != "linux" || metadata.Profile != "linux_worker_cgroup_v2@1" {
		return core.Conflict
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'process_containment_bound',$4)`, scope.company, newID(), sessionID, data)
	return err
}

func (k *Kernel) TXBindWorkerProcessHost(ctx context.Context, binding Binding, metadata runner.ProcessContainmentMetadata) error {
	if !core.ValidID(binding.session) || !metadata.Valid() {
		return core.Malformed
	}
	input := struct {
		SessionID string
		Metadata  runner.ProcessContainmentMetadata
	}{binding.session, metadata}
	metadataData, marshalErr := json.Marshal(metadata)
	if marshalErr != nil {
		return marshalErr
	}
	metadataDigest := sha256.Sum256(metadataData)
	requestID := "worker-containment-bind-" + binding.session + "-" + hex.EncodeToString(metadataDigest[:6])
	_, err := k.TXWrite(ctx, binding.scope, nil, requestID, "worker.process_containment_bound", input, func(tx pgx.Tx) (Receipt, error) {
		state, err := k.checkSession(ctx, tx, binding, false)
		if err != nil {
			return Receipt{}, err
		}
		if state != "restoring" {
			return Receipt{}, core.ConflictError{Reason: "process containment must be bound before process creation", CurrentState: state}
		}
		var processPID pgtype.Int8
		if err = tx.QueryRow(ctx, `SELECT process_pid FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE`, binding.scope.company, binding.session).Scan(&processPID); err != nil {
			return Receipt{}, err
		}
		if processPID.Valid {
			return Receipt{}, core.Conflict
		}
		if err = txBindWorkerProcessContainment(ctx, tx, binding.scope, binding.session, metadata); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: binding.session, Status: "bound"}, nil
	})
	return err
}

func (k *Kernel) WorkerSessionRecoveryCandidates(ctx context.Context) ([]WorkerProcessRecoveryCandidate, error) {
	rows, err := k.pool.Query(ctx, `SELECT s.company_id,s.id,s.task_id,s.employee_id,s.state,COALESCE(s.process_pid,0),s.epoch,s.incarnation,o.data
FROM worker_sessions s
JOIN LATERAL (
 SELECT data FROM worker_observations
 WHERE company_id=s.company_id AND session_id=s.id AND reason IN ('process_containment','process_containment_bound')
 ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1
) o ON true
WHERE s.state<>'stopped'
ORDER BY s.company_id,s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]WorkerProcessRecoveryCandidate, 0)
	for rows.Next() {
		var candidate WorkerProcessRecoveryCandidate
		var encoded []byte
		if err = rows.Scan(&candidate.CompanyID, &candidate.SessionID, &candidate.TaskID, &candidate.EmployeeID, &candidate.State, &candidate.PID, &candidate.Epoch, &candidate.Incarnation, &encoded); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(encoded, &candidate.Containment); err != nil {
			return nil, core.Integrity
		}
		if !candidate.Containment.Valid() || candidate.PID < 0 {
			return nil, core.Integrity
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (k *Kernel) TXConfirmWindowsWorkerTreeStopped(ctx context.Context, candidate WorkerProcessRecoveryCandidate, proof runner.WindowsProcessTreeStopProof) error {
	if !core.ValidID(candidate.CompanyID) || !core.ValidID(candidate.SessionID) || (candidate.State != "restoring" && candidate.State != "reconcile_required") || !candidate.Containment.Valid() || candidate.Containment.HostOS != "windows" || candidate.Containment.Profile != "windows_worker_job_object@1" || !proof.ForWorkerSession(candidate.SessionID, candidate.PID) {
		return core.Denied
	}
	input := struct {
		Candidate WorkerProcessRecoveryCandidate
		Status    string
	}{candidate, "active_process_count_zero"}
	requestID := "worker-host-stop-" + candidate.SessionID
	_, err := k.TXWrite(ctx, Scope{candidate.CompanyID}, nil, requestID, "worker.host_tree_stopped", input, func(tx pgx.Tx) (Receipt, error) {
		var state, taskID, employeeID, incarnation string
		var epoch int64
		var processPID pgtype.Int8
		if err := tx.QueryRow(ctx, `SELECT state,task_id,employee_id,epoch,incarnation,process_pid
FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE`, candidate.CompanyID, candidate.SessionID).Scan(&state, &taskID, &employeeID, &epoch, &incarnation, &processPID); err != nil {
			return Receipt{}, err
		}
		if state != candidate.State || state == "stopped" || taskID != candidate.TaskID || employeeID != candidate.EmployeeID || epoch != candidate.Epoch || incarnation != candidate.Incarnation || (processPID.Valid && int(processPID.Int64) != candidate.PID) || (!processPID.Valid && candidate.PID != 0) {
			return Receipt{}, core.Conflict
		}
		var encoded []byte
		if err := tx.QueryRow(ctx, `SELECT data FROM worker_observations WHERE company_id=$1 AND session_id=$2 AND reason IN ('process_containment','process_containment_bound') ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1`, candidate.CompanyID, candidate.SessionID).Scan(&encoded); err != nil {
			return Receipt{}, err
		}
		var metadata runner.ProcessContainmentMetadata
		if json.Unmarshal(encoded, &metadata) != nil || metadata != candidate.Containment {
			return Receipt{}, core.Conflict
		}
		tag, err := tx.Exec(ctx, `UPDATE worker_sessions SET state='stopped',stop_receipt=$4
WHERE company_id=$1 AND id=$2 AND state=$3 AND process_pid IS NOT DISTINCT FROM $5`, candidate.CompanyID, candidate.SessionID, candidate.State, fmt.Sprintf("windows-worker-job-zero:%d", candidate.PID), nullableProcessPID(candidate.PID))
		if err != nil {
			return Receipt{}, err
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Conflict
		}
		evidence, err := json.Marshal(struct {
			HostOS      string `json:"host_os"`
			Profile     string `json:"profile"`
			PID         int    `json:"process_pid,omitempty"`
			TreeStopped bool   `json:"tree_stopped"`
		}{"windows", candidate.Containment.Profile, candidate.PID, true})
		if err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'host_reconciliation',$4)`, candidate.CompanyID, newID(), candidate.SessionID, evidence); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: candidate.SessionID, Status: "stopped_by_host_reconciliation"}, nil
	})
	return err
}

type linuxWorkerHostStopObservation struct {
	HostOS       string `json:"host_os"`
	Profile      string `json:"profile"`
	PID          int    `json:"process_pid"`
	GroupEmpty   bool   `json:"process_group_empty,omitempty"`
	CgroupID     string `json:"cgroup_id,omitempty"`
	CgroupEmpty  bool   `json:"cgroup_empty,omitempty"`
	CgroupRootID string `json:"cgroup_root_id,omitempty"`
	CgroupBootID string `json:"cgroup_boot_id,omitempty"`
}

func linuxWorkerHostStopReceiptMatches(state, stopReceipt string, processPID int, metadata runner.ProcessContainmentMetadata, observation linuxWorkerHostStopObservation) bool {
	return state == "stopped" && processPID > 0 && stopReceipt == fmt.Sprintf("linux-worker-process-group-empty:%d", processPID) &&
		metadata.HostOS == "linux" && metadata.Profile == "linux_process_group@1" &&
		observation.HostOS == "linux" && observation.Profile == metadata.Profile && observation.PID == processPID && observation.GroupEmpty
}

func linuxWorkerCgroupStopReceiptMatches(state, stopReceipt string, processPID int, metadata runner.ProcessContainmentMetadata, observation linuxWorkerHostStopObservation) bool {
	return state == "stopped" && metadata.HostOS == "linux" && metadata.Profile == "linux_worker_cgroup_v2@1" &&
		metadata.Valid() && stopReceipt == "linux-worker-cgroup-empty:"+metadata.CgroupID &&
		observation.HostOS == "linux" && observation.Profile == metadata.Profile && observation.PID == processPID &&
		observation.CgroupID == metadata.CgroupID && observation.CgroupEmpty &&
		observation.CgroupRootID == metadata.CgroupRootID && observation.CgroupBootID == metadata.CgroupBootID
}

func (k *Kernel) linuxWorkerHostStopAlreadyConfirmed(ctx context.Context, companyID, sessionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(sessionID) {
		return false, core.Malformed
	}
	var state, stopReceipt string
	var processPID int64
	var containmentRaw, hostRaw []byte
	err := k.pool.QueryRow(ctx, `SELECT s.state,COALESCE(s.stop_receipt,''),COALESCE(s.process_pid,0),containment.data,host_observation.data
FROM worker_sessions s
JOIN LATERAL (
 SELECT data FROM worker_observations
 WHERE company_id=s.company_id AND session_id=s.id AND reason IN ('process_containment','process_containment_bound')
 ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1
) containment ON true
JOIN LATERAL (
 SELECT data FROM worker_observations
 WHERE company_id=s.company_id AND session_id=s.id AND reason='host_reconciliation'
 ORDER BY id DESC LIMIT 1
) host_observation ON true
WHERE s.company_id=$1 AND s.id=$2`, companyID, sessionID).Scan(&state, &stopReceipt, &processPID, &containmentRaw, &hostRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var metadata runner.ProcessContainmentMetadata
	var observation linuxWorkerHostStopObservation
	if json.Unmarshal(containmentRaw, &metadata) != nil || json.Unmarshal(hostRaw, &observation) != nil {
		return false, core.Integrity
	}
	processID := int(processPID)
	return linuxWorkerHostStopReceiptMatches(state, stopReceipt, processID, metadata, observation) ||
		linuxWorkerCgroupStopReceiptMatches(state, stopReceipt, processID, metadata, observation), nil
}

func (k *Kernel) TXConfirmLinuxWorkerTreeStopped(ctx context.Context, candidate WorkerProcessRecoveryCandidate, proof runner.LinuxProcessGroupStopProof) error {
	return core.Denied
}

func (k *Kernel) TXConfirmLinuxWorkerCgroupStopped(ctx context.Context, candidate WorkerProcessRecoveryCandidate, proof environment.LinuxWorkerCgroupStopProof) error {
	expectedName, nameErr := environment.LinuxWorkerCgroupName(candidate.SessionID)
	legacyPlaceholder := candidate.Containment.Profile == "linux_process_group@1"
	if !core.ValidID(candidate.CompanyID) || !core.ValidID(candidate.SessionID) ||
		(candidate.State != "restoring" && candidate.State != "reconcile_required") ||
		!candidate.Containment.Valid() || candidate.Containment.HostOS != "linux" ||
		(candidate.Containment.Profile != "linux_worker_cgroup_v2@1" && !legacyPlaceholder) ||
		nameErr != nil || !proof.ForWorkerSession(candidate.SessionID, expectedName) ||
		(legacyPlaceholder && (candidate.Containment.CgroupID != "" || !proof.WasPresent())) ||
		(!legacyPlaceholder && (candidate.Containment.CgroupID != expectedName || candidate.Containment.CgroupHostID != proof.HostIdentity() ||
			(candidate.Containment.CgroupBootID == proof.BootID() && candidate.Containment.CgroupRootID != proof.RootIdentity()))) {
		return core.Denied
	}
	acceptedMetadata := runner.ProcessContainmentMetadata{
		HostOS: "linux", Profile: "linux_worker_cgroup_v2@1", CgroupID: expectedName,
		CgroupHostID: proof.HostIdentity(), CgroupRootID: proof.RootIdentity(), CgroupBootID: proof.BootID(),
	}
	input := struct {
		Candidate WorkerProcessRecoveryCandidate
		Status    string
	}{candidate, "cgroup_empty"}
	requestID := "worker-host-cgroup-stop-" + candidate.SessionID
	_, err := k.TXWrite(ctx, Scope{candidate.CompanyID}, nil, requestID, "worker.host_tree_stopped", input, func(tx pgx.Tx) (Receipt, error) {
		var state, taskID, employeeID, incarnation string
		var epoch int64
		var processPID pgtype.Int8
		if scanErr := tx.QueryRow(ctx, `SELECT state,task_id,employee_id,epoch,incarnation,process_pid
FROM worker_sessions WHERE company_id=$1 AND id=$2 FOR UPDATE`, candidate.CompanyID, candidate.SessionID).Scan(&state, &taskID, &employeeID, &epoch, &incarnation, &processPID); scanErr != nil {
			return Receipt{}, scanErr
		}
		if state != candidate.State || state == "stopped" || taskID != candidate.TaskID || employeeID != candidate.EmployeeID ||
			epoch != candidate.Epoch || incarnation != candidate.Incarnation ||
			(processPID.Valid && (candidate.PID <= 0 || int(processPID.Int64) != candidate.PID)) || (!processPID.Valid && candidate.PID != 0) {
			return Receipt{}, core.Conflict
		}
		var reason string
		var encoded []byte
		if scanErr := tx.QueryRow(ctx, `SELECT reason,data FROM worker_observations WHERE company_id=$1 AND session_id=$2 AND reason IN ('process_containment','process_containment_bound') ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1`, candidate.CompanyID, candidate.SessionID).Scan(&reason, &encoded); scanErr != nil {
			return Receipt{}, scanErr
		}
		var metadata runner.ProcessContainmentMetadata
		if json.Unmarshal(encoded, &metadata) != nil || metadata != candidate.Containment {
			return Receipt{}, core.Conflict
		}
		if legacyPlaceholder {
			if reason != "process_containment" {
				return Receipt{}, core.Conflict
			}
			boundData, marshalErr := json.Marshal(acceptedMetadata)
			if marshalErr != nil {
				return Receipt{}, marshalErr
			}
			if _, execErr := tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'process_containment_bound',$4)`, candidate.CompanyID, newID(), candidate.SessionID, boundData); execErr != nil {
				return Receipt{}, execErr
			}
		} else if acceptedMetadata != metadata {
			if reason != "process_containment_bound" {
				return Receipt{}, core.Conflict
			}
			boundData, marshalErr := json.Marshal(acceptedMetadata)
			if marshalErr != nil {
				return Receipt{}, marshalErr
			}
			if _, execErr := tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'process_containment_bound',$4)`, candidate.CompanyID, newID(), candidate.SessionID, boundData); execErr != nil {
				return Receipt{}, execErr
			}
		}
		stopReceipt := "linux-worker-cgroup-empty:" + expectedName
		tag, execErr := tx.Exec(ctx, `UPDATE worker_sessions SET state='stopped',stop_receipt=$4
WHERE company_id=$1 AND id=$2 AND state=$3 AND process_pid IS NOT DISTINCT FROM $5`, candidate.CompanyID, candidate.SessionID, candidate.State, stopReceipt, nullableProcessPID(candidate.PID))
		if execErr != nil {
			return Receipt{}, execErr
		}
		if tag.RowsAffected() != 1 {
			return Receipt{}, core.Conflict
		}
		evidence, marshalErr := json.Marshal(linuxWorkerHostStopObservation{
			HostOS: "linux", Profile: acceptedMetadata.Profile, PID: candidate.PID,
			CgroupID: expectedName, CgroupEmpty: true, CgroupRootID: acceptedMetadata.CgroupRootID, CgroupBootID: acceptedMetadata.CgroupBootID,
		})
		if marshalErr != nil {
			return Receipt{}, marshalErr
		}
		if _, execErr = tx.Exec(ctx, `INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,'host_reconciliation',$4)`, candidate.CompanyID, newID(), candidate.SessionID, evidence); execErr != nil {
			return Receipt{}, execErr
		}
		return Receipt{ID: candidate.SessionID, Status: "stopped_by_host_reconciliation"}, nil
	})
	return err
}

func linuxWorkerCgroupManagerMatches(metadata runner.ProcessContainmentMetadata, manager environment.LinuxWorkerCgroupManager) error {
	if manager == nil {
		return errors.New("Linux WorkerSession cgroup manager is unavailable")
	}
	if err := manager.WorkerContainmentReady(); err != nil {
		return err
	}
	if metadata.Profile != "linux_worker_cgroup_v2@1" {
		return nil
	}
	if metadata.CgroupHostID != manager.WorkerCgroupHostIdentity() {
		return errors.New("Linux WorkerSession cgroup belongs to a different host")
	}
	if metadata.CgroupBootID == manager.WorkerCgroupBootID() && metadata.CgroupRootID != manager.WorkerCgroupRootIdentity() {
		return errors.New("Linux WorkerSession cgroup belongs to a different delegated root")
	}
	return nil
}

func (k *Kernel) ReconcileLinuxWorkerSession(ctx context.Context, companyID, sessionID string, cgroupManagers ...environment.LinuxWorkerCgroupManager) error {
	if runtime.GOOS != "linux" || !core.ValidID(companyID) || !core.ValidID(sessionID) {
		return core.Denied
	}
	confirmed, err := k.linuxWorkerHostStopAlreadyConfirmed(ctx, companyID, sessionID)
	if err != nil || confirmed {
		return err
	}
	candidates, err := k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if candidate.CompanyID != companyID || candidate.SessionID != sessionID {
			continue
		}
		if (candidate.State != "restoring" && candidate.State != "reconcile_required") || candidate.Containment.HostOS != "linux" {
			return core.Denied
		}
		if candidate.Containment.Profile != "linux_worker_cgroup_v2@1" && candidate.Containment.Profile != "linux_process_group@1" {
			return core.Denied
		}
		var manager environment.LinuxWorkerCgroupManager
		if len(cgroupManagers) > 0 {
			manager = cgroupManagers[0]
		}
		if manager == nil {
			return errors.New("Linux WorkerSession cgroup manager is unavailable")
		}
		if identityErr := linuxWorkerCgroupManagerMatches(candidate.Containment, manager); identityErr != nil {
			return identityErr
		}
		expectedName, nameErr := environment.LinuxWorkerCgroupName(candidate.SessionID)
		if nameErr != nil || candidate.Containment.CgroupID != expectedName {
			return core.Integrity
		}
		proof, reconcileErr := manager.ReconcileWorker(candidate.SessionID)
		if reconcileErr != nil {
			return reconcileErr
		}
		if candidate.Containment.Profile == "linux_process_group@1" && !proof.WasPresent() {
			return errors.New("legacy Linux WorkerSession has no bound cgroup to prove descendant shutdown")
		}
		return k.TXConfirmLinuxWorkerCgroupStopped(ctx, candidate, proof)
	}
	return core.OutOfScope
}

func (k *Kernel) ReconcileLinuxWorkerSessions(ctx context.Context, cgroupManagers ...environment.LinuxWorkerCgroupManager) (WorkerHostReconciliationSummary, error) {
	var summary WorkerHostReconciliationSummary
	if runtime.GOOS != "linux" {
		return summary, core.Denied
	}
	candidates, err := k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		return summary, err
	}
	var failures []error
	knownSessionIDs := make([]string, 0)
	for _, candidate := range candidates {
		if candidate.Containment.HostOS != "linux" {
			continue
		}
		if candidate.Containment.Profile == "linux_process_group@1" || candidate.Containment.Profile == "linux_worker_cgroup_v2@1" {
			knownSessionIDs = append(knownSessionIDs, candidate.SessionID)
		}
		if candidate.State != "reconcile_required" && candidate.State != "restoring" {
			continue
		}
		summary.Candidates++
		if candidate.Containment.Profile != "linux_worker_cgroup_v2@1" && candidate.Containment.Profile != "linux_process_group@1" {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux WorkerSession %s has unsupported containment profile", candidate.SessionID))
			continue
		}
		var manager environment.LinuxWorkerCgroupManager
		if len(cgroupManagers) > 0 {
			manager = cgroupManagers[0]
		}
		if manager == nil {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux WorkerSession %s cgroup manager is unavailable", candidate.SessionID))
			continue
		}
		if identityErr := linuxWorkerCgroupManagerMatches(candidate.Containment, manager); identityErr != nil {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux WorkerSession %s cgroup host/root identity mismatch: %w", candidate.SessionID, identityErr))
			continue
		}
		expectedName, nameErr := environment.LinuxWorkerCgroupName(candidate.SessionID)
		if nameErr != nil || candidate.Containment.CgroupID != expectedName {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux WorkerSession %s cgroup identity is invalid", candidate.SessionID))
			continue
		}
		proof, reconcileErr := manager.ReconcileWorker(candidate.SessionID)
		if reconcileErr == nil && candidate.Containment.Profile == "linux_process_group@1" && !proof.WasPresent() {
			reconcileErr = errors.New("legacy Linux WorkerSession has no bound cgroup to prove descendant shutdown")
		}
		if reconcileErr == nil {
			reconcileErr = k.TXConfirmLinuxWorkerCgroupStopped(ctx, candidate, proof)
		}
		if reconcileErr != nil {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux worker session %s host reconciliation failed", candidate.SessionID))
			continue
		}
		summary.Stopped++
	}
	if len(cgroupManagers) > 0 && cgroupManagers[0] != nil {
		if orphanErr := cgroupManagers[0].ReconcileWorkerOrphans(knownSessionIDs); orphanErr != nil {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("Linux Worker cgroup orphan reconciliation failed: %w", orphanErr))
		}
	}
	return summary, errors.Join(failures...)
}

func (k *Kernel) ReconcileWorkerSession(ctx context.Context, companyID, sessionID string, cgroupManagers ...environment.LinuxWorkerCgroupManager) error {
	switch runtime.GOOS {
	case "windows":
		return k.ReconcileWindowsWorkerSession(ctx, companyID, sessionID)
	case "linux":
		return k.ReconcileLinuxWorkerSession(ctx, companyID, sessionID, cgroupManagers...)
	default:
		return core.Denied
	}
}

func (k *Kernel) ReconcileWorkerSessions(ctx context.Context, cgroupManagers ...environment.LinuxWorkerCgroupManager) (WorkerHostReconciliationSummary, error) {
	switch runtime.GOOS {
	case "windows":
		return k.ReconcileWindowsWorkerSessions(ctx)
	case "linux":
		return k.ReconcileLinuxWorkerSessions(ctx, cgroupManagers...)
	default:
		return WorkerHostReconciliationSummary{}, core.Denied
	}
}

func (k *Kernel) ReconcileWindowsWorkerSession(ctx context.Context, companyID, sessionID string) error {
	if runtime.GOOS != "windows" || !core.ValidID(companyID) || !core.ValidID(sessionID) {
		return core.Denied
	}
	confirmed, err := k.windowsWorkerHostStopAlreadyConfirmed(ctx, companyID, sessionID)
	if err != nil || confirmed {
		return err
	}
	candidates, err := k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if candidate.CompanyID != companyID || candidate.SessionID != sessionID {
			continue
		}
		if (candidate.State != "restoring" && candidate.State != "reconcile_required") || candidate.Containment.HostOS != "windows" || candidate.Containment.Profile != "windows_worker_job_object@1" {
			return core.Denied
		}
		reconcileCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		proof, reconcileErr := runner.ReconcileWindowsWorkerProcessTree(reconcileCtx, candidate.SessionID, candidate.PID)
		cancel()
		if reconcileErr != nil {
			return reconcileErr
		}
		return k.TXConfirmWindowsWorkerTreeStopped(ctx, candidate, proof)
	}
	return core.OutOfScope
}

func nullableProcessPID(pid int) any {
	if pid == 0 {
		return nil
	}
	return pid
}

func (k *Kernel) ReconcileWindowsWorkerSessions(ctx context.Context) (WorkerHostReconciliationSummary, error) {
	var summary WorkerHostReconciliationSummary
	if runtime.GOOS != "windows" {
		return summary, core.Denied
	}
	candidates, err := k.WorkerSessionRecoveryCandidates(ctx)
	if err != nil {
		return summary, err
	}
	var failures []error
	for _, candidate := range candidates {
		if candidate.State != "reconcile_required" || candidate.Containment.HostOS != "windows" || candidate.Containment.Profile != "windows_worker_job_object@1" {
			continue
		}
		summary.Candidates++
		candidateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		proof, reconcileErr := runner.ReconcileWindowsWorkerProcessTree(candidateCtx, candidate.SessionID, candidate.PID)
		cancel()
		if reconcileErr == nil {
			reconcileErr = k.TXConfirmWindowsWorkerTreeStopped(ctx, candidate, proof)
		}
		if reconcileErr != nil {
			summary.Unresolved++
			failures = append(failures, fmt.Errorf("worker session %s host reconciliation failed", candidate.SessionID))
			continue
		}
		summary.Stopped++
	}
	return summary, errors.Join(failures...)
}
