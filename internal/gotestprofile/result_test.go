// pattern: Functional Core
package gotestprofile

import (
	"strings"
	"testing"
)

func TestClassifyResultRequiresEveryFrozenTestAndCleanRunnerReceipt(t *testing.T) {
	valid := []byte(strings.Join([]string{
		`{"Action":"run","Package":"fixture","Test":"TestFrozenCompatibility"}`,
		`{"Action":"pass","Package":"fixture","Test":"TestFrozenCompatibility","Elapsed":0.001}`,
		`{"Action":"run","Package":"fixture","Test":"TestFrozenNeighborChange"}`,
		`{"Action":"pass","Package":"fixture","Test":"TestFrozenNeighborChange","Elapsed":0.001}`,
		`{"Action":"pass","Package":"fixture","Elapsed":0.002}`,
	}, "\n"))
	receipt := successfulExecutionFacts(valid)
	result := ClassifyResult(receipt)
	if result.Outcome != OutcomePassed || result.ReasonCode != "fixed_test_set_passed" || len(result.PassedTests) != len(frozenTestNames) {
		t.Fatalf("valid fixed test receipt was rejected: %+v", result)
	}
	for name, mutate := range map[string]func(*ExecutionFacts){
		"missing frozen test event": func(facts *ExecutionFacts) {
			facts.Stdout = []byte(`{"Action":"output","Package":"fixture","Output":"PASS"}`)
		},
		"candidate PASS text": func(facts *ExecutionFacts) {
			facts.Stdout = []byte(`{"Action":"output","Package":"fixture","Output":"PASS: candidate claims success"}`)
		},
		"truncated logs":                 func(facts *ExecutionFacts) { facts.StdoutTruncated = true },
		"process tree not reaped":        func(facts *ExecutionFacts) { facts.ProcessTreeReaped = false },
		"no effective resource controls": func(facts *ExecutionFacts) { facts.CgroupControlsApplied = false },
		"no network isolation":           func(facts *ExecutionFacts) { facts.NetworkIsolationApplied = false },
		"no scratch quota":               func(facts *ExecutionFacts) { facts.ScratchQuotaApplied = false },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := receipt
			candidate.Stdout = append([]byte(nil), receipt.Stdout...)
			mutate(&candidate)
			if got := ClassifyResult(candidate); got.Outcome == OutcomePassed {
				t.Fatalf("unqualified result was marked passed: %+v", got)
			}
		})
	}
}

func TestClassifyResultSeparatesCandidateFailureFromInfrastructureAndDependencyFailure(t *testing.T) {
	failure := successfulExecutionFacts([]byte(strings.Join([]string{
		`{"Action":"run","Package":"fixture","Test":"TestFrozenCompatibility"}`,
		`{"Action":"fail","Package":"fixture","Test":"TestFrozenCompatibility","Elapsed":0.001}`,
		`{"Action":"fail","Package":"fixture","Elapsed":0.002}`,
	}, "\n")))
	failure.ExitCode = 1
	if got := ClassifyResult(failure); got.Outcome != OutcomeCandidateFailed {
		t.Fatalf("test failure was not classified as candidate failure: %+v", got)
	}

	dependency := successfulExecutionFacts([]byte(`{"Action":"output","Package":"fixture","Output":"module lookup disabled by GOPROXY=off"}`))
	dependency.ExitCode = 1
	if got := ClassifyResult(dependency); got.Outcome != OutcomeDependencyUnavailable || got.ReasonCode != "offline_dependency_unavailable" {
		t.Fatalf("offline dependency miss was not classified safely: %+v", got)
	}

	resource := successfulExecutionFacts([]byte(`{"Action":"output","Package":"fixture","Output":"PASS"}`))
	resource.ResourceLimit = "memory"
	if got := ClassifyResult(resource); got.Outcome != OutcomeResourceLimit {
		t.Fatalf("resource limit was not distinguished: %+v", got)
	}

	infrastructure := successfulExecutionFacts([]byte("not-json"))
	infrastructure.SetupFailure = "trusted_runner_setup_failed"
	if got := ClassifyResult(infrastructure); got.Outcome != OutcomeInfrastructureFailed {
		t.Fatalf("runner setup failure was conflated with candidate failure: %+v", got)
	}
}

func successfulExecutionFacts(stdout []byte) ExecutionFacts {
	return ExecutionFacts{
		ExitCodeKnown: true, ExitCode: 0, Stdout: stdout, ProcessTreeReaped: true, CgroupControlsApplied: true,
		ScratchQuotaApplied: true, NetworkIsolationApplied: true,
	}
}
