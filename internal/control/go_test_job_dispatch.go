// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/kernel"
	"time"
)

type activeGoJob struct {
	request  gotestsupervisor.Request
	executor gotestsupervisor.Executor
}

// SetGoTestJobExecutor configures a fixed-profile adapter. Injected adapters
// are always simulation provenance; HTTP callers cannot select an executor.
func (s *Service) SetGoTestJobExecutor(e gotestsupervisor.Executor) error {
	s.projectJobsMu.Lock()
	defer s.projectJobsMu.Unlock()
	if s.projectJobsClosed || len(s.projectJobs) > 0 {
		return core.Conflict
	}
	s.goJobRunner = e
	return nil
}

func goExecutionKind(e gotestsupervisor.Executor) string {
	if _, ok := e.(*gotestsupervisor.SystemdExecutor); ok {
		return "real"
	}
	return "simulation"
}

func (s *Service) dispatchGoTestJob(ctx context.Context, company string, b gotestprofile.JobBinding, e gotestsupervisor.Executor, h gotestsupervisor.HostCheck) (kernel.JobRunRecord, error) {
	lifecycleCtx, commandDone, _, err := s.beginProjectJobCommand()
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	defer commandDone()
	r, err := s.runtime.GoTestJobRequest(ctx, company, b.JobRunID)
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	r.HostQualificationSHA256 = h.QualificationSHA256
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return kernel.JobRunRecord{}, err
	}
	claimed, err := s.runtime.TXClaimGoTestJobDispatch(ctx, company, b.JobRunID, hex.EncodeToString(nonce[:]))
	if errors.Is(err, core.Conflict) || !claimed && err == nil {
		return s.runtime.GetJobRun(ctx, company, b.JobRunID)
	}
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	jobCtx, cancel := context.WithCancel(lifecycleCtx)
	active := &activeProjectJob{companyID: company, jobID: b.JobRunID, ctx: jobCtx, cancel: cancel, done: make(chan struct{}), goRun: &activeGoJob{request: r, executor: e}}
	s.projectJobsMu.Lock()
	if s.projectJobsClosed {
		cancel()
	}
	s.projectJobs[b.JobRunID] = active
	s.projectJobsWG.Add(1)
	s.projectJobsMu.Unlock()
	go s.monitorGoTestJob(active)
	return s.runtime.GetJobRun(ctx, company, b.JobRunID)
}

func (s *Service) monitorGoTestJob(active *activeProjectJob) {
	defer func() {
		active.cancel()
		s.projectJobsMu.Lock()
		delete(s.projectJobs, active.jobID)
		s.projectJobsMu.Unlock()
		close(active.done)
		s.projectJobsWG.Done()
	}()
	r := active.goRun.request
	e := active.goRun.executor
	report := e.Run(active.ctx, r)
	report.ExecutionKind = goExecutionKind(e)
	if report.State == "blocked" {
		report.State = "failed"
	}
	if report.ExecutionKind == "simulation" && report.Outcome == gotestprofile.OutcomePassed {
		report = gotestsupervisor.NewUnstartedReport(r.Binding, "simulation", "failed", "simulated_pass_not_authoritative", gotestsupervisor.HostCheck{})
	}
	if gotestsupervisor.ValidateReport(r.Binding, report) != nil {
		report = gotestsupervisor.NewUnstartedReport(r.Binding, goExecutionKind(e), "outcome_unknown", "supervisor_report_invalid", gotestsupervisor.HostCheck{})
		report.DispatchAccepted = true
		report.CleanupConfirmed = false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.runtime.TXRecordGoTestJobReport(ctx, active.companyID, "go-result-"+active.jobID, report); err != nil {
		log.Printf("go job result persistence unavailable: company=%s job=%s", active.companyID, active.jobID)
	}
	// Persistence failure leaves a durable starting intent for reconciliation;
	// it never reruns candidate code or invents a terminal success.
}

type GoTestJobCollectionService interface {
	CollectGoTestJobResult(context.Context, string, string, string) (kernel.JobRunRecord, error)
}

func (s *Service) CollectGoTestJobResult(ctx context.Context, company, jobID, requestID string) (kernel.JobRunRecord, error) {
	if validateRequestID(requestID) != nil {
		return kernel.JobRunRecord{}, core.Malformed
	}
	job, err := s.runtime.GetJobRun(ctx, company, jobID)
	if err != nil {
		return job, err
	}
	if job.Kind != "go_test" {
		return job, core.Denied
	}
	if job.State != "starting" && job.State != "running" {
		return job, nil
	}
	s.projectJobsMu.Lock()
	active := s.projectJobs[jobID]
	executor := s.goJobRunner
	s.projectJobsMu.Unlock()
	if active != nil {
		return job, nil
	}
	if executor == nil {
		executor = gotestsupervisor.NewSystemdExecutor(gotestsupervisor.InstalledBinary, os.Getenv("POLIS_GO_TOOLCHAIN_ROOT"))
	}
	collector, ok := executor.(interface {
		Collect(context.Context, gotestsupervisor.Request) gotestsupervisor.Report
	})
	if !ok {
		return job, core.Denied
	}
	r, err := s.runtime.GoTestJobRequest(ctx, company, jobID)
	if err != nil {
		return job, err
	}
	report := collector.Collect(ctx, r)
	if report.State == "running" || report.State == "starting" {
		return job, nil
	}
	report.ExecutionKind = goExecutionKind(executor)
	if report.ExecutionKind == "simulation" && report.Outcome == gotestprofile.OutcomePassed {
		return job, core.Denied
	}
	if _, err = s.runtime.TXRecordGoTestJobReport(ctx, company, requestID, report); err != nil {
		return job, err
	}
	return s.runtime.GetJobRun(ctx, company, jobID)
}

func (s *Service) stopGoTestJob(ctx context.Context, company, jobID string) (kernel.JobRunRecord, error) {
	s.projectJobsMu.Lock()
	active := s.projectJobs[jobID]
	s.projectJobsMu.Unlock()
	if active == nil {
		job, err := s.runtime.GetJobRun(ctx, company, jobID)
		if err != nil {
			return job, err
		}
		if job.State != "starting" && job.State != "running" && job.State != "outcome_unknown" {
			return job, nil
		}
		s.projectJobsMu.Lock()
		executor := s.goJobRunner
		s.projectJobsMu.Unlock()
		if executor == nil {
			executor = gotestsupervisor.NewSystemdExecutor(gotestsupervisor.InstalledBinary, os.Getenv("POLIS_GO_TOOLCHAIN_ROOT"))
		}
		if err = executor.Stop(ctx, jobID); err != nil {
			return job, err
		}
		if _, err = s.runtime.TXConfirmGoJobStopped(ctx, company, jobID, "go-stop-"+jobID); err != nil {
			return job, err
		}
		return s.runtime.GetJobRun(ctx, company, jobID)
	}
	active.markStopRequested()
	active.cancel()
	select {
	case <-active.done:
		return s.runtime.GetJobRun(ctx, company, jobID)
	case <-ctx.Done():
		return kernel.JobRunRecord{}, ctx.Err()
	}
}
