// pattern: Functional Core
package environment

import (
	"strings"
	"testing"
)

func TestValidateWindowsNodeWorkspaceStorageRequiresDedicatedBoundedNTFSVolume(t *testing.T) {
	valid := WindowsNodeWorkspaceStorageInfo{
		WorkspaceRoot: `W:\PolisWorkspace`, VolumeRoot: `W:\`, VolumeGUID: `\\?\Volume{12345678-1234-1234-1234-123456789abc}\`, VolumeSerial: 22, VolumeLabel: "POLIS_WORKSPACE", FileSystem: "NTFS",
		TotalBytes: 8 << 30, AvailableBytes: 4 << 30,
	}
	if err := ValidateWindowsNodeWorkspaceStorageInfo(valid, 11, 12); err != nil {
		t.Fatalf("valid dedicated workspace volume rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*WindowsNodeWorkspaceStorageInfo)
	}{
		{"missing root", func(info *WindowsNodeWorkspaceStorageInfo) { info.WorkspaceRoot = "" }},
		{"not NTFS", func(info *WindowsNodeWorkspaceStorageInfo) { info.FileSystem = "ReFS" }},
		{"invalid volume GUID", func(info *WindowsNodeWorkspaceStorageInfo) { info.VolumeGUID = "W:\\" }},
		{"wrong volume label", func(info *WindowsNodeWorkspaceStorageInfo) { info.VolumeLabel = "DATA" }},
		{"shares control volume", func(info *WindowsNodeWorkspaceStorageInfo) { info.VolumeSerial = 11 }},
		{"shares system volume", func(info *WindowsNodeWorkspaceStorageInfo) { info.VolumeSerial = 12 }},
		{"too small", func(info *WindowsNodeWorkspaceStorageInfo) { info.TotalBytes = 256 << 20 }},
		{"above hard capacity", func(info *WindowsNodeWorkspaceStorageInfo) {
			info.TotalBytes = WindowsNodeWorkspaceVolumeLimitBytes + 1
		}},
		{"no available space", func(info *WindowsNodeWorkspaceStorageInfo) { info.AvailableBytes = 0 }},
		{"workspace outside volume", func(info *WindowsNodeWorkspaceStorageInfo) { info.WorkspaceRoot = `X:\Polis\Workspace` }},
		{"nested workspace base", func(info *WindowsNodeWorkspaceStorageInfo) { info.WorkspaceRoot = `W:\Polis\Workspace` }},
		{"available exceeds volume", func(info *WindowsNodeWorkspaceStorageInfo) { info.AvailableBytes = info.TotalBytes + 1 }},
		{"zero volume identity", func(info *WindowsNodeWorkspaceStorageInfo) { info.VolumeSerial = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := valid
			test.mutate(&info)
			if err := ValidateWindowsNodeWorkspaceStorageInfo(info, 11, 12); err == nil {
				t.Fatalf("invalid workspace volume was accepted: %+v", info)
			}
		})
	}
}

func TestWindowsWorkspaceStorageFingerprintBindsVolumeIdentityAndCapacity(t *testing.T) {
	info := WindowsNodeWorkspaceStorageInfo{
		WorkspaceRoot: `W:\PolisWorkspace`, VolumeRoot: `W:\`, VolumeGUID: `\\?\Volume{12345678-1234-1234-1234-123456789abc}\`, VolumeSerial: 22, VolumeLabel: "POLIS_WORKSPACE",
		FileSystem: "NTFS", TotalBytes: 8 << 30, AvailableBytes: 4 << 30,
	}
	policy := strings.Repeat("a", 64)
	base, err := WindowsWorkspaceStorageFingerprint(policy, info)
	if err != nil {
		t.Fatal(err)
	}
	changedSerial := info
	changedSerial.VolumeSerial++
	serialDigest, err := WindowsWorkspaceStorageFingerprint(policy, changedSerial)
	if err != nil || serialDigest == base {
		t.Fatalf("volume serial change did not change fingerprint: digest=%s err=%v", serialDigest, err)
	}
	changedCapacity := info
	changedCapacity.TotalBytes -= 1 << 20
	capacityDigest, err := WindowsWorkspaceStorageFingerprint(policy, changedCapacity)
	if err != nil || capacityDigest == base {
		t.Fatalf("volume capacity change did not change fingerprint: digest=%s err=%v", capacityDigest, err)
	}
	changedFreeSpace := info
	changedFreeSpace.AvailableBytes--
	freeSpaceDigest, err := WindowsWorkspaceStorageFingerprint(policy, changedFreeSpace)
	if err != nil || freeSpaceDigest != base {
		t.Fatalf("volatile free-space change changed fingerprint: digest=%s want=%s err=%v", freeSpaceDigest, base, err)
	}
}

func TestWindowsWorkspacePreparationRequiresTheSameAuthorizedStorageFingerprint(t *testing.T) {
	authorized := strings.Repeat("a", 64)
	if !WindowsWorkspacePreparationFingerprintMatches(authorized, authorized) {
		t.Fatal("unchanged authorized workspace fingerprint was rejected")
	}
	if WindowsWorkspacePreparationFingerprintMatches(authorized, strings.Repeat("b", 64)) {
		t.Fatal("changed workspace volume fingerprint was accepted")
	}
	if WindowsWorkspacePreparationFingerprintMatches("", authorized) || WindowsWorkspacePreparationFingerprintMatches(authorized, "") {
		t.Fatal("missing workspace authorization fingerprint was accepted")
	}
}
