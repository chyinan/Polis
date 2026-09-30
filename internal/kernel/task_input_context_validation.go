// pattern: Functional Core
package kernel

import (
	"reflect"

	"polis/internal/core"
)

func verifyProductTaskInputContext(canonical, supplied ProductTaskInputContext) error {
	if canonical.ManifestDigest != supplied.ManifestDigest || !reflect.DeepEqual(canonical.Payload, supplied.Payload) {
		return core.Integrity
	}
	return nil
}
