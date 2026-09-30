// pattern: Functional Core
package environment

import (
	"errors"
	"testing"
)

func TestValidateLinuxNodeWorkspaceFilesystemSnapshot(t *testing.T) {
	valid := LinuxNodeWorkspaceFilesystemSnapshot{
		RuntimeDevice: 1, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 1, CgroupDevice: 3,
		FilesystemType: LinuxNodeXFSFilesystemMagic, CapacityBytes: 15 << 30,
	}
	tests := []struct {
		name     string
		snapshot LinuxNodeWorkspaceFilesystemSnapshot
		limit    uint64
		wantErr  bool
	}{
		{name: "dedicated sized XFS filesystem", snapshot: valid, limit: 16 << 30},
		{name: "dedicated sized ext filesystem", snapshot: LinuxNodeWorkspaceFilesystemSnapshot{RuntimeDevice: 1, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 1, CgroupDevice: 3, FilesystemType: LinuxNodeExtFilesystemMagic, CapacityBytes: 8 << 30}, limit: 16 << 30},
		{name: "same device as runtime", snapshot: LinuxNodeWorkspaceFilesystemSnapshot{RuntimeDevice: 2, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 1, CgroupDevice: 3, FilesystemType: LinuxNodeXFSFilesystemMagic, CapacityBytes: 8 << 30}, limit: 16 << 30, wantErr: true},
		{name: "same device as cache", snapshot: LinuxNodeWorkspaceFilesystemSnapshot{RuntimeDevice: 1, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 2, CgroupDevice: 3, FilesystemType: LinuxNodeXFSFilesystemMagic, CapacityBytes: 8 << 30}, limit: 16 << 30, wantErr: true},
		{name: "filesystem larger than configured hard bound", snapshot: valid, limit: 8 << 30, wantErr: true},
		{name: "unsupported filesystem", snapshot: LinuxNodeWorkspaceFilesystemSnapshot{RuntimeDevice: 1, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 1, CgroupDevice: 3, FilesystemType: 0x01021994, CapacityBytes: 8 << 30}, limit: 16 << 30, wantErr: true},
		{name: "zero capacity", snapshot: LinuxNodeWorkspaceFilesystemSnapshot{RuntimeDevice: 1, SystemImageDevice: 1, WorkspaceDevice: 2, NPMCacheDevice: 1, CgroupDevice: 3, FilesystemType: LinuxNodeXFSFilesystemMagic}, limit: 16 << 30, wantErr: true},
		{name: "zero policy cap", snapshot: valid, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateLinuxNodeWorkspaceFilesystemSnapshot(test.snapshot, test.limit)
			if errors.Is(err, ErrLinuxNodeWorkspaceDiskLimitUnavailable) != test.wantErr {
				t.Fatalf("validation error=%v wantErr=%t", err, test.wantErr)
			}
		})
	}
}
