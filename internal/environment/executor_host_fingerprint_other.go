//go:build !windows && !linux

// pattern: Imperative Shell
package environment

import "errors"

func currentHostBuildIdentity() (string, error) {
	return "", errors.New("host executor fingerprinting is unavailable on this platform")
}
