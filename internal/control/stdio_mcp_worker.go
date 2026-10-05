// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"polis/internal/kernel"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
	"polis/internal/runner"
)

type controlledMCPProcess interface {
	CallTool(context.Context, string, []byte) (mcptransport.StdioToolResult, error)
	ToolSchemaSHA256() string
	Stop(context.Context) error
}

type controlledMCPDispatchPermitProcess interface {
	CallToolWithPermit(context.Context, string, []byte, func(context.Context) error) (mcptransport.StdioToolResult, error)
}

type controlledMCPProcessFactory interface {
	Start(context.Context, mcpowner.ProcessSpec) (controlledMCPProcess, error)
}

type streamableHTTPMCPProcessFactory interface {
	StartStreamableHTTP(context.Context, string, string) (controlledMCPProcess, error)
}

type directStreamableHTTPMCPProcessFactory struct{}

func (directStreamableHTTPMCPProcessFactory) StartStreamableHTTP(ctx context.Context, endpoint, expectedToolSchema string) (controlledMCPProcess, error) {
	if os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") != "1" {
		return nil, errors.New("Streamable HTTP MCP Worker egress is disabled")
	}
	if ctx == nil || ctx.Err() != nil || !validSHA256Digest(expectedToolSchema) {
		return nil, errors.New("Streamable HTTP MCP Worker profile is unavailable")
	}
	client, err := mcptransport.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return newStreamableHTTPMCPProcess(client, expectedToolSchema), nil
}

var errStdioMCPProcessOwnerBusy = errors.New("a controlled MCP process owner is already active")

type stdioMCPProcessLease struct {
	active chan struct{}
}

func newStdioMCPProcessLease() *stdioMCPProcessLease {
	return &stdioMCPProcessLease{active: make(chan struct{}, 1)}
}

func (lease *stdioMCPProcessLease) tryAcquire() bool {
	if lease == nil || lease.active == nil {
		return false
	}
	select {
	case lease.active <- struct{}{}:
		return true
	default:
		return false
	}
}

func (lease *stdioMCPProcessLease) release() {
	if lease == nil || lease.active == nil {
		return
	}
	select {
	case <-lease.active:
	default:
	}
}

type appContainerControlledMCPFactory struct {
	sandbox *runner.AppContainerSandbox
}

type appContainerControlledMCPProcess struct {
	owner *mcpowner.Owner
}

func (factory *appContainerControlledMCPFactory) Start(ctx context.Context, spec mcpowner.ProcessSpec) (controlledMCPProcess, error) {
	owner, err := mcpowner.Start(ctx, factory.sandbox, spec)
	if owner == nil {
		return nil, err
	}
	return &appContainerControlledMCPProcess{owner: owner}, err
}

func (process *appContainerControlledMCPProcess) CallTool(ctx context.Context, name string, arguments []byte) (mcptransport.StdioToolResult, error) {
	return mcptransport.StdioToolResult{}, errors.New("MCP dispatch permit is required")
}

func (process *appContainerControlledMCPProcess) CallToolWithPermit(ctx context.Context, name string, arguments []byte, consume func(context.Context) error) (mcptransport.StdioToolResult, error) {
	if process == nil || process.owner == nil {
		return mcptransport.StdioToolResult{}, errors.New("stdio MCP process owner is unavailable")
	}
	if consume == nil {
		return mcptransport.StdioToolResult{}, errors.New("MCP dispatch permit is required")
	}
	if err := consume(ctx); err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	return process.owner.CallTool(ctx, name, arguments)
}

func (process *appContainerControlledMCPProcess) ToolSchemaSHA256() string {
	if process == nil || process.owner == nil {
		return ""
	}
	return process.owner.ToolSchemaSHA256()
}

func (process *appContainerControlledMCPProcess) Stop(ctx context.Context) error {
	if process == nil || process.owner == nil {
		return nil
	}
	return process.owner.Stop(ctx)
}

func (factory *appContainerControlledMCPFactory) Close() error {
	if factory == nil || factory.sandbox == nil {
		return nil
	}
	return factory.sandbox.Close()
}

type stdioMCPKernel interface {
	LockStdioMCPPackageServer(context.Context, string, string) (context.Context, func(), error)
	AuthorizeStdioMCPToolCall(context.Context, kernel.Binding, string, string, string) (kernel.StdioMCPToolAuthorization, error)
	TXRecordStdioMCPToolSchemaDrift(context.Context, string, kernel.StdioMCPToolSchemaDriftInput) error
	TXCompleteStdioMCPToolCall(context.Context, kernel.Binding, string, mcptransport.StdioToolResult) error
	TXMarkStdioMCPToolCallsUnknownForSession(context.Context, string, string, string) (int, error)
}

type stdioMCPDispatchPermitKernel interface {
	TXBeginStdioMCPToolCall(context.Context, kernel.Binding, kernel.StdioMCPToolCallIntentInput) (kernel.StdioMCPToolDispatchPermit, error)
	TXConsumeStdioMCPToolCallPermit(context.Context, kernel.Binding, kernel.StdioMCPToolDispatchPermitConsumption) (kernel.StdioMCPToolCallRecord, error)
}

