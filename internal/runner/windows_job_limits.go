package runner

import "errors"

const (
	minWindowsJobMemoryBytes  uint64 = 64 << 20
	maxWindowsJobMemoryBytes  uint64 = 64 << 30
	maxWindowsJobProcesses           = 256
	windowsJobCPUPercentScale uint32 = 100
)

// WindowsJobResourceLimits is the resource envelope applied to one AppContainer
// Job Object. The Windows adapter applies CPU, per-process memory, aggregate
// Job memory, and process-count limits in the kernel.
type WindowsJobResourceLimits struct {
	CPUPercent         uint32
	ProcessMemoryBytes uint64
	JobMemoryBytes     uint64
	MaxActiveProcesses uint32
}

func DefaultWindowsJobResourceLimits() WindowsJobResourceLimits {
	return WindowsJobResourceLimits{
		CPUPercent:         50,
		ProcessMemoryBytes: 2 << 30,
		JobMemoryBytes:     2 << 30,
		MaxActiveProcesses: 64,
	}
}

func (limits WindowsJobResourceLimits) Validate() error {
	if limits.CPUPercent < 1 || limits.CPUPercent > 100 ||
		limits.ProcessMemoryBytes < minWindowsJobMemoryBytes || limits.ProcessMemoryBytes > maxWindowsJobMemoryBytes ||
		limits.JobMemoryBytes < limits.ProcessMemoryBytes || limits.JobMemoryBytes > maxWindowsJobMemoryBytes ||
		limits.MaxActiveProcesses < 1 || limits.MaxActiveProcesses > maxWindowsJobProcesses {
		return errors.New("Windows Job Object resource limits are outside the supported bounds")
	}
	return nil
}

func (limits WindowsJobResourceLimits) CPURateControlValue() (uint32, error) {
	if err := limits.Validate(); err != nil {
		return 0, err
	}
	return limits.CPUPercent * windowsJobCPUPercentScale, nil
}
