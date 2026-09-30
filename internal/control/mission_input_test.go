// pattern: Functional Core
package control

import (
	"testing"

	"polis/internal/core"
)

func TestValidateUploadMissionInputRequestRequiresMissionAndIdempotencyScope(t *testing.T) {
	valid := UploadMissionInputRequest{MissionID: "mission-1", RequestID: "upload-1"}
	if err := validateUploadMissionInputRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	for _, request := range []UploadMissionInputRequest{
		{RequestID: "upload-1"},
		{MissionID: "mission-1"},
		{MissionID: "../mission", RequestID: "upload-1"},
		{MissionID: "mission-1", InputID: "../input", RequestID: "upload-1"},
	} {
		if err := validateUploadMissionInputRequest(request); err != core.Malformed {
			t.Errorf("validateUploadMissionInputRequest(%+v) = %v, want malformed", request, err)
		}
	}
}
