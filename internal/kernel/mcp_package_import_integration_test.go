// pattern: Imperative Shell
package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/mcptransport"
)

func TestStdioMCPPackageImportPinsCompanyCASAndReplaysByRequestID(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	bundle := prepareKernelTestStdioMCPBundle(t, "server-data-v1")
	input := StdioMCPPackageInput{Revision: "1.0.0", Archive: kernelTestStdioMCPArchive(t, bundle), RequestID: "mcp-package-import-1"}
	imported, err := k.TXImportStdioMCPPackage(ctx, scope.company, input)
	if err != nil {
		t.Fatalf("import local stdio MCP source package: %v", err)
	}
	if imported.CompanyID != scope.company || imported.ServerID == "" || imported.Revision != input.Revision || imported.ManifestDigest != bundle.ManifestDigest {
		t.Fatalf("imported package identity=%+v", imported)
	}
	files, err := k.ReadStdioMCPPackageFiles(ctx, scope.company, imported.ServerID, imported.ID)
	if err != nil {
		t.Fatalf("read immutable package files from company CAS: %v", err)
	}
	if err = capabilitysource.VerifyStdioMCPBundle(bundle.Manifest, bundle.ManifestDigest, files); err != nil {
		t.Fatalf("CAS package readback differs from frozen import: %v", err)
	}
	replayed, err := k.TXImportStdioMCPPackage(ctx, scope.company, input)
	if err != nil || replayed.ID != imported.ID || replayed.ManifestDigest != imported.ManifestDigest {
		t.Fatalf("same package request replay=(%+v,%v), want original package revision", replayed, err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, scope.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.MCPServers) != 1 || catalog.MCPServers[0].Status != "unverified" || len(catalog.MCPPackages) != 1 || catalog.MCPPackages[0].ID != imported.ID {
		t.Fatalf("package import advanced approval or failed catalog readback: servers=%+v packages=%+v", catalog.MCPServers, catalog.MCPPackages)
	}
	if _, err = k.ReadStdioMCPPackageFiles(ctx, "another-company", imported.ServerID, imported.ID); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("cross-company package read error=%v, want %s", err, core.OutOfScope)
	}
	_, err = k.pool.Exec(ctx, `UPDATE mcp_server_package_revisions SET revision=revision || '-mutated' WHERE company_id=$1 AND package_revision_id=$2`, scope.company, imported.ID)
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "P0001" || postgresError.Message != "task_validation_bindings are immutable" {
		t.Fatalf("immutable package revision update error=%v, want append-only trigger rejection", err)
	}
}

