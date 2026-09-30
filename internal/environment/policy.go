// pattern: Functional Core
package environment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var errEnvironmentAllowlist = errors.New("invalid environment variable allowlist")
var errEnvironmentPolicy = errors.New("invalid Windows Node/npm environment policy")
var environmentVariableNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)
var environmentServiceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func ValidProjectServiceID(value string) bool { return environmentServiceIDPattern.MatchString(value) }

const WindowsNodeInstallPolicy = "npm ci --ignore-scripts --no-audit --no-fund"
const LinuxNodeNPMInstallPolicy = "npm ci --ignore-scripts --no-audit --no-fund --offline"

type ProjectEnvironmentPolicyManifest struct {
	SchemaVersion          string                     `json:"schemaVersion"`
	ProfileID              string                     `json:"profileId"`
	RegistryHosts          []string                   `json:"registryHosts"`
	InstallPolicy          string                     `json:"installPolicy"`
	LifecycleScriptsPolicy string                     `json:"lifecycleScriptsPolicy"`
	NetworkPolicy          string                     `json:"networkPolicy"`
	TimeoutMS              int                        `json:"timeoutMs"`
	OutputLimitBytes       int                        `json:"outputLimitBytes"`
	Services               []ProjectServiceDefinition `json:"services,omitempty"`
}

const MaxProjectEnvironmentServices = 8

type ProjectServiceDefinition struct {
	ID         string           `json:"id"`
	ScriptPath string           `json:"scriptPath"`
	Probe      ServiceProbeSpec `json:"probe"`
}

const ProjectEnvironmentPolicySchema = "project-environment-policy@1"

func PolicyForNodeNPMPlan(plan NodeNPMProjectPlan, timeoutMS, outputLimitBytes int) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	if len(plan.RegistryHosts) == 0 {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	switch plan.ProfileID {
	case WindowsNodeNPMProfile:
		if plan.InstallPolicy != WindowsNodeInstallPolicy {
			return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
		}
		return BuildWindowsNodeEnvironmentPolicy(plan.RegistryHosts, timeoutMS, outputLimitBytes)
	case LinuxNodeNPMProfile:
		if plan.InstallPolicy != LinuxNodeNPMInstallPolicy {
			return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
		}
		return BuildLinuxNodeEnvironmentPolicy(plan.RegistryHosts, timeoutMS, outputLimitBytes)
	default:
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
}

// BuildWindowsNodeEnvironmentPolicy produces a bounded policy with lifecycle
// scripts disabled and a fixed registry-only install contract.
func BuildWindowsNodeEnvironmentPolicy(registryHosts []string, timeoutMS, outputLimitBytes int) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	manifest := ProjectEnvironmentPolicyManifest{
		SchemaVersion:          ProjectEnvironmentPolicySchema,
		ProfileID:              WindowsNodeNPMProfile,
		RegistryHosts:          registryHosts,
		InstallPolicy:          WindowsNodeInstallPolicy,
		LifecycleScriptsPolicy: "ignore",
		NetworkPolicy:          "registry_allowlist",
		TimeoutMS:              timeoutMS,
		OutputLimitBytes:       outputLimitBytes,
	}
	return CanonicalizeWindowsNodeEnvironmentPolicy(manifest)
}

// ParseWindowsNodeEnvironmentPolicy rejects unknown fields before returning
// the canonical manifest bytes and their digest.
func ParseWindowsNodeEnvironmentPolicy(raw []byte) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest ProjectEnvironmentPolicyManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	return CanonicalizeWindowsNodeEnvironmentPolicy(manifest)
}

func BuildLinuxNodeEnvironmentPolicy(registryHosts []string, timeoutMS, outputLimitBytes int) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	manifest := ProjectEnvironmentPolicyManifest{
		SchemaVersion:          ProjectEnvironmentPolicySchema,
		ProfileID:              LinuxNodeNPMProfile,
		RegistryHosts:          registryHosts,
		InstallPolicy:          LinuxNodeNPMInstallPolicy,
		LifecycleScriptsPolicy: "ignore",
		NetworkPolicy:          "deny_all",
		TimeoutMS:              timeoutMS,
		OutputLimitBytes:       outputLimitBytes,
	}
	return CanonicalizeLinuxNodeEnvironmentPolicy(manifest)
}

func ParseProjectEnvironmentPolicy(raw []byte) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest ProjectEnvironmentPolicyManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	switch manifest.ProfileID {
	case WindowsNodeNPMProfile:
		return CanonicalizeWindowsNodeEnvironmentPolicy(manifest)
	case LinuxNodeNPMProfile:
		return CanonicalizeLinuxNodeEnvironmentPolicy(manifest)
	default:
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
}

