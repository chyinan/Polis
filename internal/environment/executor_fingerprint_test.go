// pattern: Functional Core
package environment

import (
	"runtime"
	"testing"
)

func TestCurrentEnvironmentExecutorFingerprintBindsBinaryHostAndPolicy(t *testing.T) {
	windows, err := CurrentEnvironmentExecutorFingerprint(WindowsNodeNPMProfile)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows Node workspace volume is not provisioned: %v", err)
		}
		t.Fatal(err)
	}
	linux, err := CurrentEnvironmentExecutorFingerprint(LinuxNodeNPMProfile)
	if err != nil {
		t.Fatal(err)
	}
	for label, digest := range map[string]string{
		"Windows executor binary":  windows.ExecutorSHA256,
		"Windows host":             windows.HostSHA256,
		"Windows isolation policy": windows.IsolationPolicySHA256,
		"Linux executor binary":    linux.ExecutorSHA256,
		"Linux host":               linux.HostSHA256,
		"Linux isolation policy":   linux.IsolationPolicySHA256,
	} {
		if !validEnvironmentFingerprintDigest(digest) {
			t.Fatalf("%s fingerprint %q is invalid", label, digest)
		}
	}
	if windows.ExecutorSHA256 != linux.ExecutorSHA256 || windows.HostSHA256 != linux.HostSHA256 {
		t.Fatal("the current process binary or host fingerprint changed across isolation profiles")
	}
	if windows.IsolationPolicySHA256 == linux.IsolationPolicySHA256 {
		t.Fatal("Windows and Linux isolation policies share an executor fingerprint")
	}
}

func TestEnvironmentIsolationPolicyFingerprintRejectsUnsupportedProfile(t *testing.T) {
	if _, ok := IsolationPolicyFingerprint("arbitrary-node-profile"); ok {
		t.Fatal("unsupported execution profile received an isolation policy fingerprint")
	}
}

func TestWindowsWorkspaceVolumeBindingAdvancesIsolationPolicyFingerprint(t *testing.T) {
	got, ok := IsolationPolicyFingerprint(WindowsNodeNPMProfile)
	if !ok {
		t.Fatal("Windows Node profile did not produce an isolation fingerprint")
	}
	isolationProfile, _ := IsolationProfileForNodeProfile(WindowsNodeNPMProfile)
	want := fingerprintSHA256("polis-executor-isolation-policy@1", WindowsNodeNPMProfile+"\x00"+isolationProfile+"\x00windows-appcontainer-node-policy@5")
	if got != want {
		t.Fatalf("Windows isolation policy fingerprint=%s, want service-listener-bound revision %s", got, want)
	}
}