func TestStdioMCPObservationReservationPreventsRequestReplay(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	bundle := prepareKernelTestStdioMCPBundle(t, "reserved observation package")
	packageRevision, err := k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		Revision: "1.0.0", Archive: kernelTestStdioMCPArchive(t, bundle), RequestID: "mcp-observation-reservation-import",
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := k.TXQualifyCapability(ctx, scope.company, CapabilityQualificationInput{
		CapabilityKind: "mcp", CapabilityID: packageRevision.ServerID, RequestID: "mcp-observation-reservation-qualify",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: packageRevision.ServerID, QualificationID: metadata.QualificationID,
		Decision: "approved", Rationale: "reviewed fixed package descriptor", RequestID: "mcp-observation-reservation-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: core.EmployeeBackendID, CapabilityKind: "mcp", CapabilityID: packageRevision.ServerID,
		QualificationID: metadata.QualificationID, Reason: "pin the approved stdio package", RequestID: "mcp-observation-reservation-bind",
	}); err != nil {
		t.Fatal(err)
	}
	input := StdioMCPRuntimeObservationReservationInput{
		CapabilityID: packageRevision.ServerID, CapabilityQualificationID: metadata.QualificationID,
		PackageRevisionID: packageRevision.ID, PackageManifestSHA256: packageRevision.ManifestDigest,
		RequestID: "mcp-observation-reservation-request",
	}
	first, err := k.TXReserveStdioMCPRuntimeObservation(ctx, scope.company, input)
	if err != nil || !first.Start || first.Existing != nil {
		t.Fatalf("first observation reservation=%+v error=%v", first, err)
	}
	if _, err = k.TXReserveStdioMCPRuntimeObservation(ctx, scope.company, input); !errors.Is(err, core.Conflict) {
		t.Fatalf("unresolved observation request replay error=%v, want %s", err, core.Conflict)
	}
	_, releaseServer, err := k.LockStdioMCPPackageServer(ctx, scope.company, packageRevision.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	serverLockReleased := false
	defer func() {
		if !serverLockReleased {
			releaseServer()
		}
	}()
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: packageRevision.ServerID, QualificationID: metadata.QualificationID,
		Decision: "revoked", Rationale: "must not revoke during a controlled call", RequestID: "mcp-observation-revoke-during-lock",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("capability revocation while server fence is held error=%v, want %s", err, core.Conflict)
	}
	if _, err = k.TXRevokeEmployeeCapability(ctx, scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: core.EmployeeBackendID, CapabilityKind: "mcp", CapabilityID: packageRevision.ServerID,
		QualificationID: metadata.QualificationID, Reason: "must not unbind during a controlled call", RequestID: "mcp-observation-unbind-during-lock",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("Employee binding revocation while server fence is held error=%v, want %s", err, core.Conflict)
	}
	startupReconciler, ok := any(k).(interface {
		TXReconcileStdioMCPRuntimeObservationIntents(context.Context, string) error
	})
	if !ok {
		t.Fatal("Kernel does not reconcile reserved MCP observation requests on startup")
	}
	if err = startupReconciler.TXReconcileStdioMCPRuntimeObservationIntents(ctx, "mcp-observation-live-reconciliation"); err != nil {
		t.Fatalf("reconcile the interrupted observation request: %v", err)
	}
	var recoveredState string
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_observation_intent_events
WHERE company_id=$1 AND observation_request_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, input.RequestID).Scan(&recoveredState); err != nil || recoveredState != "reserved" {
		t.Fatalf("reconciliation converted an active reservation to unknown: state=%q error=%v", recoveredState, err)
	}
	releaseServer()
	serverLockReleased = true
	if err = startupReconciler.TXReconcileStdioMCPRuntimeObservationIntents(ctx, "mcp-observation-recovery-startup"); err != nil {
		t.Fatalf("reconcile the interrupted observation after its server lease was released: %v", err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_observation_intent_events
WHERE company_id=$1 AND observation_request_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, input.RequestID).Scan(&recoveredState); err != nil || recoveredState != "outcome_unknown" {
		t.Fatalf("startup did not fence the abandoned observation: state=%q error=%v", recoveredState, err)
	}
	if _, err = k.TXReserveStdioMCPRuntimeObservation(ctx, scope.company, input); !errors.Is(err, core.Conflict) {
		t.Fatalf("unknown observation request replay error=%v, want %s", err, core.Conflict)
	}
}

func TestStdioMCPPackageImportSharesServerLockWithRuntimeObservation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	firstBundle := prepareKernelTestStdioMCPBundle(t, "first locked package")
	first, err := k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		Revision: "1.0.0", Archive: kernelTestStdioMCPArchive(t, firstBundle), RequestID: "mcp-observation-lock-import-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	lockedContext, release, err := k.LockStdioMCPPackageServer(ctx, scope.company, first.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	secondBundle := prepareKernelTestStdioMCPBundle(t, "second locked package")
	secondFileDigest := kernelTestBundleFileDigest(t, secondBundle, secondBundle.Manifest.EntryPoint)
	if _, err = k.TXImportStdioMCPPackage(lockedContext, scope.company, StdioMCPPackageInput{
		ServerID: first.ServerID, Revision: "2.0.0", Archive: kernelTestStdioMCPArchive(t, secondBundle), RequestID: "mcp-observation-lock-import-2",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("package import during observation lock error=%v, want %s", err, core.Conflict)
	}
	if _, err = os.Stat(filepath.Join(k.root, scope.company, secondFileDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blocked package update wrote CAS bytes before acquiring the server lock: %v", err)
	}
	release()
	released = true
	if _, err = k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		ServerID: first.ServerID, Revision: "2.0.0", Archive: kernelTestStdioMCPArchive(t, secondBundle), RequestID: "mcp-observation-lock-import-3",
	}); err != nil {
		t.Fatalf("package import after observation lock release: %v", err)
	}
}

