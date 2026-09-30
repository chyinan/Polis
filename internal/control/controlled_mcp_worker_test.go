// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"polis/internal/kernel"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
)

type controlledMCPKernelFixture struct {
	order       *[]string
	intent      kernel.StdioMCPToolCallRecord
	authority   kernel.StdioMCPToolAuthorization
	beginErr    error
	resultErr   error
	unknownErr  error
	driftDigest string
	lockErr     error
	lockHeld    bool
	lockRelease func()
	lockMissing []string
}

func (fixture *controlledMCPKernelFixture) LockStdioMCPPackageServer(ctx context.Context, _, _ string) (context.Context, func(), error) {
	if fixture.lockErr != nil {
		return nil, nil, fixture.lockErr
	}
	fixture.lockHeld = true
	return ctx, func() {
		fixture.lockHeld = false
		if fixture.lockRelease != nil {
			fixture.lockRelease()
		}
	}, nil
}

func (fixture *controlledMCPKernelFixture) TXBeginStdioMCPToolCall(_ context.Context, _ kernel.Binding, _ kernel.StdioMCPToolCallIntentInput) (kernel.StdioMCPToolCallRecord, error) {
	if !fixture.lockHeld {
		fixture.lockMissing = append(fixture.lockMissing, "intent")
	}
	*fixture.order = append(*fixture.order, "intent")
	return fixture.intent, fixture.beginErr
}

func (fixture *controlledMCPKernelFixture) AuthorizeStdioMCPToolCall(_ context.Context, _ kernel.Binding, _, _, _ string) (kernel.StdioMCPToolAuthorization, error) {
	if !fixture.lockHeld {
		fixture.lockMissing = append(fixture.lockMissing, "authorize")
	}
	*fixture.order = append(*fixture.order, "authorize")
	return fixture.authority, nil
}

func (fixture *controlledMCPKernelFixture) TXRecordStdioMCPToolSchemaDrift(_ context.Context, _ string, input kernel.StdioMCPToolSchemaDriftInput) error {
	*fixture.order = append(*fixture.order, "schema_drift")
	fixture.driftDigest = input.ObservedToolSchemaSHA256
	return nil
}

func (fixture *controlledMCPKernelFixture) TXCompleteStdioMCPToolCall(_ context.Context, _ kernel.Binding, _ string, _ mcptransport.StdioToolResult) error {
	if !fixture.lockHeld {
		fixture.lockMissing = append(fixture.lockMissing, "complete")
	}
	*fixture.order = append(*fixture.order, "complete")
	return fixture.resultErr
}

func (fixture *controlledMCPKernelFixture) TXMarkStdioMCPToolCallsUnknownForSession(_ context.Context, _, _, _ string) (int, error) {
	*fixture.order = append(*fixture.order, "unknown")
	return 1, fixture.unknownErr
}

type controlledMCPProcessFixture struct {
	order     *[]string
	digest    string
	result    mcptransport.StdioToolResult
	callErr   error
	stopErr   error
	callCount int
	stopCount int
	lockHeld  func() bool
	lockMiss  bool
}

func (process *controlledMCPProcessFixture) CallTool(_ context.Context, _ string, _ []byte) (mcptransport.StdioToolResult, error) {
	if process.lockHeld != nil && !process.lockHeld() {
		process.lockMiss = true
	}
	*process.order = append(*process.order, "call")
	process.callCount++
	return process.result, process.callErr
}
func (process *controlledMCPProcessFixture) ToolSchemaSHA256() string { return process.digest }
func (process *controlledMCPProcessFixture) Stop(context.Context) error {
	*process.order = append(*process.order, "owner_stop")
	process.stopCount++
	return process.stopErr
}

type controlledMCPFactoryFixture struct {
	order      *[]string
	process    *controlledMCPProcessFixture
	startErr   error
	remoteErr  error
	remoteURL  string
	remoteHash string
	lockHeld   func() bool
	lockMiss   bool
	closeErr   error
	closeCalls int
}

func (factory *controlledMCPFactoryFixture) Start(context.Context, mcpowner.ProcessSpec) (controlledMCPProcess, error) {
	if factory.lockHeld != nil && !factory.lockHeld() {
		factory.lockMiss = true
	}
	*factory.order = append(*factory.order, "owner_start")
	if factory.process == nil {
		return nil, factory.startErr
	}
	return factory.process, factory.startErr
}

