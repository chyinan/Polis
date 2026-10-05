// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) ListTaskTakeoverLeases(ctx context.Context, companyID, missionID string) ([]kernel.TaskTakeoverLease, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	return s.runtime.TaskTakeoverLeases(ctx, s.runtime.LocalScope(companyID), missionID)
}

func (s *Service) CreateTaskTakeoverLease(ctx context.Context, companyID, missionID, taskID string, request TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(taskID) || !core.ValidID(request.RequestID) {
		return kernel.TaskTakeoverLease{}, core.Malformed
	}
	return s.runtime.TXCreateTaskTakeoverLease(ctx, s.runtime.LocalScope(companyID), missionID, taskID, request.RequestID)
}

func (s *Service) SubmitTaskTakeoverSnapshot(ctx context.Context, companyID, missionID, leaseID string, request TaskTakeoverSnapshotCommand) (kernel.TaskTakeoverLease, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(leaseID) || !core.ValidID(request.RequestID) {
		return kernel.TaskTakeoverLease{}, core.Malformed
	}
	return s.runtime.TXSubmitTaskTakeoverSnapshot(ctx, s.runtime.LocalScope(companyID), missionID, leaseID, kernel.TaskTakeoverSnapshotInput{
		RequestID: request.RequestID, BaseWorkspaceDigest: request.BaseWorkspaceDigest,
		BaseWorkspaceRevision: request.BaseWorkspaceRevision, Content: request.Content, HumanEffortSeconds: request.HumanEffortSeconds,
	})
}

func (s *Service) ReleaseTaskTakeoverLease(ctx context.Context, companyID, missionID, leaseID string, request TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(leaseID) || !core.ValidID(request.RequestID) {
		return kernel.TaskTakeoverLease{}, core.Malformed
	}
	return s.runtime.TXReleaseTaskTakeoverLease(ctx, s.runtime.LocalScope(companyID), missionID, leaseID, request.RequestID)
}

func (s *Service) GetTaskTakeoverWorkspaceManifest(ctx context.Context, companyID, missionID, leaseID string) (kernel.TaskTakeoverWorkspaceManifest, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(leaseID) {
		return kernel.TaskTakeoverWorkspaceManifest{}, core.Malformed
	}
	return s.runtime.TaskTakeoverWorkspaceManifest(ctx, s.runtime.LocalScope(companyID), missionID, leaseID)
}

func (s *Service) ReadTaskTakeoverWorkspaceFile(ctx context.Context, companyID, missionID, leaseID, relativePath string) (kernel.TaskTakeoverWorkspaceFile, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(leaseID) || !kernel.ValidWorkspaceRelativePath(relativePath) {
		return kernel.TaskTakeoverWorkspaceFile{}, core.Malformed
	}
	return s.runtime.ReadTaskTakeoverWorkspaceFile(ctx, s.runtime.LocalScope(companyID), missionID, leaseID, relativePath)
}

func (s *Service) SubmitTaskTakeoverDirectorySnapshot(ctx context.Context, companyID, missionID, leaseID string, request TaskTakeoverDirectorySnapshotCommand) (kernel.TaskTakeoverLease, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(leaseID) || !core.ValidID(request.RequestID) ||
		len(request.BaseWorkspaceTreeSHA256) != 64 || len(request.Files) == 0 {
		return kernel.TaskTakeoverLease{}, core.Malformed
	}
	for _, file := range request.Files {
		if !kernel.ValidWorkspaceRelativePath(file.RelativePath) {
			return kernel.TaskTakeoverLease{}, core.Malformed
		}
	}
	return s.runtime.TXSubmitTaskTakeoverDirectorySnapshot(ctx, s.runtime.LocalScope(companyID), missionID, leaseID, kernel.TaskTakeoverDirectorySnapshotInput{
		RequestID: request.RequestID, BaseWorkspaceTreeSHA256: request.BaseWorkspaceTreeSHA256,
		Files: request.Files, HumanEffortSeconds: request.HumanEffortSeconds,
	})
}
