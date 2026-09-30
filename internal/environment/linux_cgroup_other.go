//go:build !linux

// pattern: Imperative Shell
package environment

func NewLinuxNodeCgroupV2Manager(runtimeRoot, root, identity string, limits LinuxNodeResourceLimits, workerBubblewrapPaths ...string) (LinuxNodeCgroupManager, error) {
	return nil, ErrLinuxNodeCgroupUnavailable
}
