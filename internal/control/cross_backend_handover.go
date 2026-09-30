// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) CreateTaskEnvironmentHandover(ctx context.Context, companyID, taskID string, request CreateProjectEnvironmentHandoverRequest) (kernel.CrossBackendHandoverRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(taskID) || !core.ValidID(request.SourceJobID) ||
		!core.ValidID(request.TargetEnvironmentRevision) || validateRequestID(request.RequestID) != nil {
		return kernel.CrossBackendHandoverRecord{}, core.Malformed
	}
	return s.runtime.TXCreateCrossBackendHandover(ctx, companyID, kernel.CrossBackendHandoverInput{
		TaskID: taskID, SourceJobID: request.SourceJobID, TargetEnvironmentRevision: request.TargetEnvironmentRevision, RequestID: request.RequestID,
	})
}
