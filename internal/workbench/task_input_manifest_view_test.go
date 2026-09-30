// pattern: Functional Core
package workbench

import (
	"testing"
	"time"

	"polis/internal/intake"
)

func TestTaskInputManifestViewProjectsImageDeliveryReference(t *testing.T) {
	manifest, digest, err := intake.PrepareModelInputManifest("company-image", "mission-image", "task-image", []intake.MissionInputReference{{
		InputID: "screen-input", Revision: 1, RequestID: "screen-upload", SourceKind: "upload", DisplayName: "screen.png",
		MediaType: "image/png", ByteSize: 100, ContentDigest: repeatTaskInputDigest('a'), State: intake.StatePartial,
	}})
	if err != nil {
		t.Fatal(err)
	}
	payloadDigest := repeatTaskInputDigest('b')
	view, err := newTaskInputManifestView("company-image", "mission-image", "task-image", digest, "local_context_loaded", &payloadDigest, manifest,
		[]intake.ModelInputDeliveryRef{{InputID: "screen-input", MediaType: "image/png", ByteSize: 100, ContentDigest: repeatTaskInputDigest('a')}},
		nil, nil, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.IncludedInputIDs) != 1 || view.IncludedInputIDs[0] != "screen-input" || len(view.IncludedInputPaths) != 1 || view.IncludedInputPaths[0].MediaType != "image/png" {
		t.Fatalf("image delivery readback=%+v", view)
	}
}

func repeatTaskInputDigest(character byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = character
	}
	return string(value)
}
