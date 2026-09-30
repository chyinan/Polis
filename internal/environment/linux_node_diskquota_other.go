//go:build !linux

// pattern: Imperative Shell
package environment

func VerifyLinuxNodeWorkspaceFilesystem(LinuxNodeSandboxPaths) error {
	return ErrLinuxNodeWorkspaceDiskLimitUnavailable
}