func ParseLinuxNodeEnvironmentPolicy(raw []byte) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	manifest, canonical, digest, err := ParseProjectEnvironmentPolicy(raw)
	if err != nil || manifest.ProfileID != LinuxNodeNPMProfile {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	return manifest, canonical, digest, nil
}

func CanonicalizeLinuxNodeEnvironmentPolicy(manifest ProjectEnvironmentPolicyManifest) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	if manifest.SchemaVersion != ProjectEnvironmentPolicySchema || manifest.ProfileID != LinuxNodeNPMProfile || manifest.InstallPolicy != LinuxNodeNPMInstallPolicy || manifest.LifecycleScriptsPolicy != "ignore" || manifest.NetworkPolicy != "deny_all" || manifest.TimeoutMS < 1000 || manifest.TimeoutMS > 600000 || manifest.OutputLimitBytes < 4096 || manifest.OutputLimitBytes > 1048576 {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	registryHosts, err := normalizeRegistryHosts(manifest.RegistryHosts)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	manifest.RegistryHosts = registryHosts
	manifest.Services, err = normalizeProjectServiceDefinitions(manifest.Services)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	digest := sha256.Sum256(canonical)
	return manifest, canonical, hex.EncodeToString(digest[:]), nil
}

func CanonicalizeWindowsNodeEnvironmentPolicy(manifest ProjectEnvironmentPolicyManifest) (ProjectEnvironmentPolicyManifest, []byte, string, error) {
	if manifest.SchemaVersion != ProjectEnvironmentPolicySchema || manifest.ProfileID != WindowsNodeNPMProfile || manifest.InstallPolicy != WindowsNodeInstallPolicy || manifest.LifecycleScriptsPolicy != "ignore" || manifest.NetworkPolicy != "registry_allowlist" || manifest.TimeoutMS < 1000 || manifest.TimeoutMS > 600000 || manifest.OutputLimitBytes < 4096 || manifest.OutputLimitBytes > 1048576 {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	registryHosts, err := normalizeRegistryHosts(manifest.RegistryHosts)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	manifest.RegistryHosts = registryHosts
	manifest.Services, err = normalizeProjectServiceDefinitions(manifest.Services)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return ProjectEnvironmentPolicyManifest{}, nil, "", errEnvironmentPolicy
	}
	digest := sha256.Sum256(canonical)
	return manifest, canonical, hex.EncodeToString(digest[:]), nil
}

func normalizeProjectServiceDefinitions(services []ProjectServiceDefinition) ([]ProjectServiceDefinition, error) {
	if len(services) > MaxProjectEnvironmentServices {
		return nil, errEnvironmentPolicy
	}
	normalized := append([]ProjectServiceDefinition(nil), services...)
	seenIDs := make(map[string]struct{}, len(normalized))
	seenEndpoints := make(map[string]struct{}, len(normalized))
	for index := range normalized {
		service := &normalized[index]
		if !ValidProjectServiceID(service.ID) {
			return nil, errEnvironmentPolicy
		}
		if _, exists := seenIDs[service.ID]; exists {
			return nil, errEnvironmentPolicy
		}
		seenIDs[service.ID] = struct{}{}
		scriptPath, err := NormalizeNodeProjectScriptPath(service.ScriptPath)
		if err != nil || scriptPath != service.ScriptPath {
			return nil, errEnvironmentPolicy
		}
		if _, err = ServiceProbeSpecSHA256(service.Probe); err != nil {
			return nil, errEnvironmentPolicy
		}
		endpoint := service.Probe.BindAddress + ":" + strconv.Itoa(int(service.Probe.Port))
		if _, exists := seenEndpoints[endpoint]; exists {
			return nil, errEnvironmentPolicy
		}
		seenEndpoints[endpoint] = struct{}{}
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left].ID < normalized[right].ID })
	return normalized, nil
}

// NormalizeEnvironmentAllowlist returns canonical variable names only. Values
// never enter this contract or the persisted job record.
func NormalizeEnvironmentAllowlist(values []string) ([]string, error) {
	if len(values) > 64 {
		return nil, errEnvironmentAllowlist
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.ToUpper(strings.TrimSpace(value))
		if !environmentVariableNamePattern.MatchString(name) {
			return nil, errEnvironmentAllowlist
		}
		if _, exists := seen[name]; exists {
			return nil, errEnvironmentAllowlist
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized, nil
}
