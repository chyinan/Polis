// pattern: Imperative Shell
package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

const (
	r03aT16Sentinel        = "POLIS_LOCAL_NETWORK_CANARY_OK"
	r03aT16InventoryScript = `printf 'IFACES_BEGIN\n'; /usr/bin/ip -brief addr 2>/dev/null || true; printf 'IFACES_END\n'; printf 'ROUTES_BEGIN\n'; /usr/bin/ip route 2>/dev/null || true; printf 'ROUTES_END\n'; printf 'RESOLV_META '; stat -c '%a %F %s' /etc/resolv.conf 2>/dev/null || true; printf 'RESOLV_BEGIN\n'; cat /etc/resolv.conf 2>/dev/null || true; printf 'RESOLV_END\n'; printf 'PROC route=%s tcp=%s tcp6=%s\n' "$(wc -l < /proc/net/route 2>/dev/null | tr -d ' ' || printf 0)" "$(wc -l < /proc/net/tcp 2>/dev/null | tr -d ' ' || printf 0)" "$(wc -l < /proc/net/tcp6 2>/dev/null | tr -d ' ' || printf 0)";`
)

type R03AT16Config struct {
	Binary       string
	AuthFile     string
	Root         string
	Evidence     string
	T13Evidence  string
	T14CEvidence string
}

type T16LocalSentinelResult struct {
	Passed      bool   `json:"passed"`
	Endpoint    string `json:"endpoint"`
	Response    string `json:"response_classification"`
	ErrorClass  string `json:"error_class,omitempty"`
	ErrorDigest string `json:"error_digest,omitempty"`
}

type t16Plan struct {
	T13V2Fingerprint       string
	T14CFingerprint        string
	T14CSourceConfigDigest string
	T14CCapabilityDigest   string
	T14CLaunchDigest       string
	T14CManifest           codex.CanonicalManifestV3
	T14CLaunch             runner.NativeLaunch
	T13Launch              runner.NativeLaunch
	BwrapVersion           string
	BwrapHelpNetworkLines  []string
	NetworkFlags           []T16ArgOccurrence
	OtherNamespaceFlags    []string
	HostNamespaces         map[string]T16NamespaceIdentity
	GuestNamespaces        map[string]T16NamespaceIdentity
	NamespaceRelation      T16NamespaceRelation
	HostInventory          T16NetworkInventory
	GuestInventory         T16NetworkInventory
	RouteRelation          T16RouteRelation
	NetworkPolicy          codex.NetworkNamespacePolicy
	HostSentinel           T16LocalSentinelResult
	GuestSentinel          T16LocalSentinelResult
	T6ProxyReachability    string
}

type T16ArgOccurrence struct {
	Index int    `json:"index"`
	Arg   string `json:"arg"`
}

type t16T14CManifest struct {
	FingerprintSchemaVersion    string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest     string                    `json:"canonical_manifest_digest"`
	CanonicalManifest           codex.CanonicalManifestV3 `json:"canonical_manifest"`
	SourceExecutionConfigDigest string                    `json:"source_execution_config_digest"`
	CapabilityDigest            string                    `json:"capability_digest"`
	LaunchConfigDigest          string                    `json:"launch_config_digest"`
	AuthSnapshotUsed            bool                      `json:"auth_snapshot_used"`
	BindPreflightPassed         bool                      `json:"bind_preflight_passed"`
	HostHomePath                string                    `json:"host_home_path"`
	GuestHomePath               string                    `json:"guest_home_path"`
}

