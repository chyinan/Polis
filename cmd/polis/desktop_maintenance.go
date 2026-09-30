package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

type desktopMaintenanceGate struct {
	transition sync.Mutex
	admission  sync.Mutex
	inFlight   sync.RWMutex
	quiescing  bool
	quiesced   bool
}

func newDesktopMaintenanceGate() *desktopMaintenanceGate {
	return &desktopMaintenanceGate{}
}

func (gate *desktopMaintenanceGate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if isDesktopMaintenanceControlRequest(request) {
			next.ServeHTTP(response, request)
			return
		}

		gate.admission.Lock()
		if gate.quiescing {
			gate.admission.Unlock()
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("Retry-After", "1")
			response.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(response).Encode(map[string]string{"error": "desktop maintenance is in progress"})
			return
		}
		gate.inFlight.RLock()
		gate.admission.Unlock()
		defer gate.inFlight.RUnlock()
		next.ServeHTTP(response, request)
	})
}

func (gate *desktopMaintenanceGate) Quiesce(
	ctx context.Context,
	hasActiveWork func(context.Context) (bool, error),
	allowActiveWork bool,
) (bool, error) {
	gate.transition.Lock()
	defer gate.transition.Unlock()

	gate.admission.Lock()
	if gate.quiescing {
		gate.admission.Unlock()
		return true, nil
	}
	gate.quiescing = true
	gate.quiesced = false
	gate.admission.Unlock()

	gate.inFlight.Lock()
	defer gate.inFlight.Unlock()

	active, err := hasActiveWork(ctx)
	if err != nil || (active && !allowActiveWork) {
		gate.admission.Lock()
		gate.quiescing = false
		gate.quiesced = false
		gate.admission.Unlock()
		return false, err
	}
	gate.admission.Lock()
	gate.quiesced = true
	gate.admission.Unlock()
	return true, nil
}

func (gate *desktopMaintenanceGate) IsQuiesced() bool {
	gate.admission.Lock()
	defer gate.admission.Unlock()
	return gate.quiesced
}

func isDesktopMaintenanceControlRequest(request *http.Request) bool {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/healthz":
		return true
	case request.Method == http.MethodGet && request.URL.Path == "/api/desktop/active-work":
		return true
	case request.Method == http.MethodPost && request.URL.Path == "/api/desktop/maintenance/quiesce":
		return true
	case request.Method == http.MethodPost && request.URL.Path == "/api/desktop/shutdown":
		return true
	default:
		return false
	}
}
