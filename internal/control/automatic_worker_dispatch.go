// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const automaticProductWorkerDispatchInterval = 30 * time.Second
const automaticProductWorkerDispatchTimeout = 10 * time.Second

type offlineFakeProductDispatchReadiness interface {
	AutomaticProductDispatchReadiness(context.Context) error
}

// StartAutomaticProductWorkerDispatcher starts the bounded, opt-in admission
// loop for an explicitly enabled zero-egress Fake @7 Worker. It only admits
// durable Tasks in already-active Missions; it never starts a Mission.
func (s *Service) StartAutomaticProductWorkerDispatcher(ctx context.Context, reportError func(error)) (func(), error) {
	if ctx == nil || s == nil || s.runtime == nil || s.worker == nil {
		return nil, errors.New("automatic product Worker dispatch dependencies are incomplete")
	}
	dispatcher, ok := s.worker.(offlineFakeProductDispatchReadiness)
	if !ok {
		return nil, errors.New("automatic product Worker dispatch requires the zero-egress Fake @7 runtime")
	}
	if err := dispatcher.AutomaticProductDispatchReadiness(ctx); err != nil {
		return nil, fmt.Errorf("automatic product Worker dispatch readiness failed: %w", err)
	}
	dispatchCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runAutomaticProductWorkerDispatcher(dispatchCtx, reportError)
	}()
	stop := func() {
		cancel()
		<-done
	}
	return stop, nil
}

func (s *Service) runAutomaticProductWorkerDispatcher(ctx context.Context, reportError func(error)) {
	ticker := time.NewTicker(automaticProductWorkerDispatchInterval)
	defer ticker.Stop()
	for {
		iterationCtx, cancel := context.WithTimeout(ctx, automaticProductWorkerDispatchTimeout)
		err := s.dispatchAutomaticProductWorkerOnce(iterationCtx)
		cancel()
		if err != nil && reportError != nil {
			reportError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) dispatchAutomaticProductWorkerOnce(ctx context.Context) error {
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()
	candidate, found, err := s.runtime.NextProductWorkerDispatchCandidate(ctx, s.automaticProductDispatchCursor)
	if err != nil {
		return err
	}
	if !found {
		s.automaticProductDispatchCursor = ""
		return nil
	}
	s.automaticProductDispatchCursor = candidate.CompanyID
	if err = s.worker.Start(ctx, candidate.CompanyID, candidate.MissionID); err != nil {
		return fmt.Errorf("automatic product Worker dispatch failed for company %s mission %s task %s: %w", candidate.CompanyID, candidate.MissionID, candidate.TaskID, err)
	}
	return nil
}
