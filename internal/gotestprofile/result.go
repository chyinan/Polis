// pattern: Functional Core
package gotestprofile

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

const (
	OutcomePassed                = "passed"
	OutcomeCandidateFailed       = "candidate_failed"
	OutcomeDependencyUnavailable = "dependency_unavailable"
	OutcomeInfrastructureFailed  = "infrastructure_failed"
	OutcomeResourceLimit         = "resource_limit"
	OutcomeTimedOut              = "timed_out"
	OutcomeCancelled             = "cancelled"
	maxGoTestLogBytes            = 512 << 10
)

var (
	frozenTestNames    = []string{"TestFrozenCompatibility", "TestFrozenNeighborChange"}
	setupCodePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	resourceLimitCodes = map[string]struct{}{
		"memory": {}, "pids": {}, "scratch_bytes": {}, "scratch_inodes": {}, "stdout_bytes": {}, "stderr_bytes": {},
	}
)

// ExecutionFacts is a trusted runner receipt. None of these fields may come
// from candidate stdout, candidate files, or a caller-supplied JobRun result.
type ExecutionFacts struct {
	ExitCodeKnown           bool
	ExitCode                int
	TimedOut                bool
	Cancelled               bool
	ResourceLimit           string
	StdoutTruncated         bool
	StderrTruncated         bool
	ProcessTreeReaped       bool
	CgroupControlsApplied   bool
	ScratchQuotaApplied     bool
	NetworkIsolationApplied bool
	SetupFailure            string
	Stdout                  []byte
	Stderr                  []byte
}

type TestResult struct {
	Outcome       string   `json:"outcome"`
	ReasonCode    string   `json:"reasonCode"`
	ExitCode      *int     `json:"exitCode,omitempty"`
	PassedTests   []string `json:"passedTests"`
	FailedTests   []string `json:"failedTests"`
	StdoutBytes   int      `json:"stdoutBytes"`
	StdoutSHA256  string   `json:"stdoutSha256"`
	StderrBytes   int      `json:"stderrBytes"`
	StderrSHA256  string   `json:"stderrSha256"`
	ResourceLimit string   `json:"resourceLimit,omitempty"`
	ResultSHA256  string   `json:"resultSha256"`
}

type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// ClassifyResult accepts PASS only when the fixed profile's complete baseline
// test set and package pass events are present and every host-control receipt
// is affirmative. The candidate's textual output is never treated as evidence.
func ClassifyResult(facts ExecutionFacts) TestResult {
	result := TestResult{
		PassedTests: []string{}, FailedTests: []string{},
		StdoutBytes: len(facts.Stdout), StdoutSHA256: digestHex(facts.Stdout),
		StderrBytes: len(facts.Stderr), StderrSHA256: digestHex(facts.Stderr), ResourceLimit: facts.ResourceLimit,
	}
	if facts.ExitCodeKnown {
		code := facts.ExitCode
		result.ExitCode = &code
	}
	setOutcome := func(outcome, reason string) TestResult {
		result.Outcome, result.ReasonCode = outcome, reason
		result.ResultSHA256 = resultDigest(result)
		return result
	}
	if len(facts.Stdout) > maxGoTestLogBytes || len(facts.Stderr) > maxGoTestLogBytes {
		return setOutcome(OutcomeInfrastructureFailed, "runner_log_bound_not_enforced")
	}
	if facts.SetupFailure != "" {
		if !setupCodePattern.MatchString(facts.SetupFailure) {
			return setOutcome(OutcomeInfrastructureFailed, "runner_setup_failed")
		}
		return setOutcome(OutcomeInfrastructureFailed, facts.SetupFailure)
	}
	if !facts.ProcessTreeReaped {
		return setOutcome(OutcomeInfrastructureFailed, "process_tree_not_reaped")
	}
	if !facts.CgroupControlsApplied || !facts.ScratchQuotaApplied || !facts.NetworkIsolationApplied {
		return setOutcome(OutcomeInfrastructureFailed, "required_host_controls_not_applied")
	}
	if facts.ResourceLimit != "" {
		if _, ok := resourceLimitCodes[facts.ResourceLimit]; !ok {
			return setOutcome(OutcomeInfrastructureFailed, "unknown_resource_limit_code")
		}
		return setOutcome(OutcomeResourceLimit, "resource_limit_exceeded")
	}
	if facts.TimedOut {
		return setOutcome(OutcomeTimedOut, "wall_clock_timeout")
	}
	if facts.Cancelled {
		return setOutcome(OutcomeCancelled, "cancelled_by_operator")
	}
	if facts.StdoutTruncated || facts.StderrTruncated {
		return setOutcome(OutcomeResourceLimit, "bounded_log_limit_exceeded")
	}
	if !facts.ExitCodeKnown {
		return setOutcome(OutcomeInfrastructureFailed, "exit_status_unavailable")
	}

	passed, failed, packagePassed, packageRan, malformed := parseTestEvents(facts.Stdout)
	result.PassedTests, result.FailedTests = passed, failed
	if malformed {
		return setOutcome(OutcomeInfrastructureFailed, "test_event_stream_malformed")
	}
	if facts.ExitCode != 0 && !packageRan && offlineDependencyDiagnostic(facts.Stdout, facts.Stderr) {
		return setOutcome(OutcomeDependencyUnavailable, "offline_dependency_unavailable")
	}
	if facts.ExitCode != 0 || len(failed) != 0 || !packagePassed || !hasEveryFrozenTest(passed) {
		reason := "fixed_test_set_failed_or_incomplete"
		if facts.ExitCode != 0 {
			reason = "nonzero_exit"
		}
		return setOutcome(OutcomeCandidateFailed, reason)
	}
	return setOutcome(OutcomePassed, "fixed_test_set_passed")
}

