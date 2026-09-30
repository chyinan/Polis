//go:build windows

// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsAppContainerDefaultProfileWriteIsDenied(t *testing.T) {
	sandbox, err := NewAppContainerSandbox("profile-write-boundary-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := sandbox.Close(); closeErr != nil {
			t.Errorf("close AppContainer: %v", closeErr)
		}
	}()
	backend, ok := sandbox.backend.(*windowsAppContainerSandbox)
	if !ok || backend.sid == nil {
		t.Fatal("AppContainer profile identity is unavailable")
	}
	root := sandbox.WorkspaceRoot()
	workspace := filepath.Join(root, "workspace")
	if err = os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(workspace, "profile-write-helper.exe")
	source, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	helper, err := os.OpenFile(helperPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(helper, source); err != nil {
		_ = helper.Close()
		t.Fatal(err)
	}
	if err = helper.Close(); err != nil {
		t.Fatal(err)
	}
	if err = restrictWindowsAppContainerDefaultProfileWrites(root, backend.sid); err != nil {
		t.Fatalf("restrict default AppContainer profile writes: %v", err)
	}
	target := filepath.Join(root, "profile-write-probe.txt")
	environment := append(BuildAppContainerEnvironment(root, os.Getenv("SystemRoot")),
		"POLIS_TEST_PROFILE_WRITE=1")
	process, err := sandbox.Launch(AppContainerLaunchSpec{
		ID: "profile-write-probe", WorkspaceRoot: workspace, Executable: helperPath,
		Argv: []string{helperPath, "-test.run=TestWindowsAppContainerDefaultProfileWriteHelper", "-polis-profile-write-target=" + target}, WorkingDirectory: workspace,
		NetworkPolicy: AppContainerNetworkDenyAll, Environment: environment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdin := process.Stdin(); stdin != nil {
		_ = stdin.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	code, err := process.Wait(ctx)
	stdout, _ := io.ReadAll(process.Stdout())
	stderr, _ := io.ReadAll(process.Stderr())
	if err != nil || code != 0 {
		t.Fatalf("AppContainer write-denial helper exit=%d err=%v stdout=%s stderr=%s", code, err, stdout, stderr)
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("AppContainer wrote into its OS-managed default profile: err=%v", err)
	}
}

func TestWindowsAppContainerDefaultProfileWriteHelper(t *testing.T) {
	if os.Getenv("POLIS_TEST_PROFILE_WRITE") != "1" {
		return
	}
	if *appContainerProfileWriteTarget == "" {
		t.Fatal("write probe path is missing")
	}
	if err := os.WriteFile(*appContainerProfileWriteTarget, []byte("unbounded"), 0600); err == nil {
		t.Fatal("package SID unexpectedly wrote to its default AppContainer profile")
	} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("default profile write failed for the wrong reason: %v", err)
	}
}

var appContainerProfileWriteTarget = flag.String("polis-profile-write-target", "", "temporary target used by the AppContainer profile write-denial test")
