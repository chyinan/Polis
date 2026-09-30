// pattern: Functional Core
//go:build windows

package runner

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWFPStructuresMatchWindowsSDK64BitLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Windows Filtering Platform layout check is scoped to 64-bit Windows")
	}
	for name, item := range map[string]struct {
		got  uintptr
		want uintptr
	}{
		"context":          {unsafe.Offsetof(fwpmFilter0{}.Context), 152},
		"reserved":         {unsafe.Offsetof(fwpmFilter0{}.Reserved), 168},
		"filter ID":        {unsafe.Offsetof(fwpmFilter0{}.FilterID), 176},
		"effective weight": {unsafe.Offsetof(fwpmFilter0{}.EffectiveWeight), 184},
	} {
		if item.got != item.want {
			t.Errorf("FWPM_FILTER0 %s offset = %d, want %d", name, item.got, item.want)
		}
	}
	for name, item := range map[string]struct {
		got  uintptr
		want uintptr
	}{
		"session":          {unsafe.Sizeof(fwpmSession0{}), 72},
		"filter value":     {unsafe.Sizeof(fwpValue0{}), 16},
		"condition value":  {unsafe.Sizeof(fwpConditionValue0{}), 16},
		"action":           {unsafe.Sizeof(fwpmAction0{}), 20},
		"filter condition": {unsafe.Sizeof(fwpmFilterCondition0{}), 40},
		"v4 mask":          {unsafe.Sizeof(fwpV4AddressAndMask{}), 8},
		"filter":           {unsafe.Sizeof(fwpmFilter0{}), 200},
	} {
		if item.got != item.want {
			t.Errorf("%s ABI size = %d, want %d", name, item.got, item.want)
		}
	}
}