func (factory *controlledMCPFactoryFixture) StartStreamableHTTP(_ context.Context, endpoint, schemaDigest string) (controlledMCPProcess, error) {
	if factory.lockHeld != nil && !factory.lockHeld() {
		factory.lockMiss = true
	}
	*factory.order = append(*factory.order, "http_start")
	factory.remoteURL = endpoint
	factory.remoteHash = schemaDigest
	if factory.process == nil {
		return nil, factory.remoteErr
	}
	return factory.process, factory.remoteErr
}

func (factory *controlledMCPFactoryFixture) Close() error {
	factory.closeCalls++
	*factory.order = append(*factory.order, "sandbox_close")
	return factory.closeErr
}

type orderedMCPObserverFixture struct {
	order      *[]string
	closeErrs  []error
	closeCalls int
}

type phasedMCPShutdownWorkerFixture struct {
	WorkerAdapter
	quiesceErrors []error
	quiesceCalls  int
	closeCalls    int
	closeAfter    int
}

func (worker *phasedMCPShutdownWorkerFixture) Close() { worker.closeCalls++ }

func (worker *phasedMCPShutdownWorkerFixture) QuiesceForMCPObserver() error {
	worker.quiesceCalls++
	if len(worker.quiesceErrors) >= worker.quiesceCalls {
		return worker.quiesceErrors[worker.quiesceCalls-1]
	}
	return nil
}

func (worker *phasedMCPShutdownWorkerFixture) CloseAfterMCPObserver() error {
	worker.closeAfter++
	return nil
}

func (*orderedMCPObserverFixture) Observe(context.Context, string, ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	return kernel.StdioMCPRuntimeQualification{}, nil
}

func (observer *orderedMCPObserverFixture) Close() error {
	observer.closeCalls++
	*observer.order = append(*observer.order, "observer_close")
	if len(observer.closeErrs) >= observer.closeCalls {
		return observer.closeErrs[observer.closeCalls-1]
	}
	return nil
}

func controlledMCPAuthorization(runtimeID, capabilityID, schema, commandDigest, packageDigest string) kernel.StdioMCPToolAuthorization {
	return kernel.StdioMCPToolAuthorization{
		RuntimeQualification: kernel.StdioMCPRuntimeQualification{
			CompanyID: "company-1", RuntimeQualificationID: runtimeID, CapabilityID: capabilityID,
			ToolSchemaSHA256: schema, CommandSHA256: commandDigest, PackageManifestSHA256: packageDigest,
		},
		ProcessSpec: mcpowner.ProcessSpec{
			CommandSHA256: commandDigest, PackageManifestSHA256: packageDigest,
			ApprovedToolSchemaSHA256: schema,
		},
		Tools: []mcptransport.StdioToolDefinition{{Name: "lookup"}},
	}
}

func TestControlledMCPWorkerCallAuthorizesCallsAndCompletesInOrder(t *testing.T) {
	const (
		capabilityID = "mcp-1"
		runtimeID    = "runtime-1"
		schema       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	order := []string{}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: schema, ContentBoundary: mcptransport.StdioResultContentBoundary, Content: []mcptransport.StdioTextContent{{Text: "untrusted fixture"}}}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-1", Status: "dispatching", CapabilityID: capabilityID, RuntimeQualificationID: runtimeID, ToolName: "lookup", ToolSchemaSHA256: schema},
		authority: controlledMCPAuthorization(runtimeID, capabilityID, schema, "command-1", "package-1"),
	}
	process := &controlledMCPProcessFixture{order: &order, digest: schema, result: result}
	factory := &controlledMCPFactoryFixture{order: &order, process: process}
	state := &stdioMCPWorkerState{}
	binding := kernel.Binding{}
	arguments := json.RawMessage(`{"key":"sample"}`)
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	response, err := state.call(context.Background(), kernelFixture, factory, "company-1", binding, "provider-call-1", request)
	if err != nil || string(response) == "" || !reflect.DeepEqual(order, []string{"authorize", "intent", "owner_start", "call", "complete"}) {
		t.Fatalf("controlled MCP dispatch order=%v response=%s err=%v", order, response, err)
	}
	var decoded mcptransport.StdioToolResult
	if err = json.Unmarshal(response, &decoded); err != nil || decoded.ContentBoundary != mcptransport.StdioResultContentBoundary || decoded.Content[0].Text != "untrusted fixture" {
		t.Fatalf("controlled MCP result=%+v err=%v", decoded, err)
	}
	if string(arguments) == "" || process.callCount != 1 || state.pending {
		t.Fatalf("controlled MCP call state pending=%t process calls=%d", state.pending, process.callCount)
	}
}

