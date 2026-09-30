// pattern: Imperative Shell
package mcpowner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"polis/internal/mcptransport"
	"polis/internal/runner"
)

const fixtureToolList = `[{"name":"lookup","description":"Read one fixture item.","inputSchema":{"type":"object","properties":{"key":{"type":"string","minLength":1,"maxLength":32}},"required":["key"],"additionalProperties":false}}]`
const fixtureDriftedToolList = `[{"name":"lookup","description":"Read one fixture item.","inputSchema":{"type":"object","properties":{"key":{"type":"string","minLength":1,"maxLength":64}},"required":["key"],"additionalProperties":false}}]`

func TestStdioOwnerStartsPinnedProcessAndDispatchesOnlyApprovedSchema(t *testing.T) {
	digest, err := mcptransport.StdioToolSchemaDigest(json.RawMessage(fixtureToolList))
	if err != nil {
		t.Fatal(err)
	}
	process := newFixtureProcess(fixtureToolList, "", 0)
	launcher := launcherFunc(func(_ context.Context, spec ProcessSpec) (ownedProcess, error) {
		if spec.Launch.NetworkPolicy != runner.AppContainerNetworkDenyAll || spec.Launch.RegistryProxyEndpoint != "" {
			t.Fatalf("MCP process launch requested network access: %+v", spec.Launch)
		}
		return process, nil
	})
	owner, err := startWithLauncher(context.Background(), launcher, fixtureProcessSpec(t, digest))
	if err != nil {
		t.Fatalf("start local stdio MCP owner: %v", err)
	}
	tools := owner.Tools()
	if len(tools) != 1 || tools[0].Name != "lookup" || owner.ToolSchemaSHA256() != digest {
		t.Fatalf("observed MCP tool binding: tools=%+v digest=%q", tools, owner.ToolSchemaSHA256())
	}
	result, err := owner.CallTool(context.Background(), "lookup", json.RawMessage(`{"key":"item-1"}`))
	if err != nil || result.ContentBoundary != mcptransport.StdioResultContentBoundary || len(result.Content) != 1 || result.Content[0].Text != "fixture-value" {
		t.Fatalf("owned MCP call result=%+v error=%v", result, err)
	}
	if err = owner.Stop(context.Background()); err != nil {
		t.Fatalf("stop local stdio MCP owner: %v", err)
	}
	if process.stopCalls() != 1 {
		t.Fatalf("process stop calls=%d, want one confirmed stop", process.stopCalls())
	}
}

func TestStdioOwnerRejectsCallsWithoutPersistedSchemaApproval(t *testing.T) {
	process := newFixtureProcess(fixtureToolList, "", 0)
	owner, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		return process, nil
	}), fixtureProcessSpec(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.CallTool(context.Background(), "lookup", json.RawMessage(`{"key":"item-1"}`)); err == nil {
		t.Fatal("MCP call was allowed without a persisted schema approval digest")
	}
	if err = owner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStdioOwnerStopsAndReportsToolSchemaDrift(t *testing.T) {
	digest, err := mcptransport.StdioToolSchemaDigest(json.RawMessage(fixtureToolList))
	if err != nil {
		t.Fatal(err)
	}
	process := newFixtureProcess(fixtureToolList, fixtureDriftedToolList, 0)
	owner, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		return process, nil
	}), fixtureProcessSpec(t, digest))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.CallTool(context.Background(), "lookup", json.RawMessage(`{"key":"item-1"}`)); !errors.Is(err, mcptransport.ErrStdioToolSchemaChanged) {
		t.Fatalf("schema drift error=%v, want ErrStdioToolSchemaChanged", err)
	}
	if process.stopCalls() != 1 {
		t.Fatalf("process stop calls after schema drift=%d, want 1", process.stopCalls())
	}
}

