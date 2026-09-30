// pattern: Functional Core
package desktop

import "testing"

func TestTokenMatchesRejectsBlankOrWrongToken(t *testing.T) {
	for _, test := range []struct {
		name      string
		expected  string
		presented string
		want      bool
	}{
		{name: "blank expected", expected: "", presented: "token", want: false},
		{name: "blank presented", expected: "token", presented: "", want: false},
		{name: "wrong", expected: "token", presented: "other", want: false},
		{name: "exact", expected: "token", presented: "token", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := TokenMatches(test.expected, test.presented); got != test.want {
				t.Fatalf("TokenMatches(%q,%q)=%v, want %v", test.expected, test.presented, got, test.want)
			}
		})
	}
}

func TestAllowedOriginIsNarrow(t *testing.T) {
	if !AllowedOrigin("tauri://localhost") || !AllowedOrigin("") {
		t.Fatal("expected desktop and empty probe origins to be allowed")
	}
	if AllowedOrigin("https://attacker.example") {
		t.Fatal("unexpected third-party origin allowed")
	}
}

func TestPresentedTokenPrefersHeaderThenBearerThenEventSourceQuery(t *testing.T) {
	if got := PresentedToken("header", "Bearer bearer", "query"); got != "header" {
		t.Fatalf("header token=%q", got)
	}
	if got := PresentedToken("", "Bearer bearer", "query"); got != "bearer" {
		t.Fatalf("bearer token=%q", got)
	}
	if got := PresentedToken("", "", "query"); got != "query" {
		t.Fatalf("query token=%q", got)
	}
}

func TestValidateServerBindingRequiresTokenForNonLoopback(t *testing.T) {
	for _, test := range []struct {
		name    string
		address string
		token   string
		wantErr bool
	}{
		{name: "loopback tokenless", address: "127.0.0.1:8080", token: "", wantErr: false},
		{name: "wildcard tokenless", address: ":8080", token: "", wantErr: true},
		{name: "public tokenless", address: "0.0.0.0:8080", token: "", wantErr: true},
		{name: "public tokenized", address: "0.0.0.0:8080", token: "session", wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateServerBinding(test.address, test.token); (err != nil) != test.wantErr {
				t.Fatalf("ValidateServerBinding(%q,%q) error=%v, wantErr=%v", test.address, test.token, err, test.wantErr)
			}
		})
	}
}

func TestValidateRemoteManagementConfigRequiresExactHTTPSOriginAndToken(t *testing.T) {
	for _, test := range []struct {
		name    string
		address string
		token   string
		origin  string
		wantErr bool
	}{
		{name: "local default", address: "127.0.0.1:8080", wantErr: false},
		{name: "explicit authenticated HTTPS reverse proxy", address: "127.0.0.1:8080", token: "proxy-service-token", origin: "https://polis.example.com", wantErr: false},
		{name: "remote origin requires token", address: "127.0.0.1:8080", origin: "https://polis.example.com", wantErr: true},
		{name: "remote mode cannot bind a public address", address: "0.0.0.0:8080", token: "proxy-service-token", origin: "https://polis.example.com", wantErr: true},
		{name: "reject HTTP origin", address: "127.0.0.1:8080", token: "proxy-service-token", origin: "http://polis.example.com", wantErr: true},
		{name: "reject path", address: "127.0.0.1:8080", token: "proxy-service-token", origin: "https://polis.example.com/workbench", wantErr: true},
		{name: "reject wildcard", address: "127.0.0.1:8080", token: "proxy-service-token", origin: "https://*.example.com", wantErr: true},
		{name: "reject user info", address: "127.0.0.1:8080", token: "proxy-service-token", origin: "https://operator@polis.example.com", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateRemoteManagementConfig(test.address, test.token, test.origin); (err != nil) != test.wantErr {
				t.Fatalf("ValidateRemoteManagementConfig error=%v, wantErr=%v", err, test.wantErr)
			}
		})
	}
}

func TestArtifactDeliveryEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/artifacts/artifact-1/manifest",
		"/api/workbench/companies/company-1/artifacts/artifact-1/download",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("portable artifact endpoint is not token-gated: %s", path)
		}
	}
	if RequiresSessionTokenPath("/api/workbench/companies/company-1/artifacts/artifact-1") {
		t.Fatal("ordinary artifact metadata unexpectedly uses the portable-content token rule")
	}
}

func TestEnvironmentPlanningAndPreparationEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/environments",
		"/api/workbench/companies/company-1/environments/env-1/policy",
		"/api/workbench/companies/company-1/environments/env-1/executor-qualification",
		"/api/workbench/companies/company-1/environments/env-1/preparation",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("environment lifecycle endpoint is not token-gated: %s", path)
		}
	}
}

func TestJobRunEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/tasks/task-1/jobs",
		"/api/workbench/companies/company-1/jobs/job-1/stop",
		"/api/workbench/companies/company-1/jobs/job-1/logs",
		"/api/workbench/companies/company-1/jobs/job-1/browser-session",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("JobRun endpoint is not token-gated: %s", path)
		}
	}
}

func TestDomainEvidenceEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/domain-workflows",
		"/api/workbench/companies/company-1/domain-workflows/content-operations-reference/qualification",
		"/api/workbench/companies/company-1/domain-evidence",
		"/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/review",
		"/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/evidence/quality/preview",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("domain evidence endpoint is not token-gated: %s", path)
		}
	}
}

func TestCapabilityMutationEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/capabilities/skills",
		"/api/workbench/companies/company-1/capabilities/mcp",
		"/api/workbench/companies/company-1/capabilities/mcp-packages",
		"/api/workbench/companies/company-1/capabilities/qualify",
		"/api/workbench/companies/company-1/capabilities/decide",
		"/api/workbench/companies/company-1/capabilities/bind",
		"/api/workbench/companies/company-1/capabilities/unbind",
		"/api/workbench/companies/company-1/capabilities/runtime-observe",
		"/api/workbench/companies/company-1/capabilities/runtime-approve",
		"/api/workbench/companies/company-1/capabilities/runtime-observe-http",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("capability mutation endpoint is not token-gated: %s", path)
		}
	}
}

func TestTaskTakeoverEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/missions/mission-1/takeover-leases",
		"/api/workbench/companies/company-1/missions/mission-1/tasks/task-1/takeover-lease",
		"/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/snapshot",
		"/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/release",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("human takeover endpoint is not token-gated: %s", path)
		}
	}
}

func TestTaskWorkspaceContentAlwaysRequiresSessionToken(t *testing.T) {
	if !RequiresSessionTokenPath("/api/workbench/companies/company-1/tasks/task-1/workspace") {
		t.Fatal("Task workspace content is not token-gated")
	}
}

func TestGitHubFeedbackBacklogMutationAlwaysRequiresSessionToken(t *testing.T) {
	if !RequiresSessionTokenPath("/api/workbench/companies/company-1/feedback/backlog") {
		t.Fatal("company feedback backlog mutation is not token-gated")
	}
	if !RequiresSessionTokenPath("/api/workbench/companies/company-1/feedback/collection-policy") {
		t.Fatal("company feedback collection policy mutation is not token-gated")
	}
}

func TestAllCompanyFeedbackEndpointsAlwaysRequireSessionToken(t *testing.T) {
	for _, path := range []string{
		"/api/workbench/companies/company-1/feedback",
		"/api/workbench/companies/company-1/feedback/sources",
		"/api/workbench/companies/company-1/feedback/sources/source-1/probe",
		"/api/workbench/companies/company-1/feedback/sources/source-1/decision",
		"/api/workbench/companies/company-1/feedback/sources/source-1/scan",
		"/api/workbench/companies/company-1/feedback/credentials",
		"/api/workbench/companies/company-1/feedback/credentials/delete",
		"/api/workbench/companies/company-1/feedback/backlog",
		"/api/workbench/companies/company-1/feedback/collection-policy",
	} {
		if !RequiresSessionTokenPath(path) {
			t.Errorf("company feedback path is not token-gated: %s", path)
		}
	}
}
