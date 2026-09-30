// pattern: Functional Core
package workbench

import "context"

type MissionInputView struct {
	CompanyID     string `json:"companyId"`
	InputID       string `json:"inputId"`
	MissionID     string `json:"missionId"`
	RequestID     string `json:"requestId"`
	Revision      string `json:"revision"`
	SourceKind    string `json:"sourceKind"`
	DisplayName   string `json:"displayName"`
	MediaType     string `json:"mediaType"`
	ByteSize      string `json:"byteSize"`
	ContentDigest string `json:"contentDigest"`
	State         string `json:"state"`
	ImageWidth    string `json:"imageWidth"`
	ImageHeight   string `json:"imageHeight"`
	CreatedAt     string `json:"createdAt"`
}

type MissionInputReader interface {
	ListMissionInputs(ctx context.Context, companyID, missionID string) ([]MissionInputView, error)
}
