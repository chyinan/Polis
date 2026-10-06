// pattern: Functional Core
package kernel

import "time"

const productDeliveryFeedbackWindow = 7 * 24 * time.Hour

func productDeliveryFeedbackExpired(state string, deadline *time.Time, now time.Time) bool {
	return state == "awaiting_feedback" && deadline != nil && !now.Before(deadline.UTC())
}
