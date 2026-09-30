// pattern: Functional Core
package runner

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAppContainerLaunchSpecRequiresContainedNoNetworkInputs(t *testing.T) {
	root := t.TempDir()
	valid := AppContainerLaunchSpec{
		ID: "job-1", WorkspaceRoot: root, Executable: filepath.Join(root, "bin", "node.exe"),
		Argv:             []string{filepath.Join(root, "bin", "node.exe"), "app.js"},
		WorkingDirectory: root, NetworkPolicy: AppContainerNetworkDenyAll,
		Environment: []string{
			"HOME=" + filepath.Join(root, "home"), "LOCALAPPDATA=" + filepath.Join(root, "appdata"), "PATH=C:\\Windows\\System32",
			"SYSTEMROOT=C:\\Windows", "TEMP=" + filepath.Join(root, "tmp"),
			"TMP=" + filepath.Join(root, "tmp"), "USERPROFILE=" + filepath.Join(root, "home"),
		},
	}
	if err := ValidateAppContainerLaunchSpec(valid); err != nil {
		t.Fatalf("valid no-network AppContainer launch spec rejected: %v", err)
	}
	serviceLaunch := valid
	serviceLaunch.ServiceListener = &AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 43123}
	if err := ValidateAppContainerLaunchSpec(serviceLaunch); err != nil {
		t.Fatalf("fixed loopback service launch rejected: %v", err)
	}
	for name, mutate := range map[string]func(*AppContainerLaunchSpec){
		"registry egress plus service listener": func(spec *AppContainerLaunchSpec) {
			spec.NetworkPolicy = AppContainerNetworkRegistryOnly
			spec.RegistryProxyEndpoint = "127.0.0.1:43124"
		},
		"wildcard service bind": func(spec *AppContainerLaunchSpec) {
			spec.ServiceListener = &AppContainerServiceListenerSpec{BindAddress: "0.0.0.0", Port: 43123}
		},
		"zero service port": func(spec *AppContainerLaunchSpec) {
			spec.ServiceListener = &AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 0}
		},
	} {
		t.Run("service-listener/"+name, func(t *testing.T) {
			candidate := serviceLaunch
			mutate(&candidate)
			if err := ValidateAppContainerLaunchSpec(candidate); err == nil {
				t.Fatal("service listener accepted a network policy outside fixed loopback-only service mode")
			}
		})
	}

	outsideExecutable := valid
	outsideExecutable.Argv = append([]string(nil), valid.Argv...)
	outsideExecutable.Executable = `C:\Windows\System32\cmd.exe`
	outsideExecutable.Argv[0] = outsideExecutable.Executable
	if err := ValidateAppContainerLaunchSpec(outsideExecutable); err == nil {
		t.Fatal("executable outside the appcontainer workspace was accepted")
	}

	registryNetwork := valid
	registryNetwork.NetworkPolicy = AppContainerNetworkRegistryOnly
	registryNetwork.RegistryProxyEndpoint = "127.0.0.1:43123"
	registryNetwork.Environment = append([]string(nil), valid.Environment...)
	registryNetwork.Environment[2] = `PATH=C:\Windows\System32;C:\Windows`
	registryNetwork.Executable = filepath.Join(root, ".polis-toolchain", "node.exe")
	registryNetwork.Argv = []string{
		registryNetwork.Executable, filepath.Join(root, ".polis-toolchain", "npm-cli.js"),
		"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org/",
	}
	if err := ValidateAppContainerLaunchSpec(registryNetwork); err != nil {
		t.Fatalf("valid AppContainer registry-proxy policy rejected: %v", err)
	}
	for name, mutate := range map[string]func(*AppContainerLaunchSpec){
		"arbitrary npm script":      func(spec *AppContainerLaunchSpec) { spec.Argv[2] = "run" },
		"lifecycle scripts enabled": func(spec *AppContainerLaunchSpec) { spec.Argv[3] = "--ignore-scripts=false" },
		"different executable": func(spec *AppContainerLaunchSpec) {
			spec.Executable = filepath.Join(root, ".polis-toolchain", "evil.exe")
			spec.Argv[0] = spec.Executable
		},
		"project npm cli":       func(spec *AppContainerLaunchSpec) { spec.Argv[1] = filepath.Join(root, "npm-cli.js") },
		"noncanonical registry": func(spec *AppContainerLaunchSpec) { spec.Argv[6] = "--registry=https://REGISTRY.NPMJS.ORG/" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := registryNetwork
			candidate.Argv = append([]string(nil), registryNetwork.Argv...)
			mutate(&candidate)
			if err := ValidateAppContainerLaunchSpec(candidate); err == nil {
				t.Fatal("registry-only sandbox accepted a launch outside the fixed npm installer contract")
			}
		})
	}
	for name, mutate := range map[string]func(*AppContainerLaunchSpec){
		"node options override": func(spec *AppContainerLaunchSpec) {
			spec.Environment = append(spec.Environment, "NODE_OPTIONS=--require=project.js")
		},
		"node path override": func(spec *AppContainerLaunchSpec) {
			spec.Environment = append(spec.Environment, "NODE_PATH=C:\\project")
		},
		"path override": func(spec *AppContainerLaunchSpec) { spec.Environment[2] = "PATH=C:\\project" },
	} {
		t.Run("environment/"+name, func(t *testing.T) {
			candidate := registryNetwork
			candidate.Argv = append([]string(nil), registryNetwork.Argv...)
			candidate.Environment = append([]string(nil), registryNetwork.Environment...)
			mutate(&candidate)
			if err := ValidateAppContainerLaunchSpec(candidate); err == nil {
				t.Fatal("registry-only npm environment accepted an override")
			}
		})
	}
	for _, endpoint := range []string{"127.0.0.1:0", "127.0.0.2:43123", "8.8.8.8:43123", "[::1]:43123"} {
		invalidNetwork := registryNetwork
		invalidNetwork.RegistryProxyEndpoint = endpoint
		if err := ValidateAppContainerLaunchSpec(invalidNetwork); err == nil {
			t.Fatalf("AppContainer registry proxy endpoint %q was accepted", endpoint)
		}
	}

	secretEnvironment := valid
	secretEnvironment.Environment = append(secretEnvironment.Environment, "API_TOKEN=must-not-enter-child")
	if err := ValidateAppContainerLaunchSpec(secretEnvironment); err == nil {
		t.Fatal("secret-bearing environment was accepted")
	}
}

