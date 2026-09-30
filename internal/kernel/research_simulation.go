// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/domainworkflow"
)

const (
	maxResearchSimulationRuns = 100
	maxResearchDatasetBytes   = 1 << 20
	maxResearchMethodBytes    = 16 << 10
)

func (k *Kernel) LockResearchSimulationRequest(ctx context.Context, companyID, requestID string) (func(), error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(requestID) {
		return nil, core.Malformed
	}
	connection, err := k.lockPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := capabilitySourceAdvisoryLockKey(companyID, "research-simulation-request", requestID)
	var acquired string
	if err = connection.QueryRow(ctx, "SELECT pg_advisory_lock($1)::text", key).Scan(&acquired); err != nil {
		discardCapabilitySourceLockConnection(connection)
		return nil, err
	}
	return func() { releaseCapabilitySourceLockKeys(connection, []int64{key}) }, nil
}

type ResearchSimulationRunRecord struct {
	CompanyID            string                                  `json:"companyId"`
	RunID                string                                  `json:"runId"`
	ProfileID            string                                  `json:"profileId"`
	ProfileRevision      string                                  `json:"profileRevision"`
	ProtocolRevision     string                                  `json:"protocolRevision"`
	DatasetInputID       string                                  `json:"datasetInputId"`
	DatasetInputRevision string                                  `json:"datasetInputRevision"`
	DatasetSHA256        string                                  `json:"datasetSha256"`
	MethodInputID        string                                  `json:"methodInputId"`
	MethodInputRevision  string                                  `json:"methodInputRevision"`
	MethodSHA256         string                                  `json:"methodSha256"`
	Seed                 string                                  `json:"seed"`
	ControlDefinition    string                                  `json:"controlDefinition"`
	RiskUnit             string                                  `json:"riskUnit"`
	RiskBudgetUnits      string                                  `json:"riskBudgetUnits"`
	RiskConsumedUnits    string                                  `json:"riskConsumedUnits"`
	OutputSHA256         string                                  `json:"outputSha256"`
	Output               domainworkflow.ResearchSimulationOutput `json:"output"`
	RequestID            string                                  `json:"requestId"`
	CreatedAt            string                                  `json:"createdAt"`
}

type ResearchSimulationRunInput struct {
	RequestID string
	Protocol  domainworkflow.ResearchProtocol
	Dataset   domainworkflow.SourceReference
	Method    domainworkflow.SourceReference
	Receipt   domainworkflow.ResearchSimulationReceipt
}

type ResearchSimulationInputSnapshot struct {
	Reference domainworkflow.SourceReference
	MediaType string
	Bytes     []byte
}

