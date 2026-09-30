// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/intake"
	"polis/internal/kernel"
)

func TestResearchSimulationPersistsExactInputBoundDeterministicRun(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("r3-research-run-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, runtime.LocalScope(companyID), "Research simulator input fixture", "persist deterministic simulation output", "r3-research-run-mission")
	if err != nil {
		t.Fatal(err)
	}
	datasetBytes := []byte(`{"control":[1,2,3,4],"treatment":[2,3,4,5]}`)
	datasetPrepared, datasetContent, err := intake.PrepareMissionInput("dataset.json", "application/json", datasetBytes)
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), mission.ID, "", "r3-research-dataset", datasetPrepared, datasetContent)
	if err != nil {
		t.Fatal(err)
	}
	methodBytes, err := json.Marshal(map[string]any{"schemaVersion": "polis-research-method@1", "algorithm": "bootstrap-mean-difference@1", "iterations": 16, "sampleSize": 4})
	if err != nil {
		t.Fatal(err)
	}
	methodPrepared, methodContent, err := intake.PrepareMissionInput("method.json", "application/json", methodBytes)
	if err != nil {
		t.Fatal(err)
	}
	method, err := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), mission.ID, "", "r3-research-method", methodPrepared, methodContent)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{runtime: runtime}
	request := ResearchSimulationCommandRequest{
		DatasetInputID: dataset.InputID, DatasetInputRevision: dataset.Revision,
		MethodInputID: method.InputID, MethodInputRevision: method.Revision,
		Seed: "42", ControlDefinition: "specified control cohort", RiskBudgetUnits: 128, RequestID: "r3-research-simulation-run",
	}
	run, err := service.RunResearchSimulation(ctx, companyID, request)
	if err != nil || run.OutputSHA256 == "" || run.RiskConsumedUnits != "128" || run.Output.RiskConsumedUnits != 128 || run.Seed != "42" {
		t.Fatalf("research simulation run=%+v err=%v", run, err)
	}
	if run.ProfileID != "research-simulation-reference" || run.ProfileRevision != "research-simulation@1" {
		t.Fatalf("simulation profile changed: %+v", run)
	}
	constraintPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer constraintPool.Close()
	_, err = constraintPool.Exec(ctx, `INSERT INTO domain_workflow_research_simulation_runs(
company_id,run_id,profile_id,profile_revision,protocol_revision,dataset_input_id,dataset_input_revision,dataset_sha256,
method_input_id,method_input_revision,method_sha256,seed,control_definition,risk_unit,risk_budget_units,risk_consumed_units,
output_sha256,output_json,request_id)
SELECT company_id,run_id||'-underreported',profile_id,profile_revision,protocol_revision,dataset_input_id,dataset_input_revision,dataset_sha256,
method_input_id,method_input_revision,method_sha256,seed,control_definition,risk_unit,1,1,repeat('d',64),
jsonb_set(output_json,'{riskConsumedUnits}','1'::jsonb),'r3-research-invalid-risk-accounting'
FROM domain_workflow_research_simulation_runs WHERE company_id=$1 AND run_id=$2`, companyID, run.RunID)
	if err == nil {
		t.Fatal("database accepted a persisted output that underreports sample draws")
	}
	replay, err := service.RunResearchSimulation(ctx, companyID, request)
	if err != nil || replay.RunID != run.RunID || replay.OutputSHA256 != run.OutputSHA256 {
		t.Fatalf("research simulation replay=%+v err=%v", replay, err)
	}
	conflictingReplay := request
	conflictingReplay.RiskBudgetUnits++
	if _, err = service.RunResearchSimulation(ctx, companyID, conflictingReplay); !errors.Is(err, core.Conflict) {
		t.Fatalf("changed request replay error=%v, want conflict", err)
	}
	tooSmallBudget := request
	tooSmallBudget.RiskBudgetUnits = 127
	tooSmallBudget.RequestID = "r3-research-over-budget"
	if _, err = service.RunResearchSimulation(ctx, companyID, tooSmallBudget); !errors.Is(err, core.Malformed) {
		t.Fatalf("over-budget simulation error=%v, want malformed rejection", err)
	}
	otherCompany := fmt.Sprintf("r3-research-other-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, otherCompany); err != nil {
		t.Fatal(err)
	}
	crossCompany := request
	crossCompany.RequestID = "r3-research-cross-company"
	crossCompany.DatasetInputID = "not-the-selected-company-input"
	if _, err = service.RunResearchSimulation(ctx, otherCompany, crossCompany); err == nil {
		t.Fatal("unavailable cross-company source was accepted")
	}
	largeMethodBytes, err := json.Marshal(map[string]any{"schemaVersion": "polis-research-method@1", "algorithm": "bootstrap-mean-difference@1", "iterations": 10000, "sampleSize": 256})
	if err != nil {
		t.Fatal(err)
	}
	largeMethodPrepared, largeMethodContent, err := intake.PrepareMissionInput("bounded-method.json", "application/json", largeMethodBytes)
	if err != nil {
		t.Fatal(err)
	}
	largeMethod, err := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), mission.ID, "", "r3-research-method-large", largeMethodPrepared, largeMethodContent)
	if err != nil {
		t.Fatal(err)
	}
	concurrentRequest := ResearchSimulationCommandRequest{
		DatasetInputID: dataset.InputID, DatasetInputRevision: dataset.Revision,
		MethodInputID: largeMethod.InputID, MethodInputRevision: largeMethod.Revision,
		Seed: "999", ControlDefinition: "concurrent duplicate fixture", RiskBudgetUnits: domainworkflow.MaxResearchSimulationRiskUnits,
		RequestID: "r3-research-simulation-concurrent",
	}
	const concurrentRetries = 6
	start := make(chan struct{})
	results := make(chan struct {
		run kernel.ResearchSimulationRunRecord
		err error
	}, concurrentRetries)
	var workers sync.WaitGroup
	workers.Add(concurrentRetries)
	for index := 0; index < concurrentRetries; index++ {
		go func() {
			defer workers.Done()
			<-start
			run, runErr := service.RunResearchSimulation(ctx, companyID, concurrentRequest)
			results <- struct {
				run kernel.ResearchSimulationRunRecord
				err error
			}{run: run, err: runErr}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	concurrentRunID := ""
	for result := range results {
		if result.err != nil {
			t.Errorf("identical concurrent research simulation retry failed: %v", result.err)
			continue
		}
		if concurrentRunID == "" {
			concurrentRunID = result.run.RunID
		} else if result.run.RunID != concurrentRunID {
			t.Errorf("identical concurrent retries returned different run IDs %q and %q", concurrentRunID, result.run.RunID)
		}
	}
	changedConcurrentRequest := concurrentRequest
	changedConcurrentRequest.RiskBudgetUnits--
	if _, err = service.RunResearchSimulation(ctx, companyID, changedConcurrentRequest); !errors.Is(err, core.Conflict) {
		t.Errorf("changed concurrent request replay error=%v, want conflict", err)
	}
	ledger, err := runtime.ListDomainEvidence(ctx, companyID)
	if err != nil || len(ledger.ResearchSimulationRuns) != 2 {
		t.Fatalf("domain ledger research runs=%d err=%v", len(ledger.ResearchSimulationRuns), err)
	}
	for _, profile := range ledger.Profiles {
		if profile.ID == "research-simulation-reference" && (profile.QualificationStatus != "not_run" || profile.ExecutionEnabled) {
			t.Fatalf("simulation run qualified or enabled its reference profile: %+v", profile)
		}
	}
}
