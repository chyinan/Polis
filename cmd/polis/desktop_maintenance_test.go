package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDesktopMaintenanceQuiesceDrainsRequestsAndRejectsNewWork(t *testing.T) {
	gate := newDesktopMaintenanceGate()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	var enteredOnce sync.Once
	router := http.NewServeMux()
	router.HandleFunc("/api/workbench/mutate", func(w http.ResponseWriter, _ *http.Request) {
		enteredOnce.Do(func() { close(entered) })
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("/api/desktop/maintenance/quiesce", func(w http.ResponseWriter, r *http.Request) {
		ok, err := gate.Quiesce(r.Context(), func(context.Context) (bool, error) {
			return false, nil
		}, false)
		if err != nil || !ok {
			http.Error(w, "quiesce failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	server := httptest.NewServer(gate.Middleware(router))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })

	mutationDone := make(chan *http.Response, 1)
	go func() {
		response, err := http.Get(server.URL + "/api/workbench/mutate")
		if err != nil {
			mutationDone <- nil
			return
		}
		mutationDone <- response
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("mutation handler did not start")
	}

	var quiesceResponse *http.Response
	var quiesceErr error
	quiesceDone := make(chan struct{})
	go func() {
		defer close(quiesceDone)
		quiesceResponse, quiesceErr = http.Post(server.URL+"/api/desktop/maintenance/quiesce", "application/json", nil)
	}()
	select {
	case <-quiesceDone:
		t.Fatal("quiesce returned before the admitted mutation drained")
	case <-time.After(50 * time.Millisecond):
	}
	if gate.IsQuiesced() {
		t.Fatal("maintenance was reported complete before active-work inspection finished")
	}
	releaseOnce.Do(func() { close(release) })
	if response := <-mutationDone; response == nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("mutation response=%v, want 204", response)
	} else {
		_ = response.Body.Close()
	}
	select {
	case <-quiesceDone:
	case <-time.After(time.Second):
		t.Fatal("quiesce did not finish after the mutation drained")
	}
	if quiesceErr != nil || quiesceResponse == nil || quiesceResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("quiesce response=%v err=%v, want 202", quiesceResponse, quiesceErr)
	}
	if !gate.IsQuiesced() {
		t.Fatal("maintenance was not marked complete after drain and active-work inspection")
	}
	if response, err := http.Get(server.URL + "/api/workbench/mutate"); err != nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("mutation while quiesced response=%v err=%v, want 503", response, err)
	} else {
		_ = response.Body.Close()
	}
}

func TestDesktopMaintenanceRefusesActiveWorkAndReopensAdmission(t *testing.T) {
	gate := newDesktopMaintenanceGate()
	router := http.NewServeMux()
	router.HandleFunc("/api/workbench/mutate", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("/api/desktop/maintenance/quiesce", func(w http.ResponseWriter, r *http.Request) {
		ok, err := gate.Quiesce(r.Context(), func(context.Context) (bool, error) {
			return true, nil
		}, false)
		if err != nil || ok {
			http.Error(w, "expected active work refusal", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusConflict)
	})
	server := httptest.NewServer(gate.Middleware(router))
	defer server.Close()
	if response, err := http.Post(server.URL+"/api/desktop/maintenance/quiesce", "application/json", nil); err != nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("quiesce response=%v err=%v, want 409", response, err)
	} else {
		_ = response.Body.Close()
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		response, err := http.Get(server.URL + "/api/workbench/mutate")
		if err != nil || response.StatusCode != http.StatusNoContent {
			t.Errorf("admission stayed closed after refusal: response=%v err=%v", response, err)
		} else {
			_ = response.Body.Close()
		}
	}()
	wg.Wait()
}
