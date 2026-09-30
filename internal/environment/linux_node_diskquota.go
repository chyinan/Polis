// pattern: Functional Core
package environment

import (
	"errors"
	"fmt"
)

const (
	// Linux reports ext2, ext3, and ext4 with the same statfs magic.
	LinuxNodeExtFilesystemMagic             uint64 = 0x0000ef53
	LinuxNodeXFSFilesystemMagic             uint64 = 0x58465342
	DefaultLinuxNodeWorkspaceDiskLimitBytes uint64 = 16 << 30
)

var ErrLinuxNodeWorkspaceDiskLimitUnavailable = errors.New("Linux Node/npm workspace disk bound is unavailable")

type LinuxNodeWorkspaceFilesystemSnapshot struct {
	RuntimeDevice     uint64
	SystemImageDevice uint64
	WorkspaceDevice   uint64
	NPMCacheDevice    uint64
	CgroupDevice      uint64
	FilesystemType    uint64
	CapacityBytes     uint64
}

func EffectiveLinuxNodeWorkspaceDiskLimitBytes(paths LinuxNodeSandboxPaths) uint64 {
	if paths.WorkspaceDiskLimitBytes != 0 {
		return paths.WorkspaceDiskLimitBytes
	}
	return DefaultLinuxNodeWorkspaceDiskLimitBytes
}

func ValidateLinuxNodeWorkspaceFilesystemSnapshot(snapshot LinuxNodeWorkspaceFilesystemSnapshot, limitBytes uint64) error {
	if limitBytes == 0 || snapshot.WorkspaceDevice == 0 {
		return fmt.Errorf("%w: configured limit and workspace filesystem identity are required", ErrLinuxNodeWorkspaceDiskLimitUnavailable)
	}
	for _, device := range []uint64{snapshot.RuntimeDevice, snapshot.SystemImageDevice, snapshot.NPMCacheDevice, snapshot.CgroupDevice} {
		if snapshot.WorkspaceDevice == device {
			return fmt.Errorf("%w: workspace root must use a separate filesystem", ErrLinuxNodeWorkspaceDiskLimitUnavailable)
		}
	}
	if snapshot.FilesystemType != LinuxNodeExtFilesystemMagic && snapshot.FilesystemType != LinuxNodeXFSFilesystemMagic {
		return fmt.Errorf("%w: workspace filesystem must be ext-family or XFS", ErrLinuxNodeWorkspaceDiskLimitUnavailable)
	}
	if snapshot.CapacityBytes == 0 || snapshot.CapacityBytes > limitBytes {
		return fmt.Errorf("%w: workspace filesystem capacity exceeds its configured maximum", ErrLinuxNodeWorkspaceDiskLimitUnavailable)
	}
	return nil
}
