// pattern: Functional Core
package probe

import (
	"errors"
	"fmt"

	"polis/internal/codex"
)

func CompareT12AuthSnapshot(expected, actual codex.AuthFingerprintManifest) error {
	if err := expected.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: expected T12 auth manifest: %w", err)
	}
	if err := actual.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: actual T12 auth manifest: %w", err)
	}
	comparison := codex.CompareAuthManifests(expected, actual)
	if !comparison.StrictExperimentComparable || expected != actual {
		if comparison.ReasonCode != "" {
			return errors.New("preflight_failed: auth material changed: " + comparison.ReasonCode)
		}
		return errors.New("preflight_failed: auth material does not exactly match the T12 manifest")
	}
	return nil
}
