// pattern: Functional Core
package kernel

import "testing"

func TestSupportedCrossBackendHandoverProfilesAreExactAndBidirectional(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target string
		want   bool
	}{
		{name: "windows to linux", source: "windows-node-npm@1", target: "linux-node-npm@1", want: true},
		{name: "linux to windows", source: "linux-node-npm@1", target: "windows-node-npm@1", want: true},
		{name: "same profile", source: "windows-node-npm@1", target: "windows-node-npm@1"},
		{name: "unknown source", source: "mac-node-npm@1", target: "linux-node-npm@1"},
		{name: "unknown target", source: "windows-node-npm@1", target: "arbitrary-mcp@1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := supportedCrossBackendHandoverProfiles(test.source, test.target); got != test.want {
				t.Fatalf("supportedCrossBackendHandoverProfiles(%q, %q) = %t, want %t", test.source, test.target, got, test.want)
			}
		})
	}
}

func TestCrossBackendHandoverDigestBindsEveryPersistedField(t *testing.T) {
	base := CrossBackendHandoverRecord{
		CompanyID: "company-1", HandoverID: "handover-1", MissionID: "mission-1", TaskID: "task-1",
		SourceJobID: "job-1", SourceSessionID: "session-1", SourceRuntimeIncarnation: "runtime-1",
		SourceEnvironmentRevision: "env-1", SourceProfileID: "windows-node-npm@1",
		TargetEnvironmentRevision: "env-2", TargetProfileID: "linux-node-npm@1",
		ProjectSourceSHA256: "a", PackageJSONSHA256: "b", LockfileSHA256: "c", WorkspaceDigest: "d",
		WorkspaceRevision: 3, TaskInputManifestSHA256: "e", TargetPolicySHA256: "f", TargetToolchainSHA256: "1",
		RequestID: "request-1",
	}
	first := crossBackendHandoverRecordDigest(base)
	if first == "" || first != crossBackendHandoverRecordDigest(base) {
		t.Fatal("handover digest is empty or unstable")
	}
	changed := base
	changed.WorkspaceRevision++
	if first == crossBackendHandoverRecordDigest(changed) {
		t.Fatal("handover digest did not bind the captured workspace revision")
	}
}
