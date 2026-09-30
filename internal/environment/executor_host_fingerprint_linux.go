//go:build linux

// pattern: Imperative Shell
package environment

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func currentHostBuildIdentity() (string, error) {
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil || len(osRelease) == 0 || len(osRelease) > 64<<10 {
		return "", fmt.Errorf("Linux distribution identity is unavailable")
	}
	kernelRelease, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil || len(kernelRelease) == 0 || len(kernelRelease) > 1024 {
		return "", fmt.Errorf("Linux kernel identity is unavailable")
	}
	return strings.Join([]string{"linux", runtime.GOARCH, strings.TrimSpace(string(kernelRelease)), strings.TrimSpace(string(osRelease))}, "\x00"), nil
}