type stdioMCPWorkerState struct {
	mu                     sync.Mutex
	owner                  controlledMCPProcess
	capabilityID           string
	runtimeQualificationID string
	commandSHA256          string
	packageManifestSHA256  string
	toolSchemaSHA256       string
	transport              string
	endpoint               string
	ownerLease             *stdioMCPProcessLease
	leaseHeld              bool
	allowStreamableHTTP    bool
	pending                bool
	blocked                bool
	closed                 bool
}

func (state *stdioMCPWorkerState) call(ctx context.Context, runtime stdioMCPKernel, factory controlledMCPProcessFactory, companyID string, binding kernel.Binding, providerCallID string, raw json.RawMessage) (json.RawMessage, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closed || state.blocked || ctx == nil || ctx.Err() != nil || runtime == nil {
		return nil, errors.New("controlled MCP worker session is unavailable")
	}
	permitRuntime, ok := runtime.(stdioMCPDispatchPermitKernel)
	if !ok {
		state.blocked = true
		return nil, errors.New("controlled MCP runtime does not support dispatch permits")
	}
	input, err := parseControlledMCPCallArguments(raw)
	if err != nil {
		return nil, err
	}
	lockedContext, releaseServerLock, err := runtime.LockStdioMCPPackageServer(ctx, companyID, input.CapabilityID)
	if err != nil {
		return nil, err
	}
	defer releaseServerLock()
	ctx = lockedContext
	if state.owner != nil && state.capabilityID != input.CapabilityID {
		state.blocked = true
		return nil, errors.New("WorkerSession is already pinned to its one controlled MCP server")
	}
	authorization, err := runtime.AuthorizeStdioMCPToolCall(ctx, binding, input.CapabilityID, input.ToolName, input.ToolSchemaSHA256)
	if err != nil {
		return nil, err
	}
	transport := authorization.Transport
	if transport == "" {
		transport = "stdio" // Compatibility for historical Worker test fixtures.
	}
	if transport == "streamable_http" && (!state.allowStreamableHTTP || os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") != "1") {
		return nil, errors.New("Streamable HTTP MCP Worker egress is disabled")
	}
	if transport == "stdio" && factory == nil {
		return nil, errors.New("controlled stdio MCP Worker client is unavailable")
	}
	if state.owner == nil && state.ownerLease != nil && !state.leaseHeld {
		if !state.ownerLease.tryAcquire() {
			return nil, errStdioMCPProcessOwnerBusy
		}
		state.leaseHeld = true
	}
	callInput := kernel.StdioMCPToolCallIntentInput{
		ProviderCallID: providerCallID, CapabilityID: input.CapabilityID, ToolName: input.ToolName,
		ToolSchemaSHA256: input.ToolSchemaSHA256, Arguments: input.Arguments,
	}
	intent := kernel.StdioMCPToolCallRecord{}
	qualification := authorization.RuntimeQualification
	processSpec := authorization.ProcessSpec
	if qualification.CapabilityID != input.CapabilityID || qualification.ToolSchemaSHA256 != input.ToolSchemaSHA256 {
		state.blocked = true
		return nil, errors.New("MCP authorization no longer matches the reserved intent")
	}
	if transport == "stdio" {
		if qualification.Transport != "" && qualification.Transport != "stdio" || qualification.CommandSHA256 == "" || qualification.PackageManifestSHA256 == "" ||
			processSpec.CommandSHA256 != qualification.CommandSHA256 || processSpec.PackageManifestSHA256 != qualification.PackageManifestSHA256 || processSpec.ApprovedToolSchemaSHA256 != input.ToolSchemaSHA256 {
			state.blocked = true
			return nil, errors.New("stdio MCP authorization no longer matches its pinned process specification")
		}
	} else if transport == "streamable_http" {
		if qualification.Transport != "streamable_http" || authorization.Endpoint == "" || authorization.Endpoint != qualification.Endpoint {
			state.blocked = true
			return nil, errors.New("Streamable HTTP MCP authorization no longer matches its pinned endpoint")
		}
	} else {
		state.blocked = true
		return nil, errors.New("MCP transport is outside the current Worker profile")
	}
	if state.owner != nil && (state.capabilityID != input.CapabilityID || state.transport != transport || state.endpoint != authorization.Endpoint ||
		state.runtimeQualificationID != qualification.RuntimeQualificationID ||
		state.commandSHA256 != qualification.CommandSHA256 ||
		state.packageManifestSHA256 != qualification.PackageManifestSHA256 ||
		state.toolSchemaSHA256 != qualification.ToolSchemaSHA256) {
		state.blocked = true
		return nil, errors.New("WorkerSession owner differs from the current MCP runtime qualification")
	}
	if state.owner == nil {
		var owner controlledMCPProcess
		var startErr error
		if transport == "streamable_http" {
			remoteFactory, ok := factory.(streamableHTTPMCPProcessFactory)
			if !ok {
				remoteFactory = directStreamableHTTPMCPProcessFactory{}
			}
			owner, startErr = remoteFactory.StartStreamableHTTP(ctx, authorization.Endpoint, input.ToolSchemaSHA256)
		} else {
			owner, startErr = factory.Start(ctx, processSpec)
		}
		if owner != nil {
			state.owner = owner
			state.capabilityID = input.CapabilityID
			state.transport = transport
			state.endpoint = authorization.Endpoint
			state.runtimeQualificationID = qualification.RuntimeQualificationID
			state.commandSHA256 = qualification.CommandSHA256
			state.packageManifestSHA256 = qualification.PackageManifestSHA256
			state.toolSchemaSHA256 = qualification.ToolSchemaSHA256
		}
		if startErr != nil {
			state.blocked = true
			driftRequestID := providerCallID
			if intent.IntentID != "" {
				driftRequestID = intent.IntentID + "-schema-drift"
			}
			return nil, errors.Join(startErr, recordStdioMCPToolSchemaDrift(ctx, runtime, companyID, qualification.RuntimeQualificationID, driftRequestID, startErr))
		}
	}
	if state.owner == nil || state.owner.ToolSchemaSHA256() != input.ToolSchemaSHA256 {
		state.blocked = true
		return nil, errors.New("MCP process owner schema differs from the reserved intent")
	}
	permitProcess, ok := state.owner.(controlledMCPDispatchPermitProcess)
	if !ok {
		state.blocked = true
		return nil, errors.New("controlled MCP process does not support dispatch permits")
	}
	// Issue only after local/runtime preparation has completed, keeping the
	// short validity window close to the final external dispatch boundary.
	state.pending = true
	permit, permitErr := permitRuntime.TXBeginStdioMCPToolCall(ctx, binding, callInput)
	if permitErr != nil {
		state.blocked = true
		return nil, permitErr
	}
	if permitErr = kernel.ValidateStdioMCPToolDispatchPermit(permit, callInput, authorization, binding); permitErr != nil {
		state.blocked = true
		return nil, permitErr
	}
	consume := func(consumeContext context.Context) error {
		consumed, consumeErr := permitRuntime.TXConsumeStdioMCPToolCallPermit(consumeContext, binding,
			kernel.StdioMCPToolDispatchPermitConsumption{PermitID: permit.PermitID, Call: callInput})
		if consumeErr != nil {
			return consumeErr
		}
		intent = consumed
		return kernel.ValidateStdioMCPToolCallStart(consumed, callInput, authorization, binding)
	}
	result, err := permitProcess.CallToolWithPermit(ctx, input.ToolName, input.Arguments, consume)
	if err != nil {
		state.blocked = true
		driftRequestID := providerCallID
		if intent.IntentID != "" {
			driftRequestID = intent.IntentID + "-schema-drift"
		}
		return nil, errors.Join(err, recordStdioMCPToolSchemaDrift(ctx, runtime, companyID, qualification.RuntimeQualificationID, driftRequestID, err))
	}
	if err = runtime.TXCompleteStdioMCPToolCall(ctx, binding, intent.IntentID, result); err != nil {
		state.blocked = true
		return nil, err
	}
	state.pending = false
	encoded, err := json.Marshal(result)
	if err != nil {
		state.blocked = true
		return nil, err
	}
	return encoded, nil
}

