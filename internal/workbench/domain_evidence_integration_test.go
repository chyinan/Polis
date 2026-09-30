// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"polis/internal/control"
	"polis/internal/domainworkflow"
	"polis/internal/kernel"
)

func TestDomainEvidenceWorkbenchPersistsAndProjectsReadinessWithoutQualification(t *testing.T) {
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
	readStore, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer readStore.Close()
	companyID := fmt.Sprintf("domain-evidence-workbench-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(nil, control.NewService(runtime, nil))
	profile := domainworkflow.ReferenceWorkflows()[0]
	input := control.DomainEvidenceCommandRequest{
		DomainEvidenceSubmission: domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision},
		RequestID:                "domain-evidence-workbench-submit",
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/domain-evidence", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("submit evidence status=%d body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"submission":{"profileId"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"evidence":[]`)) || bytes.Contains(response.Body.Bytes(), []byte(`"ProfileID"`)) {
		t.Fatalf("evidence DTO JSON field names are not camelCase: %s", response.Body.String())
	}
	var record kernel.DomainEvidenceRecord
	if err = json.Unmarshal(response.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.ReadinessStatus != domainworkflow.DomainEvidenceIncomplete || record.QualificationStatus != "not_run" || record.ExecutionEnabled {
		t.Fatalf("API submission unexpectedly qualified the domain: %+v", record)
	}
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/domain-workflows", nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("read evidence ledger status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}
	if !bytes.Contains(getResponse.Body.Bytes(), []byte(`"requiredEvidence"`)) || bytes.Contains(getResponse.Body.Bytes(), []byte(`"RequiredEvidence"`)) {
		t.Fatalf("domain profile JSON field names are not camelCase: %s", getResponse.Body.String())
	}
	var ledger kernel.DomainEvidenceLedger
	if err = json.Unmarshal(getResponse.Body.Bytes(), &ledger); err != nil {
		t.Fatal(err)
	}
	if ledger.CompanyID != companyID || len(ledger.Profiles) != 2 || len(ledger.Submissions) != 1 || ledger.Submissions[0].RecordID != record.RecordID {
		t.Fatalf("Workbench did not project persisted company evidence: %+v", ledger)
	}
}
