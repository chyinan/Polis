package gotestsupervisor

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUnitControlArgsMapOnlyFixedJobActions(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		jobID     string
		want      []string
	}{
		{
			name:      "start",
			operation: "job-start",
			jobID:     "job-a_1",
			want:      []string{"--disable-internal-agent", InstalledUnitControl, "job-start", "job-a_1"},
		},
		{
			name:      "stop",
			operation: "job-stop",
			jobID:     "job-a_1",
			want:      []string{"--disable-internal-agent", InstalledUnitControl, "job-stop", "job-a_1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := unitControlArgs(test.operation, test.jobID)
			if err != nil {
				t.Fatalf("unitControlArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("unitControlArgs() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUnitControlArgsRejectUnknownOperationAndInvalidJobID(t *testing.T) {
	for _, test := range []struct {
		operation string
		jobID     string
	}{
		{operation: "job-restart", jobID: "job-safe"},
		{operation: "job-start", jobID: "not-a-job"},
		{operation: "job-start", jobID: "job-../escape"},
		{operation: "job-stop", jobID: "job-"},
	} {
		if _, err := unitControlArgs(test.operation, test.jobID); err == nil {
			t.Fatalf("unitControlArgs(%q, %q) unexpectedly succeeded", test.operation, test.jobID)
		}
	}
}

func TestInstalledUnitControlSealWhenExplicitlyRequested(t *testing.T) {
	if os.Getenv("POLIS_SELF_DEV_REAL_GO_PATCH_E2E") != "1" {
		t.Skip("installed host trust probe is opt-in")
	}
	for _, path := range []string{InstalledUnitControl, UnitControlPolicyPath, "/usr/bin/pkexec"} {
		if !trustedRootPath(path) {
			t.Fatalf("installed unit-control trust path failed: %s", path)
		}
	}
	raw, err := readRootQualification(UnitControlSealPath)
	if err != nil {
		t.Fatalf("installed unit-control seal is untrusted: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 || lines[0] != UnitControlSealMarker {
		t.Fatalf("installed unit-control seal shape is invalid: %#v", lines)
	}
	for index, path := range []string{InstalledUnitControl, UnitControlPolicyPath} {
		actual, hashErr := fileSHA(path)
		if hashErr != nil || actual != lines[index+1] {
			t.Fatalf("installed unit-control seal digest mismatch for %s: actual=%s expected=%s err=%v", path, actual, lines[index+1], hashErr)
		}
	}
	if !validDigest(lines[3]) {
		t.Fatalf("installed unit-control pkla digest is not canonical: %q", lines[3])
	}
	if !trustedInstalledUnitControl() {
		t.Fatal("installed unit-control trust bundle failed its production readiness check")
	}
	if !verifyInstalledUnitControl(context.Background()) {
		t.Fatal("root-mediated unit-control seal verification failed")
	}
}
