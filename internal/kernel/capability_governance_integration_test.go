// pattern: Imperative Shell
package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
	"polis/internal/runner"
)

func TestCapabilityApprovalBindingAndRevocationStayVersionPinned(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("capability-governance-%d", time.Now().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	skill, err := k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{
		Revision: "1.0.0", Archive: capabilitySkillZIPForTest(t, "approved-reference-skill"),
	}, "capability-skill-import")
	if err != nil {
		t.Fatal(err)
	}
	var importedManifest capabilitysource.ReadOnlySkillManifest
	if err = json.Unmarshal(skill.Manifest, &importedManifest); err != nil || !importedManifest.ReadOnly || importedManifest.SchemaVersion != capabilitysource.ReadOnlySkillBundleSchema || len(importedManifest.Files) != 2 || skill.SourceRef != readOnlySkillSourceRef {
		t.Fatalf("Skill revision did not bind the verified source manifest: %+v error=%v", importedManifest, err)
	}
	replayedSkill, err := k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{
		Revision: "1.0.0", Archive: capabilitySkillZIPForTest(t, "approved-reference-skill"),
	}, "capability-skill-import")
	if err != nil || replayedSkill.ID != skill.ID {
		t.Fatalf("identical Skill import replay=(%+v,%v), want the original revision", replayedSkill, err)
	}
	duplicateArchive := capabilitySkillZIPForTest(t, "approved-reference-skill", "Different source bytes for the same immutable revision.\n")
	duplicateBundle, err := capabilitysource.PrepareReadOnlySkillBundle(duplicateArchive)
	if err != nil {
		t.Fatal(err)
	}
	var duplicateOnlyDigest string
	for _, file := range duplicateBundle.Files {
		if file.RelativePath == "references/guide.md" {
			duplicateOnlyDigest = file.ContentSHA256
		}
	}
	if duplicateOnlyDigest == "" {
		t.Fatal("alternate Skill fixture lost its reference file")
	}
	if _, err = k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{Revision: "1.0.0", Archive: duplicateArchive}, "capability-skill-import"); !errors.Is(err, core.Conflict) {
		t.Fatalf("changed Skill import replay error=%v, want %s", err, core.Conflict)
	}
	if _, err = os.Stat(filepath.Join(k.root, companyID, duplicateOnlyDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("changed request replay wrote CAS before rejecting its fingerprint: stat error=%v", err)
	}
	if _, err = k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{Revision: "1.0.0", Archive: duplicateArchive}, "capability-skill-duplicate-revision"); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate Skill package revision error=%v, want %s", err, core.Conflict)
	}
	if _, err = os.Stat(filepath.Join(k.root, companyID, duplicateOnlyDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("predictable duplicate revision left an unreferenced CAS blob: stat error=%v", err)
	}
	qualification, err := k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "skill", CapabilityID: skill.ID, RequestID: "capability-skill-qualification"})
	if err != nil || qualification.Status != "metadata_verified" {
		t.Fatalf("read-only Skill metadata qualification = %+v, %v", qualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Decision: "approved", Rationale: "reviewed fixed read-only manifest", RequestID: "capability-skill-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, companyID, EmployeeCapabilityBindingInput{
		EmployeeID: core.EmployeeBackendID, CapabilityKind: "skill", CapabilityID: skill.ID,
		QualificationID: qualification.QualificationID, Reason: "backend source review", RequestID: "capability-skill-bind",
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Bindings) != 1 || catalog.Bindings[0].State != "bound" || catalog.Bindings[0].VersionDigest != skill.ContentDigest || catalog.Bindings[0].ExecutionStatus != "runtime_unqualified" {
		t.Fatalf("Skill binding projection does not pin its digest or runtime gate: %+v", catalog.Bindings)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Decision: "revoked", Rationale: "test revocation barrier", RequestID: "capability-skill-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err = k.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}

	forgedDigest := sha256.Sum256([]byte("metadata-only skill candidate"))
	forged, err := k.TXImportSkillRevisionCommand(ctx, companyID, SkillRevisionInput{
		ID: "skill-forged-source-manifest", PublisherScope: "company", PackageID: "forged-source", Revision: "1.0.0",
		DisplayName: "Metadata-only declaration", SourceRef: readOnlySkillSourceRef,
		ContentDigest: hex.EncodeToString(forgedDigest[:]), Manifest: json.RawMessage(`{"readOnly":true,"schemaVersion":"polis-read-only-skill@1"}`),
	}, "capability-skill-forged-source-import")
	if err != nil {
		t.Fatal(err)
	}
	forgedQualification, err := k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "skill", CapabilityID: forged.ID, RequestID: "capability-skill-forged-source-qualification"})
	if err != nil || forgedQualification.Status != "needs_external_qualification" {
		t.Fatalf("caller-supplied read-only/source declaration was trusted: %+v, %v", forgedQualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: forged.ID, QualificationID: forgedQualification.QualificationID,
		Decision: "approved", Rationale: "metadata-only declarations cannot qualify", RequestID: "capability-skill-forged-source-approve",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("metadata-only source approval error = %v, want %s", err, core.Denied)
	}

	missingSource, err := k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{
		Revision: "1.0.0", Archive: capabilitySkillZIPForTest(t, "missing-cas-skill"),
	}, "capability-skill-missing-cas-import")
	if err != nil {
		t.Fatal(err)
	}
	var missingManifest capabilitysource.ReadOnlySkillManifest
	if err = json.Unmarshal(missingSource.Manifest, &missingManifest); err != nil || len(missingManifest.Files) == 0 {
		t.Fatalf("imported source manifest is invalid: %+v error=%v", missingManifest, err)
	}
	if err = os.Remove(filepath.Join(k.root, companyID, missingManifest.Files[0].ContentSHA256)); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "skill", CapabilityID: missingSource.ID, RequestID: "capability-skill-missing-cas-qualification"}); !errors.Is(err, core.Integrity) {
		t.Fatalf("missing Skill CAS source qualification error = %v, want %s", err, core.Integrity)
	}
	if len(catalog.Bindings) != 1 || catalog.Bindings[0].State != "revoked" || len(catalog.Decisions) != 2 || catalog.Decisions[0].Actor != "local-owner" {
		t.Fatalf("revocation did not append the binding barrier and human decision: bindings=%+v decisions=%+v", catalog.Bindings, catalog.Decisions)
	}

	mutableDigest := sha256.Sum256([]byte("skill without read-only declaration"))
	mutable, err := k.TXImportSkillRevisionCommand(ctx, companyID, SkillRevisionInput{
		ID: "skill-needs-review", PublisherScope: "company", PackageID: "mutable-reference", Revision: "1.0.0",
		DisplayName: "Skill requiring review", SourceRef: "catalog://reference/mutable@1.0.0",
		ContentDigest: hex.EncodeToString(mutableDigest[:]), Manifest: json.RawMessage(`{"readOnly":false}`),
	}, "capability-skill-mutable-import")
	if err != nil {
		t.Fatal(err)
	}
	mutableQualification, err := k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "skill", CapabilityID: mutable.ID, RequestID: "capability-skill-mutable-qualification"})
	if err != nil || mutableQualification.Status != "needs_external_qualification" {
		t.Fatalf("mutable Skill was not held for additional qualification: %+v, %v", mutableQualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: mutable.ID, QualificationID: mutableQualification.QualificationID,
		Decision: "approved", Rationale: "must not pass the metadata-only gate", RequestID: "capability-skill-mutable-approve",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("mutable Skill approval error = %v, want %s", err, core.Denied)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, companyID, EmployeeCapabilityBindingInput{
		EmployeeID: "emp-review", CapabilityKind: "skill", CapabilityID: mutable.ID,
		QualificationID: mutableQualification.QualificationID, Reason: "must remain unbound", RequestID: "capability-skill-mutable-bind",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("unapproved Skill binding error = %v, want %s", err, core.Denied)
	}
}

func capabilitySkillZIPForTest(t *testing.T, name string, referenceText ...string) []byte {
	t.Helper()
	guide := "# Guide\nRead-only supporting material.\n"
	if len(referenceText) > 0 {
		guide = referenceText[0]
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := []struct{ path, content string }{
		{path: "SKILL.md", content: "---\nname: " + name + "\ndescription: A read-only reference skill.\n---\nUse the references as sources; never execute code.\n"},
		{path: "references/guide.md", content: guide},
	}
	for _, file := range files {
		entry, err := writer.Create(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestCapabilitySkillConcurrentSameRevisionImportPreflightsCASWrites(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("capability-skill-concurrent-%d", time.Now().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	archives := [][]byte{
		capabilitySkillZIPForTest(t, "concurrent-skill", "reference from contender A\n"),
		capabilitySkillZIPForTest(t, "concurrent-skill", "reference from contender B\n"),
	}
	var candidateDigests [2]string
	for index, archive := range archives {
		bundle, prepareErr := capabilitysource.PrepareReadOnlySkillBundle(archive)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		for _, file := range bundle.Manifest.Files {
			if file.RelativePath == "references/guide.md" {
				candidateDigests[index] = file.ContentSHA256
			}
		}
		if candidateDigests[index] == "" {
			t.Fatalf("candidate %d has no reference file digest", index)
		}
	}
	type result struct {
		item SkillRevision
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, len(archives))
	var group sync.WaitGroup
	for index, archive := range archives {
		group.Add(1)
		go func(index int, archive []byte) {
			defer group.Done()
			<-start
			item, importErr := k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{Revision: "1.0.0", Archive: archive}, fmt.Sprintf("concurrent-skill-import-%d", index))
			results <- result{item: item, err: importErr}
		}(index, archive)
	}
	close(start)
	group.Wait()
	close(results)
	wins, conflicts := 0, 0
	winnerDigest := ""
	for result := range results {
		if result.err == nil {
			wins++
			var manifest capabilitysource.ReadOnlySkillManifest
			if err = json.Unmarshal(result.item.Manifest, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, file := range manifest.Files {
				if file.RelativePath == "references/guide.md" {
					winnerDigest = file.ContentSHA256
				}
			}
		} else if errors.Is(result.err, core.Conflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent import returned unexpected error: %v", result.err)
		}
	}
	if wins != 1 || conflicts != 1 || winnerDigest == "" {
		t.Fatalf("same package/revision import results: wins=%d conflicts=%d winnerDigest=%q", wins, conflicts, winnerDigest)
	}
	for _, candidateDigest := range candidateDigests {
		_, statErr := os.Stat(filepath.Join(k.root, companyID, candidateDigest))
		if candidateDigest == winnerDigest {
			if statErr != nil {
				t.Fatalf("committed source CAS blob is missing: %v", statErr)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("losing concurrent import left an unreferenced CAS blob: %v", statErr)
		}
	}
}

func TestCapabilitySkillDistinctRevisionImportsDoNotExhaustDatabasePool(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("capability-skill-distinct-concurrent-%d", time.Now().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	const count = 12
	archives := make([][]byte, count)
	for index := range archives {
		archives[index] = capabilitySkillZIPForTest(t, fmt.Sprintf("parallel-skill-%02d", index))
	}
	type result struct{ err error }
	start := make(chan struct{})
	results := make(chan result, count)
	for index := 0; index < count; index++ {
		index := index
		go func() {
			<-start
			_, importErr := k.TXImportReadOnlySkillPackage(ctx, companyID, ReadOnlySkillPackageInput{
				Revision: "1.0.0",
				Archive:  archives[index],
			}, fmt.Sprintf("parallel-skill-import-%02d", index))
			results <- result{err: importErr}
		}()
	}
	close(start)
	for completed := 0; completed < count; completed++ {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatalf("distinct concurrent import %d/%d failed: %v", completed+1, count, result.err)
			}
		case <-ctx.Done():
			t.Fatalf("distinct concurrent imports did not finish before deadline: %v", ctx.Err())
		}
	}
}

func TestCapabilitySkillLoadRequiresCurrentEmployeeBindingAndFlowsThroughHandover(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	skill, err := k.TXImportReadOnlySkillPackage(context.Background(), scope.company, ReadOnlySkillPackageInput{
		Revision: "1.0.0", Archive: capabilitySkillZIPForTest(t, "loadable-reference-skill", "# Evidence guide\nUse the exact cited revision.\n"),
	}, "skill-load-import")
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := k.TXQualifyCapability(context.Background(), scope.company, CapabilityQualificationInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, RequestID: "skill-load-qualification",
	})
	if err != nil || qualification.Status != "metadata_verified" {
		t.Fatalf("Skill source qualification=(%+v,%v)", qualification, err)
	}
	if _, err = k.TXDecideCapability(context.Background(), scope.company, CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Decision: "approved", Rationale: "approved static reference material", RequestID: "skill-load-approval",
	}); err != nil {
		t.Fatal(err)
	}
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)
	tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true, SkillLoadSurface: true}
	args, _ := json.Marshal(map[string]string{"skill_id": skill.ID, "relative_path": "SKILL.md"})
	if result := tools.Call(context.Background(), "skills_load", "skill-load-unbound", args); result.Error != core.Denied.Error() {
		t.Fatalf("unbound employee loaded Skill: %+v", result)
	}
	if _, err = k.TXBindEmployeeCapability(context.Background(), scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: binding.employee, CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Reason: "fixed source review reference", RequestID: "skill-load-bind",
	}); err != nil {
		t.Fatal(err)
	}
	current, err := k.Handover(context.Background(), binding)
	if err != nil || len(current.SkillCatalog) != 1 || current.SkillCatalog[0].SkillID != skill.ID || len(current.SkillCatalog[0].References) != 2 {
		t.Fatalf("permission-filtered Skill catalog=(%+v,%v)", current.SkillCatalog, err)
	}
	legacyTools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	legacyCurrent := legacyTools.Call(context.Background(), "work_current", "skill-load-legacy-context", []byte(`{}`))
	legacyBundle, legacyOK := legacyCurrent.Data.(HandoverBundle)
	if legacyCurrent.Error != "" || !legacyOK || len(legacyBundle.SkillCatalog) != 0 || len(legacyBundle.SkillLoads) != 0 {
		t.Fatalf("qualified-v4 work_current output changed: %+v", legacyCurrent)
	}
	v5Current := tools.Call(context.Background(), "work_current", "skill-load-v5-context", []byte(`{}`))
	v5Bundle, v5OK := v5Current.Data.(HandoverBundle)
	if v5Current.Error != "" || !v5OK || len(v5Bundle.SkillCatalog) != 1 || v5Bundle.SkillCatalog[0].SkillID != skill.ID {
		t.Fatalf("v5 work_current omitted its bound Skill catalog: %+v", v5Current)
	}
	args, _ = json.Marshal(map[string]string{"skill_id": skill.ID, "relative_path": "references/guide.md"})
	loaded := tools.Call(context.Background(), "skills_load", "skill-load-reference", args)
	document, ok := loaded.Data.(SkillLoadDocument)
	if loaded.Error != "" || loaded.Receipt == nil || !ok || document.Content != "# Evidence guide\nUse the exact cited revision.\n" || document.VersionDigest != skill.ContentDigest || document.ContentBoundary != "approved_static_text_no_additional_permissions" {
		t.Fatalf("approved bound Skill reference load=(%+v, type=%T)", loaded, loaded.Data)
	}
	secondLoad := tools.Call(context.Background(), "skills_load", "skill-load-reference-again", args)
	if secondLoad.Error != "" || secondLoad.Receipt == nil || secondLoad.Receipt.ID != loaded.Receipt.ID {
		t.Fatalf("same-session reference load did not return its prior reference: first=%+v second=%+v", loaded, secondLoad)
	}
	replayed := tools.Call(context.Background(), "skills_load", "skill-load-reference", args)
	if replayed.Error != "" || replayed.Receipt == nil || replayed.Receipt.ID != loaded.Receipt.ID {
		t.Fatalf("same native call replay changed receipt: first=%+v replay=%+v", loaded, replayed)
	}
	current, err = k.Handover(context.Background(), binding)
	if err != nil || len(current.SkillLoads) != 1 || current.SkillLoads[0].VersionDigest != skill.ContentDigest || current.SkillLoads[0].RelativePath != "references/guide.md" {
		t.Fatalf("Task handover omitted exact Skill usage=(%+v,%v)", current.SkillLoads, err)
	}
	if _, err = k.TXRevokeEmployeeCapability(context.Background(), scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: binding.employee, CapabilityKind: "skill", CapabilityID: skill.ID,
		Reason: "remove the employee grant", RequestID: "skill-load-unbind",
	}); err != nil {
		t.Fatal(err)
	}
	if result := tools.Call(context.Background(), "skills_load", "skill-load-after-revoke", args); result.Error != core.Denied.Error() {
		t.Fatalf("revoked employee binding allowed a new Skill load: %+v", result)
	}
	if result := tools.Call(context.Background(), "skills_load", "skill-load-reference", args); result.Error != core.Denied.Error() {
		t.Fatalf("idempotent replay exposed Skill content after revocation: %+v", result)
	}
	current, err = k.Handover(context.Background(), binding)
	if err != nil || len(current.SkillCatalog) != 0 || len(current.SkillLoads) != 1 {
		t.Fatalf("revoked Skill remains available or usage is missing from handover: catalog=%+v loads=%+v err=%v", current.SkillCatalog, current.SkillLoads, err)
	}
}

