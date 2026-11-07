// pattern: Functional Core
package probe

import "fmt"

var h3Order = []string{"reviewer_session_end", "review_record_frozen", "hidden_verifier", "runtime_isolation", "composite_assessment"}

func validateH3Ordering(events []string) error {
	if len(events) != len(h3Order) {
		return fmt.Errorf("H3 ordering requires %d phases", len(h3Order))
	}
	for i, phase := range h3Order {
		if events[i] != phase {
			return fmt.Errorf("H3 phase %d is %q, want %q", i, events[i], phase)
		}
	}
	return nil
}

func h3CompositeVerdict(reviewer, hidden, isolation string) string {
	if reviewer == "passed" && hidden == "passed" && isolation == "passed" {
		return "passed"
	}
	return "inconclusive"
}