func TestStdioOwnerRetainsFailedStartCleanupForRetry(t *testing.T) {
	process := newFixtureProcess(fixtureToolList, "", 1)
	observedDigest, digestErr := mcptransport.StdioToolSchemaDigest(json.RawMessage(fixtureToolList))
	if digestErr != nil {
		t.Fatal(digestErr)
	}
	owner, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		return process, nil
	}), fixtureProcessSpec(t, strings.Repeat("b", 64)))
	if owner == nil || err == nil {
		t.Fatalf("start mismatch should retain an owner when cleanup is unresolved: owner=%v err=%v", owner, err)
	}
	var drift *mcptransport.ToolSchemaDriftError
	if !errors.As(err, &drift) || drift.ObservedDigest != observedDigest {
		t.Fatalf("startup schema drift error=%v observed=%+v, want digest %s", err, drift, observedDigest)
	}
	if err = owner.Stop(context.Background()); err != nil {
		t.Fatalf("retry failed startup cleanup: %v", err)
	}
	if process.stopCalls() != 2 {
		t.Fatalf("process stop attempts=%d, want failed attempt plus successful retry", process.stopCalls())
	}
}

func TestStdioOwnerRejectsUnsafeProcessSpecBeforeLaunching(t *testing.T) {
	unsafeCases := []struct {
		name   string
		change func(*ProcessSpec)
	}{
		{name: "network access", change: func(spec *ProcessSpec) { spec.Launch.NetworkPolicy = runner.AppContainerNetworkRegistryOnly }},
		{name: "missing executable pin", change: func(spec *ProcessSpec) { spec.PinnedFiles = nil }},
		{name: "path escape", change: func(spec *ProcessSpec) {
			spec.PinnedFiles[0].Path = filepath.Join(filepath.Dir(spec.Launch.WorkspaceRoot), "outside.exe")
		}},
		{name: "invalid approved digest", change: func(spec *ProcessSpec) { spec.ApprovedToolSchemaSHA256 = "not-a-digest" }},
		{name: "command differs from qualification", change: func(spec *ProcessSpec) { spec.Launch.Argv = append(spec.Launch.Argv, "--mode=changed") }},
	}
	for _, testCase := range unsafeCases {
		t.Run(testCase.name, func(t *testing.T) {
			spec := fixtureProcessSpec(t, "")
			testCase.change(&spec)
			launched := false
			_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
				launched = true
				return newFixtureProcess(fixtureToolList, "", 0), nil
			}), spec)
			if err == nil || launched {
				t.Fatalf("unsafe process spec reached launcher: launched=%v err=%v", launched, err)
			}
		})
	}
}

func TestStdioOwnerReportsUnexpectedProcessExit(t *testing.T) {
	process := newFixtureProcess(fixtureToolList, "", 0)
	owner, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		return process, nil
	}), fixtureProcessSpec(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err = process.Stop(); err != nil {
		t.Fatal(err)
	}
	<-owner.processDone
	owner.mu.Lock()
	unexpectedExit := owner.unexpectedExit
	owner.mu.Unlock()
	if !unexpectedExit {
		t.Fatal("process exit before Stop was not marked unexpected")
	}
	if _, err = owner.CallTool(context.Background(), "lookup", json.RawMessage(`{"key":"item-1"}`)); !errors.Is(err, ErrOwnerExited) {
		t.Fatalf("call after process exit error=%v, want ErrOwnerExited", err)
	}
	if err = owner.Stop(context.Background()); err != nil {
		t.Fatalf("confirm process-tree stop after unexpected exit: %v", err)
	}
}

func TestStdioOwnerPublicStartRequiresAppContainerSandbox(t *testing.T) {
	_, err := Start(context.Background(), nil, fixtureProcessSpec(t, ""))
	if !errors.Is(err, runner.ErrAppContainerUnavailable) {
		t.Fatalf("Start without the concrete AppContainer sandbox error=%v, want ErrAppContainerUnavailable", err)
	}
}

func TestInjectedPseudoServerCannotIssueRuntimeQualificationObservation(t *testing.T) {
	digest, err := mcptransport.StdioToolSchemaDigest(json.RawMessage(fixtureToolList))
	if err != nil {
		t.Fatal(err)
	}
	process := newFixtureProcess(fixtureToolList, "", 0)
	owner, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		return process, nil
	}), fixtureProcessSpec(t, digest))
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if observation, observeErr := owner.RuntimeObservation(); observeErr == nil || observation.Valid() {
		t.Fatalf("package-private pseudo-server produced trusted qualification evidence: observation=%+v error=%v", observation, observeErr)
	}
}

