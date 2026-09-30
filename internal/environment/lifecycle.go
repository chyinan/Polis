// pattern: Functional Core
package environment

type PreparationState string

const (
	PreparationBlockedPolicy           = "blocked_policy"
	PreparationBlockedSourceUnverified = "blocked_source_unverified"
	PreparationBlockedUnqualified      = "blocked_unqualified"
	PreparationAccepted                = "accepted"
	PreparationStarting                = "starting"
	PreparationRunning                 = "running"
	PreparationReady                   = "ready"
	PreparationFailed                  = "failed"
	PreparationCancelled               = "cancelled"
	PreparationOutcomeUnknown          = "outcome_unknown"
)

type JobState string

const (
	JobAccepted       = "accepted"
	JobStarting       = "starting"
	JobRunning        = "running"
	JobExited         = "exited"
	JobFailed         = "failed"
	JobCancelled      = "cancelled"
	JobOutcomeUnknown = "outcome_unknown"
)

type ServiceReadiness string

const (
	WindowsNodeIsolationProfile = "windows_appcontainer@1"
	LinuxNodeIsolationProfile   = "linux_bwrap@1"
)

func IsolationProfileForNodeProfile(profileID string) (string, bool) {
	switch profileID {
	case WindowsNodeNPMProfile:
		return WindowsNodeIsolationProfile, true
	case LinuxNodeNPMProfile:
		return LinuxNodeIsolationProfile, true
	default:
		return "", false
	}
}

const (
	ServiceNotReady  = "not_ready"
	ServiceReady     = "ready"
	ServiceUnhealthy = "unhealthy"
	ServiceRevoked   = "revoked"
)

func CanTransitionPreparation(from, to string) bool {
	if from == "" {
		return to == string(PreparationBlockedPolicy) || to == string(PreparationBlockedSourceUnverified) || to == string(PreparationBlockedUnqualified) || to == string(PreparationAccepted)
	}
	switch PreparationState(from) {
	case PreparationBlockedPolicy:
		return PreparationState(to) == PreparationBlockedSourceUnverified || PreparationState(to) == PreparationBlockedUnqualified || PreparationState(to) == PreparationAccepted
	case PreparationBlockedSourceUnverified:
		return PreparationState(to) == PreparationBlockedPolicy || PreparationState(to) == PreparationAccepted
	case PreparationBlockedUnqualified:
		return PreparationState(to) == PreparationAccepted
	case PreparationAccepted:
		return PreparationState(to) == PreparationBlockedUnqualified || PreparationState(to) == PreparationStarting || PreparationState(to) == PreparationFailed || PreparationState(to) == PreparationCancelled || PreparationState(to) == PreparationOutcomeUnknown
	case PreparationStarting:
		return PreparationState(to) == PreparationRunning || PreparationState(to) == PreparationFailed || PreparationState(to) == PreparationCancelled || PreparationState(to) == PreparationOutcomeUnknown
	case PreparationRunning:
		return PreparationState(to) == PreparationReady || PreparationState(to) == PreparationFailed || PreparationState(to) == PreparationCancelled || PreparationState(to) == PreparationOutcomeUnknown
	case PreparationReady:
		return PreparationState(to) == PreparationOutcomeUnknown
	default:
		return false
	}
}

func CanTransitionJob(from, to string) bool {
	if from == "" {
		return JobState(to) == JobAccepted
	}
	switch JobState(from) {
	case JobAccepted:
		return JobState(to) == JobStarting || JobState(to) == JobCancelled || JobState(to) == JobOutcomeUnknown
	case JobStarting:
		return JobState(to) == JobRunning || JobState(to) == JobFailed || JobState(to) == JobCancelled || JobState(to) == JobOutcomeUnknown
	case JobRunning:
		return JobState(to) == JobRunning || JobState(to) == JobExited || JobState(to) == JobFailed || JobState(to) == JobCancelled || JobState(to) == JobOutcomeUnknown
	case JobOutcomeUnknown:
		return JobState(to) == JobCancelled
	default:
		return false
	}
}

func CanTransitionServiceReadiness(from, to string) bool {
	if from == to && (ServiceReadiness(from) == ServiceNotReady || ServiceReadiness(from) == ServiceReady || ServiceReadiness(from) == ServiceUnhealthy) {
		return true
	}
	switch ServiceReadiness(from) {
	case ServiceNotReady:
		return ServiceReadiness(to) == ServiceReady || ServiceReadiness(to) == ServiceUnhealthy || ServiceReadiness(to) == ServiceRevoked
	case ServiceReady:
		return ServiceReadiness(to) == ServiceUnhealthy || ServiceReadiness(to) == ServiceRevoked
	case ServiceUnhealthy:
		return ServiceReadiness(to) == ServiceReady || ServiceReadiness(to) == ServiceRevoked
	default:
		return false
	}
}
