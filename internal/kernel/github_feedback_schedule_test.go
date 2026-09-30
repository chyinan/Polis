// pattern: Functional Core
package kernel

import "testing"

func TestGitHubFeedbackCollectionIntervalIsBounded(t *testing.T) {
	for _, test := range []struct {
		interval int
		want     bool
	}{
		{interval: 0, want: false},
		{interval: 899, want: false},
		{interval: 900, want: true},
		{interval: 3600, want: true},
		{interval: 86400, want: true},
		{interval: 86401, want: false},
	} {
		if got := validGitHubFeedbackCollectionInterval(test.interval); got != test.want {
			t.Errorf("validGitHubFeedbackCollectionInterval(%d)=%t, want %t", test.interval, got, test.want)
		}
	}
}
