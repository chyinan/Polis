// pattern: Functional Core
package codex

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testExecutionCombination() ExecutionCombination {
	return ExecutionCombination{
		CodexVersion: "0.151.0", BinarySHA256: "binary-a", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy-a", AuthSourceClass: "local-auth-file", CodeModeHostSHA256: "host-a", CapabilityDigest: "cap-a", NativeProtocolDigest: "native-a",
	}
}

func qualifiedBase(c ExecutionCombination, now time.Time) QualificationRecord {
	return QualificationRecord{Layer: QualificationL1BaseTransport, Status: QualificationQualified, EvidenceResult: EvidencePassed, ExecutionKey: c.Fingerprint(), Combination: c, CreatedAt: now, ExpiresAt: now.Add(time.Hour), EvidenceRef: "canary-pass"}
}

func TestCurrentFingerprintDoesNotInterpretLegacyHashAsManifestDigest(t *testing.T) {
	c := testExecutionCombination()
	legacy := c.Fingerprint()
	current := c.CurrentFingerprint()
	if current.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersion {
		t.Fatalf("current fingerprint schema = %q", current.FingerprintSchemaVersion)
	}
	if current.CanonicalManifestDigest == legacy || !current.Matches(c) {
		t.Fatalf("current fingerprint was not separated from legacy hash: legacy=%s current=%+v", legacy, current)
	}
	legacyAsCurrent := QualificationFingerprint{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersion, CanonicalManifestDigest: legacy}
	if legacyAsCurrent.Matches(c) {
		t.Fatal("legacy hash was silently accepted as canonical manifest digest")
	}
	now := time.Unix(50, 0)
	record := QualificationRecord{Layer: QualificationL1BaseTransport, Status: QualificationQualified, EvidenceResult: EvidencePassed, FingerprintSchemaVersion: current.FingerprintSchemaVersion, CanonicalManifestDigest: current.CanonicalManifestDigest, Combination: c, CreatedAt: now, ExpiresAt: now.Add(time.Hour), EvidenceRef: "canonical-manifest-pass"}
	if decision := EvaluateBusinessGate(now, c, []QualificationRecord{record}); !decision.Allowed {
		t.Fatalf("canonical manifest qualification was not accepted: %+v", decision)
	}
}

func TestBusinessGateAllowsExactQualifiedCombination(t *testing.T) {
	now := time.Unix(100, 0)
	c := testExecutionCombination()
	decision := EvaluateBusinessGate(now, c, []QualificationRecord{qualifiedBase(c, now)})
	if !decision.Allowed || decision.Status != QualificationQualified {
		t.Fatalf("exact qualified combination was denied: %+v", decision)
	}
}

func TestBusinessGateMapsT6InconclusiveEvidenceToSchedulingBlock(t *testing.T) {
	now := time.Unix(200, 0)
	c := testExecutionCombination()
	record := QualificationRecord{Layer: QualificationL1BaseTransport, Status: QualificationUnqualified, EvidenceResult: EvidenceInconclusive, ExecutionKey: c.Fingerprint(), Combination: c, CreatedAt: now, ExpiresAt: now.Add(time.Hour), Reason: "no_first_valid_output_after_structured_reconnects"}
	decision := EvaluateBusinessGate(now, c, []QualificationRecord{record})
	if decision.Allowed || decision.Status != QualificationUnqualified || decision.EvidenceResult != EvidenceInconclusive {
		t.Fatalf("T6 block was collapsed or allowed: %+v", decision)
	}
}

func TestExecutionChangesMakeQualificationStale(t *testing.T) {
	now := time.Unix(300, 0)
	base := testExecutionCombination()
	for name, mutate := range map[string]func(*ExecutionCombination){
		"binary":     func(c *ExecutionCombination) { c.BinarySHA256 = "binary-b" },
		"proxy":      func(c *ExecutionCombination) { c.ProxyConfigDigest = "proxy-b" },
		"model":      func(c *ExecutionCombination) { c.Model = "gpt-other" },
		"effort":     func(c *ExecutionCombination) { c.Effort = "high" },
		"runtime":    func(c *ExecutionCombination) { c.RuntimeProfile = "wsl-linux-arm64" },
		"code host":  func(c *ExecutionCombination) { c.CodeModeHostSHA256 = "host-b" },
		"capability": func(c *ExecutionCombination) { c.CapabilityDigest = "cap-b" },
	} {
		changed := base
		mutate(&changed)
		decision := EvaluateBusinessGate(now, changed, []QualificationRecord{qualifiedBase(base, now)})
		if decision.Allowed || decision.Status != QualificationStale {
			t.Fatalf("%s change did not stale qualification: %+v", name, decision)
		}
	}
}

func TestFixtureAndToolSurfaceDoNotChangeL1Key(t *testing.T) {
	c := testExecutionCombination()
	if c.Fingerprint() != testExecutionCombination().Fingerprint() {
		t.Fatal("same execution combination fingerprint changed")
	}
	now := time.Unix(400, 0)
	base := qualifiedBase(c, now)
	toolA := QualificationRecord{Layer: QualificationL2ToolSurface, Status: QualificationQualified, EvidenceResult: EvidencePassed, ExecutionKey: c.Fingerprint(), ToolSurfaceDigest: "tools-a", Combination: c, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	toolB := toolA
	toolB.ToolSurfaceDigest = "tools-b"
	if decision := EvaluateBusinessGate(now, c, []QualificationRecord{base, toolB}); !decision.Allowed {
		t.Fatalf("tool-surface change incorrectly invalidated L1: %+v", decision)
	}
	if decision := EvaluateToolSurfaceGate(now, c, "tools-a", []QualificationRecord{toolB}); decision.Allowed {
		t.Fatal("mismatched L2 tool surface was allowed")
	}
}

func TestStaleRecordDoesNotReviveOlderQualifiedEvidence(t *testing.T) {
	now := time.Unix(500, 0)
	c := testExecutionCombination()
	old := qualifiedBase(c, now.Add(-time.Hour))
	newer := old
	newer.CreatedAt = now.Add(-time.Minute)
	newer.Status = QualificationStale
	newer.Reason = "binary changed"
	decision := EvaluateBusinessGate(now, c, []QualificationRecord{old, newer})
	if decision.Allowed || decision.Status != QualificationStale {
		t.Fatalf("older qualified record revived after stale record: %+v", decision)
	}
}

func TestBusinessGateConcurrentReadersCannotBypassDecision(t *testing.T) {
	now := time.Unix(600, 0)
	c := testExecutionCombination()
	record := QualificationRecord{Layer: QualificationL1BaseTransport, Status: QualificationUnqualified, EvidenceResult: EvidenceInconclusive, ExecutionKey: c.Fingerprint(), Combination: c, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	var wg sync.WaitGroup
	results := make(chan GateDecision, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- EvaluateBusinessGate(now, c, []QualificationRecord{record}) }()
	}
	wg.Wait()
	close(results)
	for decision := range results {
		if decision.Allowed || decision.Status != QualificationUnqualified {
			t.Fatalf("concurrent reader bypassed gate: %+v", decision)
		}
	}
}

func TestQualificationGateDoesNotWriteBusinessAllowanceOrSideEffects(t *testing.T) {
	dir := t.TempDir()
	c := testExecutionCombination()
	_ = EvaluateBusinessGate(time.Unix(700, 0), c, nil)
	entries, err := os.ReadDir(filepath.Clean(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("pure qualification gate wrote side effects: %d entries", len(entries))
	}
}