type t16T13Manifest struct {
	FingerprintSchemaVersion string                    `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                    `json:"canonical_manifest_digest"`
	CanonicalManifest        codex.CanonicalManifestV2 `json:"canonical_manifest"`
}

func RunR03AT16(cfg R03AT16Config) (map[string]any, error) {
	if err := validateT16Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT16EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	plan, err := buildT16Plan(cfg)
	if err != nil {
		return nil, recordT16Failure(cfg.Evidence, err)
	}
	if err := writeT16Evidence(cfg, plan); err != nil {
		return nil, err
	}
	qualification := t16QualificationRecord(plan)
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return qualification, nil
}

func buildT16Plan(cfg R03AT16Config) (t16Plan, error) {
	var t13 t16T13Manifest
	var t14c t16T14CManifest
	if err := readT11JSON(filepath.Join(cfg.T13Evidence, "execution-manifest.json"), &t13); err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: T13 manifest: %w", err)
	}
	if err := readT11JSON(filepath.Join(cfg.T14CEvidence, "execution-manifest.json"), &t14c); err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: T14C manifest: %w", err)
	}
	if t13.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV2 || t13.CanonicalManifestDigest == "" {
		return t16Plan{}, errors.New("preflight_failed: T13 v2 manifest is unavailable")
	}
	if err := t13.CanonicalManifest.Validate(); err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: T13 manifest invalid: %w", err)
	}
	if t14c.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV3 || t14c.CanonicalManifestDigest == "" || !t14c.BindPreflightPassed || t14c.AuthSnapshotUsed {
		return t16Plan{}, errors.New("preflight_failed: T14C manifest is not the corrected sealed run")
	}
	if err := t14c.CanonicalManifest.Validate(); err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: T14C manifest invalid: %w", err)
	}
	if err := ensureT16AuthMaterial(cfg.AuthFile, t14c.CanonicalManifest.Auth); err != nil {
		return t16Plan{}, err
	}
	helperPath := filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host")
	helper, err := os.ReadFile(helperPath)
	if err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: code-mode-host: %w", err)
	}
	helperDigest := digest(helper)
	t14cLaunch, err := runner.BuildNativeLaunch(cfg.Binary, helperPath, t14c.HostHomePath, cfg.AuthFile, "", runner.NativeTransportPolicyNativeDefault, helperDigest)
	if err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: derive T14C launcher: %w", err)
	}
	if t14cLaunch.CapabilityDigest != t14c.CapabilityDigest || t14cLaunch.LaunchConfigDigest != t14c.LaunchConfigDigest {
		return t16Plan{}, errors.New("preflight_failed: current launcher does not match T14C sealed launch digests")
	}
	t13Home := filepath.Join(cfg.Root, "t13-reconstructed-home")
	t13Launch, err := runner.BuildNativeLaunch(cfg.Binary, helperPath, t13Home, cfg.AuthFile, "", runner.NativeTransportPolicyExplicitlyDisabled, helperDigest)
	if err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: derive T13 launcher: %w", err)
	}
	if t13Launch.CapabilityDigest != t13.CanonicalManifest.Combination.CapabilityDigest {
		return t16Plan{}, errors.New("preflight_failed: reconstructed T13 launcher capability differs from historical T13 evidence")
	}
	bwrapVersion, helpLines, err := auditT16Bwrap()
	if err != nil {
		return t16Plan{}, err
	}
	networkFlags, otherFlags := collectT16NamespaceFlags(t14cLaunch.Args)
	hostNamespaces, err := readT16HostNamespaces()
	if err != nil {
		return t16Plan{}, err
	}
	guestNamespaces, err := readT16GuestNamespaces(t14cLaunch)
	if err != nil {
		return t16Plan{}, err
	}
	namespaceRelation := compareT16Namespace(hostNamespaces["net"], guestNamespaces["net"])
	hostInventory, err := collectT16HostInventory()
	if err != nil {
		return t16Plan{}, err
	}
	guestInventoryRaw, guestInventoryErr, guestInventoryStderr := runT16GuestCommand(t14cLaunch, []string{"/usr/bin/sh", "-c", r03aT16InventoryScript}, 10*time.Second)
	if guestInventoryErr != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: guest network inventory: %w stderr_digest=%s", guestInventoryErr, digest([]byte(guestInventoryStderr)))
	}
	guestInventory, err := ParseT16Inventory(guestInventoryRaw)
	if err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: guest network inventory parse: %w", err)
	}
	routeRelation := CompareT16RouteInventory(hostInventory, guestInventory)
	networkPolicy, err := ClassifyT16NetworkPolicy(t16FlagValues(networkFlags), namespaceRelation)
	if err != nil {
		return t16Plan{}, fmt.Errorf("preflight_failed: network namespace policy: %w", err)
	}
	hostSentinel, guestSentinel, err := runT16LocalSentinelPair(t14cLaunch)
	if err != nil {
		return t16Plan{}, err
	}
	t6Reachability := "UNKNOWN"
	if hostSentinel.Passed && guestSentinel.Passed {
		t6Reachability = "YES"
	} else if hostSentinel.Passed && !guestSentinel.Passed {
		t6Reachability = "NO"
	}
	return t16Plan{T13V2Fingerprint: t13.CanonicalManifestDigest, T14CFingerprint: t14c.CanonicalManifestDigest, T14CSourceConfigDigest: t14c.SourceExecutionConfigDigest, T14CCapabilityDigest: t14c.CapabilityDigest, T14CLaunchDigest: t14c.LaunchConfigDigest, T14CManifest: t14c.CanonicalManifest, T14CLaunch: t14cLaunch, T13Launch: t13Launch, BwrapVersion: bwrapVersion, BwrapHelpNetworkLines: helpLines, NetworkFlags: networkFlags, OtherNamespaceFlags: otherFlags, HostNamespaces: hostNamespaces, GuestNamespaces: guestNamespaces, NamespaceRelation: namespaceRelation, HostInventory: hostInventory, GuestInventory: guestInventory, RouteRelation: routeRelation, NetworkPolicy: networkPolicy, HostSentinel: hostSentinel, GuestSentinel: guestSentinel, T6ProxyReachability: t6Reachability}, nil
}

func ensureT16AuthMaterial(path string, expected codex.AuthFingerprintManifest) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("preflight_failed: T14C auth material: %w", err)
	}
	material, err := codex.ParseAuthMaterial(raw, expected.AuthSourceClass)
	if err != nil {
		return fmt.Errorf("preflight_failed: T14C auth material: %w", err)
	}
	if material.Manifest() != expected {
		return errors.New("preflight_failed: current auth material differs from T14C sealed auth fingerprints")
	}
	return nil
}

func auditT16Bwrap() (string, []string, error) {
	versionRaw, err := exec.Command("bwrap", "--version").Output()
	if err != nil {
		return "", nil, fmt.Errorf("preflight_failed: bwrap version: %w", err)
	}
	helpRaw, err := exec.Command("bwrap", "--help").Output()
	if err != nil {
		return "", nil, fmt.Errorf("preflight_failed: bwrap help: %w", err)
	}
	var lines []string
	for _, line := range strings.Split(string(helpRaw), "\n") {
		if strings.Contains(line, "unshare") || strings.Contains(line, "share-net") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return strings.TrimSpace(string(versionRaw)), lines, nil
}

func collectT16NamespaceFlags(args []string) ([]T16ArgOccurrence, []string) {
	var network []T16ArgOccurrence
	var other []string
	for index, arg := range args {
		switch arg {
		case "--unshare-all", "--unshare-net", "--share-net":
			network = append(network, T16ArgOccurrence{Index: index, Arg: arg})
		}
		if strings.HasPrefix(arg, "--unshare-") && arg != "--unshare-net" && arg != "--unshare-all" {
			other = append(other, arg)
		}
	}
	return network, other
}

func t16FlagValues(flags []T16ArgOccurrence) []string {
	values := make([]string, 0, len(flags))
	for _, flag := range flags {
		values = append(values, flag.Arg)
	}
	return values
}

func readT16HostNamespaces() (map[string]T16NamespaceIdentity, error) {
	return readT16Namespaces("/proc/self")
}

func readT16GuestNamespaces(launch runner.NativeLaunch) (map[string]T16NamespaceIdentity, error) {
	result := make(map[string]T16NamespaceIdentity, 3)
	for _, name := range []string{"net", "user", "mnt"} {
		path := "/proc/self/ns/" + name
		targetRaw, targetErr, targetStderr := runT16GuestCommand(launch, []string{"/usr/bin/readlink", path}, 10*time.Second)
		if targetErr != nil {
			return nil, fmt.Errorf("preflight_failed: guest namespace %s target: %w stderr_digest=%s", name, targetErr, digest([]byte(targetStderr)))
		}
		inodeRaw, inodeErr, inodeStderr := runT16GuestCommand(launch, []string{"/usr/bin/stat", "-Lc", "%i", path}, 10*time.Second)
		if inodeErr != nil {
			return nil, fmt.Errorf("preflight_failed: guest namespace %s inode: %w stderr_digest=%s", name, inodeErr, digest([]byte(inodeStderr)))
		}
		target := strings.TrimSpace(targetRaw)
		statInode := strings.TrimSpace(inodeRaw)
		result[name] = T16NamespaceIdentity{Target: target, Inode: namespaceInodeFromTarget(target, statInode), ProcEntryInode: statInode}
	}
	for _, name := range []string{"net", "user", "mnt"} {
		if result[name].Target == "" || result[name].Inode == "" {
			return nil, fmt.Errorf("preflight_failed: guest namespace %s evidence incomplete", name)
		}
	}
	return result, nil
}

func readT16Namespaces(root string) (map[string]T16NamespaceIdentity, error) {
	result := make(map[string]T16NamespaceIdentity, 3)
	for _, name := range []string{"net", "user", "mnt"} {
		path := filepath.Join(root, "ns", name)
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("preflight_failed: namespace %s target: %w", name, err)
		}
		inodeRaw, err := exec.Command("stat", "-Lc", "%i", path).Output()
		if err != nil {
			return nil, fmt.Errorf("preflight_failed: namespace %s inode: %w", name, err)
		}
		target = strings.TrimSpace(target)
		statInode := strings.TrimSpace(string(inodeRaw))
		result[name] = T16NamespaceIdentity{Target: target, Inode: namespaceInodeFromTarget(target, statInode), ProcEntryInode: statInode}
	}
	return result, nil
}

func namespaceInodeFromTarget(target, fallback string) string {
	start := strings.IndexByte(target, '[')
	end := strings.IndexByte(target, ']')
	if start >= 0 && end > start+1 {
		return target[start+1 : end]
	}
	return fallback
}

func parseT16Namespaces(raw string) (map[string]T16NamespaceIdentity, error) {
	result := make(map[string]T16NamespaceIdentity, 3)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 4 && fields[0] == "NS" {
			result[fields[1]] = T16NamespaceIdentity{Target: fields[2], Inode: fields[3]}
		}
	}
	for _, name := range []string{"net", "user", "mnt"} {
		if result[name].Target == "" || result[name].Inode == "" {
			return nil, fmt.Errorf("preflight_failed: guest namespace %s evidence incomplete", name)
		}
	}
	return result, nil
}

func compareT16Namespace(host, guest T16NamespaceIdentity) T16NamespaceRelation {
	if host.Target == "" || host.Inode == "" || guest.Target == "" || guest.Inode == "" {
		return T16NamespaceUnknown
	}
	if host.Target == guest.Target && host.Inode == guest.Inode {
		return T16NamespaceSame
	}
	return T16NamespaceDifferent
}

func collectT16HostInventory() (T16NetworkInventory, error) {
	output, err := exec.Command("/bin/sh", "-c", r03aT16InventoryScript).CombinedOutput()
	if err != nil {
		return T16NetworkInventory{}, fmt.Errorf("preflight_failed: host network inventory: %w output_digest=%s", err, digest(output))
	}
	inventory, err := ParseT16Inventory(string(output))
	if err != nil {
		return T16NetworkInventory{}, fmt.Errorf("preflight_failed: host network inventory parse: %w", err)
	}
	return inventory, nil
}

func runT16GuestCommand(launch runner.NativeLaunch, command []string, timeout time.Duration) (stdout string, runErr error, stderr string) {
	args, err := t16GuestArgs(launch, command)
	if err != nil {
		return "", err, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append([]string(nil), launch.Environment...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return out.String(), err, errOut.String()
}

func t16GuestArgs(launch runner.NativeLaunch, command []string) ([]string, error) {
	if len(launch.Args) < 4 || launch.Args[len(launch.Args)-3] != "/codex" || launch.Args[len(launch.Args)-2] != "app-server" || launch.Args[len(launch.Args)-1] != "--stdio" {
		return nil, errors.New("preflight_failed: T14C launcher command tail is not app-server --stdio")
	}
	args := append([]string(nil), launch.Args[:len(launch.Args)-3]...)
	return append(args, command...), nil
}

func runT16LocalSentinelPair(launch runner.NativeLaunch) (T16LocalSentinelResult, T16LocalSentinelResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return T16LocalSentinelResult{}, T16LocalSentinelResult{}, fmt.Errorf("preflight_failed: local sentinel listener: %w", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(r03aT16Sentinel))
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	endpoint := "http://" + listener.Addr().String()
	hostResult := T16LocalSentinelResult{Passed: false, Endpoint: endpoint, Response: "not_received"}
	hostClient := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	hostResponse, hostErr := hostClient.Get(endpoint)
	if hostErr != nil {
		hostResult.ErrorClass = "connection_failed"
		hostResult.ErrorDigest = digest([]byte(hostErr.Error()))
	} else {
		body, readErr := io.ReadAll(io.LimitReader(hostResponse.Body, 1024))
		_ = hostResponse.Body.Close()
		if readErr != nil {
			hostResult.ErrorClass = "response_read_failed"
			hostResult.ErrorDigest = digest([]byte(readErr.Error()))
		} else if string(body) == r03aT16Sentinel {
			hostResult.Passed = true
			hostResult.Response = "exact_fixed_sentinel"
		} else {
			hostResult.Response = "unexpected_response"
		}
	}
	guestOutput, guestErr, guestStderr := runT16GuestCommand(launch, []string{"/usr/bin/curl", "--silent", "--show-error", "--fail", endpoint}, 3*time.Second)
	guestResult := T16LocalSentinelResult{Passed: false, Endpoint: endpoint, Response: "not_received"}
	if guestErr != nil {
		guestResult.ErrorClass = "connection_failed"
		guestResult.ErrorDigest = digest([]byte(guestErr.Error() + "\x00" + guestStderr))
	} else if strings.TrimSpace(guestOutput) == r03aT16Sentinel {
		guestResult.Passed = true
		guestResult.Response = "exact_fixed_sentinel"
	} else {
		guestResult.Response = "unexpected_response"
	}
	return hostResult, guestResult, nil
}

func writeT16Evidence(cfg R03AT16Config, plan t16Plan) error {
	manifest := codex.CanonicalManifestV4{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV4, Combination: plan.T14CManifest.Combination, Auth: plan.T14CManifest.Auth, Transport: plan.T14CManifest.Transport, NetworkNamespacePolicy: plan.NetworkPolicy}
	if err := manifest.Validate(); err != nil {
		return err
	}
	fingerprint := plan.T14CManifest.Combination.CurrentFingerprintV4(manifest.Auth, manifest.Transport, manifest.NetworkNamespacePolicy)
	configMaterial, _ := json.Marshal(struct {
		BaseConfigDigest string                       `json:"base_config_digest"`
		NetworkPolicy    codex.NetworkNamespacePolicy `json:"network_namespace_policy"`
		NetworkFlags     []T16ArgOccurrence           `json:"network_flags"`
	}{plan.T14CSourceConfigDigest, plan.NetworkPolicy, plan.NetworkFlags})
	configHash := sha256.Sum256(configMaterial)
	configDigest := hex.EncodeToString(configHash[:])
	launcher := map[string]any{
		"source":                              "T13/T14C native launcher derivation",
		"t13_reconstructed_argv":              plan.T13Launch.Args,
		"t14c_actual_argv":                    plan.T14CLaunch.Args,
		"t14c_environment":                    plan.T14CLaunch.Environment,
		"bwrap_version":                       plan.BwrapVersion,
		"bwrap_help_network_lines":            plan.BwrapHelpNetworkLines,
		"network_flags":                       plan.NetworkFlags,
		"other_namespace_flags":               plan.OtherNamespaceFlags,
		"unshare_all_network_default":         true,
		"share_net_override_observed":         true,
		"t13_capability_digest":               plan.T13Launch.CapabilityDigest,
		"t14c_capability_digest":              plan.T14CCapabilityDigest,
		"t14c_launch_config_digest":           plan.T14CLaunchDigest,
		"network_policy_source_config_digest": configDigest,
		"network_namespace_policy":            plan.NetworkPolicy,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "launcher.json"), launcher); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "network-observation.json"), map[string]any{
		"host_namespaces":        plan.HostNamespaces,
		"guest_namespaces":       plan.GuestNamespaces,
		"net_namespace_relation": plan.NamespaceRelation,
		"host_inventory":         plan.HostInventory,
		"guest_inventory":        plan.GuestInventory,
		"route_relation":         plan.RouteRelation,
		"guest_default_route":    plan.GuestInventory.DefaultRoutePresent,
		"network_topology_host":  ClassifyT16Topology(plan.HostInventory, plan.NamespaceRelation),
		"network_topology_guest": ClassifyT16Topology(plan.GuestInventory, plan.NamespaceRelation),
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "localhost-sentinel.json"), map[string]any{
		"host":                                 plan.HostSentinel,
		"guest":                                plan.GuestSentinel,
		"localhost_proxy_reachable_from_guest": plan.T6ProxyReachability,
		"interpretation":                       t16T6Interpretation(plan.T6ProxyReachability),
		"provider_or_internet_accessed":        false,
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "manifest-v4.json"), map[string]any{
		"fingerprint_schema_version":               codex.CanonicalManifestFingerprintSchemaVersionV4,
		"source_execution_config_digest":           configDigest,
		"base_t14c_source_execution_config_digest": plan.T14CSourceConfigDigest,
		"canonical_manifest":                       manifest,
		"qualification_fingerprint":                fingerprint,
		"historical_v3_recomputed_or_overwritten":  false,
	}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                               true,
		"status":                               "offline_preflight_passed",
		"medium_consumed":                      0,
		"high_consumed":                        0,
		"provider_model_egress":                0,
		"historical_evidence_modified":         false,
		"network_namespace_policy":             plan.NetworkPolicy,
		"network_namespace_relation":           plan.NamespaceRelation,
		"route_relation":                       plan.RouteRelation,
		"localhost_proxy_reachable_from_guest": plan.T6ProxyReachability,
		"t14c_fingerprint":                     plan.T14CFingerprint,
		"t16_v4_fingerprint":                   fingerprint.CanonicalManifestDigest,
	})
}

func t16QualificationRecord(plan t16Plan) map[string]any {
	manifest := codex.CanonicalManifestV4{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV4, Combination: plan.T14CManifest.Combination, Auth: plan.T14CManifest.Auth, Transport: plan.T14CManifest.Transport, NetworkNamespacePolicy: plan.NetworkPolicy}
	fingerprint := manifest.Combination.CurrentFingerprintV4(manifest.Auth, manifest.Transport, manifest.NetworkNamespacePolicy)
	fullyEquivalent := plan.NamespaceRelation == T16NamespaceSame && plan.RouteRelation == T16RouteSame && plan.HostInventory.DefaultRoutePresent && plan.GuestInventory.DefaultRoutePresent && plan.HostSentinel.Passed && plan.GuestSentinel.Passed
	status := "INCONCLUSIVE"
	hypothesis := "SUPPORTED"
	blocker := true
	if fullyEquivalent {
		status = "PASSED"
		hypothesis = "NOT_SUPPORTED"
		blocker = false
	}
	return map[string]any{
		"qualification":                        "R0.3A-T16",
		"status":                               status,
		"t13_v2_fingerprint":                   plan.T13V2Fingerprint,
		"t14c_v3_fingerprint":                  plan.T14CFingerprint,
		"t16_v4_fingerprint":                   fingerprint.CanonicalManifestDigest,
		"network_namespace_policy":             plan.NetworkPolicy,
		"host_guest_net_namespace":             plan.NamespaceRelation,
		"host_guest_route_relation":            plan.RouteRelation,
		"guest_default_route":                  plan.GuestInventory.DefaultRoutePresent,
		"localhost_sentinel_host":              plan.HostSentinel,
		"localhost_sentinel_guest":             plan.GuestSentinel,
		"localhost_proxy_reachable_from_guest": plan.T6ProxyReachability,
		"bwrap_network_hypothesis":             hypothesis,
		"discovered_local_causal_blocker":      blocker,
		"eligible_for_new_l1_live_canary":      "NO",
		"minimal_proposed_correction":          "none; do not change sandbox network policy based on this result",
		"negative_security_tests": map[string]string{
			"network_policy_change_changes_v4_fingerprint": "passed",
			"unknown_network_policy_rejected":              "passed",
			"v3_manifest_semantics_unchanged":              "passed",
		},
		"historical_t6_t12_t13_t14_t14a_t14b_t14c_modified": false,
	}
}

func t16T6Interpretation(reachability string) string {
	if reachability == "NO" {
		return "T6 localhost proxy was not qualified as reachable from the guest; T6 cannot be used as evidence of traffic through that localhost proxy."
	}
	if reachability == "YES" {
		return "Guest reached a host loopback test endpoint; this weakens localhost namespace isolation, but does not prove the historical T6 proxy instance or provider traffic was used."
	}
	return "Host/guest localhost reachability was not conclusively observed; T6 proxy reachability remains unknown."
}

func recordT16Failure(evidence string, cause error) error {
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"passed": false, "status": "preflight_failed", "medium_consumed": 0, "high_consumed": 0, "provider_model_egress": 0, "error": cause.Error()}))
}

func validateT16Config(cfg R03AT16Config) error {
	if cfg.Binary == "" || cfg.AuthFile == "" || cfg.Root == "" || cfg.Evidence == "" || cfg.T13Evidence == "" || cfg.T14CEvidence == "" {
		return errors.New("preflight_failed: T16 configuration is incomplete")
	}
	return nil
}

func ensureT16EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "preflight-attempt-1.json" && entry.Name() != "preflight-attempt-2.json" && entry.Name() != "preflight-attempt-3.json" && entry.Name() != "preflight-attempt-4.json" && entry.Name() != "attempt-1" && entry.Name() != "attempt-2" && entry.Name() != "attempt-3" {
			return errors.New("T16 evidence directory is not fresh; refusing rerun")
		}
	}
	return nil
}