func TestBuildAppContainerEnvironmentUsesOnlyControlledWorkspaceDirectories(t *testing.T) {
	root := `C:\Users\test\AppData\Local\Packages\Polis\AC`
	environment := BuildAppContainerEnvironment(root, `C:\Windows`)
	if len(environment) != 7 {
		t.Fatalf("controlled AppContainer environment has %d entries: %v", len(environment), environment)
	}
	values := make(map[string]string, len(environment))
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			t.Fatalf("invalid environment entry: %q", item)
		}
		values[name] = value
	}
	for _, name := range []string{"HOME", "USERPROFILE", "TEMP", "TMP", "LOCALAPPDATA"} {
		if !strings.HasPrefix(values[name], root) {
			t.Fatalf("%s escaped appcontainer root: %q", name, values[name])
		}
	}
	if values["PATH"] != `C:\Windows\System32;C:\Windows` || values["SYSTEMROOT"] != `C:\Windows` {
		t.Fatalf("system environment is not pinned: %v", values)
	}
}

func TestRegistryOnlySandboxExposesEndpointWithoutCredentials(t *testing.T) {
	endpoint := "127.0.0.1:43123"
	sandbox := &AppContainerSandbox{
		networkPolicy:         AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: endpoint,
	}
	if got := sandbox.RegistryProxyEndpoint(); got != endpoint {
		t.Fatalf("registry proxy endpoint = %q, want %q", got, endpoint)
	}
	if got := (&AppContainerSandbox{networkPolicy: AppContainerNetworkDenyAll, registryProxyEndpoint: endpoint}).RegistryProxyEndpoint(); got != "" {
		t.Fatalf("deny-all sandbox exposed a registry proxy endpoint: %q", got)
	}
	var nilSandbox *AppContainerSandbox
	if got := nilSandbox.RegistryProxyEndpoint(); got != "" {
		t.Fatalf("nil sandbox exposed a registry proxy endpoint: %q", got)
	}
}
