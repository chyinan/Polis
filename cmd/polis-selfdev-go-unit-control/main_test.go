package main

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSystemdArgsOnlyMapsFixedJobOperations(t *testing.T) {
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
			want:      []string{"start", "--no-block", "polis-self-dev-go-job@job-a_1.service"},
		},
		{
			name:      "stop",
			operation: "job-stop",
			jobID:     "job-a_1",
			want:      []string{"stop", "polis-self-dev-go-job@job-a_1.service"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := systemdArgs(test.operation, test.jobID)
			if err != nil {
				t.Fatalf("systemdArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("systemdArgs() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSystemdArgsRejectsUnboundedCommandsAndIDs(t *testing.T) {
	tests := []struct {
		operation string
		jobID     string
	}{
		{operation: "restart", jobID: "job-a"},
		{operation: "job-start;rm", jobID: "job-a"},
		{operation: "job-start", jobID: "not-a-job"},
		{operation: "job-start", jobID: "job-../escape"},
		{operation: "job-stop", jobID: "job-"},
		{operation: "job-start", jobID: strings.Repeat("a", 129)},
	}
	for _, test := range tests {
		if _, err := systemdArgs(test.operation, test.jobID); err == nil {
			t.Fatalf("systemdArgs(%q, %q) unexpectedly succeeded", test.operation, test.jobID)
		}
	}
}

func TestExecuteRejectsNonRootBeforeSystemd(t *testing.T) {
	called := false
	code := execute([]string{"job-start", "job-a"}, 1000, "1000", io.Discard, io.Discard, func(context.Context, string, ...string) error {
		called = true
		return nil
	}, nil)
	if code == 0 {
		t.Fatal("non-root invocation succeeded")
	}
	if called {
		t.Fatal("systemctl was invoked for a non-root helper process")
	}
}

func TestProtocolInfoIsReadOnlyAndDoesNotRequireRoot(t *testing.T) {
	var output strings.Builder
	called := false
	code := execute([]string{"--protocol-info"}, 1000, "", &output, io.Discard, func(context.Context, string, ...string) error {
		called = true
		return nil
	}, nil)
	if code != 0 || output.String() != protocol+"\n" {
		t.Fatalf("protocol info result = (%d, %q)", code, output.String())
	}
	if called {
		t.Fatal("protocol info invoked systemctl")
	}
}

func TestExecuteRejectsUnexpectedPkexecCaller(t *testing.T) {
	called := false
	code := execute([]string{"job-start", "job-a"}, 0, "1001", io.Discard, io.Discard, func(context.Context, string, ...string) error {
		called = true
		return nil
	}, nil)
	if code == 0 {
		t.Fatal("unexpected pkexec caller was accepted")
	}
	if called {
		t.Fatal("systemctl was invoked for an unexpected caller")
	}
}

func TestExecuteInvokesOnlyTheMappedSystemdCommand(t *testing.T) {
	var gotPath string
	var gotArgs []string
	var gotDeadline time.Duration
	code := execute([]string{"job-start", "job-safe"}, 0, "1000", io.Discard, io.Discard, func(ctx context.Context, path string, args ...string) error {
		gotPath = path
		gotArgs = append([]string(nil), args...)
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("systemd call has no timeout")
		}
		gotDeadline = time.Until(deadline)
		return nil
	}, nil)
	if code != 0 {
		t.Fatalf("execute() = %d, want 0", code)
	}
	if gotPath != systemctlPath {
		t.Fatalf("systemctl path = %q, want %q", gotPath, systemctlPath)
	}
	wantArgs := []string{"start", "--no-block", "polis-self-dev-go-job@job-safe.service"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("systemctl args = %#v, want %#v", gotArgs, wantArgs)
	}
	if gotDeadline <= 0 || gotDeadline > systemctlTimeout {
		t.Fatalf("systemctl timeout remaining = %v, want (0, %v]", gotDeadline, systemctlTimeout)
	}
}

func TestExecuteDoesNotReportAcceptedOnSystemdFailure(t *testing.T) {
	var output strings.Builder
	code := execute([]string{"job-stop", "job-safe"}, 0, "1000", &output, io.Discard, func(context.Context, string, ...string) error {
		return errors.New("systemctl failure detail must not be forwarded")
	}, nil)
	if code == 0 {
		t.Fatal("failed systemctl action was accepted")
	}
	if strings.Contains(output.String(), "accepted") {
		t.Fatalf("failed action reported accepted: %q", output.String())
	}
}

func TestVerifyInstallationBindsAllRootSealDigests(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		root + "/helper",
		root + "/policy",
		root + "/pkla",
	}
	contents := [][]byte{[]byte("helper bytes"), []byte("policy bytes"), []byte("pkla bytes")}
	lines := []string{sealMarker}
	for index, path := range paths {
		if err := os.WriteFile(path, contents[index], 0600); err != nil {
			t.Fatal(err)
		}
		digest, err := boundedSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, digest)
	}
	seal := root + "/seal"
	if err := os.WriteFile(seal, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyInstallationFiles(seal, paths[0], paths[1], paths[2]); err != nil {
		t.Fatalf("valid unit-control seal rejected: %v", err)
	}
	if err := os.WriteFile(paths[2], []byte("changed pkla"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyInstallationFiles(seal, paths[0], paths[1], paths[2]); err == nil {
		t.Fatal("changed local authorization file passed seal verification")
	}
}

func TestExecuteRunsReadOnlyInstallationVerificationAfterPkexecIdentity(t *testing.T) {
	var stdout strings.Builder
	verifyCalled := false
	code := execute([]string{"--verify-installation"}, 0, "1000", &stdout, io.Discard, func(context.Context, string, ...string) error {
		t.Fatal("installation verification invoked systemctl")
		return nil
	}, func() error {
		verifyCalled = true
		return nil
	})
	if code != 0 || stdout.String() != "unit_control_trusted=yes\n" || !verifyCalled {
		t.Fatalf("verification result = code %d, stdout %q, called=%t", code, stdout.String(), verifyCalled)
	}

	verifyCalled = false
	code = execute([]string{"--verify-installation"}, 0, "1001", io.Discard, io.Discard, nil, func() error {
		verifyCalled = true
		return nil
	})
	if code == 0 || verifyCalled {
		t.Fatal("installation verification accepted a caller other than the pinned user")
	}
}
