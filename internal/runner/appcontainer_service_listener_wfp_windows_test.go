// pattern: Functional Core
//go:build windows

package runner

import (
	"errors"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
)

func TestBuildWindowsServiceListenerFilterSetsRequiredDisplayName(t *testing.T) {
	sid := fakeWFPTestSID()
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 43123})
	if err != nil {
		t.Fatal(err)
	}
	request, err := buildWindowsServiceListenerFilterRequest(sid, plan.Port, plan.Filters[0])
	if err != nil {
		t.Fatal(err)
	}
	if request.displayName != "Polis AppContainer service listener permit ALE_RESOURCE_ASSIGNMENT_V4" || request.filter.DisplayData.Name == nil {
		t.Fatalf("WFP filter display name=%q nativeName=%v", request.displayName, request.filter.DisplayData.Name != nil)
	}
	if request.filter.NumFilterConditions != uint32(len(request.conditions)) || request.filter.FilterCondition == nil || request.filter.Weight.Type != fwpDataUint8 || request.filter.Action.Type != fwpActionPermit {
		t.Fatalf("native WFP filter does not reference the generated conditions: %+v", request.filter)
	}
}

func TestInstallWindowsAppContainerServiceListenerFiltersUsesDynamicLease(t *testing.T) {
	sid := fakeWFPTestSID()
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 43123})
	if err != nil {
		t.Fatal(err)
	}
	api := &recordingServiceListenerWFPAPI{}
	lease, err := installWindowsAppContainerServiceListenerFiltersWithAPI(api, sid, plan)
	if err != nil {
		t.Fatal(err)
	}
	if api.openCalls != 1 || !reflect.DeepEqual(api.filters, plan.Filters) || api.closeCalls != 0 {
		t.Fatalf("WFP setup open=%d filters=%d close=%d", api.openCalls, len(api.filters), api.closeCalls)
	}
	for _, filterPort := range api.ports {
		if filterPort != plan.Port {
			t.Fatalf("WFP filter port = %d, want pinned port %d", filterPort, plan.Port)
		}
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil || api.closeCalls != 1 {
		t.Fatalf("WFP close replay error=%v closeCalls=%d", err, api.closeCalls)
	}
}

func TestInstallWindowsAppContainerServiceListenerFiltersClosesPartialSession(t *testing.T) {
	sid := fakeWFPTestSID()
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "::1", Port: 52119})
	if err != nil {
		t.Fatal(err)
	}
	api := &recordingServiceListenerWFPAPI{failAt: 4}
	if _, err = installWindowsAppContainerServiceListenerFiltersWithAPI(api, sid, plan); err == nil {
		t.Fatal("WFP filter failure was ignored")
	}
	if api.openCalls != 1 || len(api.filters) != 4 || api.closeCalls != 1 {
		t.Fatalf("partial WFP cleanup open=%d filters=%d close=%d", api.openCalls, len(api.filters), api.closeCalls)
	}
}

func TestPartialServiceListenerWFPFailureRetainsCleanupLease(t *testing.T) {
	sid := fakeWFPTestSID()
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 43123})
	if err != nil {
		t.Fatal(err)
	}
	closeFailure := errors.New("injected WFP dynamic-session close failure")
	api := &recordingServiceListenerWFPAPI{failAt: 3, closeErr: closeFailure}
	lease, installErr := installWindowsAppContainerServiceListenerFiltersWithAPI(api, sid, plan)
	if lease == nil || !errors.Is(installErr, closeFailure) {
		t.Fatalf("partial install lease=%v error=%v", lease != nil, installErr)
	}
	api.closeErr = nil
	if err = lease.Close(); err != nil || api.closeCalls != 2 {
		t.Fatalf("partial WFP lease retry error=%v closeCalls=%d", err, api.closeCalls)
	}
}

func fakeWFPTestSID() *windows.SID {
	// Injected WFP APIs only validate the presence of a SID pointer; keeping this
	// fixture zero-valued ensures these tests never invoke host policy or account APIs.
	return &windows.SID{}
}

type recordingServiceListenerWFPAPI struct {
	openCalls  int
	closeCalls int
	failAt     int
	filters    []ServiceListenerFilter
	ports      []uint16
	closeErr   error
}

func (api *recordingServiceListenerWFPAPI) openDynamicSession() (windows.Handle, error) {
	api.openCalls++
	return windows.Handle(19), nil
}

func (api *recordingServiceListenerWFPAPI) addServiceListenerFilter(_ windows.Handle, _ *windows.SID, port uint16, filter ServiceListenerFilter) (uint64, error) {
	api.filters = append(api.filters, filter)
	api.ports = append(api.ports, port)
	if api.failAt > 0 && len(api.filters) == api.failAt {
		return 0, errors.New("injected WFP service-listener filter failure")
	}
	return uint64(len(api.filters)), nil
}

func (api *recordingServiceListenerWFPAPI) closeDynamicSession(_ windows.Handle) error {
	api.closeCalls++
	return api.closeErr
}
