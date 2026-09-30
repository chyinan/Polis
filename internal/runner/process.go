// pattern: Imperative Shell

package runner

import (
	"errors"
	"fmt"
	"strings"
)

var ErrProcessTreeStopUnconfirmed = errors.New("process-tree stop is unconfirmed")
var ErrLinuxProcessGroupStopUnconfirmed = errors.New("Linux process-group stop is unconfirmed")

type ProcessExitError struct {
	ExitCode uint32
}

func (err ProcessExitError) Error() string {
	return fmt.Sprintf("process exited with code %d", err.ExitCode)
}

type ProcessContainmentMetadata struct {
	HostOS       string `json:"host_os"`
	Profile      string `json:"profile"`
	CgroupID     string `json:"cgroup_id,omitempty"`
	CgroupHostID string `json:"cgroup_host_id,omitempty"`
	CgroupRootID string `json:"cgroup_root_id,omitempty"`
	CgroupBootID string `json:"cgroup_boot_id,omitempty"`
}

type WorkerProcessCgroup interface {
	FileDescriptor() int
	Name() string
	CgroupRootPath() string
	BubblewrapPath() string
	HostIdentity() string
	RootIdentity() string
	BootID() string
	Cleanup() error
}

func (metadata ProcessContainmentMetadata) Valid() bool {
	if metadata.HostOS == "windows" {
		return metadata.Profile == "windows_worker_job_object@1" && metadata.CgroupID == "" && metadata.CgroupHostID == "" && metadata.CgroupRootID == "" && metadata.CgroupBootID == ""
	}
	if metadata.HostOS != "linux" {
		return false
	}
	switch metadata.Profile {
	case "linux_process_group@1", "linux_cgroup_process_group@1":
		return metadata.CgroupID == "" && metadata.CgroupHostID == "" && metadata.CgroupRootID == "" && metadata.CgroupBootID == ""
	case "linux_worker_cgroup_v2@1":
		return validLinuxWorkerCgroupID(metadata.CgroupID) && validLinuxWorkerCgroupHostID(metadata.CgroupHostID) &&
			validLinuxWorkerCgroupHostID(metadata.CgroupRootID) && validLinuxWorkerCgroupBootID(metadata.CgroupBootID)
	default:
		return false
	}
}

func validLinuxWorkerCgroupBootID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validLinuxWorkerCgroupHostID(identity string) bool {
	if len(identity) != 64 {
		return false
	}
	for _, character := range identity {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validLinuxWorkerCgroupID(name string) bool {
	if !strings.HasPrefix(name, "polis-worker-") {
		return false
	}
	suffix := strings.TrimPrefix(name, "polis-worker-")
	if len(suffix) != 24 {
		return false
	}
	for _, character := range suffix {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

// StopProof cannot be fabricated by a model or constructed outside this package.
type StopProof struct {
	id      string
	pid     int
	stopped bool
}

// WindowsProcessTreeStopProof is returned only after the persisted named Job
// Object is absent or its active-process count reaches zero.
type WindowsProcessTreeStopProof struct {
	identity string
	pid      int
	scope    string
	stopped  bool
}

// LinuxProcessGroupStopProof is returned only after the persisted process group
// has no observable members in procfs.
type LinuxProcessGroupStopProof struct {
	identity       string
	processGroupID int
	stopped        bool
}

func (proof WindowsProcessTreeStopProof) For(identity string) bool {
	return proof.stopped && proof.scope == "appcontainer" && identity != "" && proof.identity == identity
}

func (proof WindowsProcessTreeStopProof) ForWorkerSession(identity string, pid int) bool {
	return proof.stopped && proof.scope == "worker_session" && identity != "" && proof.identity == identity && proof.pid == pid
}

func (proof LinuxProcessGroupStopProof) ForWorkerSession(identity string, processGroupID int) bool {
	return proof.stopped && identity != "" && proof.identity == identity && processGroupID > 0 && proof.processGroupID == processGroupID
}

func (p StopProof) For(id string) bool { return p.stopped && p.id == id && p.pid > 0 }
func (p StopProof) PID() int           { return p.pid }

func processStopDescription(prefix string, pid int) string {
	return fmt.Sprintf("%s:%d:waited", prefix, pid)
}
