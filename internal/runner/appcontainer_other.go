// pattern: Imperative Shell
//go:build !windows

package runner

func newAppContainerBackend(string) (appContainerBackend, error) {
	return nil, ErrAppContainerUnavailable
}

func newRegistryEgressAppContainerBackend(string, string, RegistryEgressPlan, AppContainerWorkspaceStorageBinding) (appContainerBackend, error) {
	return nil, ErrAppContainerUnavailable
}
