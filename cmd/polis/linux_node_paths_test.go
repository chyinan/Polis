// pattern: Imperative Shell
package main

import (
	"strings"
	"testing"

	"polis/internal/environment"
)

func TestLinuxNodeSandboxPathsParsesBoundedWorkspaceDiskLimit(t *testing.T) {
	t.Setenv("POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES", "")
	paths, err := linuxNodeSandboxPathsFromEnvironment()
	if err != nil || paths.WorkspaceDiskLimitBytes != environment.DefaultLinuxNodeWorkspaceDiskLimitBytes {
		t.Fatalf("default workspace disk limit=%d error=%v", paths.WorkspaceDiskLimitBytes, err)
	}
	t.Setenv("POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES", "8589934592")
	paths, err = linuxNodeSandboxPathsFromEnvironment()
	if err != nil || paths.WorkspaceDiskLimitBytes != 8<<30 {
		t.Fatalf("configured workspace disk limit=%d error=%v", paths.WorkspaceDiskLimitBytes, err)
	}
}

func TestLinuxNodeInstanceIdentityBindsDatabaseAndBlobRootWithoutPassword(t *testing.T) {
	base := "postgres://polis_runtime:secret@localhost:5432/polis_fixture?sslmode=disable"
	identity, err := linuxNodeInstanceIdentity(base, "/var/lib/polis/blobs")
	if err != nil || len(identity) != 64 {
		t.Fatalf("valid local instance identity=%q error=%v", identity, err)
	}
	rotatedPassword, err := linuxNodeInstanceIdentity("postgres://polis_runtime:new-secret@localhost:5432/polis_fixture?sslmode=disable", "/var/lib/polis/blobs")
	if err != nil || rotatedPassword != identity {
		t.Fatalf("password rotation changed installation identity=%q original=%q error=%v", rotatedPassword, identity, err)
	}
	differentServer, err := linuxNodeInstanceIdentity("postgres://polis_runtime:secret@db.internal:5432/polis_fixture?sslmode=disable", "/var/lib/polis/blobs")
	if err != nil || differentServer == identity {
		t.Fatalf("different PostgreSQL host reused installation identity=%q error=%v", differentServer, err)
	}
	differentPort, err := linuxNodeInstanceIdentity("postgres://polis_runtime:secret@localhost:5433/polis_fixture?sslmode=disable", "/var/lib/polis/blobs")
	if err != nil || differentPort == identity {
		t.Fatalf("different PostgreSQL port reused installation identity=%q error=%v", differentPort, err)
	}
	caseSensitiveSocket, err := linuxNodeInstanceIdentity("host=/var/run/PG-A dbname=polis_fixture user=polis_runtime port=5432", "/var/lib/polis/blobs")
	if err != nil {
		t.Fatalf("parse uppercase Unix socket identity: %v", err)
	}
	differentCaseSocket, err := linuxNodeInstanceIdentity("host=/var/run/pg-a dbname=polis_fixture user=polis_runtime port=5432", "/var/lib/polis/blobs")
	if err != nil || differentCaseSocket == caseSensitiveSocket {
		t.Fatalf("case-sensitive Unix socket directories collided: upper=%q lower=%q error=%v", caseSensitiveSocket, differentCaseSocket, err)
	}
	upperDNS, err := linuxNodeInstanceIdentity("postgres://polis_runtime:secret@DB.INTERNAL:5432/polis_fixture?sslmode=disable", "/var/lib/polis/blobs")
	lowerDNS, lowerErr := linuxNodeInstanceIdentity("postgres://polis_runtime:secret@db.internal:5432/polis_fixture?sslmode=disable", "/var/lib/polis/blobs")
	if err != nil || lowerErr != nil || upperDNS != lowerDNS {
		t.Fatalf("DNS hostname case changed identity: upper=%q lower=%q errors=(%v,%v)", upperDNS, lowerDNS, err, lowerErr)
	}
	differentDatabase, err := linuxNodeInstanceIdentity("postgres://polis_runtime:secret@localhost:5432/another_db?sslmode=disable", "/var/lib/polis/blobs")
	if err != nil || differentDatabase == identity {
		t.Fatalf("different database reused installation identity=%q error=%v", differentDatabase, err)
	}
	differentBlobRoot, err := linuxNodeInstanceIdentity(base, "/var/lib/polis/other-blobs")
	if err != nil || differentBlobRoot == identity {
		t.Fatalf("different blob root reused installation identity=%q error=%v", differentBlobRoot, err)
	}
	canonicalBlobRoot, err := linuxNodeInstanceIdentity(base, "/var/lib/polis/blobs/")
	if err != nil || canonicalBlobRoot != identity {
		t.Fatalf("equivalent BlobRoot path changed installation identity=%q original=%q error=%v", canonicalBlobRoot, identity, err)
	}
	if _, err = linuxNodeInstanceIdentity(base, "relative/blobs"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("invalid root identity error leaked or accepted configuration: %v", err)
	}
}

func TestLinuxNodeSandboxPathsRejectsMalformedWorkspaceDiskLimit(t *testing.T) {
	for _, value := range []string{"0", "not-bytes", "-1"} {
		t.Setenv("POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES", value)
		if _, err := linuxNodeSandboxPathsFromEnvironment(); err == nil {
			t.Errorf("workspace disk limit %q was accepted", value)
		}
	}
}

func TestLinuxNodeStartupRequiresRecoveryRootForUnrestoredWork(t *testing.T) {
	if err := validateLinuxNodeStartupRecoveryRoot("linux", true, "", ""); err == nil {
		t.Fatal("Linux startup accepted unrestored Node work without its runtime and delegated cgroup roots")
	}
	if err := validateLinuxNodeStartupRecoveryRoot("linux", true, "/run/polis/runtime", "/run/polis/runtime/linux-node-cgroups"); err != nil {
		t.Fatalf("configured Linux recovery root was rejected: %v", err)
	}
	if err := validateLinuxNodeStartupRecoveryRoot("linux", false, "", ""); err != nil {
		t.Fatalf("Linux startup without outstanding work required a cgroup root: %v", err)
	}
	if err := validateLinuxNodeStartupRecoveryRoot("linux", false, "/var/lib/polis/runtime", ""); err != nil {
		t.Fatalf("Linux startup with an unused runtime root required a cgroup root: %v", err)
	}
	if err := validateLinuxNodeStartupRecoveryRoot("linux", false, "", "/run/polis/runtime/linux-node-cgroups"); err == nil {
		t.Fatal("Linux startup accepted a cgroup root without its runtime root")
	}
	if err := validateLinuxNodeStartupRecoveryRoot("windows", true, "", ""); err != nil {
		t.Fatalf("Windows startup incorrectly required a Linux cgroup root: %v", err)
	}
}
