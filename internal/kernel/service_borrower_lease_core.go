// pattern: Functional Core
package kernel

import (
	"time"

	"polis/internal/core"
)

const (
	ServiceBorrowerLeaseMaxTTL    = 15 * time.Minute
	ServiceBorrowerLeaseIdleGrace = 2 * time.Minute
)

func validateServiceBorrowerLeaseScope(company, ownerMission, borrowerMission, ownerTask, borrowerTask, ownerSession, borrowerSession string, generation int) error {
	if company == "" || ownerMission == "" || borrowerMission == "" || ownerMission != borrowerMission || ownerTask == "" || borrowerTask == "" || ownerSession == "" || borrowerSession == "" || generation <= 0 || ownerTask == borrowerTask {
		return core.OutOfScope
	}
	return nil
}

func serviceBorrowerLeaseTimes(now, endpointExpiry time.Time) (time.Time, time.Time) {
	maxExpiry := now.Add(ServiceBorrowerLeaseMaxTTL)
	if endpointExpiry.Before(maxExpiry) {
		maxExpiry = endpointExpiry
	}
	idleUntil := now.Add(ServiceBorrowerLeaseIdleGrace)
	if idleUntil.After(maxExpiry) {
		idleUntil = maxExpiry
	}
	return maxExpiry.UTC(), idleUntil.UTC()
}

func serviceBorrowerLeaseExpired(now, expiresAt, idleUntil time.Time) bool {
	return !now.Before(expiresAt) || !now.Before(idleUntil)
}
