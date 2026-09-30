// pattern: Functional Core
package probe

import (
	"encoding/json"
	"errors"
	"strings"

	"polis/internal/fixture"
	"polis/internal/kernel"
)

type H1FrozenSubject struct {
	SubjectRevision           string
	ProblemKey                string
	CompanyID                 string
	Contract                  kernel.PeerContractRevision
	MessageID                 string
	ObligationID              string
	ObligationOwner           string
	FinalObligationState      string
	BackendArtifactID         string
	FrontendArtifactID        string
	BackendDigest             string
	FrontendDigest            string
	BackendContent            string
	FrontendContent           string
	BackendSessionID          string
	BackendSessionIncarnation string
	FrontendInitialSession    R03ATurn
	FrontendSuccessorSession  R03ATurn
	PositiveIntegration       bool
	NegativeControlPassed     bool
	OldWriterRejection        bool
	RuntimeIncarnation        string
	SourceRuntimeIncarnation  string
	PolicyRevisions           map[string]string
	AllowedRefs               []string
}

type H1HiddenVerification struct {
	Passed   bool     `json:"passed"`
	Findings []string `json:"findings"`
}

type H1CompositeResult struct {
	ReviewerVerdict      string `json:"reviewer_verdict"`
	HiddenVerifierResult string `json:"hidden_verifier_result"`
	ReviewIsolation      string `json:"review_isolation_result"`
	ReviewerQuality      string `json:"reviewer_quality"`
	Composite            string `json:"r0_3a_composite"`
}

func validateH1FrozenSubject(subject H1FrozenSubject) error {
	if subject.ProblemKey != R03AProblemKey || subject.Contract.ID == "" || subject.Contract.Revision != 3 || subject.Contract.State != "accepted" || subject.MessageID == "" || subject.ObligationID != subject.MessageID || subject.ObligationOwner != "emp-frontend" || subject.FinalObligationState != "fulfilled" || subject.BackendArtifactID == "" || subject.FrontendArtifactID == "" || subject.BackendDigest == "" || subject.FrontendDigest == "" || subject.BackendContent == "" || subject.FrontendContent == "" {
		return errors.New("frozen R0.3A subject anchors are incomplete")
	}
	if subject.FrontendInitialSession.Employee != "emp-frontend" || subject.FrontendSuccessorSession.Employee != "emp-frontend" || subject.FrontendInitialSession.SessionID == "" || subject.FrontendSuccessorSession.SessionID == "" || subject.FrontendInitialSession.SessionID == subject.FrontendSuccessorSession.SessionID || subject.FrontendSuccessorSession.Epoch <= subject.FrontendInitialSession.Epoch || subject.FrontendInitialSession.Incarnation == "" || subject.FrontendSuccessorSession.Incarnation == "" || subject.FrontendInitialSession.Incarnation != subject.FrontendSuccessorSession.Incarnation || !subject.FrontendInitialSession.TurnStarted || !subject.FrontendInitialSession.ProviderEgress || !subject.FrontendInitialSession.TurnCompleted || !subject.FrontendInitialSession.StopConfirmed || !subject.FrontendSuccessorSession.TurnStarted || !subject.FrontendSuccessorSession.ProviderEgress || !subject.FrontendSuccessorSession.TurnCompleted || !subject.FrontendSuccessorSession.StopConfirmed {
		return errors.New("frontend handover identity or terminal session facts are invalid")
	}
	contract := fixture.PeerContractSpec{Endpoint: subject.Contract.Endpoint, Schema: subject.Contract.Schema}
	if !fixture.CandidateCriteriaPassed(fixture.CheckPeerBackendCandidate(subject.BackendContent, contract)) || !fixture.CandidateCriteriaPassed(fixture.CheckPeerFrontendCandidate(subject.FrontendContent, contract)) {
		return errors.New("frozen candidate does not satisfy the public final contract")
	}
	if !subject.PositiveIntegration || subject.NegativeControlPassed || !subject.OldWriterRejection {
		return errors.New("frozen integration or old-writer evidence is inconsistent")
	}
	return nil
}

func h1HiddenVerify(subject H1FrozenSubject) H1HiddenVerification {
	if err := validateH1FrozenSubject(subject); err != nil {
		return H1HiddenVerification{Passed: false, Findings: []string{err.Error()}}
	}
	return H1HiddenVerification{Passed: true, Findings: []string{"frozen public candidate, contract, collaboration and handover invariants passed"}}
}

func h1ReviewerIsolation(toolEvents []string, protocol []byte) bool {
	allowed := map[string]bool{"work_current": true, "context_read": true, "workspace_read": true, "review_submit": true}
	seen := map[string]bool{}
	for _, event := range toolEvents {
		if !allowed[event] {
			return false
		}
		seen[event] = true
	}
	for _, required := range []string{"work_current", "context_read", "workspace_read", "review_submit"} {
		if !seen[required] {
			return false
		}
	}
	text := strings.ToLower(string(protocol))
	for _, forbidden := range []string{"workspace_replace", "artifact_submit", "contract_propose", "contract_accept", "collab_send", "collab_apply", "obligation_resolve", "hidden verifier source", "negative-control implementation", "expected answer"} {
		if strings.Contains(text, forbidden) {
			return false
		}
	}
	return true
}

func h1Composite(reviewer, hidden, isolation string) H1CompositeResult {
	result := H1CompositeResult{ReviewerVerdict: reviewer, HiddenVerifierResult: hidden, ReviewIsolation: isolation, ReviewerQuality: "FAIL", Composite: "INCONCLUSIVE"}
	if reviewer == "passed" && hidden == "passed" && isolation == "passed" {
		result.ReviewerQuality = "PASS"
		result.Composite = "PASSED"
		return result
	}
	if reviewer == "failed" && hidden == "passed" && isolation == "passed" {
		result.ReviewerQuality = "FAIL_DISAGREEMENT"
		result.Composite = "FAILED"
		return result
	}
	if hidden == "failed" || isolation == "failed" || reviewer == "failed" {
		result.Composite = "FAILED"
	}
	return result
}

func h1ReviewerPrompt() string {
	return "You are an independent High Reviewer for the frozen R0.3A peer-collaboration result. Use only the registered read-only review tools. Inspect the frozen public contract, final Backend and Frontend candidates, Message/Obligation identity and handover facts yourself. Submit exactly one formal verdict with concrete findings, allowlisted evidence references, confidence and limitations. Do not modify any candidate, workspace, contract, message, obligation or artifact. Do not rely on any hidden verifier or prior worker transcript; natural language is not the system verdict."
}

func h1SubjectDigest(subject H1FrozenSubject) string {
	copy := subject
	copy.BackendContent = ""
	copy.FrontendContent = ""
	raw, _ := json.Marshal(copy)
	return digest(raw)
}