func TestControlledMCPWorkerHoldsPackageFenceThroughToolCompletion(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: digest, ContentBoundary: mcptransport.StdioResultContentBoundary}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-fenced", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", digest, "command-1", "package-1"),
	}
	kernelFixture.lockRelease = func() { order = append(order, "server_unlock") }
	process := &controlledMCPProcessFixture{order: &order, digest: digest, result: result, lockHeld: func() bool { return kernelFixture.lockHeld }}
	factory := &controlledMCPFactoryFixture{order: &order, process: process, lockHeld: func() bool { return kernelFixture.lockHeld }}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-fenced", request); err != nil {
		t.Fatal(err)
	}
	if len(kernelFixture.lockMissing) != 0 || factory.lockMiss || process.lockMiss || kernelFixture.lockHeld {
		t.Fatalf("MCP call escaped package revision fence: unlocked=%v factory=%t process=%t heldAfterReturn=%t", kernelFixture.lockMissing, factory.lockMiss, process.lockMiss, kernelFixture.lockHeld)
	}
	state.quiesce()
	if err := state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-1"); err != nil {
		t.Fatal(err)
	}
}

func TestControlledMCPWorkerDoesNotReserveIntentWhenPackageImportOwnsFence(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lockErr := errors.New("package import owns server lock")
	order := []string{}
	kernelFixture := &controlledMCPKernelFixture{order: &order, lockErr: lockErr}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, &controlledMCPFactoryFixture{order: &order}, "company-1", kernel.Binding{}, "provider-call-lock-conflict", request); !errors.Is(err, lockErr) {
		t.Fatalf("MCP call while package import owns lock error=%v, want %v", err, lockErr)
	}
	if len(order) != 0 || state.pending || state.blocked {
		t.Fatalf("lock conflict reached dispatch or poisoned session: order=%v pending=%t blocked=%t", order, state.pending, state.blocked)
	}
}

