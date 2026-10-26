package kernel

import (
	"context"
	"errors"
	"testing"
)

type unqualifiedBrowserExecutor struct{}

func (unqualifiedBrowserExecutor) ExecuteBrowserRun(context.Context, Binding, BrowserRunRequest, BrowserRunRecord) (BrowserRunSuccessInput, error) {
	return BrowserRunSuccessInput{}, nil
}

type qualifiedBrowserExecutor struct{ authorizationErr error }

func (executor qualifiedBrowserExecutor) ExecuteBrowserRun(context.Context, Binding, BrowserRunRequest, BrowserRunRecord) (BrowserRunSuccessInput, error) {
	return BrowserRunSuccessInput{}, nil
}

func (executor qualifiedBrowserExecutor) AuthorizeBrowserRunExecution(context.Context, Binding, BrowserRunRequest, BrowserRunRecord) error {
	return executor.authorizationErr
}

type unqualifiedResearchExecutor struct{}

func (unqualifiedResearchExecutor) ExecuteResearchOperation(context.Context, Binding, ResearchOperationRequest, ResearchOperationRecord) (ResearchOperationSuccessInput, error) {
	return ResearchOperationSuccessInput{}, nil
}

type qualifiedResearchExecutor struct{ authorizationErr error }

func (executor qualifiedResearchExecutor) ExecuteResearchOperation(context.Context, Binding, ResearchOperationRequest, ResearchOperationRecord) (ResearchOperationSuccessInput, error) {
	return ResearchOperationSuccessInput{}, nil
}

func (executor qualifiedResearchExecutor) AuthorizeResearchOperationExecution(context.Context, Binding, ResearchOperationRequest, ResearchOperationRecord) error {
	return executor.authorizationErr
}

func TestOperationExecutionRequiresSeparateRuntimeQualificationAuthorizer(t *testing.T) {
	ctx := context.Background()
	if browserRunExecutionAuthorized(ctx, unqualifiedBrowserExecutor{}, Binding{}, BrowserRunRequest{}, BrowserRunRecord{}) {
		t.Fatal("BrowserRun executor without host/profile/network authorization was admitted")
	}
	if browserRunExecutionAuthorized(ctx, qualifiedBrowserExecutor{authorizationErr: errors.New("host policy unqualified")}, Binding{}, BrowserRunRequest{}, BrowserRunRecord{}) {
		t.Fatal("BrowserRun executor ignored a negative qualification result")
	}
	if !browserRunExecutionAuthorized(ctx, qualifiedBrowserExecutor{}, Binding{}, BrowserRunRequest{}, BrowserRunRecord{}) {
		t.Fatal("BrowserRun executor rejected an explicit positive qualification result")
	}
	if researchOperationExecutionAuthorized(ctx, unqualifiedResearchExecutor{}, Binding{}, ResearchOperationRequest{}, ResearchOperationRecord{}) {
		t.Fatal("ResearchOperation executor without endpoint/credential/host authorization was admitted")
	}
	if researchOperationExecutionAuthorized(ctx, qualifiedResearchExecutor{authorizationErr: errors.New("endpoint unqualified")}, Binding{}, ResearchOperationRequest{}, ResearchOperationRecord{}) {
		t.Fatal("ResearchOperation executor ignored a negative qualification result")
	}
	if !researchOperationExecutionAuthorized(ctx, qualifiedResearchExecutor{}, Binding{}, ResearchOperationRequest{}, ResearchOperationRecord{}) {
		t.Fatal("ResearchOperation executor rejected an explicit positive qualification result")
	}
}
