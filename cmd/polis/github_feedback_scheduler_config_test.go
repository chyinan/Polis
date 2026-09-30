// pattern: Functional Core
package main

import "testing"

func TestGitHubFeedbackSchedulerConfigRequiresBothGlobalGatesAndProtectedStore(t *testing.T) {
	for _, test := range []struct {
		name           string
		readOnlyFlag   string
		schedulerFlag  string
		protectedStore bool
		wantEnabled    bool
		wantErr        bool
	}{
		{name: "default off", protectedStore: true, wantEnabled: false},
		{name: "read only alone", readOnlyFlag: "1", protectedStore: true, wantEnabled: false},
		{name: "scheduler without global egress", schedulerFlag: "1", protectedStore: true, wantErr: true},
		{name: "scheduler without credential store", readOnlyFlag: "1", schedulerFlag: "1", wantErr: true},
		{name: "both gates", readOnlyFlag: "1", schedulerFlag: "1", protectedStore: true, wantEnabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := githubFeedbackSchedulerConfig(test.readOnlyFlag, test.schedulerFlag, test.protectedStore)
			if (err != nil) != test.wantErr || got != test.wantEnabled {
				t.Fatalf("githubFeedbackSchedulerConfig()=(%t,%v), want enabled=%t err=%t", got, err, test.wantEnabled, test.wantErr)
			}
		})
	}
}