func TestStdioMCPPackageUpdateRevokesObservedRuntimeQualification(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	firstBundle := prepareKernelTestStdioMCPBundle(t, "source revision one")
	first, err := k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		Revision: "1.0.0", Archive: kernelTestStdioMCPArchive(t, firstBundle), RequestID: "mcp-package-runtime-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	argsJSON, err := json.Marshal(firstBundle.Manifest.Args)
	if err != nil {
		t.Fatal(err)
	}
	descriptorDigest, err := canonicalMCPDescriptorDigest(firstBundle.Manifest.Name, "stdio", firstBundle.Manifest.Command, argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := k.TXQualifyCapability(ctx, scope.company, CapabilityQualificationInput{CapabilityKind: "mcp", CapabilityID: first.ServerID, RequestID: "mcp-package-runtime-qualify"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: first.ServerID, QualificationID: metadata.QualificationID,
		Decision: "approved", Rationale: "reviewed fixed package descriptor", RequestID: "mcp-package-runtime-approve",
	}); err != nil {
		t.Fatal(err)
	}
	const runtimeID = "package-runtime-qualification-1"
	toolDigest := strings.Repeat("c", 64)
	commandDigest, packageDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	processSpec, err := json.Marshal(map[string]any{
		"CommandSHA256": commandDigest, "PackageManifestSHA256": packageDigest, "ApprovedToolSchemaSHA256": toolDigest,
		"SourcePackageRevisionID": first.ID, "SourcePackageManifestSHA256": first.ManifestDigest,
		"Launch": map[string]any{"NetworkPolicy": "deny_all", "RegistryProxyEndpoint": "", "WorkspaceRoot": `C:\\polis-old-appcontainer`},
	})
	if err != nil {
		t.Fatal(err)
	}
	tools := []byte(`[{}]`)
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'windows',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		scope.company, runtimeID, first.ServerID, metadata.QualificationID, descriptorDigest,
		mcptransport.StdioProfile20260728, stdioMCPHostProfileWindowsAppContainer, commandDigest, packageDigest,
		firstBundle.Manifest.ServerName, firstBundle.Manifest.ServerVersion, mcptransport.ProtocolVersion20260728,
		toolDigest, tools, processSpec, strings.Repeat("d", 64), "package-runtime-record"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,'package-runtime-event',$2,$3,$4,'observed_unqualified',$5,'fixture observation; no MCP program launched','package-runtime-observation')`,
		scope.company, runtimeID, first.ServerID, descriptorDigest, toolDigest); err != nil {
		t.Fatal(err)
	}
	secondBundle := prepareKernelTestStdioMCPBundle(t, "source revision two")
	if _, err = k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		ServerID: first.ServerID, Revision: "2.0.0", Archive: kernelTestStdioMCPArchive(t, secondBundle), RequestID: "mcp-package-runtime-v2",
	}); err != nil {
		t.Fatal(err)
	}
	var latestStatus string
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_qualification_events
WHERE company_id=$1 AND runtime_qualification_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, runtimeID).Scan(&latestStatus); err != nil || latestStatus != "revoked" {
		t.Fatalf("package update did not revoke the older observed runtime: status=%q error=%v", latestStatus, err)
	}
	const staleRuntimeID = "package-runtime-stale-qualification"
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'windows',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		scope.company, staleRuntimeID, first.ServerID, metadata.QualificationID, descriptorDigest,
		mcptransport.StdioProfile20260728, stdioMCPHostProfileWindowsAppContainer, commandDigest, packageDigest,
		firstBundle.Manifest.ServerName, firstBundle.Manifest.ServerVersion, mcptransport.ProtocolVersion20260728,
		toolDigest, tools, processSpec, strings.Repeat("e", 64), "package-runtime-record-stale"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,'package-runtime-stale-event',$2,$3,$4,'observed_unqualified',$5,'fixture for stale package approval gate','package-runtime-stale-observation')`,
		scope.company, staleRuntimeID, first.ServerID, descriptorDigest, toolDigest); err != nil {
		t.Fatal(err)
	}
	if err = k.TXApproveStdioMCPRuntimeQualification(ctx, scope.company, StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: staleRuntimeID, Rationale: "must reject stale package observation", RequestID: "package-runtime-stale-approval",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("stale runtime package approval error=%v, want %s", err, core.Denied)
	}
	if err = k.TXRevokeStdioMCPRuntimeQualificationsForPackageRevision(ctx, scope.company, first.ServerID, first.ID, "package-runtime-close-instance"); err != nil {
		t.Fatalf("revoke runtime qualification when its AppContainer workspace closes: %v", err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_qualification_events
WHERE company_id=$1 AND runtime_qualification_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, staleRuntimeID).Scan(&latestStatus); err != nil || latestStatus != "revoked" {
		t.Fatalf("workspace close did not revoke runtime qualification: status=%q error=%v", latestStatus, err)
	}
	const staleSandboxRuntimeID = "package-runtime-stale-sandbox"
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'windows',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		scope.company, staleSandboxRuntimeID, first.ServerID, metadata.QualificationID, descriptorDigest,
		mcptransport.StdioProfile20260728, stdioMCPHostProfileWindowsAppContainer, commandDigest, packageDigest,
		firstBundle.Manifest.ServerName, firstBundle.Manifest.ServerVersion, mcptransport.ProtocolVersion20260728,
		toolDigest, tools, processSpec, strings.Repeat("f", 64), "package-runtime-record-stale-sandbox"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,'package-runtime-stale-sandbox-event',$2,$3,$4,'observed_unqualified',$5,'fixture for stale workspace startup gate','package-runtime-stale-sandbox-observation')`,
		scope.company, staleSandboxRuntimeID, first.ServerID, descriptorDigest, toolDigest); err != nil {
		t.Fatal(err)
	}
	if err = k.TXRevokeStdioMCPRuntimeQualificationsOutsideSandbox(ctx, `C:\\polis-new-appcontainer`, "package-runtime-new-instance"); err != nil {
		t.Fatalf("revoke stale AppContainer workspace qualifications at startup: %v", err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_qualification_events
WHERE company_id=$1 AND runtime_qualification_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, staleSandboxRuntimeID).Scan(&latestStatus); err != nil || latestStatus != "revoked" {
		t.Fatalf("startup reconciliation did not revoke prior workspace: status=%q error=%v", latestStatus, err)
	}
}

func TestStdioMCPPackageImportSerializesSameRevisionBeforeCASWrites(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	firstBundle := prepareKernelTestStdioMCPBundle(t, "same-revision-first")
	secondBundle := prepareKernelTestStdioMCPBundle(t, "same-revision-second")
	server := registerKernelTestStdioMCP(t, k, scope, firstBundle, "mcp-package-race", "mcp-package-race-register")
	archives := [][]byte{kernelTestStdioMCPArchive(t, firstBundle), kernelTestStdioMCPArchive(t, secondBundle)}
	type importResult struct {
		index int
		err   error
	}
	results := make(chan importResult, 2)
	start := make(chan struct{})
	for index, archive := range archives {
		go func(index int, archive []byte) {
			<-start
			_, importErr := k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
				ServerID: server.ID, Revision: "1.0.0", Archive: archive,
				RequestID: fmt.Sprintf("mcp-package-race-%d", index),
			})
			results <- importResult{index: index, err: importErr}
		}(index, archive)
	}
	close(start)
	var importErrors [2]error
	for range importErrors {
		result := <-results
		importErrors[result.index] = result.err
	}
	firstErr, secondErr := importErrors[0], importErrors[1]
	if (firstErr == nil) == (secondErr == nil) || (firstErr != nil && !errors.Is(firstErr, core.Conflict)) || (secondErr != nil && !errors.Is(secondErr, core.Conflict)) {
		t.Fatalf("same-revision import results=(%v,%v), want exactly one winner and one conflict", firstErr, secondErr)
	}
	losingBundle := firstBundle
	if firstErr == nil {
		losingBundle = secondBundle
	}
	var losingDigest string
	for _, file := range losingBundle.Manifest.Files {
		if file.RelativePath == "server/main.mjs" {
			losingDigest = file.ContentSHA256
		}
	}
	if losingDigest == "" {
		t.Fatal("losing package test fixture has no unique source file")
	}
	if _, err := os.Stat(filepath.Join(k.root, scope.company, losingDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("losing same-revision package wrote an unreferenced CAS blob: stat err=%v", err)
	}
}

func TestStdioMCPPackageImportSerializesReusedRequestIDAcrossTargetsBeforeCASWrites(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	firstBundle := prepareKernelTestStdioMCPBundle(t, "request-id-first-target")
	secondBundle := prepareKernelTestStdioMCPBundle(t, "request-id-second-target")
	firstServer := registerKernelTestStdioMCP(t, k, scope, firstBundle, "mcp-request-lock-first", "mcp-request-lock-register-first")
	secondServer := registerKernelTestStdioMCP(t, k, scope, secondBundle, "mcp-request-lock-second", "mcp-request-lock-register-second")
	requestID := "mcp-package-reused-request"
	firstArchive := kernelTestStdioMCPArchive(t, firstBundle)
	secondArchive := kernelTestStdioMCPArchive(t, secondBundle)
	unlock, err := k.lockCapabilityMCPPackageImport(ctx, scope.company, requestID, firstServer.ID, "1.0.0")
	if err != nil {
		t.Fatalf("reserve the shared request-id lock: %v", err)
	}
	inputs := []StdioMCPPackageInput{
		{ServerID: firstServer.ID, Revision: "1.0.0", Archive: firstArchive, RequestID: requestID},
		{ServerID: secondServer.ID, Revision: "2.0.0", Archive: secondArchive, RequestID: requestID},
	}
	results := make(chan error, len(inputs))
	start := make(chan struct{})
	for _, input := range inputs {
		go func(input StdioMCPPackageInput) {
			<-start
			_, importErr := k.TXImportStdioMCPPackage(ctx, scope.company, input)
			results <- importErr
		}(input)
	}
	close(start)
	for range inputs {
		if importErr := <-results; !errors.Is(importErr, core.Conflict) {
			unlock()
			t.Fatalf("import under a held request-id lock error=%v, want %s", importErr, core.Conflict)
		}
	}
	for _, bundle := range []capabilitysource.StdioMCPBundle{firstBundle, secondBundle} {
		if digest := kernelTestBundleFileDigest(t, bundle, "server/main.mjs"); digest != "" {
			if _, statErr := os.Stat(filepath.Join(k.root, scope.company, digest)); !errors.Is(statErr, os.ErrNotExist) {
				unlock()
				t.Fatalf("request-id conflict wrote a CAS blob before lock acquisition: stat err=%v", statErr)
			}
		}
	}
	unlock()
	if _, err = k.TXImportStdioMCPPackage(ctx, scope.company, inputs[0]); err != nil {
		t.Fatalf("import after request-id lock release: %v", err)
	}
	if _, err = k.TXImportStdioMCPPackage(ctx, scope.company, inputs[1]); !errors.Is(err, core.Conflict) {
		t.Fatalf("reused request-id import error=%v, want %s", err, core.Conflict)
	}
	losingDigest := kernelTestBundleFileDigest(t, secondBundle, "server/main.mjs")
	if _, err = os.Stat(filepath.Join(k.root, scope.company, losingDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("same request-id loser left an unreferenced CAS blob: stat err=%v", err)
	}
}

func TestCapabilitySkillAndMCPImportsShareRequestIDLockBeforeCASWrites(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	requestID := "shared-capability-import-request"
	skillArchive := capabilitySkillZIPForTest(t, "shared-request-skill", "skill-only CAS bytes for cross-type request lock\n")
	skillBundle, err := capabilitysource.PrepareReadOnlySkillBundle(skillArchive)
	if err != nil {
		t.Fatal(err)
	}
	skillOnlyDigest := ""
	for _, file := range skillBundle.Files {
		if file.RelativePath == "references/guide.md" {
			skillOnlyDigest = file.ContentSHA256
		}
	}
	if skillOnlyDigest == "" {
		t.Fatal("Skill test fixture has no unique reference blob")
	}
	mcpBundle := prepareKernelTestStdioMCPBundle(t, "mcp-only CAS bytes for cross-type request lock")
	mcpArchive := kernelTestStdioMCPArchive(t, mcpBundle)
	mcpOnlyDigest := kernelTestBundleFileDigest(t, mcpBundle, "server/main.mjs")
	unlock, err := k.lockCapabilityMCPPackageImport(ctx, scope.company, requestID, "mcp-lock-holder", "1.0.0")
	if err != nil {
		t.Fatalf("reserve the MCP side of the shared request lock: %v", err)
	}
	if _, err = k.TXImportReadOnlySkillPackage(ctx, scope.company, ReadOnlySkillPackageInput{Revision: "1.0.0", Archive: skillArchive}, requestID); !errors.Is(err, core.Conflict) {
		unlock()
		t.Fatalf("Skill import under shared request lock error=%v, want %s", err, core.Conflict)
	}
	unlock()
	if _, err = os.Stat(filepath.Join(k.root, scope.company, skillOnlyDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Skill import wrote CAS before acquiring the shared request lock: stat err=%v", err)
	}
	if _, err = k.TXImportReadOnlySkillPackage(ctx, scope.company, ReadOnlySkillPackageInput{Revision: "1.0.0", Archive: skillArchive}, requestID); err != nil {
		t.Fatalf("import Skill after releasing the shared request lock: %v", err)
	}
	if _, err = k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{Revision: "1.0.0", Archive: mcpArchive, RequestID: requestID}); !errors.Is(err, core.Conflict) {
		t.Fatalf("MCP import reusing a Skill receipt error=%v, want %s", err, core.Conflict)
	}
	if _, err = os.Stat(filepath.Join(k.root, scope.company, mcpOnlyDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("MCP import wrote CAS before rejecting the cross-type request replay: stat err=%v", err)
	}
}

func TestCapabilitySourceRequestLockBlocksUncoordinatedTXWrite(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	requestID := "shared-txwrite-request-lock"
	unlock, err := k.lockCapabilityMCPPackageImport(ctx, scope.company, requestID, "mcp-package-lock-holder", "1.0.0")
	if err != nil {
		t.Fatalf("reserve a CAS-import request ID: %v", err)
	}
	command := "runtime/node.exe"
	args := []string{"server/main.mjs"}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		unlock()
		t.Fatal(err)
	}
	descriptorDigest, err := canonicalMCPDescriptorDigest("shared-request-registration", "stdio", command, argsJSON)
	if err != nil {
		unlock()
		t.Fatal(err)
	}
	const competingWrites = 16
	results := make(chan error, competingWrites)
	start := make(chan struct{})
	for range competingWrites {
		go func() {
			<-start
			callContext, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_, registrationErr := k.TXRegisterMCPServerDefinitionCommand(callContext, scope.company, MCPServerDefinitionInput{
				ID: "shared-request-registration", Name: "shared-request-registration", Transport: "stdio",
				Command: &command, Args: args, DescriptorDigest: descriptorDigest,
			}, requestID)
			results <- registrationErr
		}()
	}
	close(start)
	for range competingWrites {
		if registrationErr := <-results; !errors.Is(registrationErr, core.Conflict) {
			unlock()
			t.Fatalf("competing TXWrite error=%v, want immediate request-ID conflict", registrationErr)
		}
	}
	unlock()
	var registered bool
	if err = k.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mcp_server_definitions WHERE company_id=$1 AND id=$2)`, scope.company, "shared-request-registration").Scan(&registered); err != nil {
		t.Fatal(err)
	}
	if registered {
		t.Fatal("timed-out competing TXWrite committed while a source import held its request ID")
	}
	registeredServer, err := k.TXRegisterMCPServerDefinitionCommand(ctx, scope.company, MCPServerDefinitionInput{
		ID: "shared-request-registration", Name: "shared-request-registration", Transport: "stdio",
		Command: &command, Args: args, DescriptorDigest: descriptorDigest,
	}, requestID)
	if err != nil {
		t.Fatalf("TXWrite did not continue after the source import released the request ID: %v", err)
	}
	unlockReplay, err := k.lockCapabilityMCPPackageImport(ctx, scope.company, requestID, "mcp-replay-lock-holder", "2.0.0")
	if err != nil {
		t.Fatalf("reserve the committed request ID for replay: %v", err)
	}
	replayed, replayErr := k.TXRegisterMCPServerDefinitionCommand(ctx, scope.company, MCPServerDefinitionInput{
		ID: "shared-request-registration", Name: "shared-request-registration", Transport: "stdio",
		Command: &command, Args: args, DescriptorDigest: descriptorDigest,
	}, requestID)
	unlockReplay()
	if replayErr != nil || replayed.ID != registeredServer.ID {
		t.Fatalf("committed receipt replay under a held source lock=(%+v,%v), want original registration %+v", replayed, replayErr, registeredServer)
	}
}

