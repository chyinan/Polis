//go:build linux

// pattern: Imperative Shell
package environment

import (
	"fmt"
	"os"
	"syscall"
)

func VerifyLinuxNodeWorkspaceFilesystem(paths LinuxNodeSandboxPaths) error {
	if err := ValidateLinuxNodeSandboxPaths(paths); err != nil {
		return fmt.Errorf("%w: invalid sandbox paths: %v", ErrLinuxNodeWorkspaceDiskLimitUnavailable, err)
	}
	snapshot, err := inspectLinuxNodeWorkspaceFilesystem(paths)
	if err != nil {
		return fmt.Errorf("%w: inspect workspace filesystem: %v", ErrLinuxNodeWorkspaceDiskLimitUnavailable, err)
	}
	if err = ValidateLinuxNodeWorkspaceFilesystemSnapshot(snapshot, EffectiveLinuxNodeWorkspaceDiskLimitBytes(paths)); err != nil {
		return err
	}
	return nil
}

func inspectLinuxNodeWorkspaceFilesystem(paths LinuxNodeSandboxPaths) (LinuxNodeWorkspaceFilesystemSnapshot, error) {
	devices := make([]uint64, 0, 4)
	for _, path := range []string{paths.RuntimeRoot, paths.SystemImageRoot, paths.WorkspaceRoot, paths.NPMCacheRoot, paths.CgroupRoot} {
		info, err := os.Stat(path)
		if err != nil {
			return LinuxNodeWorkspaceFilesystemSnapshot{}, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return LinuxNodeWorkspaceFilesystemSnapshot{}, fmt.Errorf("stat data is unavailable for %s", path)
		}
		devices = append(devices, uint64(stat.Dev))
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(paths.WorkspaceRoot, &filesystem); err != nil {
		return LinuxNodeWorkspaceFilesystemSnapshot{}, err
	}
	blockSize := filesystem.Frsize
	if blockSize <= 0 {
		blockSize = filesystem.Bsize
	}
	if blockSize <= 0 {
		return LinuxNodeWorkspaceFilesystemSnapshot{}, fmt.Errorf("filesystem block size is unavailable")
	}
	blockSizeBytes := uint64(blockSize)
	totalBlocks := uint64(filesystem.Blocks)
	if totalBlocks > ^uint64(0)/blockSizeBytes {
		return LinuxNodeWorkspaceFilesystemSnapshot{}, fmt.Errorf("filesystem capacity overflows byte count")
	}
	return LinuxNodeWorkspaceFilesystemSnapshot{
		RuntimeDevice: devices[0], SystemImageDevice: devices[1], WorkspaceDevice: devices[2], NPMCacheDevice: devices[3], CgroupDevice: devices[4],
		FilesystemType: uint64(filesystem.Type), CapacityBytes: totalBlocks * blockSizeBytes,
	}, nil
}
