// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
	"polis/internal/kernel"
)

// StartProductTaskJob is the Worker-tool bridge to the existing project-job
// lifecycle. Identity is taken only from the trusted Kernel binding.
func (s *Service) StartProductTaskJob(ctx context.Context, binding kernel.Binding, input kernel.ProductTaskJobStartCommand) (kernel.JobRunRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil ||
		!core.ValidID(binding.CompanyID()) || !core.ValidID(binding.TaskID()) || !core.ValidID(binding.SessionID()) ||
		!core.ValidID(input.EnvironmentRevisionID) || validateRequestID(input.RequestID) != nil {
		return kernel.JobRunRecord{}, core.Malformed
	}
	return s.StartProjectJob(ctx, binding.CompanyID(), StartProjectJobRequest{
		TaskID:                binding.TaskID(),
		SessionID:             binding.SessionID(),
		EnvironmentRevisionID: input.EnvironmentRevisionID,
		Kind:                  input.Kind,
		ServiceID:             input.ServiceID,
		ScriptPath:            input.ScriptPath,
		Args:                  append([]string(nil), input.Args...),
		RequestID:             input.RequestID,
	})
}

// StopProductTaskJob first proves the active WorkerSession owns this exact
// Task JobRun, then delegates process cleanup and durable reconciliation to
// the existing Control lifecycle.
func (s *Service) StopProductTaskJob(ctx context.Context, binding kernel.Binding, jobID, requestID string) (kernel.JobRunRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil ||
		!core.ValidID(binding.CompanyID()) || !core.ValidID(binding.TaskID()) || !core.ValidID(binding.SessionID()) ||
		!core.ValidID(jobID) || validateRequestID(requestID) != nil {
		return kernel.JobRunRecord{}, core.Malformed
	}
	if _, err := s.runtime.ProductTaskJobStatus(ctx, binding, jobID); err != nil {
		return kernel.JobRunRecord{}, err
	}
	return s.StopProjectJob(ctx, binding.CompanyID(), jobID, StopProjectJobRequest{RequestID: requestID})
}

var _ kernel.ProductTaskJobLifecycle = (*Service)(nil)
