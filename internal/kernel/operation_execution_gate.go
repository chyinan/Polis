// pattern: Functional Core
package kernel

import "context"

func browserRunExecutionAuthorized(ctx context.Context, executor BrowserRunExecutor, binding Binding, request BrowserRunRequest, record BrowserRunRecord) bool {
	if ctx == nil || executor == nil {
		return false
	}
	authorizer, ok := executor.(BrowserRunExecutionAuthorizer)
	return ok && authorizer.AuthorizeBrowserRunExecution(ctx, binding, request, record) == nil
}

func researchOperationExecutionAuthorized(ctx context.Context, executor ResearchOperationExecutor, binding Binding, request ResearchOperationRequest, record ResearchOperationRecord) bool {
	if ctx == nil || executor == nil {
		return false
	}
	authorizer, ok := executor.(ResearchOperationExecutionAuthorizer)
	return ok && authorizer.AuthorizeResearchOperationExecution(ctx, binding, request, record) == nil
}
