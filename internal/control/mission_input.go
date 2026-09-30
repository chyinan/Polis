// pattern: Functional Core
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/intake"
)

type UploadMissionInputRequest struct {
	MissionID string
	InputID   string
	RequestID string
}

type MissionInputCommandReceipt struct {
	CompanyID     string `json:"companyId"`
	InputID       string `json:"inputId"`
	MissionID     string `json:"missionId"`
	Revision      int64  `json:"revision"`
	RequestID     string `json:"requestId"`
	SourceKind    string `json:"sourceKind"`
	DisplayName   string `json:"displayName"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentDigest string `json:"contentDigest"`
	State         string `json:"state"`
}

type MissionInputService interface {
	UploadMissionInput(ctx context.Context, companyID string, request UploadMissionInputRequest, filename, mediaType string, content []byte) (MissionInputCommandReceipt, error)
}

type MissionDirectoryInputService interface {
	UploadMissionDirectoryInput(ctx context.Context, companyID string, request UploadMissionInputRequest, files []intake.DirectoryInputFile) (MissionInputCommandReceipt, error)
}

func validateUploadMissionInputRequest(request UploadMissionInputRequest) error {
	if !core.ValidID(request.MissionID) || (request.InputID != "" && !core.ValidID(request.InputID)) {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}