func TestRegistryWFPLeaseAddsExactPlanAndClosesDynamicSession(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI{}
	lease, err := installWindowsRegistryEgressFiltersWithAPI(api, sid, plan)
	if err != nil {
		t.Fatal(err)
	}
	if api.opened != 1 || len(api.filters) != len(plan.Filters) || api.closed != 0 || api.endpointPort != plan.ProxyPort {
		t.Fatalf("WFP lease state = %+v", api)
	}
	for index := range plan.Filters {
		if !reflect.DeepEqual(api.filters[index], plan.Filters[index]) {
			t.Fatalf("WFP filter %d = %+v, want %+v", index, api.filters[index], plan.Filters[index])
		}
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if api.closed != 1 {
		t.Fatalf("dynamic WFP session close count = %d, want 1", api.closed)
	}
}

func TestRegistryWFPLeaseClosesPartialSessionAfterFilterFailure(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI{failAt: 2}
	if _, err = installWindowsRegistryEgressFiltersWithAPI(api, sid, plan); err == nil {
		t.Fatal("partial WFP filter installation unexpectedly succeeded")
	}
	if api.opened != 1 || len(api.filters) != 1 || api.closed != 1 {
		t.Fatalf("partial WFP cleanup state = %+v", api)
	}
}

func TestRegistryWFPLeaseRevokesPermitAndRetainsDefaultBlocks(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI2{}
	lease, err := installWindowsRegistryEgressFiltersWithAPI(api, sid, plan)
	if err != nil {
		t.Fatal(err)
	}
	permitFilterID := api.filterIDs[0]
	if err = lease.RevokePermit(); err != nil {
		t.Fatal(err)
	}
	if err = lease.RevokePermit(); err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != permitFilterID || len(api.filters) != 3 || api.closed != 0 {
		t.Fatalf("WFP revocation removed more than its permit filter: ids=%v filters=%d closed=%d", api.deleted, len(api.filters), api.closed)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if api.closed != 1 {
		t.Fatalf("dynamic WFP session close count = %d, want 1", api.closed)
	}
}

func TestRegistryWFPLeaseKeepsPermitWhenFilterDeletionFails(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI2{failDelete: true}
	lease, err := installWindowsRegistryEgressFiltersWithAPI(api, sid, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.RevokePermit(); err == nil {
		t.Fatal("WFP permit filter deletion failure was ignored")
	}
	api.failDelete = false
	if err = lease.RevokePermit(); err != nil {
		t.Fatalf("retry WFP permit revocation: %v", err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != api.filterIDs[0] {
		t.Fatalf("WFP permit deletion attempts = %v, want %d", api.deleted, api.filterIDs[0])
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
}

type recordingRegistryWFPFilterAPI struct {
	opened       int
	closed       int
	failAt       int
	endpointPort uint16
	filters      []RegistryEgressFilter
	filterIDs    []uint64
	deleted      []uint64
}

func (api *recordingRegistryWFPFilterAPI) openDynamicSession() (windows.Handle, error) {
	api.opened++
	return windows.Handle(17), nil
}

func (api *recordingRegistryWFPFilterAPI) addFilter(engine windows.Handle, _ *windows.SID, port uint16, filter RegistryEgressFilter) (uint64, error) {
	if engine != windows.Handle(17) {
		return 0, errors.New("unexpected WFP engine handle")
	}
	api.endpointPort = port
	if api.failAt > 0 && len(api.filters)+1 == api.failAt {
		return 0, errors.New("injected WFP filter add failure")
	}
	api.filters = append(api.filters, filter)
	filterID := uint64(1700 + len(api.filterIDs))
	api.filterIDs = append(api.filterIDs, filterID)
	return filterID, nil
}

func (api *recordingRegistryWFPFilterAPI) deleteFilter(engine windows.Handle, filterID uint64) error {
	if engine != windows.Handle(17) {
		return errors.New("unexpected WFP engine handle")
	}
	api.deleted = append(api.deleted, filterID)
	return nil
}

func (api *recordingRegistryWFPFilterAPI) closeDynamicSession(engine windows.Handle) error {
	if engine != windows.Handle(17) {
		return errors.New("unexpected WFP engine handle")
	}
	api.closed++
	return nil
}

func TestRegistryWFPFilterLeaseInstallsWholeDynamicPolicyAndClosesOnce(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI2{}
	lease, err := installWindowsRegistryEgressFiltersWithAPI(api, sid, plan)
	if err != nil {
		t.Fatal(err)
	}
	if api.opened != 1 || len(api.filters) != 3 || api.closed != 0 {
		t.Fatalf("WFP installation state = opened:%d filters:%d closed:%d", api.opened, len(api.filters), api.closed)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if api.closed != 1 {
		t.Fatalf("dynamic WFP session close count = %d, want 1", api.closed)
	}
}

func TestRegistryWFPFilterLeaseClosesPartialDynamicPolicyOnError(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	sid := fakeWFPTestSID()
	api := &recordingRegistryWFPFilterAPI2{failAt: 2}
	if _, err = installWindowsRegistryEgressFiltersWithAPI(api, sid, plan); err == nil {
		t.Fatal("partial WFP policy install unexpectedly succeeded")
	}
	if api.opened != 1 || len(api.filters) != 1 || api.closed != 1 {
		t.Fatalf("partial WFP cleanup state = opened:%d filters:%d closed:%d", api.opened, len(api.filters), api.closed)
	}
}

type recordingRegistryWFPFilterAPI2 struct {
	opened     int
	closed     int
	failAt     int
	failDelete bool
	filters    []RegistryEgressFilter
	filterIDs  []uint64
	deleted    []uint64
}

func (api *recordingRegistryWFPFilterAPI2) openDynamicSession() (windows.Handle, error) {
	api.opened++
	return windows.Handle(42), nil
}

func (api *recordingRegistryWFPFilterAPI2) addFilter(engine windows.Handle, _ *windows.SID, _ uint16, spec RegistryEgressFilter) (uint64, error) {
	if engine != windows.Handle(42) {
		return 0, errors.New("unexpected WFP session handle")
	}
	if api.failAt > 0 && len(api.filters)+1 == api.failAt {
		return 0, errors.New("injected filter add failure")
	}
	api.filters = append(api.filters, spec)
	filterID := uint64(4200 + len(api.filterIDs))
	api.filterIDs = append(api.filterIDs, filterID)
	return filterID, nil
}

func (api *recordingRegistryWFPFilterAPI2) deleteFilter(engine windows.Handle, filterID uint64) error {
	if engine != windows.Handle(42) {
		return errors.New("unexpected WFP session handle")
	}
	if api.failDelete {
		return errors.New("injected WFP filter deletion failure")
	}
	api.deleted = append(api.deleted, filterID)
	return nil
}

func (api *recordingRegistryWFPFilterAPI2) closeDynamicSession(engine windows.Handle) error {
	if engine != windows.Handle(42) {
		return errors.New("unexpected WFP session handle")
	}
	api.closed++
	return nil
}
