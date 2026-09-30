// pattern: Imperative Shell
package runner

import (
	"errors"
	"fmt"
)

func enableLoopbackSIDWithRollback(set func(bool) (bool, error)) error {
	if set == nil {
		return errors.New("AppContainer loopback configuration is unavailable")
	}
	committed, enableErr := set(true)
	if enableErr == nil {
		return nil
	}
	if !committed {
		return enableErr
	}
	if _, rollbackErr := set(false); rollbackErr != nil {
		return errors.Join(enableErr, fmt.Errorf("failed to roll back committed AppContainer loopback change: %w", rollbackErr))
	}
	return enableErr
}