func TestStdioOwnerRejectsUnpinnedEntryPointBeforeLaunching(t *testing.T) {
	spec := fixtureProcessSpec(t, "")
	entryPoint := filepath.Join(spec.PinnedPackageRoot, "server.js")
	spec.EntryPoint = entryPoint
	spec.Launch.Argv = append(spec.Launch.Argv, entryPoint)
	spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
	launched := false
	_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		launched = true
		return newFixtureProcess(fixtureToolList, "", 0), nil
	}), spec)
	if err == nil || launched {
		t.Fatalf("unpinned entry point reached launcher: launched=%v err=%v", launched, err)
	}
}

func TestStdioOwnerRejectsUnpinnedRelativeConfigOutsidePackage(t *testing.T) {
	spec := fixtureProcessSpec(t, "")
	spec.Launch.WorkingDirectory = spec.Launch.WorkspaceRoot
	spec.Launch.Argv = append(spec.Launch.Argv, "--config", "secrets.toml")
	spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
	launched := false
	_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		launched = true
		return newFixtureProcess(fixtureToolList, "", 0), nil
	}), spec)
	if err == nil || launched {
		t.Fatalf("relative config outside pinned package reached launcher: launched=%v err=%v", launched, err)
	}
}

func TestStdioOwnerRejectsDriveRelativeCommandArgument(t *testing.T) {
	spec := fixtureProcessSpec(t, "")
	spec.Launch.Argv = append(spec.Launch.Argv, "D:secrets.toml")
	spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
	launched := false
	_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		launched = true
		return newFixtureProcess(fixtureToolList, "", 0), nil
	}), spec)
	if err == nil || launched {
		t.Fatalf("drive-relative path reached launcher: launched=%v err=%v", launched, err)
	}
}

func TestStdioOwnerRejectsDriveRelativeEqualsArgument(t *testing.T) {
	for _, argument := range []string{"--config=D:secrets.toml", `--config="D:secrets.toml"`} {
		t.Run(argument, func(t *testing.T) {
			spec := fixtureProcessSpec(t, "")
			spec.Launch.Argv = append(spec.Launch.Argv, argument)
			spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
			launched := false
			_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
				launched = true
				return newFixtureProcess(fixtureToolList, "", 0), nil
			}), spec)
			if err == nil || launched {
				t.Fatalf("drive-relative equals option reached launcher: argument=%q launched=%v err=%v", argument, launched, err)
			}
		})
	}
}

func TestStdioOwnerRejectsAlternateDataStreamArgument(t *testing.T) {
	spec := fixtureProcessSpec(t, "")
	spec.Launch.Argv = append(spec.Launch.Argv, "--config=fixture-mcp.exe:secret")
	spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
	launched := false
	_, err := startWithLauncher(context.Background(), launcherFunc(func(context.Context, ProcessSpec) (ownedProcess, error) {
		launched = true
		return newFixtureProcess(fixtureToolList, "", 0), nil
	}), spec)
	if err == nil || launched {
		t.Fatalf("alternate data stream argument reached launcher: launched=%v err=%v", launched, err)
	}
}

func TestStdioOwnerRejectsIncompletePackageManifest(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "fixture-mcp.exe")
	entryPoint := filepath.Join(root, "server.js")
	if err := os.WriteFile(executable, []byte("binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPoint, []byte("server"), 0600); err != nil {
		t.Fatal(err)
	}
	pinned := []PinnedFile{{Path: executable, SHA256: fileSHA256(t, executable)}}
	if err := validateCompletePackageManifest(root, pinned); err == nil {
		t.Fatal("incomplete package file manifest was accepted")
	}
}

