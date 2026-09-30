// pattern: Imperative Shell
package control

import (
	"context"
	"fmt"

	"polis/internal/core"
	"polis/internal/intake"
)

func (s *Service) UploadMissionDirectoryInput(ctx context.Context, companyID string, request UploadMissionInputRequest, files []intake.DirectoryInputFile) (MissionInputCommandReceipt, error) {
	if !core.ValidID(companyID) {
		return MissionInputCommandReceipt{}, core.Malformed
	}
	if err := validateUploadMissionInputRequest(request); err != nil {
		return MissionInputCommandReceipt{}, err
	}
	prepared, err := intake.PrepareDirectorySnapshot(files)
	if err != nil {
		return MissionInputCommandReceipt{}, fmt.Errorf("%w: %v", core.Malformed, err)
	}
	revision, err := s.runtime.TXAddMissionInput(ctx, s.runtime.LocalScope(companyID), request.MissionID, request.InputID, request.RequestID, prepared.Upload, prepared.Archive)
	if err != nil {
		return MissionInputCommandReceipt{}, err
	}
	return MissionInputCommandReceipt{
		CompanyID: companyID, InputID: revision.InputID, MissionID: revision.MissionID, Revision: revision.Revision,
		RequestID: revision.RequestID, SourceKind: revision.SourceKind, DisplayName: revision.DisplayName,
		MediaType: revision.MediaType, ByteSize: revision.ByteSize, ContentDigest: revision.ContentDigest, State: revision.State,
	}, nil
}
