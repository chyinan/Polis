// pattern: Functional Core
package fixture

import "testing"

func TestFrontendConsumptionBindingAcceptsReferenceAndFrozenV6Shape(t *testing.T) {
	if report := CheckFrontendPublicBindingV1(FrontendConsumptionReferenceSource); !report.Passed {
		t.Fatalf("reference binding rejected: %+v", report)
	}
	if report := CheckFrontendPublicBindingV1(FrozenV6Revision12FrontendCandidate); !report.Passed {
		t.Fatalf("frozen V6 function signature should remain source-compatible: %+v", report)
	}
}

func TestFrontendConsumptionBindingRejectsOldUnsupportedShape(t *testing.T) {
	source := "package frontend\nfunc ConsumeItems(body string) string { return body }\nfunc Fetch(body string) string { return body }\n"
	report := CheckFrontendPublicBindingV1(source)
	if report.Passed || (report.ReasonCode != "declaration_not_allowed" && report.ReasonCode != "entrypoint_invalid") {
		t.Fatalf("unsupported frontend declarations were accepted: %+v", report)
	}
}
