// pattern: Functional Core
package environment

import "testing"

func TestPreparationStateTransitionsAreExplicitAndTerminal(t *testing.T) {
	valid := [][2]string{
		{"", PreparationBlockedPolicy},
		{"", PreparationBlockedSourceUnverified},
		{"", PreparationBlockedUnqualified},
		{"", PreparationAccepted},
		{PreparationBlockedPolicy, PreparationBlockedUnqualified},
		{PreparationBlockedPolicy, PreparationAccepted},
		{PreparationBlockedSourceUnverified, PreparationBlockedPolicy},
		{PreparationBlockedSourceUnverified, PreparationAccepted},
		{PreparationBlockedUnqualified, PreparationAccepted},
		{PreparationAccepted, PreparationStarting},
		{PreparationAccepted, PreparationBlockedUnqualified},
		{PreparationAccepted, PreparationFailed},
		{PreparationStarting, PreparationRunning},
		{PreparationStarting, PreparationFailed},
		{PreparationRunning, PreparationReady},
		{PreparationRunning, PreparationFailed},
		{PreparationRunning, PreparationCancelled},
		{PreparationRunning, PreparationOutcomeUnknown},
		{PreparationReady, PreparationOutcomeUnknown},
	}
	for _, transition := range valid {
		if !CanTransitionPreparation(transition[0], transition[1]) {
			t.Errorf("preparation transition %q -> %q was rejected", transition[0], transition[1])
		}
	}
	invalid := [][2]string{
		{"", PreparationReady},
		{PreparationBlockedPolicy, PreparationRunning},
		{PreparationBlockedSourceUnverified, PreparationRunning},
		{PreparationBlockedUnqualified, PreparationRunning},
		{PreparationAccepted, PreparationReady},
		{PreparationReady, PreparationRunning},
		{PreparationFailed, PreparationAccepted},
		{PreparationCancelled, PreparationRunning},
		{PreparationOutcomeUnknown, PreparationReady},
	}
	for _, transition := range invalid {
		if CanTransitionPreparation(transition[0], transition[1]) {
			t.Errorf("preparation transition %q -> %q was accepted", transition[0], transition[1])
		}
	}
}

func TestJobStateTransitionsSeparateExitFromReadiness(t *testing.T) {
	valid := [][2]string{
		{"", JobAccepted},
		{JobAccepted, JobStarting},
		{JobAccepted, JobCancelled},
		{JobStarting, JobRunning},
		{JobRunning, JobRunning},
		{JobRunning, JobExited},
		{JobRunning, JobFailed},
		{JobRunning, JobCancelled},
		{JobRunning, JobOutcomeUnknown},
		{JobOutcomeUnknown, JobCancelled},
	}
	for _, transition := range valid {
		if !CanTransitionJob(transition[0], transition[1]) {
			t.Errorf("job transition %q -> %q was rejected", transition[0], transition[1])
		}
	}
	invalid := [][2]string{
		{"", JobRunning},
		{JobAccepted, JobExited},
		{JobExited, JobRunning},
		{JobFailed, JobAccepted},
		{JobCancelled, JobExited},
		{JobOutcomeUnknown, JobExited},
	}
	for _, transition := range invalid {
		if CanTransitionJob(transition[0], transition[1]) {
			t.Errorf("job transition %q -> %q was accepted", transition[0], transition[1])
		}
	}
}

func TestServiceReadinessTransitionsAreIndependentFromJobExit(t *testing.T) {
	valid := [][2]string{
		{ServiceNotReady, ServiceReady},
		{ServiceNotReady, ServiceUnhealthy},
		{ServiceReady, ServiceUnhealthy},
		{ServiceUnhealthy, ServiceReady},
	}
	for _, transition := range valid {
		if !CanTransitionServiceReadiness(transition[0], transition[1]) {
			t.Errorf("readiness transition %q -> %q was rejected", transition[0], transition[1])
		}
	}
	if CanTransitionServiceReadiness(ServiceReady, JobExited) {
		t.Fatal("job terminal state was accepted as a service readiness state")
	}
}

func TestServiceReadinessAllowsLeaseRenewalAndTerminalRevocation(t *testing.T) {
	for _, state := range []string{ServiceNotReady, ServiceReady, ServiceUnhealthy} {
		if !CanTransitionServiceReadiness(state, state) {
			t.Errorf("same readiness %q should permit a fresh lease observation", state)
		}
		if !CanTransitionServiceReadiness(state, ServiceRevoked) {
			t.Errorf("readiness %q should permit endpoint revocation", state)
		}
	}
	if CanTransitionServiceReadiness(ServiceRevoked, ServiceReady) || CanTransitionServiceReadiness(ServiceRevoked, ServiceUnhealthy) || CanTransitionServiceReadiness(ServiceRevoked, ServiceRevoked) {
		t.Fatal("revoked service endpoint must be terminal")
	}
}

func TestEnvironmentAllowlistNormalizesNamesAndRejectsValues(t *testing.T) {
	got, err := NormalizeEnvironmentAllowlist([]string{"Path", "SYSTEMROOT", " temp "})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PATH", "SYSTEMROOT", "TEMP"}
	if len(got) != len(want) {
		t.Fatalf("normalized allowlist = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("normalized allowlist = %v, want %v", got, want)
		}
	}
	for _, invalid := range [][]string{{"API_KEY=secret"}, {"bad-name"}, {"PATH", "path"}} {
		if _, err := NormalizeEnvironmentAllowlist(invalid); err == nil {
			t.Errorf("invalid variable list %q was accepted", invalid)
		}
	}
}

func TestNodeProfilesMapToDistinctIsolationImplementations(t *testing.T) {
	if profile, ok := IsolationProfileForNodeProfile(WindowsNodeNPMProfile); !ok || profile != WindowsNodeIsolationProfile {
		t.Fatalf("Windows isolation mapping=%q ok=%v", profile, ok)
	}
	if profile, ok := IsolationProfileForNodeProfile(LinuxNodeNPMProfile); !ok || profile != LinuxNodeIsolationProfile {
		t.Fatalf("Linux isolation mapping=%q ok=%v", profile, ok)
	}
	if _, ok := IsolationProfileForNodeProfile("unknown-node-profile"); ok {
		t.Fatal("unknown environment profile received an isolation implementation")
	}
}