func TestControlledMCPWorkerDispatchesStreamableHTTPThroughQualifiedEndpoint(t *testing.T) {
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "1")
	const schema = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: schema, ContentBoundary: mcptransport.StreamableHTTPResultContentBoundary}
	kernelFixture := &controlledMCPKernelFixture{
		order:  &order,
		intent: kernel.StdioMCPToolCallRecord{IntentID: "http-intent", Status: "dispatching", CapabilityID: "mcp-http-1", RuntimeQualificationID: "http-runtime-1", ToolName: "lookup", ToolSchemaSHA256: schema},
		authority: kernel.StdioMCPToolAuthorization{
			RuntimeQualification: kernel.StdioMCPRuntimeQualification{CompanyID: "company-1", RuntimeQualificationID: "http-runtime-1", CapabilityID: "mcp-http-1", Transport: "streamable_http", Endpoint: "https://mcp.example.com/v1/mcp", ToolSchemaSHA256: schema},
			Tools:                []mcptransport.StdioToolDefinition{{Name: "lookup"}}, Transport: "streamable_http", Endpoint: "https://mcp.example.com/v1/mcp",
		},
	}
	process := &controlledMCPProcessFixture{order: &order, digest: schema, result: result}
	factory := &controlledMCPFactoryFixture{order: &order, process: process}
	state := &stdioMCPWorkerState{allowStreamableHTTP: true}
	request := json.RawMessage(`{"capability_id":"mcp-http-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	response, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "http-provider-call", request)
	if err != nil || len(response) == 0 {
		t.Fatalf("Streamable HTTP MCP Worker call response=%s error=%v", response, err)
	}
	if factory.remoteURL != "https://mcp.example.com/v1/mcp" || factory.remoteHash != schema || factory.lockMiss ||
		!reflect.DeepEqual(order, []string{"authorize", "intent", "http_start", "call", "complete"}) {
		t.Fatalf("Streamable HTTP Worker dispatch used unexpected endpoint/order: endpoint=%q schema=%q lockMiss=%t order=%v", factory.remoteURL, factory.remoteHash, factory.lockMiss, order)
	}
	state.quiesce()
	if err = state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-http"); err != nil {
		t.Fatal(err)
	}
}

func TestControlledMCPWorkerDoesNotReserveIntentWhenHTTPGateIsDisabled(t *testing.T) {
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "")
	const schema = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	kernelFixture := &controlledMCPKernelFixture{
		order: &order,
		authority: kernel.StdioMCPToolAuthorization{
			RuntimeQualification: kernel.StdioMCPRuntimeQualification{RuntimeQualificationID: "http-runtime-1", CapabilityID: "mcp-http-1", Transport: "streamable_http", Endpoint: "https://mcp.example.com/v1/mcp", ToolSchemaSHA256: schema},
			Transport:            "streamable_http", Endpoint: "https://mcp.example.com/v1/mcp",
		},
	}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-http-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, &controlledMCPFactoryFixture{order: &order}, "company-1", kernel.Binding{}, "http-disabled-call", request); err == nil {
		t.Fatal("Streamable HTTP Worker egress gate was ignored")
	}
	if !reflect.DeepEqual(order, []string{"authorize"}) || state.pending || state.blocked {
		t.Fatalf("disabled HTTP Worker call consumed an intent or contacted a server: order=%v pending=%t blocked=%t", order, state.pending, state.blocked)
	}
}

func TestServiceClosesSharedSandboxOnlyAfterObserverCleanup(t *testing.T) {
	order := []string{}
	factory := &controlledMCPFactoryFixture{order: &order}
	adapter := &RealProviderWorkerAdapter{mcpFactory: factory}
	observer := &orderedMCPObserverFixture{order: &order, closeErrs: []error{errors.New("injected observer cleanup failure")}}
	service := NewService(nil, adapter)
	service.SetStdioMCPRuntimeObserver(observer)
	if err := service.Close(); err == nil {
		t.Fatal("first shared observer cleanup failure was ignored")
	}
	if factory.closeCalls != 0 || service.mcpRuntimeObserver != observer {
		t.Fatalf("sandbox closed or observer discarded before cleanup retry: closes=%d retained=%t", factory.closeCalls, service.mcpRuntimeObserver == observer)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("retry shared observer cleanup and sandbox close: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"observer_close", "observer_close", "sandbox_close"}) || factory.closeCalls != 1 || service.mcpRuntimeObserver != nil {
		t.Fatalf("shared sandbox shutdown order=%v closeCalls=%d observerRetained=%t", order, factory.closeCalls, service.mcpRuntimeObserver != nil)
	}
}

func TestServiceRetriesPhasedWorkerCleanupAfterObserverCloses(t *testing.T) {
	workerErr := errors.New("injected worker quiesce failure")
	worker := &phasedMCPShutdownWorkerFixture{quiesceErrors: []error{workerErr}}
	service := NewService(nil, worker)
	service.SetStdioMCPRuntimeObserver(&orderedMCPObserverFixture{order: &[]string{}})

	if err := service.Close(); !errors.Is(err, workerErr) {
		t.Fatalf("first phased shutdown error=%v, want %v", err, workerErr)
	}
	if service.mcpRuntimeObserver != nil {
		t.Fatal("successfully closed observer was retained")
	}
	if worker.closeAfter != 0 || worker.closeCalls != 0 {
		t.Fatalf("sandbox close ran while worker quiescence was unresolved: closeAfter=%d legacyClose=%d", worker.closeAfter, worker.closeCalls)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("retry phased worker cleanup: %v", err)
	}
	if worker.quiesceCalls != 2 || worker.closeAfter != 1 || worker.closeCalls != 0 {
		t.Fatalf("retry did not retain phased cleanup: quiesce=%d closeAfter=%d legacyClose=%d", worker.quiesceCalls, worker.closeAfter, worker.closeCalls)
	}
}

func TestControlledMCPWorkerQuiesceStopsOwnerBeforeUnknownSweep(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	process := &controlledMCPProcessFixture{order: &order, digest: digest, callErr: errors.New("transport lost after send")}
	factory := &controlledMCPFactoryFixture{order: &order, process: process}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-1", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", digest, "command-1", "package-1"),
	}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-1", request); err == nil || !state.pending {
		t.Fatalf("ambiguous MCP call result err=%v pending=%t", err, state.pending)
	}
	state.quiesce()
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-2", request); err == nil {
		t.Fatal("quiesced session admitted another MCP call")
	}
	if err := state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-1"); err != nil {
		t.Fatalf("owner stop and unknown reconciliation: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"authorize", "intent", "owner_start", "call", "owner_stop", "unknown"}) {
		t.Fatalf("MCP cleanup order=%v", order)
	}
}

func TestControlledMCPWorkerSchemaDriftRevokesRuntimeBeforeCleanup(t *testing.T) {
	const approved = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const observed = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	order := []string{}
	process := &controlledMCPProcessFixture{order: &order, digest: approved, callErr: &mcptransport.ToolSchemaDriftError{ObservedDigest: observed}}
	factory := &controlledMCPFactoryFixture{order: &order, process: process}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-1", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: approved},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", approved, "command-1", "package-1"),
	}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-1", request); err == nil {
		t.Fatal("schema-drifted MCP call was accepted")
	}
	if kernelFixture.driftDigest != observed || !state.blocked || !state.pending {
		t.Fatalf("schema drift was not recorded and fenced: digest=%q blocked=%t pending=%t", kernelFixture.driftDigest, state.blocked, state.pending)
	}
	state.quiesce()
	if err := state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"authorize", "intent", "owner_start", "call", "schema_drift", "owner_stop", "unknown"}) {
		t.Fatalf("schema drift cleanup order=%v", order)
	}
}

func TestControlledMCPWorkerKeepsCommittedIntentUnknownWhenBeginReadFails(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	kernelFixture := &controlledMCPKernelFixture{
		order:    &order,
		intent:   kernel.StdioMCPToolCallRecord{IntentID: "intent-ambiguous", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		beginErr: errors.New("post-commit intent read failed"),
	}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, &controlledMCPFactoryFixture{order: &order}, "company-1", kernel.Binding{}, "provider-call-1", request); err == nil {
		t.Fatal("ambiguous intent reservation failure was accepted")
	}
	if !state.pending || !state.blocked {
		t.Fatalf("ambiguous intent reservation must block and remain pending: pending=%t blocked=%t", state.pending, state.blocked)
	}
	state.quiesce()
	if err := state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"authorize", "intent", "unknown"}) {
		t.Fatalf("ambiguous reservation reconciliation order=%v", order)
	}
}

func TestControlledMCPWorkerDoesNotReuseOwnerAcrossRuntimeQualificationChanges(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	order := []string{}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: digest, ContentBoundary: mcptransport.StdioResultContentBoundary}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-1", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", digest, "command-1", "package-1"),
	}
	process := &controlledMCPProcessFixture{order: &order, digest: digest, result: result}
	factory := &controlledMCPFactoryFixture{order: &order, process: process}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-1", request); err != nil {
		t.Fatal(err)
	}
	kernelFixture.intent.IntentID = "intent-2"
	kernelFixture.intent.RuntimeQualificationID = "runtime-2"
	kernelFixture.authority = controlledMCPAuthorization("runtime-2", "mcp-1", digest, "command-2", "package-2")
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-2", request); err == nil {
		t.Fatal("WorkerSession reused an owner after runtime qualification and pinned package changed")
	}
	if process.callCount != 1 || !state.blocked || !state.pending {
		t.Fatalf("stale owner was reused or current intent not fenced: calls=%d blocked=%t pending=%t", process.callCount, state.blocked, state.pending)
	}
}

func TestControlledMCPWorkerAuditsStartupSchemaDrift(t *testing.T) {
	const (
		approved = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		observed = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)
	order := []string{}
	kernelFixture := &controlledMCPKernelFixture{
		order:     &order,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-1", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: approved},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", approved, "command-1", "package-1"),
	}
	factory := &controlledMCPFactoryFixture{order: &order, startErr: &mcptransport.ToolSchemaDriftError{ObservedDigest: observed}}
	state := &stdioMCPWorkerState{}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := state.call(context.Background(), kernelFixture, factory, "company-1", kernel.Binding{}, "provider-call-1", request); err == nil {
		t.Fatal("startup schema drift was accepted")
	}
	if kernelFixture.driftDigest != observed || !state.blocked || !state.pending {
		t.Fatalf("startup schema drift was not audited and fenced: digest=%q blocked=%t pending=%t", kernelFixture.driftDigest, state.blocked, state.pending)
	}
	state.quiesce()
	if err := state.stopAfterWorkerStop(context.Background(), kernelFixture, "company-1", "session-1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"authorize", "intent", "owner_start", "schema_drift", "unknown"}) {
		t.Fatalf("startup drift cleanup order=%v", order)
	}
}

func TestControlledMCPWorkerOwnerLeaseRejectsConcurrentSessionBeforeIntent(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lease := newStdioMCPProcessLease()
	firstOrder := []string{}
	secondOrder := []string{}
	result := mcptransport.StdioToolResult{ToolName: "lookup", ToolSchemaSHA256: digest, ContentBoundary: mcptransport.StdioResultContentBoundary}
	firstKernel := &controlledMCPKernelFixture{
		order:     &firstOrder,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-first", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", digest, "command-1", "package-1"),
	}
	secondKernel := &controlledMCPKernelFixture{
		order:     &secondOrder,
		intent:    kernel.StdioMCPToolCallRecord{IntentID: "intent-second", Status: "dispatching", CapabilityID: "mcp-1", RuntimeQualificationID: "runtime-1", ToolName: "lookup", ToolSchemaSHA256: digest},
		authority: controlledMCPAuthorization("runtime-1", "mcp-1", digest, "command-1", "package-1"),
	}
	firstProcess := &controlledMCPProcessFixture{order: &firstOrder, digest: digest, result: result}
	secondProcess := &controlledMCPProcessFixture{order: &secondOrder, digest: digest, result: result}
	firstState := &stdioMCPWorkerState{ownerLease: lease}
	secondState := &stdioMCPWorkerState{ownerLease: lease}
	request := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	if _, err := firstState.call(context.Background(), firstKernel, &controlledMCPFactoryFixture{order: &firstOrder, process: firstProcess}, "company-1", kernel.Binding{}, "provider-first", request); err != nil {
		t.Fatal(err)
	}
	if _, err := secondState.call(context.Background(), secondKernel, &controlledMCPFactoryFixture{order: &secondOrder, process: secondProcess}, "company-1", kernel.Binding{}, "provider-second", request); !errors.Is(err, errStdioMCPProcessOwnerBusy) {
		t.Fatalf("concurrent MCP owner call error=%v, want busy error", err)
	}
	if !reflect.DeepEqual(secondOrder, []string{"authorize"}) || secondState.pending || secondState.blocked {
		t.Fatalf("busy call consumed an intent or blocked its session: order=%v pending=%t blocked=%t", secondOrder, secondState.pending, secondState.blocked)
	}
	firstState.quiesce()
	stopErr := errors.New("owner process-tree stop is unresolved")
	firstProcess.stopErr = stopErr
	if err := firstState.stopAfterWorkerStop(context.Background(), firstKernel, "company-1", "session-first"); !errors.Is(err, stopErr) {
		t.Fatalf("unconfirmed owner stop error=%v", err)
	}
	if _, err := secondState.call(context.Background(), secondKernel, &controlledMCPFactoryFixture{order: &secondOrder, process: secondProcess}, "company-1", kernel.Binding{}, "provider-second", request); !errors.Is(err, errStdioMCPProcessOwnerBusy) {
		t.Fatalf("owner lease released before stop confirmation: %v", err)
	}
	if !reflect.DeepEqual(secondOrder, []string{"authorize", "authorize"}) {
		t.Fatalf("busy retry consumed an intent before the first owner stopped: %v", secondOrder)
	}
	firstProcess.stopErr = nil
	if err := firstState.stopAfterWorkerStop(context.Background(), firstKernel, "company-1", "session-first"); err != nil {
		t.Fatal(err)
	}
	if _, err := secondState.call(context.Background(), secondKernel, &controlledMCPFactoryFixture{order: &secondOrder, process: secondProcess}, "company-1", kernel.Binding{}, "provider-second", request); err != nil {
		t.Fatalf("MCP owner lease was not released after confirmed stop: %v", err)
	}
	secondState.quiesce()
	if err := secondState.stopAfterWorkerStop(context.Background(), secondKernel, "company-1", "session-second"); err != nil {
		t.Fatal(err)
	}
}