func parseTestEvents(stdout []byte) (passed, failed []string, packagePassed, packageRan, malformed bool) {
	passedSet := make(map[string]bool, len(frozenTestNames))
	failedSet := make(map[string]bool, len(frozenTestNames))
	packagePass := false
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 4096), maxGoTestLogBytes+1)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event goTestEvent
		decoder := json.NewDecoder(bytes.NewReader(line))
		if decoder.Decode(&event) != nil {
			malformed = true
			break
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF || event.Package != "fixture" || !validGoTestEvent(event) {
			malformed = true
			break
		}
		if event.Test == "" {
			if event.Action == "run" {
				packageRan = true
			}
			if event.Action == "pass" {
				packagePass = true
			}
			continue
		}
		if !isFrozenTest(event.Test) {
			continue
		}
		switch event.Action {
		case "pass":
			passedSet[event.Test] = true
			delete(failedSet, event.Test)
		case "fail":
			failedSet[event.Test] = true
			delete(passedSet, event.Test)
		}
	}
	if scanner.Err() != nil {
		malformed = true
	}
	for _, name := range frozenTestNames {
		if passedSet[name] {
			passed = append(passed, name)
		}
		if failedSet[name] {
			failed = append(failed, name)
		}
	}
	return passed, failed, packagePass, packageRan, malformed
}

func validGoTestEvent(event goTestEvent) bool {
	if len(event.Action) == 0 || strings.ContainsRune(event.Action, '\x00') || strings.ContainsRune(event.Package, '\x00') || strings.ContainsRune(event.Test, '\x00') {
		return false
	}
	switch event.Action {
	case "start", "run", "pause", "cont", "pass", "fail", "skip", "output", "bench":
		return true
	default:
		return false
	}
}

func isFrozenTest(name string) bool {
	for _, expected := range frozenTestNames {
		if name == expected {
			return true
		}
	}
	return false
}

func hasEveryFrozenTest(passed []string) bool {
	if len(passed) != len(frozenTestNames) {
		return false
	}
	for _, expected := range frozenTestNames {
		found := false
		for _, actual := range passed {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func offlineDependencyDiagnostic(stdout, stderr []byte) bool {
	combined := strings.ToLower(string(stdout) + "\n" + string(stderr))
	return strings.Contains(combined, "module lookup disabled by goproxy=off") ||
		strings.Contains(combined, "cannot find module providing package") ||
		strings.Contains(combined, "module not found")
}

func resultDigest(result TestResult) string {
	result.ResultSHA256 = ""
	canonical, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}