func fixtureProcessSpec(t *testing.T, approvedDigest string) ProcessSpec {
	t.Helper()
	root := t.TempDir()
	packageRoot := filepath.Join(root, "mcp-package")
	if err := os.Mkdir(packageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(packageRoot, "fixture-mcp.exe")
	spec := ProcessSpec{
		Launch: runner.AppContainerLaunchSpec{
			ID: "fixture-mcp-session", WorkspaceRoot: root, Executable: executable,
			Argv: []string{executable}, WorkingDirectory: packageRoot,
			NetworkPolicy: runner.AppContainerNetworkDenyAll,
			Environment:   runner.BuildAppContainerEnvironment(filepath.Join(root, "container"), filepath.Join(root, "windows")),
		},
		PinnedPackageRoot:        packageRoot,
		EntryPoint:               executable,
		PinnedFiles:              []PinnedFile{{Path: executable, SHA256: strings.Repeat("a", 64)}},
		ExpectedServer:           mcptransport.StdioServerIdentity{Name: "polis-fixture", Version: "1.0.0"},
		ApprovedToolSchemaSHA256: approvedDigest,
		GracefulStopTimeout:      250 * time.Millisecond,
	}
	spec.CommandSHA256, _ = ComputeCommandSHA256(spec)
	spec.PackageManifestSHA256, _ = ComputePackageManifestSHA256(spec.PinnedPackageRoot, spec.PinnedFiles)
	return spec
}

func TestProcessSpecSourcePackageBindingRequiresRevisionAndManifestDigest(t *testing.T) {
	spec := fixtureProcessSpec(t, strings.Repeat("b", 64))
	spec.SourcePackageRevisionID = "package-revision-1"
	if err := ValidateProcessSpec(spec); err == nil {
		t.Fatal("source package revision without its immutable manifest digest was accepted")
	}
	spec.SourcePackageManifestSHA256 = strings.Repeat("c", 64)
	if err := ValidateProcessSpec(spec); err != nil {
		t.Fatalf("complete source package binding was rejected: %v", err)
	}
	spec.SourcePackageRevisionID = ""
	if err := ValidateProcessSpec(spec); err == nil {
		t.Fatal("source package manifest digest without its revision was accepted")
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	digest, err := hashPinnedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

type launcherFunc func(context.Context, ProcessSpec) (ownedProcess, error)

func (starter launcherFunc) Launch(ctx context.Context, spec ProcessSpec) (ownedProcess, error) {
	return starter(ctx, spec)
}

type fixtureProcess struct {
	client       net.Conn
	server       net.Conn
	done         chan struct{}
	once         sync.Once
	mu           sync.Mutex
	listCount    int
	drifted      string
	stopFailures int
	stopCount    int
}

func newFixtureProcess(initialTools, driftedTools string, stopFailures int) *fixtureProcess {
	client, server := net.Pipe()
	process := &fixtureProcess{client: client, server: server, done: make(chan struct{}), drifted: driftedTools, stopFailures: stopFailures}
	go process.serve(initialTools)
	return process
}

func (process *fixtureProcess) Stdin() io.WriteCloser { return process.client }
func (process *fixtureProcess) Stdout() io.ReadCloser { return process.client }
func (process *fixtureProcess) Stderr() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }

func (process *fixtureProcess) Wait(ctx context.Context) (int, error) {
	select {
	case <-process.done:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (process *fixtureProcess) Stop() error {
	process.mu.Lock()
	process.stopCount++
	if process.stopFailures > 0 {
		process.stopFailures--
		process.mu.Unlock()
		return errors.New("injected MCP process stop failure")
	}
	process.mu.Unlock()
	_ = process.client.Close()
	process.once.Do(func() {
		_ = process.server.Close()
		close(process.done)
	})
	return nil
}

func (process *fixtureProcess) stopCalls() int {
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.stopCount
}

func (process *fixtureProcess) serve(initialTools string) {
	defer process.once.Do(func() {
		_ = process.server.Close()
		close(process.done)
	})
	scanner := bufio.NewScanner(process.server)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || len(request.ID) == 0 {
			continue
		}
		var result any
		switch request.Method {
		case "server/discover":
			result = map[string]any{
				"resultType": "complete", "supportedVersions": []string{mcptransport.ProtocolVersion20260728},
				"capabilities": map[string]any{"tools": map[string]any{}},
				"_meta":        map[string]any{mcptransport.StdioServerInfoMetadataKey: map[string]any{"name": "polis-fixture", "version": "1.0.0"}},
				"ttlMs":        0, "cacheScope": "private",
			}
		case "tools/list":
			process.mu.Lock()
			process.listCount++
			toolList := initialTools
			if process.listCount > 1 && process.drifted != "" {
				toolList = process.drifted
			}
			process.mu.Unlock()
			result = map[string]any{"resultType": "complete", "tools": json.RawMessage(toolList), "ttlMs": 0, "cacheScope": "private"}
		case "tools/call":
			result = map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": "fixture-value"}}, "isError": false}
		default:
			result = map[string]any{}
		}
		response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		response = append(response, '\n')
		if _, err := process.server.Write(response); err != nil {
			return
		}
	}
}
