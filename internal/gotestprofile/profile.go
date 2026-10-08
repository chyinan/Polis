// pattern: Functional Core
package gotestprofile

import "errors"

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"polis/internal/fixture"
)

const (
	SchemaVersion          = "polis-go-test-profile@1"
	FixtureRevision        = "polis-go-test-fixture@1"
	FixtureGoVersion       = "go1.26.8"
	FixtureToolchainSHA256 = "d9a2fa19c7ef8b57f420012c21f49f235c46f08a68c12077d9c753dbb6ccdc34"
	fixtureGoMod           = "module fixture\n\ngo 1.26\n"
)

var (
	ErrInvalidProfile    = errors.New("invalid fixed Go test profile")
	ErrInvalidJobBinding = errors.New("invalid fixed Go JobRun binding")
)

type ResourceLimits struct {
	CPUQuotaMicros  uint64 `json:"cpuQuotaMicros"`
	CPUPeriodMicros uint64 `json:"cpuPeriodMicros"`
	MemoryBytes     uint64 `json:"memoryBytes"`
	SwapBytes       uint64 `json:"swapBytes"`
	PIDs            uint64 `json:"pids"`
	WallTimeoutMS   uint64 `json:"wallTimeoutMs"`
	ScratchBytes    uint64 `json:"scratchBytes"`
	ScratchInodes   uint64 `json:"scratchInodes"`
	StdoutBytes     uint64 `json:"stdoutBytes"`
	StderrBytes     uint64 `json:"stderrBytes"`
}

type Manifest struct {
	SchemaVersion      string         `json:"schemaVersion"`
	ProfileRevision    string         `json:"profileRevision"`
	GoVersion          string         `json:"goVersion"`
	ToolchainSHA256    string         `json:"toolchainSha256"`
	RunnerSHA256       string         `json:"runnerSha256"`
	TestManifestSHA256 string         `json:"testManifestSha256"`
	PackageScope       []string       `json:"packageScope"`
	Command            []string       `json:"command"`
	Environment        []string       `json:"environment"`
	NetworkPolicy      string         `json:"networkPolicy"`
	DependenciesPolicy string         `json:"dependenciesPolicy"`
	EditablePaths      []string       `json:"editablePaths"`
	ProtectedPaths     []string       `json:"protectedPaths"`
	AllowNewTestFiles  bool           `json:"allowNewTestFiles"`
	Limits             ResourceLimits `json:"limits"`
}

func FixtureManifest(runnerSHA256 string) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion, ProfileRevision: FixtureRevision, GoVersion: FixtureGoVersion,
		ToolchainSHA256: FixtureToolchainSHA256, RunnerSHA256: runnerSHA256, TestManifestSHA256: FixtureTestManifestSHA256(),
		PackageScope: []string{"."},
		Command:      []string{"/go/bin/go", "test", "-json", "-v", "-count=1", "-timeout=120s", "."},
		Environment: []string{
			"CGO_ENABLED=0", "GOCACHE=/scratch/gocache", "GOMODCACHE=/deps/gomodcache", "GOPATH=/scratch/gopath",
			"GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOROOT=/go", "GOTMPDIR=/scratch/go-tmp",
			"HOME=/scratch/home", "PATH=/go/bin", "TMPDIR=/scratch/tmp",
		},
		NetworkPolicy: "deny_all", DependenciesPolicy: "offline_only",
		EditablePaths: []string{"formatter.go"}, ProtectedPaths: []string{"frozen_test.go", "go.mod", "go.sum"}, AllowNewTestFiles: true,
		Limits: ResourceLimits{
			CPUQuotaMicros: 200000, CPUPeriodMicros: 100000, MemoryBytes: 2 << 30, SwapBytes: 0,
			PIDs: 256, WallTimeoutMS: 120000, ScratchBytes: 1 << 30, ScratchInodes: 4096,
			StdoutBytes: 512 << 10, StderrBytes: 512 << 10,
		},
	}
}

func FixtureTestManifestSHA256() string {
	canonical, err := json.Marshal(struct {
		Revision string `json:"revision"`
		GoMod    string `json:"goMod"`
		Tests    string `json:"tests"`
	}{fixture.Revision, fixtureGoMod, fixture.Tests("full")})
	if err != nil {
		panic("go test fixture manifest is not serializable: " + err.Error())
	}
	return digest(canonical)
}

func Canonicalize(manifest Manifest) ([]byte, string, error) {
	if manifest.SchemaVersion != SchemaVersion || manifest.ProfileRevision != FixtureRevision || manifest.GoVersion != FixtureGoVersion ||
		manifest.ToolchainSHA256 != FixtureToolchainSHA256 || !validSHA256(manifest.RunnerSHA256) || manifest.TestManifestSHA256 != FixtureTestManifestSHA256() ||
		manifest.NetworkPolicy != "deny_all" || manifest.DependenciesPolicy != "offline_only" || !manifest.AllowNewTestFiles ||
		!sameStrings(manifest.PackageScope, []string{"."}) ||
		!sameStrings(manifest.Command, []string{"/go/bin/go", "test", "-json", "-v", "-count=1", "-timeout=120s", "."}) ||
		!sameStrings(manifest.Environment, []string{
			"CGO_ENABLED=0", "GOCACHE=/scratch/gocache", "GOMODCACHE=/deps/gomodcache", "GOPATH=/scratch/gopath",
			"GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOROOT=/go", "GOTMPDIR=/scratch/go-tmp",
			"HOME=/scratch/home", "PATH=/go/bin", "TMPDIR=/scratch/tmp",
		}) ||
		!sameStrings(manifest.EditablePaths, []string{"formatter.go"}) || !sameStrings(manifest.ProtectedPaths, []string{"frozen_test.go", "go.mod", "go.sum"}) ||
		manifest.Limits != (ResourceLimits{CPUQuotaMicros: 200000, CPUPeriodMicros: 100000, MemoryBytes: 2 << 30, SwapBytes: 0, PIDs: 256, WallTimeoutMS: 120000, ScratchBytes: 1 << 30, ScratchInodes: 4096, StdoutBytes: 512 << 10, StderrBytes: 512 << 10}) {
		return nil, "", ErrInvalidProfile
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", ErrInvalidProfile
	}
	return canonical, digest(canonical), nil
}

func CommandSHA256(manifest Manifest) string {
	_, _, err := Canonicalize(manifest)
	if err != nil {
		return ""
	}
	command, err := json.Marshal(manifest.Command)
	if err != nil {
		return ""
	}
	return digest(command)
}

func Parse(raw []byte) (Manifest, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if decoder.Decode(&manifest) != nil {
		return Manifest{}, "", ErrInvalidProfile
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return Manifest{}, "", ErrInvalidProfile
	}
	canonical, profileSHA256, err := Canonicalize(manifest)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Manifest{}, "", ErrInvalidProfile
	}
	return manifest, profileSHA256, nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digest(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}