func kernelTestBundleFileDigest(t *testing.T, bundle capabilitysource.StdioMCPBundle, relativePath string) string {
	t.Helper()
	for _, file := range bundle.Manifest.Files {
		if file.RelativePath == relativePath {
			return file.ContentSHA256
		}
	}
	t.Fatalf("test fixture is missing package path %q", relativePath)
	return ""
}

func TestStdioMCPPackageImportRequiresDescriptorAndManifestToMatch(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, _ := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	bundle := prepareKernelTestStdioMCPBundle(t, "server-data-v1")
	wrongCommand := "different/server.exe"
	wrongArgs, _ := json.Marshal([]string{"server/main.mjs"})
	digest, err := canonicalMCPDescriptorDigest(bundle.Manifest.Name, "stdio", wrongCommand, wrongArgs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRegisterMCPServerDefinitionCommand(ctx, scope.company, MCPServerDefinitionInput{
		ID: "mcp-package-mismatch-version", Name: bundle.Manifest.Name, Transport: "stdio",
		Command: &wrongCommand, Args: bundle.Manifest.Args, DescriptorDigest: digest,
	}, "mcp-package-wrong-version"); err != nil {
		t.Fatal(err)
	}
	_, err = k.TXImportStdioMCPPackage(ctx, scope.company, StdioMCPPackageInput{
		ServerID: "mcp-package-mismatch-version", Revision: "1.0.0", Archive: kernelTestStdioMCPArchive(t, bundle), RequestID: "mcp-package-wrong-import",
	})
	if !errors.Is(err, core.Denied) {
		t.Fatalf("package import with descriptor mismatch error=%v, want %s", err, core.Denied)
	}
}

func registerKernelTestStdioMCP(t *testing.T, k *Kernel, scope Scope, bundle capabilitysource.StdioMCPBundle, id, requestID string) MCPServerDefinition {
	t.Helper()
	argsJSON, err := json.Marshal(bundle.Manifest.Args)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := canonicalMCPDescriptorDigest(bundle.Manifest.Name, "stdio", bundle.Manifest.Command, argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	server, err := k.TXRegisterMCPServerDefinitionCommand(context.Background(), scope.company, MCPServerDefinitionInput{
		ID: id, Name: bundle.Manifest.Name, Transport: "stdio", Command: &bundle.Manifest.Command,
		Args: bundle.Manifest.Args, DescriptorDigest: digest,
	}, requestID)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func prepareKernelTestStdioMCPBundle(t *testing.T, content string) capabilitysource.StdioMCPBundle {
	t.Helper()
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`)
	archive := kernelTestStdioMCPArchiveFiles(t, manifest, map[string][]byte{
		"runtime/node.exe": []byte("node-runtime-fixture"),
		"server/main.mjs":  []byte("export const data = '" + content + "';\n"),
	})
	bundle, err := capabilitysource.PrepareStdioMCPBundle(archive)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func kernelTestStdioMCPArchive(t *testing.T, bundle capabilitysource.StdioMCPBundle) []byte {
	t.Helper()
	files := make(map[string][]byte, len(bundle.Files))
	for _, file := range bundle.Files {
		files[file.RelativePath] = file.Content
	}
	declaration, err := json.Marshal(struct {
		SchemaVersion string   `json:"schemaVersion"`
		Name          string   `json:"name"`
		ServerName    string   `json:"serverName"`
		ServerVersion string   `json:"serverVersion"`
		Command       string   `json:"command"`
		EntryPoint    string   `json:"entryPoint"`
		Args          []string `json:"args"`
	}{
		SchemaVersion: bundle.Manifest.SchemaVersion,
		Name:          bundle.Manifest.Name,
		ServerName:    bundle.Manifest.ServerName,
		ServerVersion: bundle.Manifest.ServerVersion,
		Command:       bundle.Manifest.Command,
		EntryPoint:    bundle.Manifest.EntryPoint,
		Args:          bundle.Manifest.Args,
	})
	if err != nil {
		t.Fatal(err)
	}
	return kernelTestStdioMCPArchiveFiles(t, declaration, files)
}

func kernelTestStdioMCPArchiveFiles(t *testing.T, manifest []byte, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	writer, err := archive.Create(capabilitysource.StdioMCPBundleManifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
