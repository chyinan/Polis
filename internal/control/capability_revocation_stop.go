// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	capabilityRevocationStopInterval       = 15 * time.Second
	capabilityRevocationStopTimeout        = 30 * time.Second
	capabilityRevocationSessionStopTimeout = 10 * time.Second
)

// WorkerSessionStopper stops one exact WorkerSession and confirms its host
// process tree is gone before returning success.
type WorkerSessionStopper interface {
	StopSession(ctx context.Context, companyID, sessionID string) error
}

// StartCapabilityRevocationWorkerStopper starts the bounded, restart-safe
// coordinator that drains the durable revoke-time session snapshots. The first
// scan runs immediately; future revocations signal the loop for prompt work.
func (s *Service) StartCapabilityRevocationWorkerStopper(ctx context.Context, reportError func(error)) (func(), error) {
	if ctx == nil || s == nil || s.runtime == nil || s.worker == nil {
		return nil, errors.New("capability revocation Worker stop dependencies are incomplete")
	}
	if _, ok := s.worker.(WorkerSessionStopper); !ok {
		return nil, errors.New("Worker adapter does not support exact-session stop")
	}
	s.capabilityRevocationStopMu.Lock()
	if s.capabilityRevocationStopRunning {
		s.capabilityRevocationStopMu.Unlock()
		return nil, errors.New("capability revocation Worker stopper is already running")
	}
	s.capabilityRevocationStopRunning = true
	wake := make(chan struct{}, 1)
	s.capabilityRevocationStopWake = wake
	s.capabilityRevocationStopMu.Unlock()

	stopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runCapabilityRevocationWorkerStopper(stopCtx, wake, reportError)
	}()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			cancel()
			<-done
			s.capabilityRevocationStopMu.Lock()
			s.capabilityRevocationStopRunning = false
			s.capabilityRevocationStopWake = nil
			s.capabilityRevocationStopMu.Unlock()
		})
	}
	return stop, nil
}

func (s *Service) runCapabilityRevocationWorkerStopper(ctx context.Context, wake <-chan struct{}, reportError func(error)) {
	ticker := time.NewTicker(capabilityRevocationStopInterval)
	defer ticker.Stop()
	cursor := capabilityRevocationStopCursor{}
	for {
		iterationCtx, cancel := context.WithTimeout(ctx, capabilityRevocationStopTimeout)
		err := s.drainCapabilityRevocationWorkerStops(iterationCtx, &cursor)
		cancel()
		if err != nil && reportError != nil {
			reportError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
		}
	}
}

type capabilityRevocationStopCursor struct {
	companyID string
	sessionID string
}

func (s *Service) drainCapabilityRevocationWorkerStops(ctx context.Context, cursor *capabilityRevocationStopCursor) error {
	candidates, err := s.runtime.CapabilityRevocationStopCandidates(ctx, cursor.companyID, cursor.sessionID)
	if err != nil {
		return fmt.Errorf("list capability-revoked WorkerSessions: %w", err)
	}
	if len(candidates) == 0 {
		*cursor = capabilityRevocationStopCursor{}
		return nil
	}
	stopper := s.worker.(WorkerSessionStopper)
	var failures []error
	for _, candidate := range candidates {
		*cursor = capabilityRevocationStopCursor{companyID: candidate.CompanyID, sessionID: candidate.SessionID}
		stopCtx, cancel := context.WithTimeout(ctx, capabilityRevocationSessionStopTimeout)
		var candidateErr error
		if s.lockProjectLifecycleUntil(stopCtx) {
			if candidate.State != "stopped" {
				candidateErr = stopper.StopSession(stopCtx, candidate.CompanyID, candidate.SessionID)
			}
			if candidateErr == nil && candidate.HasDispatchingMCPToolCalls {
				_, candidateErr = s.runtime.TXMarkStdioMCPToolCallsUnknownForSession(stopCtx, candidate.CompanyID, candidate.SessionID, "worker_interrupted_during_call")
			}
			s.projectLifecycleMu.Unlock()
		} else {
			candidateErr = stopCtx.Err()
		}
		cancel()
		if candidateErr != nil {
			failures = append(failures, fmt.Errorf("stop capability-revoked WorkerSession %s/%s: %w", candidate.CompanyID, candidate.SessionID, candidateErr))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) lockProjectLifecycleUntil(ctx context.Context) bool {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.projectLifecycleMu.TryLock() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func (s *Service) wakeCapabilityRevocationWorkerStopper() {
	if s == nil {
		return
	}
	s.capabilityRevocationStopMu.Lock()
	wake := s.capabilityRevocationStopWake
	s.capabilityRevocationStopMu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}
