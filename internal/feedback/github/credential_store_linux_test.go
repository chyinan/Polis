// pattern: Functional Core
//go:build linux

package github

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestLinuxProtectedGitHubCredentialStoreUsesSecretToolWithoutPuttingTokenInArguments(t *testing.T) {
	const token = "github_pat_" + "Abcdefghijklmnopqrstuvwxyz0123456789"
	runner := &recordingSecretToolRunner{lookupOutput: []byte(token + "\n")}
	store := &linuxProtectedGitHubCredentialStore{executable: "/usr/bin/secret-tool", runner: runner}
	if err := store.StoreToken("default-readonly", token); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || runner.calls[0].args[0] != "store" || string(runner.calls[0].input) != token {
		t.Fatalf("secret store call=%+v", runner.calls)
	}
	if runner.calls[0].maxOutput != 0 {
		t.Fatalf("secret store unexpectedly captured stdout with limit=%d", runner.calls[0].maxOutput)
	}
	if strings.Contains(strings.Join(runner.calls[0].args, " "), token) {
		t.Fatal("credential token was passed as a process argument")
	}
	loaded, err := store.LoadToken("default-readonly")
	if err != nil || loaded != token {
		t.Fatalf("loaded token=%q error=%v", loaded, err)
	}
	if len(runner.calls) != 2 || !reflect.DeepEqual(runner.calls[1].args, []string{"lookup", "service", "polis-github-readonly", "account", "default-readonly"}) {
		t.Fatalf("secret lookup call=%+v", runner.calls[1])
	}
	if runner.calls[1].maxOutput != linuxGitHubCredentialOutput {
		t.Fatalf("secret lookup output limit=%d", runner.calls[1].maxOutput)
	}
	if err = store.DeleteToken("default-readonly"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 3 || !reflect.DeepEqual(runner.calls[2].args, []string{"clear", "service", "polis-github-readonly", "account", "default-readonly"}) {
		t.Fatalf("secret delete call=%+v", runner.calls[2])
	}
}

func TestSecretServiceEnvironmentAndOutputAreBounded(t *testing.T) {
	environment := secretServiceEnvironment([]string{
		"HOME=/home/polis", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus", "XDG_RUNTIME_DIR=/run/user/1000",
		"PATH=/usr/bin", "POLIS_GITHUB_TOKEN=must-not-cross", "LD_PRELOAD=/tmp/untrusted.so",
	})
	if strings.Contains(strings.Join(environment, "\n"), "POLIS_GITHUB_TOKEN") || strings.Contains(strings.Join(environment, "\n"), "LD_PRELOAD") {
		t.Fatalf("secret-tool child inherited an unapproved environment variable: %v", environment)
	}
	var output boundedSecretOutput
	output.limit = 4
	if _, err := output.Write([]byte("safe")); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("!")); !errors.Is(err, ErrProtectedGitHubCredentialStoreUnavailable) || !bytes.Equal(output.data.Bytes(), []byte("safe")) {
		t.Fatalf("oversized keyring result err=%v data=%q", err, output.data.Bytes())
	}
}

func TestLinuxProtectedGitHubCredentialStoreRejectsInvalidOrUnavailableCalls(t *testing.T) {
	runner := &recordingSecretToolRunner{runErr: errors.New("keyring failure with token github_pat_secret_should_not_escape")}
	store := &linuxProtectedGitHubCredentialStore{executable: "/usr/bin/secret-tool", runner: runner}
	if err := store.StoreToken("bad/ref", "github_pat_"+strings.Repeat("a", 40)); err == nil {
		t.Fatal("invalid credential reference was accepted")
	}
	if _, err := store.LoadToken("bad/ref"); !errors.Is(err, ErrProtectedGitHubCredentialStoreUnavailable) {
		t.Fatalf("invalid credential lookup error=%v", err)
	}
	if err := store.StoreToken("default-readonly", "short"); err == nil {
		t.Fatal("invalid token was accepted")
	}
	if _, err := store.LoadToken("default-readonly"); !errors.Is(err, ErrProtectedGitHubCredentialStoreUnavailable) || strings.Contains(err.Error(), "github_pat_secret") {
		t.Fatalf("secret-service failure was not redacted: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("invalid calls reached Secret Service: %d", len(runner.calls))
	}
}

type secretToolCall struct {
	executable string
	args       []string
	input      []byte
	maxOutput  int
}

type recordingSecretToolRunner struct {
	calls        []secretToolCall
	lookupOutput []byte
	runErr       error
}

func (runner *recordingSecretToolRunner) run(_ context.Context, executable string, args []string, input []byte, maxOutput int) ([]byte, error) {
	runner.calls = append(runner.calls, secretToolCall{executable: executable, args: append([]string(nil), args...), input: append([]byte(nil), input...), maxOutput: maxOutput})
	if runner.runErr != nil {
		return nil, runner.runErr
	}
	if args[0] == "lookup" {
		return append([]byte(nil), runner.lookupOutput...), nil
	}
	return nil, nil
}