func (k *Kernel) ReadResearchSimulationInput(ctx context.Context, companyID, inputID string, revision int64, maxBytes int64) (ResearchSimulationInputSnapshot, error) {
	if !core.ValidID(companyID) || !core.ValidID(inputID) || revision < 1 || maxBytes < 1 || maxBytes > 8<<20 {
		return ResearchSimulationInputSnapshot{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ResearchSimulationInputSnapshot{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, Scope{companyID}, nil); err != nil {
		return ResearchSimulationInputSnapshot{}, err
	}
	var companyState, digest, inputState, mediaType string
	var byteSize int64
	err = tx.QueryRow(ctx, `SELECT c.state,i.content_digest,i.state,i.media_type,i.byte_size
FROM companies c JOIN mission_inputs i ON i.company_id=c.id
WHERE c.id=$1 AND i.input_id=$2 AND i.revision=$3`, companyID, inputID, revision).Scan(&companyState, &digest, &inputState, &mediaType, &byteSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchSimulationInputSnapshot{}, core.OutOfScope
	}
	if err != nil {
		return ResearchSimulationInputSnapshot{}, err
	}
	if companyState != "active" || inputState != "usable" || byteSize < 1 || byteSize > maxBytes || !(mediaType == "application/json" || strings.HasPrefix(mediaType, "text/")) {
		return ResearchSimulationInputSnapshot{}, core.Denied
	}
	if err = tx.Commit(ctx); err != nil {
		return ResearchSimulationInputSnapshot{}, err
	}
	content, err := readBlobBounded(k.root, companyID, digest, maxBytes)
	if err != nil {
		return ResearchSimulationInputSnapshot{}, err
	}
	if int64(len(content)) != byteSize {
		return ResearchSimulationInputSnapshot{}, core.Integrity
	}
	return ResearchSimulationInputSnapshot{
		Reference: domainworkflow.SourceReference{InputID: inputID, Revision: revision, SHA256: digest},
		MediaType: mediaType, Bytes: content,
	}, nil
}

func (k *Kernel) TXRecordResearchSimulationRun(ctx context.Context, companyID string, input ResearchSimulationRunInput) (ResearchSimulationRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RequestID) || input.Protocol.Revision < 1 ||
		input.Protocol.Mode != domainworkflow.ExecutionSimulation || !domainworkflow.ValidResearchSimulationReceipt(input.Protocol, input.Dataset, input.Method, input.Receipt) {
		return ResearchSimulationRunRecord{}, core.Malformed
	}
	outputBytes, err := json.Marshal(input.Receipt.Output)
	if err != nil || !bytes.Equal(outputBytes, input.Receipt.OutputBytes) {
		return ResearchSimulationRunRecord{}, core.Integrity
	}
	outputHash := sha256.Sum256(outputBytes)
	if hex.EncodeToString(outputHash[:]) != input.Receipt.OutputSHA256 {
		return ResearchSimulationRunRecord{}, core.Integrity
	}
	runID := stableCapabilityID("research-simulation-run", companyID, input.RequestID)
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "domain.research.simulation.run", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if err := verifyResearchSimulationInputTx(ctx, tx, companyID, input.Dataset, maxResearchDatasetBytes); err != nil {
			return Receipt{}, err
		}
		if err := verifyResearchSimulationInputTx(ctx, tx, companyID, input.Method, maxResearchMethodBytes); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO domain_workflow_research_simulation_runs(
company_id,run_id,profile_id,profile_revision,protocol_revision,dataset_input_id,dataset_input_revision,dataset_sha256,
method_input_id,method_input_revision,method_sha256,seed,control_definition,risk_unit,risk_budget_units,risk_consumed_units,
output_sha256,output_json,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb,$19)`,
			companyID, runID, domainworkflow.ResearchSimulationProfileID, domainworkflow.ResearchSimulationProfileRevision, input.Protocol.Revision,
			input.Dataset.InputID, input.Dataset.Revision, input.Dataset.SHA256,
			input.Method.InputID, input.Method.Revision, input.Method.SHA256, input.Receipt.Output.Seed,
			input.Protocol.ControlDefinition, input.Protocol.RiskUnit, input.Protocol.RiskBudgetUnits, input.Receipt.Output.RiskConsumedUnits,
			input.Receipt.OutputSHA256, outputBytes, input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: runID, Status: "simulated"}, nil
	})
	if err != nil {
		return ResearchSimulationRunRecord{}, err
	}
	return k.GetResearchSimulationRun(ctx, companyID, runID)
}

func (k *Kernel) GetResearchSimulationRun(ctx context.Context, companyID, runID string) (ResearchSimulationRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(runID) {
		return ResearchSimulationRunRecord{}, core.Malformed
	}
	return readResearchSimulationRun(ctx, k.pool, companyID, runID)
}

func (k *Kernel) GetResearchSimulationRunByRequest(ctx context.Context, companyID, requestID string) (ResearchSimulationRunRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return ResearchSimulationRunRecord{}, false, core.Malformed
	}
	var runID string
	err := k.pool.QueryRow(ctx, "SELECT run_id FROM domain_workflow_research_simulation_runs WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchSimulationRunRecord{}, false, nil
	}
	if err != nil {
		return ResearchSimulationRunRecord{}, false, err
	}
	record, err := k.GetResearchSimulationRun(ctx, companyID, runID)
	return record, err == nil, err
}

func (k *Kernel) listResearchSimulationRuns(ctx context.Context, tx pgx.Tx, companyID string) ([]ResearchSimulationRunRecord, error) {
	rows, err := tx.Query(ctx, `SELECT run_id FROM domain_workflow_research_simulation_runs
WHERE company_id=$1 ORDER BY created_at DESC,run_id DESC LIMIT $2`, companyID, maxResearchSimulationRuns)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, maxResearchSimulationRuns)
	for rows.Next() {
		var runID string
		if err = rows.Scan(&runID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, runID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	runs := make([]ResearchSimulationRunRecord, 0, len(ids))
	for _, runID := range ids {
		record, readErr := readResearchSimulationRun(ctx, tx, companyID, runID)
		if readErr != nil {
			return nil, readErr
		}
		runs = append(runs, record)
	}
	return runs, nil
}

type researchSimulationRunQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readResearchSimulationRun(ctx context.Context, queryer researchSimulationRunQueryer, companyID, runID string) (ResearchSimulationRunRecord, error) {
	var record ResearchSimulationRunRecord
	var rawOutput []byte
	err := queryer.QueryRow(ctx, `SELECT company_id,run_id,profile_id,profile_revision,protocol_revision::text,
dataset_input_id,dataset_input_revision::text,dataset_sha256,method_input_id,method_input_revision::text,method_sha256,
seed,control_definition,risk_unit,risk_budget_units::text,risk_consumed_units::text,output_sha256,output_json,request_id,created_at::text
FROM domain_workflow_research_simulation_runs WHERE company_id=$1 AND run_id=$2`, companyID, runID).Scan(
		&record.CompanyID, &record.RunID, &record.ProfileID, &record.ProfileRevision, &record.ProtocolRevision,
		&record.DatasetInputID, &record.DatasetInputRevision, &record.DatasetSHA256, &record.MethodInputID, &record.MethodInputRevision, &record.MethodSHA256,
		&record.Seed, &record.ControlDefinition, &record.RiskUnit, &record.RiskBudgetUnits, &record.RiskConsumedUnits, &record.OutputSHA256, &rawOutput, &record.RequestID, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchSimulationRunRecord{}, core.OutOfScope
	}
	if err != nil {
		return ResearchSimulationRunRecord{}, err
	}
	if err = json.Unmarshal(rawOutput, &record.Output); err != nil {
		return ResearchSimulationRunRecord{}, core.Integrity
	}
	canonicalOutput, err := json.Marshal(record.Output)
	if err != nil {
		return ResearchSimulationRunRecord{}, err
	}
	if !validPersistedResearchSimulationRunOutput(record, canonicalOutput) {
		return ResearchSimulationRunRecord{}, core.Integrity
	}
	return record, nil
}

func verifyResearchSimulationInputTx(ctx context.Context, tx pgx.Tx, companyID string, reference domainworkflow.SourceReference, maxBytes int64) error {
	if !core.ValidID(companyID) || !domainworkflow.ValidResearchSimulationReference(reference) || maxBytes < 1 {
		return core.Malformed
	}
	var digest, state, mediaType string
	var size int64
	err := tx.QueryRow(ctx, `SELECT content_digest,state,media_type,byte_size FROM mission_inputs
WHERE company_id=$1 AND input_id=$2 AND revision=$3`, companyID, reference.InputID, reference.Revision).Scan(&digest, &state, &mediaType, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if digest != reference.SHA256 || state != "usable" || size < 1 || size > maxBytes || !(mediaType == "application/json" || strings.HasPrefix(mediaType, "text/")) {
		return core.Integrity
	}
	return nil
}
