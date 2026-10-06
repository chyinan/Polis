// pattern: Functional Core
package kernel

import (
	"math"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
)

type ProductDeliveryManifestInvalidationCommand struct {
	ArtifactID               string
	ExpectedManifestRevision int64
	State                    string
	Reason                   string
	RequestID                string
}

func validateProductDeliveryManifestInvalidationCommand(command ProductDeliveryManifestInvalidationCommand) error {
	if !core.ValidID(command.ArtifactID) || !core.ValidID(command.RequestID) || command.ExpectedManifestRevision <= 0 || command.ExpectedManifestRevision == math.MaxInt64 || (command.State != "invalidated" && command.State != "withdrawn") {
		return core.Malformed
	}
	reason := strings.TrimSpace(command.Reason)
	if reason == "" || !utf8.ValidString(reason) || strings.ContainsRune(reason, '\x00') || len(reason) > 4096 {
		return core.Malformed
	}
	return nil
}