func (state *stdioMCPWorkerState) shouldStopAfterFailure() bool {
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.blocked
}

func (state *stdioMCPWorkerState) quiesce() {
	if state == nil {
		return
	}
	state.mu.Lock()
	state.closed = true
	state.mu.Unlock()
}

func (state *stdioMCPWorkerState) stopAfterWorkerStop(ctx context.Context, runtime stdioMCPKernel, companyID, sessionID string) error {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.owner != nil {
		if err := state.owner.Stop(ctx); err != nil {
			return err
		}
		state.owner = nil
		state.capabilityID = ""
		state.runtimeQualificationID = ""
		state.commandSHA256 = ""
		state.packageManifestSHA256 = ""
		state.toolSchemaSHA256 = ""
		state.transport = ""
		state.endpoint = ""
	}
	if state.leaseHeld {
		state.ownerLease.release()
		state.leaseHeld = false
	}
	if state.pending {
		if _, err := runtime.TXMarkStdioMCPToolCallsUnknownForSession(ctx, companyID, sessionID, "worker_interrupted_during_call"); err != nil {
			return err
		}
		state.pending = false
	}
	return nil
}

func recordStdioMCPToolSchemaDrift(ctx context.Context, runtime stdioMCPKernel, companyID, runtimeQualificationID, requestID string, cause error) error {
	var drift *mcptransport.ToolSchemaDriftError
	if !errors.As(cause, &drift) || drift.ObservedDigest == "" {
		return nil
	}
	return runtime.TXRecordStdioMCPToolSchemaDrift(ctx, companyID, kernel.StdioMCPToolSchemaDriftInput{
		RuntimeQualificationID:   runtimeQualificationID,
		ObservedToolSchemaSHA256: drift.ObservedDigest,
		RequestID:                requestID,
	})
}
