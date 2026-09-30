// pattern: Functional Core
package workbench

import (
	"strings"
	"testing"
)

func TestHumanInterventionAttentionUsesAllowlistedLabelsAndEvidenceRefs(t *testing.T) {
	item := humanInterventionAttention(humanInterventionRow{
		ID: "0123456789abcdef", Severity: "critical", ReasonCode: "external_outcome_unknown",
		ProtectionAction: "readonly_mode", RequiredAction: "review_unknown_outcome", WorkflowState: "acknowledged",
		NotificationState: "outcome_unknown", EvidenceRefs: []string{"receipt-1"},
	})
	if item.Tone != "danger" || item.Subject.Label != "INC-01234567" || item.WorkflowState != "acknowledged" || item.NotificationState != "outcome_unknown" || len(item.EvidenceRefs) != 1 || item.EvidenceRefs[0] != "receipt-1" {
		t.Fatalf("human intervention attention = %+v", item)
	}
	for _, forbidden := range []string{"raw token", "client_secret", "password"} {
		if strings.Contains(item.Description, forbidden) {
			t.Fatalf("attention description contains unsafe content %q", forbidden)
		}
	}
}

func TestHumanInterventionAttentionSerializesMissingEvidenceAsEmptyArray(t *testing.T) {
	item := humanInterventionAttention(humanInterventionRow{
		ID: "0123456789abcdef", Severity: "high", ReasonCode: "handover_required",
		ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	})
	if item.EvidenceRefs == nil || len(item.EvidenceRefs) != 0 {
		t.Fatalf("missing evidence refs = %#v, want a non-nil empty slice", item.EvidenceRefs)
	}
}
