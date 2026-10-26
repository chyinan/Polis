// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/kernel"
)

const maxDependencyChangeResolverLogBytes = 64 << 10

func (s *Service) MaterializeDependencyChange(ctx context.Context, companyID string, request MaterializeDependencyChangeRequest) (kernel.DependencyChangeMaterializationRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) ||
		!core.ValidID(request.ProposalID) || validateRequestID(request.RequestID) != nil {
		return kernel.DependencyChangeMaterializationRecord{}, core.Malformed
	}
	scope := s.runtime.LocalScope(companyID)
	if existing, err := s.runtime.DependencyChangeMaterializationByRequest(ctx, scope, request.RequestID); err == nil {
		if existing.ProposalID != request.ProposalID {
			return kernel.DependencyChangeMaterializationRecord{}, core.Conflict
		}
		return existing, nil
	} else if !errors.Is(err, core.OutOfScope) {
		return kernel.DependencyChangeMaterializationRecord{}, err
	}
	proposal, err := s.runtime.DependencyChangeProposal(ctx, scope, request.ProposalID)
	if err != nil {
		return kernel.DependencyChangeMaterializationRecord{}, err
	}
	if proposal.State != "approved" {
		return kernel.DependencyChangeMaterializationRecord{}, core.ConflictError{Reason: "dependency change proposal is not approved", CurrentState: proposal.State}
	}
	baseSnapshot, err := s.runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, proposal.BaseRevisionID)
	if err != nil {
		return kernel.DependencyChangeMaterializationRecord{}, err
	}
	if baseSnapshot.Revision.MissionID != proposal.MissionID {
		return kernel.DependencyChangeMaterializationRecord{}, core.OutOfScope
	}
	if baseSnapshot.Revision.ProfileID != environment.WindowsNodeNPMProfile {
		return kernel.DependencyChangeMaterializationRecord{}, core.Denied
	}
	fingerprint, available := s.runtime.CurrentEnvironmentExecutorFingerprint(baseSnapshot.Revision.ProfileID)
	if !available || fingerprint.IsolationPolicySHA256 == "" {
		return kernel.DependencyChangeMaterializationRecord{}, core.Denied
	}
	baseSnapshot.AuthorizedIsolationPolicySHA256 = fingerprint.IsolationPolicySHA256
	_, updatedPackageJSON, err := environment.PrepareDependencyChangeProjectFiles(baseSnapshot.Files, baseSnapshot.Plan, proposal.Proposal)
	if err != nil {
		return kernel.DependencyChangeMaterializationRecord{}, err
	}
	s.preparationMu.RLock()
	preparationExecutor := s.preparationRunner
	s.preparationMu.RUnlock()
	resolver, ok := preparationExecutor.(ProjectDependencyChangeResolver)
	if !ok || resolver == nil {
		return kernel.DependencyChangeMaterializationRecord{}, core.Denied
	}
	attempt, started, err := s.runtime.TXBeginDependencyChangeMaterialization(ctx, scope, request.ProposalID, request.RequestID)
	if err != nil {
		return kernel.DependencyChangeMaterializationRecord{}, err
	}
	if !started {
		return attempt, nil
	}

	resolution, resolveErr := resolver.ResolveProjectDependencyChange(ctx, attempt.MaterializationID, baseSnapshot, proposal.Proposal, updatedPackageJSON)
	resolverLogs := redactRegistryProxyCredentials(resolution.Logs)
	if len(resolverLogs) > maxDependencyChangeResolverLogBytes {
		resolverLogs = resolverLogs[:maxDependencyChangeResolverLogBytes]
	}
	resolverLogsSHA256 := ""
	if len(resolverLogs) > 0 {
		resolverLogsSHA256, err = s.runtime.StoreDependencyChangeMaterializationLogs(ctx, scope, attempt.MaterializationID, resolverLogs)
		if err != nil {
			return s.finishDependencyChangeAttempt(ctx, scope, attempt, "outcome_unknown", "resolver_logs_persist_outcome_unknown")
		}
	}
	if resolveErr != nil {
		var resolverFailure *DependencyChangeResolverError
		if errors.As(resolveErr, &resolverFailure) && (resolverFailure.State == "blocked" || resolverFailure.State == "failed" || resolverFailure.State == "outcome_unknown") {
			return s.finishDependencyChangeAttempt(ctx, scope, attempt, resolverFailure.State, resolverFailure.ReasonCode, resolverLogsSHA256)
		}
		return s.finishDependencyChangeAttempt(ctx, scope, attempt, "outcome_unknown", "resolver_outcome_unknown", resolverLogsSHA256)
	}
	derivedFiles, resolvedPlan, validationErr := environment.ValidateDependencyChangeMaterialization(
		baseSnapshot.Files, baseSnapshot.Plan, proposal.Proposal, updatedPackageJSON, resolution.Lockfile, baseSnapshot.Policy.RegistryHosts,
	)
	if validationErr != nil {
		return s.finishDependencyChangeAttempt(ctx, scope, attempt, "failed", "resolver_output_invalid", resolverLogsSHA256)
	}
	inputFiles := make([]intake.DirectoryInputFile, len(derivedFiles))
	for index, file := range derivedFiles {
		inputFiles[index] = intake.DirectoryInputFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: file.Content}
	}
	directory, err := intake.PrepareDirectorySnapshot(inputFiles)
	if err != nil {
		return s.finishDependencyChangeAttempt(ctx, scope, attempt, "failed", "derived_input_invalid", resolverLogsSHA256)
	}
	inputRequestID := dependencyChangeDerivedRequestID("input", attempt.RequestID)
	derivedInput, err := s.runtime.TXAddMissionInput(ctx, scope, proposal.MissionID, "", inputRequestID, directory.Upload, directory.Archive)
	if err != nil {
		return s.finishDependencyChangeAttempt(ctx, scope, attempt, "outcome_unknown", "derived_input_persist_outcome_unknown", resolverLogsSHA256)
	}
	revisionRequestID := dependencyChangeDerivedRequestID("revision", attempt.RequestID)
	successor, err := s.runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: baseSnapshot.Revision.ProfileID, MissionID: proposal.MissionID,
		SourceInputID: derivedInput.InputID, SourceInputRevision: derivedInput.Revision,
		PolicyManifest: append([]byte(nil), baseSnapshot.Revision.PolicyManifest...), ToolchainSHA256: baseSnapshot.Revision.ToolchainSHA256,
		PackageJSONSHA256: resolvedPlan.PackageJSONSHA256, LockfileSHA256: resolvedPlan.LockfileSHA256,
	}, revisionRequestID)
	if err != nil {
		return s.finishDependencyChangeAttempt(ctx, scope, attempt, "outcome_unknown", "successor_revision_persist_outcome_unknown", resolverLogsSHA256)
	}
	completed, finishErr := s.runtime.TXFinishDependencyChangeMaterialization(ctx, scope, kernel.DependencyChangeMaterializationFinish{
		MaterializationID: attempt.MaterializationID, State: "materialized",
		DerivedInputID: derivedInput.InputID, DerivedInputRevision: derivedInput.Revision,
		SuccessorRevisionID: successor.RevisionID,
		PackageJSONSHA256:   resolvedPlan.PackageJSONSHA256, LockfileSHA256: resolvedPlan.LockfileSHA256, ResolverLogsSHA256: resolverLogsSHA256,
	})
	if finishErr == nil {
		return completed, nil
	}
	unknown, unknownErr := s.finishDependencyChangeAttempt(ctx, scope, attempt, "outcome_unknown", "materialization_finalize_outcome_unknown")
	if unknownErr == nil {
		return unknown, nil
	}
	current, readErr := s.runtime.DependencyChangeMaterialization(ctx, scope, attempt.MaterializationID)
	if readErr == nil && (current.State == "materialized" || current.State == "outcome_unknown" || current.State == "blocked" || current.State == "failed") {
		return current, nil
	}
	return kernel.DependencyChangeMaterializationRecord{}, errors.Join(finishErr, unknownErr, readErr)
}

func (s *Service) finishDependencyChangeAttempt(ctx context.Context, scope kernel.Scope, attempt kernel.DependencyChangeMaterializationRecord, state, reason string, logsDigest ...string) (kernel.DependencyChangeMaterializationRecord, error) {
	logsSHA256 := ""
	if len(logsDigest) > 0 {
		logsSHA256 = logsDigest[0]
	}
	return s.runtime.TXFinishDependencyChangeMaterialization(ctx, scope, kernel.DependencyChangeMaterializationFinish{
		MaterializationID: attempt.MaterializationID, State: state, ReasonCode: reason, ResolverLogsSHA256: logsSHA256,
	})
}

func dependencyChangeDerivedRequestID(kind, requestID string) string {
	digest := sha256.Sum256([]byte(requestID))
	return "dependency-change-" + kind + "-" + hex.EncodeToString(digest[:16])
}

var _ DependencyChangeService = (*Service)(nil)