func TestControlledStdioMCPApprovalAndEmployeeUnbindAreAudited(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("capability-mcp-governance-%d", time.Now().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	args := []string{"--stdio"}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := canonicalMCPDescriptorDigest("Reference local MCP", "stdio", "node", argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	command := "node"
	mcp, err := k.TXRegisterMCPServerDefinitionCommand(ctx, companyID, MCPServerDefinitionInput{
		ID: "reference-mcp", Name: "Reference local MCP", Transport: "stdio", Command: &command, Args: args, DescriptorDigest: digest,
	}, "capability-mcp-register")
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "mcp", CapabilityID: mcp.ID, RequestID: "capability-mcp-qualification"})
	if err != nil || qualification.Status != "metadata_verified" {
		t.Fatalf("stdio descriptor qualification = %+v, %v", qualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: mcp.ID, QualificationID: qualification.QualificationID,
		Decision: "approved", Rationale: "fixed descriptor reviewed", RequestID: "capability-mcp-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, companyID, EmployeeCapabilityBindingInput{
		EmployeeID: "emp-frontend", CapabilityKind: "mcp", CapabilityID: mcp.ID,
		QualificationID: qualification.QualificationID, Reason: "frontend read-only reference", RequestID: "capability-mcp-bind",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRevokeEmployeeCapability(ctx, companyID, EmployeeCapabilityBindingInput{
		EmployeeID: "emp-frontend", CapabilityKind: "mcp", CapabilityID: mcp.ID,
		Reason: "employee assignment ended", RequestID: "capability-mcp-unbind",
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Bindings) != 1 || catalog.Bindings[0].State != "revoked" || catalog.Bindings[0].ExecutionStatus != "runtime_unqualified" {
		t.Fatalf("MCP employee unbind projection = %+v", catalog.Bindings)
	}
}

func TestStdioMCPRuntimeQualificationRejectsUnsealedObservation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	command := "node"
	args := []string{}
	argsJSON, _ := json.Marshal(args)
	descriptorDigest, err := canonicalMCPDescriptorDigest("Unobserved MCP", "stdio", command, argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := k.TXRegisterMCPServerDefinitionCommand(ctx, scope.company, MCPServerDefinitionInput{
		ID: "unobserved-mcp", Name: "Unobserved MCP", Transport: "stdio", Command: &command, Args: args, DescriptorDigest: descriptorDigest,
	}, "unobserved-mcp-register")
	if err != nil {
		t.Fatal(err)
	}
	metadataQualification, err := k.TXQualifyCapability(ctx, scope.company, CapabilityQualificationInput{CapabilityKind: "mcp", CapabilityID: mcp.ID, RequestID: "unobserved-mcp-metadata-qualify"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: mcp.ID, QualificationID: metadataQualification.QualificationID,
		Decision: "approved", Rationale: "fixed descriptor only; runtime still unobserved", RequestID: "unobserved-mcp-metadata-approve",
	}); err != nil {
		t.Fatal(err)
	}
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)
	if _, err = k.TXBindEmployeeCapability(ctx, scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: binding.employee, CapabilityKind: "mcp", CapabilityID: mcp.ID,
		QualificationID: metadataQualification.QualificationID, Reason: "pin reviewed metadata revision", RequestID: "unobserved-mcp-bind",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = k.TXRecordStdioMCPRuntimeQualification(ctx, scope.company, StdioMCPRuntimeObservationInput{
		CapabilityID: mcp.ID, CapabilityQualificationID: metadataQualification.QualificationID,
		Observation: mcpowner.RuntimeObservation{}, RequestID: "unsealed-runtime-observation",
	})
	if !errors.Is(err, core.Denied) {
		t.Fatalf("forged or absent process-owner observation error=%v, want %s", err, core.Denied)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", strings.Repeat("a", 64)); !errors.Is(err, core.Denied) {
		t.Fatalf("MCP call without a sealed runtime observation was authorized: %v", err)
	}
	if err = k.TXApproveStdioMCPRuntimeQualification(ctx, scope.company, StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: "missing-runtime-qualification", Rationale: "must not authorize absent evidence", RequestID: "approve-missing-runtime-qualification",
	}); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("approval without an observation error=%v, want %s", err, core.OutOfScope)
	}
}
func TestStdioMCPRuntimeAuthorizationRequiresCurrentApprovalBindingAndSchema(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	ctx := context.Background()
	workspace := t.TempDir()
	packageRoot := filepath.Join(workspace, "mcp-package")
	if err := os.Mkdir(packageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(packageRoot, "fixture-mcp.exe")
	executableBytes := []byte("fixture-only MCP executable bytes; never launched")
	if err := os.WriteFile(executable, executableBytes, 0600); err != nil {
		t.Fatal(err)
	}
	toolSchema := json.RawMessage(`[{"name":"lookup","description":"Read one fixture item.","inputSchema":{"type":"object","properties":{"key":{"type":"string","minLength":1,"maxLength":32}},"required":["key"],"additionalProperties":false}}]`)
	var tools []mcptransport.StdioToolDefinition
	if err := json.Unmarshal(toolSchema, &tools); err != nil {
		t.Fatal(err)
	}
	toolSchemaDigest, err := mcptransport.StdioToolSchemaDigest(toolSchema)
	if err != nil {
		t.Fatal(err)
	}
	processSpec := mcpowner.ProcessSpec{
		Launch: runner.AppContainerLaunchSpec{
			ID: "fixture-runtime-qualification", WorkspaceRoot: workspace, Executable: executable,
			Argv: []string{executable}, WorkingDirectory: packageRoot,
			NetworkPolicy: runner.AppContainerNetworkDenyAll,
			Environment:   runner.BuildAppContainerEnvironment(filepath.Join(workspace, "container"), filepath.Join(workspace, "windows")),
		},
		PinnedPackageRoot: packageRoot, EntryPoint: executable,
		PinnedFiles:              []mcpowner.PinnedFile{{Path: executable, SHA256: digestCapabilityBytes(executableBytes)}},
		ExpectedServer:           mcptransport.StdioServerIdentity{Name: "fixture-mcp", Version: "1.0.0"},
		ApprovedToolSchemaSHA256: toolSchemaDigest,
	}
	processSpec.CommandSHA256, err = mcpowner.ComputeCommandSHA256(processSpec)
	if err != nil {
		t.Fatal(err)
	}
	processSpec.PackageManifestSHA256, err = mcpowner.ComputePackageManifestSHA256(packageRoot, processSpec.PinnedFiles)
	if err != nil {
		t.Fatal(err)
	}
	command := executable
	args := []string{}
	argsJSON, _ := json.Marshal(args)
	descriptorDigest, err := canonicalMCPDescriptorDigest("Fixture runtime MCP", "stdio", command, argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := k.TXRegisterMCPServerDefinitionCommand(ctx, scope.company, MCPServerDefinitionInput{
		ID: "fixture-runtime-mcp", Name: "Fixture runtime MCP", Transport: "stdio", Command: &command, Args: args, DescriptorDigest: descriptorDigest,
	}, "fixture-runtime-mcp-register")
	if err != nil {
		t.Fatal(err)
	}
	metadataQualification, err := k.TXQualifyCapability(ctx, scope.company, CapabilityQualificationInput{CapabilityKind: "mcp", CapabilityID: mcp.ID, RequestID: "fixture-runtime-mcp-metadata-qualification"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: mcp.ID, QualificationID: metadataQualification.QualificationID,
		Decision: "approved", Rationale: "fixed fixture descriptor reviewed", RequestID: "fixture-runtime-mcp-metadata-approval",
	}); err != nil {
		t.Fatal(err)
	}
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	workerStopped := false
	stopWorker := func() {
		if !workerStopped {
			stopProductDeliveryWorker(t, k, binding, process)
			workerStopped = true
		}
	}
	defer stopWorker()

	processSpecJSON, err := json.Marshal(processSpec)
	if err != nil {
		t.Fatal(err)
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	const runtimeQualificationID = "fixture-runtime-qualification"
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'windows',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'fixture-runtime-record')`,
		scope.company, runtimeQualificationID, mcp.ID, metadataQualification.QualificationID, descriptorDigest,
		mcptransport.StdioProfile20260728, stdioMCPHostProfileWindowsAppContainer, processSpec.CommandSHA256,
		processSpec.PackageManifestSHA256, processSpec.ExpectedServer.Name, processSpec.ExpectedServer.Version,
		mcptransport.ProtocolVersion20260728, toolSchemaDigest, toolsJSON, processSpecJSON, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,'fixture-runtime-event',$2,$3,$4,'observed_unqualified',$5,'test-only fixture; no process launched','fixture-runtime-observation')`, scope.company, runtimeQualificationID, mcp.ID, descriptorDigest, toolSchemaDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", toolSchemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("observed-only MCP runtime was authorized: %v", err)
	}
	if err = k.TXApproveStdioMCPRuntimeQualification(ctx, scope.company, StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: runtimeQualificationID, Rationale: "test fixture runtime approval", RequestID: "fixture-runtime-approval",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", toolSchemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("unbound employee was authorized for a qualified MCP runtime: %v", err)
	}
	if unboundTools, listErr := k.BoundStdioMCPToolSets(ctx, binding); listErr != nil || len(unboundTools) != 0 {
		t.Fatalf("unbound MCP capability leaked into Worker context=(%+v,%v)", unboundTools, listErr)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, scope.company, EmployeeCapabilityBindingInput{
		EmployeeID: binding.employee, CapabilityKind: "mcp", CapabilityID: mcp.ID,
		QualificationID: metadataQualification.QualificationID, Reason: "bind the exact fixture revision", RequestID: "fixture-runtime-binding",
	}); err != nil {
		t.Fatal(err)
	}
	if authorization, authErr := k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", toolSchemaDigest); authErr != nil || authorization.RuntimeQualification.RuntimeQualificationID != runtimeQualificationID {
		t.Fatalf("approved and bound fixture tool authorization=(%+v,%v)", authorization, authErr)
	}
	boundTools, err := k.BoundStdioMCPToolSets(ctx, binding)
	if err != nil || len(boundTools) != 1 || boundTools[0].CapabilityID != mcp.ID || boundTools[0].RuntimeQualificationID != runtimeQualificationID ||
		boundTools[0].ToolSchemaSHA256 != toolSchemaDigest || len(boundTools[0].Tools) != 1 || boundTools[0].Tools[0].Name != "lookup" {
		t.Fatalf("bound MCP tool context=(%+v,%v), want exact approved employee-bound fixture", boundTools, err)
	}
	currentResult := (EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true, ControlledMCPSurface: true, ControlledStdioMCPEnabled: true}).Call(ctx, "work_current", "fixture-mcp-context", json.RawMessage(`{}`))
	current, ok := currentResult.Data.(HandoverBundle)
	if currentResult.Error != "" || !ok || len(current.MCPToolSets) != 1 || current.MCPToolSets[0].CapabilityID != mcp.ID || current.MCPToolSets[0].Tools[0].Name != "lookup" {
		t.Fatalf("worker-facing approved MCP context=(%+v,%t)", currentResult, ok)
	}
	intent, err := k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-completed", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"needle"}`),
	})
	if err != nil || intent.Status != "dispatching" || intent.RuntimeQualificationID != runtimeQualificationID {
		t.Fatalf("MCP call intent=(%+v,%v), want a one-shot dispatch reservation", intent, err)
	}
	if _, duplicateErr := k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-completed", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"needle"}`),
	}); !errors.Is(duplicateErr, core.Conflict) {
		t.Fatalf("duplicate provider call reservation error=%v, want %s", duplicateErr, core.Conflict)
	}
	toolResult := mcptransport.StdioToolResult{
		ToolName: "lookup", ToolSchemaSHA256: toolSchemaDigest, ContentBoundary: mcptransport.StdioResultContentBoundary,
		Content: []mcptransport.StdioTextContent{{Text: "fixture result; untrusted"}},
	}
	if err = k.TXCompleteStdioMCPToolCall(ctx, binding, intent.IntentID, toolResult); err != nil {
		t.Fatalf("record completed MCP tool result: %v", err)
	}
	if err = k.TXCompleteStdioMCPToolCall(ctx, binding, intent.IntentID, toolResult); err != nil {
		t.Fatalf("identical MCP result persistence retry: %v", err)
	}
	completedCall, err := k.GetStdioMCPToolCall(ctx, scope.company, intent.IntentID)
	if err != nil || completedCall.Status != "completed" || completedCall.CreatedAt != intent.CreatedAt || completedCall.Result == nil || len(completedCall.Result.Content) != 1 || completedCall.Result.Content[0].Text != "fixture result; untrusted" {
		t.Fatalf("persisted MCP completion=(%+v,%v)", completedCall, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_tool_call_events SET status='dispatching' WHERE company_id=$1 AND intent_id=$2`, scope.company, intent.IntentID); err == nil {
		t.Fatal("MCP tool-call result history was mutable")
	}
	if _, err = k.pool.Exec(ctx, `DELETE FROM mcp_tool_call_intents WHERE company_id=$1 AND intent_id=$2`, scope.company, intent.IntentID); err == nil {
		t.Fatal("MCP tool-call intent history was deletable")
	}
	wideIntent, err := k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-empty-content-boundary", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"bounded"}`),
	})
	if err != nil {
		t.Fatalf("reserve bounded empty-content result call: %v", err)
	}
	wideResult := toolResult
	wideResult.Content = make([]mcptransport.StdioTextContent, 5800)
	if err = k.TXCompleteStdioMCPToolCall(ctx, binding, wideIntent.IntentID, wideResult); err != nil {
		t.Fatalf("persist bounded compact JSON result with many empty text blocks: %v", err)
	}
	wideCall, err := k.GetStdioMCPToolCall(ctx, scope.company, wideIntent.IntentID)
	if err != nil || wideCall.Status != "completed" || wideCall.Result == nil || len(wideCall.Result.Content) != len(wideResult.Content) {
		t.Fatalf("bounded empty-content result persistence=(%+v,%v)", wideCall, err)
	}
	unknownIntent, err := k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-unknown", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"ambiguous"}`),
	})
	if err != nil {
		t.Fatalf("reserve MCP call with an unresolved result: %v", err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, scope.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.RuntimeQualifications) != 1 || catalog.RuntimeQualifications[0].Status != "qualified" || len(catalog.Bindings) != 1 || catalog.Bindings[0].ExecutionStatus != "runtime_qualified_dispatch_unavailable" {
		t.Fatalf("runtime qualification projection=%+v bindings=%+v", catalog.RuntimeQualifications, catalog.Bindings)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "unlisted_tool", toolSchemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("tool outside the pinned schema was authorized: %v", err)
	}
	if err = k.TXRecordStdioMCPToolSchemaDrift(ctx, scope.company, StdioMCPToolSchemaDriftInput{
		RuntimeQualificationID: runtimeQualificationID, ObservedToolSchemaSHA256: digestCapabilityBytes([]byte("changed fixture schema")),
		RequestID: "fixture-runtime-schema-drift",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", toolSchemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("schema-drifted runtime remained authorized: %v", err)
	}
	if _, err = k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-after-drift", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"no-dispatch"}`),
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("schema-drifted runtime created a dispatch intent: %v", err)
	}
	catalog, err = k.ListCapabilityCatalog(ctx, scope.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.RuntimeQualifications) != 1 || catalog.RuntimeQualifications[0].Status != "schema_drift" || catalog.Bindings[0].ExecutionStatus != "runtime_unqualified" {
		t.Fatalf("schema drift was not projected as disabled: %+v bindings=%+v", catalog.RuntimeQualifications, catalog.Bindings)
	}
	const secondRuntimeQualificationID = "fixture-runtime-qualification-2"
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'windows',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'fixture-runtime-record-2')`,
		scope.company, secondRuntimeQualificationID, mcp.ID, metadataQualification.QualificationID, descriptorDigest,
		mcptransport.StdioProfile20260728, stdioMCPHostProfileWindowsAppContainer, processSpec.CommandSHA256,
		processSpec.PackageManifestSHA256, processSpec.ExpectedServer.Name, processSpec.ExpectedServer.Version,
		mcptransport.ProtocolVersion20260728, toolSchemaDigest, toolsJSON, processSpecJSON, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,'fixture-runtime-event-2',$2,$3,$4,'observed_unqualified',$5,'test-only fixture; no process launched','fixture-runtime-observation-2')`, scope.company, secondRuntimeQualificationID, mcp.ID, descriptorDigest, toolSchemaDigest); err != nil {
		t.Fatal(err)
	}
	if err = k.TXApproveStdioMCPRuntimeQualification(ctx, scope.company, StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: secondRuntimeQualificationID, Rationale: "test fixture runtime approval", RequestID: "fixture-runtime-approval-2",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXDecideCapability(ctx, scope.company, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: mcp.ID, QualificationID: metadataQualification.QualificationID,
		Decision: "revoked", Rationale: "revoke fixture MCP grant", RequestID: "fixture-runtime-capability-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	var revokedRuntimeStatus string
	if err = k.pool.QueryRow(ctx, `SELECT status FROM mcp_runtime_qualification_events WHERE company_id=$1 AND runtime_qualification_id=$2 ORDER BY event_seq DESC LIMIT 1`, scope.company, secondRuntimeQualificationID).Scan(&revokedRuntimeStatus); err != nil || revokedRuntimeStatus != "revoked" {
		t.Fatalf("capability revocation did not revoke the current runtime qualification: status=%q err=%v", revokedRuntimeStatus, err)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", toolSchemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("revoked capability remained authorized: %v", err)
	}
	if revokedTools, listErr := k.BoundStdioMCPToolSets(ctx, binding); listErr != nil || len(revokedTools) != 0 {
		t.Fatalf("revoked MCP capability remained in Worker context=(%+v,%v)", revokedTools, listErr)
	}
	if _, err = k.TXMarkStdioMCPToolCallsUnknownForSession(ctx, scope.company, binding.SessionID(), "worker_interrupted_during_call"); !errors.Is(err, core.Denied) {
		t.Fatalf("active WorkerSession allowed an unknown-outcome sweep: %v", err)
	}
	if err = k.TXBeginStop(ctx, binding); err != nil {
		t.Fatalf("transition interrupted WorkerSession to stopping before process cleanup: %v", err)
	}
	if _, err = k.TXMarkStdioMCPToolCallsUnknownForSession(ctx, scope.company, binding.SessionID(), "worker_interrupted_during_call"); !errors.Is(err, core.Denied) {
		t.Fatalf("stopping WorkerSession allowed an unknown-outcome sweep before process cleanup: %v", err)
	}
	stopProof, err := process.Stop()
	if err != nil {
		t.Fatalf("stop provider process before MCP result reconciliation: %v", err)
	}
	if err = k.TXConfirmStopped(ctx, binding, stopProof); err != nil {
		t.Fatalf("confirm WorkerSession process stop before MCP result reconciliation: %v", err)
	}
	workerStopped = true
	if count, sweepErr := k.TXMarkStdioMCPToolCallsUnknownForSession(ctx, scope.company, binding.SessionID(), "worker_interrupted_during_call"); sweepErr != nil || count != 1 {
		t.Fatalf("mark pending MCP call outcomes unknown count=%d error=%v", count, sweepErr)
	}
	unknownCall, err := k.GetStdioMCPToolCall(ctx, scope.company, unknownIntent.IntentID)
	if err != nil || unknownCall.Status != "outcome_unknown" {
		t.Fatalf("interrupted MCP call=(%+v,%v), want outcome_unknown", unknownCall, err)
	}
	if _, err = k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "provider-call-unknown", CapabilityID: mcp.ID, ToolName: "lookup",
		ToolSchemaSHA256: toolSchemaDigest, Arguments: json.RawMessage(`{"key":"ambiguous"}`),
	}); err == nil {
		t.Fatalf("unknown MCP side effect was replayable: %v", err)
	}
	if err = k.TXCompleteStdioMCPToolCall(ctx, binding, unknownIntent.IntentID, toolResult); err == nil {
		t.Fatalf("late MCP result changed an unknown one-shot outcome: %v", err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_runtime_qualification_records SET process_spec='{}'::jsonb WHERE company_id=$1 AND runtime_qualification_id=$2`, scope.company, runtimeQualificationID); err == nil {
		t.Fatal("runtime qualification process snapshot was mutable")
	}
	if _, err = k.pool.Exec(ctx, `DELETE FROM mcp_runtime_qualification_events WHERE company_id=$1 AND runtime_qualification_id=$2`, scope.company, runtimeQualificationID); err == nil {
		t.Fatal("runtime qualification event history was deletable")
	}
}

func TestStreamableHTTPMCPUsesPinned20260728ProfileWithoutNetworkProbe(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	t.Setenv("POLIS_R05B5_TEST_DSN", dsn)
	ctx := context.Background()
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	companyID := scope.company
	var err error
	endpoint, digest, err := canonicalMCPStreamableHTTPDescriptorDigest("Read-only reference", "https://mcp.example.com/v1/mcp")
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := k.TXRegisterMCPServerDefinitionCommand(ctx, companyID, MCPServerDefinitionInput{
		ID: "http-reference-mcp", Name: "Read-only reference", Transport: "streamable_http", Endpoint: &endpoint, DescriptorDigest: digest,
	}, "http-mcp-register")
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := k.TXQualifyCapability(ctx, companyID, CapabilityQualificationInput{CapabilityKind: "mcp", CapabilityID: mcp.ID, RequestID: "http-mcp-qualification"})
	if err != nil || qualification.Status != "metadata_verified" || qualification.Profile != "streamable_http_mcp_2026_07_28@1" {
		t.Fatalf("Streamable HTTP profile qualification=%+v error=%v", qualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, CapabilityDecisionInput{
		CapabilityKind: "mcp", CapabilityID: mcp.ID, QualificationID: qualification.QualificationID,
		Decision: "approved", Rationale: "fixed endpoint profile reviewed without contacting it", RequestID: "http-mcp-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, companyID, EmployeeCapabilityBindingInput{
		EmployeeID: "emp-backend", CapabilityKind: "mcp", CapabilityID: mcp.ID,
		QualificationID: qualification.QualificationID, Reason: "pin the reviewed endpoint profile", RequestID: "http-mcp-bind",
	}); err != nil {
		t.Fatal(err)
	}
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)
	if target, targetErr := k.StreamableHTTPMCPObservationTarget(ctx, companyID, mcp.ID, qualification.QualificationID); targetErr != nil || target != endpoint {
		t.Fatalf("approved Streamable HTTP observation target=%q error=%v", target, targetErr)
	}
	toolList := json.RawMessage(`{"tools":[{"name":"lookup","description":"Read one bounded fixture item.","inputSchema":{"type":"object","properties":{"key":{"type":"string","minLength":1,"maxLength":32}},"required":["key"],"additionalProperties":false}}]}`)
	runtimeQualification, err := k.TXRecordStreamableHTTPMCPRuntimeQualification(ctx, companyID, StreamableHTTPMCPRuntimeObservationInput{
		CapabilityID: mcp.ID, CapabilityQualificationID: qualification.QualificationID,
		ToolList: toolList, RequestID: "http-mcp-runtime-observe",
	})
	if err != nil || runtimeQualification.Status != "observed_unqualified" || runtimeQualification.Transport != "streamable_http" || runtimeQualification.Endpoint != endpoint {
		t.Fatalf("Streamable HTTP runtime observation=%+v error=%v", runtimeQualification, err)
	}
	if err = k.TXApproveStdioMCPRuntimeQualification(ctx, companyID, StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: runtimeQualification.RuntimeQualificationID, Rationale: "reviewed fixed HTTP endpoint and pinned read-only tool schema", RequestID: "http-mcp-runtime-approve",
	}); err != nil {
		t.Fatal(err)
	}
	withoutEgress, err := k.BoundMCPToolSets(ctx, binding, true, false)
	if err != nil || len(withoutEgress) != 0 {
		t.Fatalf("disabled Streamable HTTP profile leaked into Worker context: tools=%+v error=%v", withoutEgress, err)
	}
	withEgress, err := k.BoundMCPToolSets(ctx, binding, false, true)
	if err != nil || len(withEgress) != 1 || withEgress[0].Transport != "streamable_http" || withEgress[0].Endpoint != endpoint || withEgress[0].Tools[0].Name != "lookup" {
		t.Fatalf("enabled Streamable HTTP profile was absent from Worker context: tools=%+v error=%v", withEgress, err)
	}
	workerContext := (EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true, ControlledMCPSurface: true, StreamableHTTPMCPEnabled: true}).Call(ctx, "work_current", "http-worker-context", json.RawMessage(`{}`))
	workerHandover, handoverOK := workerContext.Data.(HandoverBundle)
	if workerContext.Error != "" || !handoverOK || len(workerHandover.MCPToolSets) != 1 || workerHandover.MCPToolSets[0].Transport != "streamable_http" || workerHandover.MCPToolSets[0].Endpoint != endpoint {
		t.Fatalf("enabled Streamable HTTP capability did not reach the actual Worker context: result=%+v cast=%t", workerContext, handoverOK)
	}
	_, schemaDigest, err := mcptransport.PrepareStreamableHTTPToolList(toolList)
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "lookup", schemaDigest)
	if err != nil || authorization.Transport != "streamable_http" || authorization.Endpoint != endpoint || authorization.RuntimeQualification.RuntimeQualificationID != runtimeQualification.RuntimeQualificationID {
		t.Fatalf("approved Streamable HTTP Worker authorization=%+v error=%v", authorization, err)
	}
	if _, err = k.AuthorizeStdioMCPToolCall(ctx, binding, mcp.ID, "unlisted", schemaDigest); !errors.Is(err, core.Denied) {
		t.Fatalf("unlisted Streamable HTTP tool authorization error=%v", err)
	}
	intent, err := k.TXBeginStdioMCPToolCall(ctx, binding, StdioMCPToolCallIntentInput{
		ProviderCallID: "http-provider-call-1", CapabilityID: mcp.ID, ToolName: "lookup", ToolSchemaSHA256: schemaDigest,
		Arguments: json.RawMessage(`{"key":"fixture"}`),
	})
	if err != nil || intent.Status != "dispatching" || intent.RuntimeQualificationID != runtimeQualification.RuntimeQualificationID {
		t.Fatalf("Streamable HTTP one-shot intent=%+v error=%v", intent, err)
	}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: schemaDigest,
		ContentBoundary: mcptransport.StreamableHTTPResultContentBoundary, Content: []mcptransport.StdioTextContent{{Text: "untrusted HTTP fixture"}}}
	if err = k.TXCompleteStdioMCPToolCall(ctx, binding, intent.IntentID, result); err != nil {
		t.Fatal(err)
	}
	completed, err := k.GetStdioMCPToolCall(ctx, companyID, intent.IntentID)
	if err != nil || completed.Status != "completed" || completed.Result == nil || completed.Result.ContentBoundary != mcptransport.StreamableHTTPResultContentBoundary {
		t.Fatalf("Streamable HTTP result ledger=%+v error=%v", completed, err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Bindings) != 1 || catalog.Bindings[0].State != "bound" || catalog.Bindings[0].ExecutionStatus == "runtime_unqualified" {
		t.Fatalf("qualified Streamable HTTP binding did not expose its runtime status: %+v", catalog.Bindings)
	}
}

func contentDigestString(digest [32]byte) string { return hex.EncodeToString(digest[:]) }
