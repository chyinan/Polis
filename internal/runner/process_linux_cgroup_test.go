//go:build linux

// pattern: Functional Core
package runner

import (
	"syscall"
	"testing"
)

func TestLinuxCgroupLaunchAttrUsesAtomicCgroupPlacement(t *testing.T) {
	attr := linuxProcessSysProcAttr(42)
	if attr == nil || !attr.UseCgroupFD || attr.CgroupFD != 42 || !attr.Setpgid || attr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("cgroup launch attributes=%+v", attr)
	}
}

func TestLinuxUnboundedLaunchAttrDoesNotSelectACgroup(t *testing.T) {
	attr := linuxProcessSysProcAttr(-1)
	if attr == nil || attr.UseCgroupFD || attr.CgroupFD != 0 || !attr.Setpgid || attr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("ordinary launch attributes=%+v", attr)
	}
}
