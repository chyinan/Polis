// pattern: Imperative Shell
package main

import (
	"errors"
	"testing"
)

type closeRetryFixture struct {
	results []error
	calls   int
}

func (fixture *closeRetryFixture) Close() error {
	fixture.calls++
	if fixture.calls <= len(fixture.results) {
		return fixture.results[fixture.calls-1]
	}
	return nil
}

func TestCloseCommandServiceWithRetryRetriesOneTransientFailure(t *testing.T) {
	firstErr := errors.New("transient worker cleanup failure")
	service := &closeRetryFixture{results: []error{firstErr, nil}}
	if err := closeCommandServiceWithRetry(service); err != nil {
		t.Fatalf("shutdown recovered on the bounded retry: %v", err)
	}
	if service.calls != 2 {
		t.Fatalf("shutdown close attempts=%d, want exactly two", service.calls)
	}
}

func TestCloseCommandServiceWithRetryPreservesBothFailures(t *testing.T) {
	firstErr := errors.New("first cleanup failure")
	retryErr := errors.New("retry cleanup failure")
	service := &closeRetryFixture{results: []error{firstErr, retryErr}}
	err := closeCommandServiceWithRetry(service)
	if !errors.Is(err, firstErr) || !errors.Is(err, retryErr) || service.calls != 2 {
		t.Fatalf("shutdown retry error=%v calls=%d, want both failures and exactly two attempts", err, service.calls)
	}
}
