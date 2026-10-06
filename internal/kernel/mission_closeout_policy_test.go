// pattern: Functional Core
package kernel

import (
	"errors"
	"testing"

	"polis/internal/core"
)

func TestValidateMissionSuccessDeliveryAcceptanceRequiresExplicitOwnerAcceptance(t *testing.T) {
	if err := validateMissionSuccessDeliveryAcceptance(true, true); err != nil {
		t.Fatalf("accepted ready delivery was rejected: %v", err)
	}
	for name, input := range map[string]struct {
		readyPassed         bool
		acceptedDisposition bool
	}{
		"verification missing":    {readyPassed: false, acceptedDisposition: true},
		"user acceptance missing": {readyPassed: true, acceptedDisposition: false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateMissionSuccessDeliveryAcceptance(input.readyPassed, input.acceptedDisposition); !errors.Is(err, core.Conflict) {
				t.Fatalf("policy error=%v, want conflict", err)
			}
		})
	}
}
