// pattern: Imperative Shell
//go:build windows

package runner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fwpV6AddressAndMask struct {
	Address      [16]byte
	PrefixLength uint8
}

type serviceListenerWFPFilterAPI interface {
	openDynamicSession() (windows.Handle, error)
	addServiceListenerFilter(windows.Handle, *windows.SID, uint16, ServiceListenerFilter) (uint64, error)
	closeDynamicSession(windows.Handle) error
}

type systemServiceListenerWFPFilterAPI struct{}

type windowsServiceListenerWFPLease struct {
	mu     sync.Mutex
	engine windows.Handle
	api    serviceListenerWFPFilterAPI
}

type windowsServiceListenerFilterRequest struct {
	filter      fwpmFilter0
	conditions  []fwpmFilterCondition0
	displayName string
	name        *uint16
	localV4     fwpV4AddressAndMask
	remoteV4    fwpV4AddressAndMask
	localV6     fwpV6AddressAndMask
	remoteV6    fwpV6AddressAndMask
}

func installWindowsAppContainerServiceListenerFilters(sid *windows.SID, plan AppContainerServiceListenerPlan) (*windowsServiceListenerWFPLease, error) {
	return installWindowsAppContainerServiceListenerFiltersWithAPI(systemServiceListenerWFPFilterAPI{}, sid, plan)
}

func installWindowsAppContainerServiceListenerFiltersWithAPI(api serviceListenerWFPFilterAPI, sid *windows.SID, plan AppContainerServiceListenerPlan) (*windowsServiceListenerWFPLease, error) {
	if sid == nil || api == nil {
		return nil, ErrInvalidAppContainerServiceListener
	}
	expected, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: plan.BindAddress, Port: plan.Port})
	if err != nil || !reflect.DeepEqual(plan, expected) {
		return nil, ErrInvalidAppContainerServiceListener
	}
	engine, err := api.openDynamicSession()
	if err != nil {
		return nil, err
	}
	lease := &windowsServiceListenerWFPLease{engine: engine, api: api}
	for _, filter := range plan.Filters {
		filterID, filterErr := api.addServiceListenerFilter(engine, sid, plan.Port, filter)
		if filterErr == nil && filterID == 0 {
			filterErr = errors.New("WFP returned an empty service-listener filter identifier")
		}
		if filterErr != nil {
			if closeErr := lease.Close(); closeErr != nil {
				return lease, errors.Join(filterErr, fmt.Errorf("failed to close partial service-listener WFP session: %w", closeErr))
			}
			return nil, filterErr
		}
	}
	return lease, nil
}

func (lease *windowsServiceListenerWFPLease) Close() error {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.engine == 0 {
		return nil
	}
	if lease.api == nil {
		return errors.New("service-listener WFP close API is unavailable")
	}
	if err := lease.api.closeDynamicSession(lease.engine); err != nil {
		return err
	}
	lease.engine = 0
	return nil
}

func (systemServiceListenerWFPFilterAPI) openDynamicSession() (windows.Handle, error) {
	return (systemRegistryWFPFilterAPI{}).openDynamicSession()
}

func (systemServiceListenerWFPFilterAPI) closeDynamicSession(engine windows.Handle) error {
	return (systemRegistryWFPFilterAPI{}).closeDynamicSession(engine)
}

func (systemServiceListenerWFPFilterAPI) addServiceListenerFilter(engine windows.Handle, sid *windows.SID, port uint16, spec ServiceListenerFilter) (uint64, error) {
	request, err := buildWindowsServiceListenerFilterRequest(sid, port, spec)
	if err != nil {
		return 0, err
	}
	var filterID uint64
	var pinner runtime.Pinner
	pinner.Pin(&request.filter)
	pinner.Pin(&request.conditions[0])
	pinner.Pin(request.name)
	pinner.Pin(&request.localV4)
	pinner.Pin(&request.remoteV4)
	pinner.Pin(&request.localV6)
	pinner.Pin(&request.remoteV6)
	pinner.Pin(&filterID)
	status, _, _ := procFwpmFilterAdd0.Call(uintptr(engine), uintptr(unsafe.Pointer(&request.filter)), 0, uintptr(unsafe.Pointer(&filterID)))
	pinner.Unpin()
	runtime.KeepAlive(sid)
	runtime.KeepAlive(request)
	runtime.KeepAlive(&filterID)
	if status != 0 {
		return 0, wfpStatusError("add AppContainer service-listener network filter", status)
	}
	return filterID, nil
}

func buildWindowsServiceListenerFilterRequest(sid *windows.SID, port uint16, spec ServiceListenerFilter) (*windowsServiceListenerFilterRequest, error) {
	if sid == nil || port == 0 {
		return nil, ErrInvalidAppContainerServiceListener
	}
	layer, ok := wfpLayerForServiceListenerFilter(spec.Layer)
	if !ok {
		return nil, errors.New("service listener WFP plan has an unsupported layer")
	}
	action, ok := wfpActionForRegistryFilter(RegistryFilterAction(spec.Action))
	if !ok {
		return nil, errors.New("service listener WFP plan has an unsupported action")
	}
	filterKey, err := newWFPFilterGUID()
	if err != nil {
		return nil, errors.New("could not allocate a WFP service-listener filter key")
	}
	displayName := fmt.Sprintf("Polis AppContainer service listener %s %s", spec.Action, spec.Layer)
	name, err := windows.UTF16PtrFromString(displayName)
	if err != nil {
		return nil, errors.New("could not encode the service-listener WFP filter name")
	}
	request := &windowsServiceListenerFilterRequest{
		conditions:  make([]fwpmFilterCondition0, 0, len(spec.Conditions)),
		displayName: displayName,
		name:        name,
	}
	for _, condition := range spec.Conditions {
		if condition.Match != ServiceListenerFilterEqual {
			return nil, errors.New("service listener WFP plan contains an unsupported match type")
		}
		var field windows.GUID
		var value fwpConditionValue0
		switch condition.Field {
		case ServiceListenerFieldAppContainerSID:
			if condition.Value != "" {
				return nil, errors.New("service listener WFP package condition has an unexpected value")
			}
			field, value = wfpConditionALEPackageID, fwpConditionValue0{Type: fwpDataSID, Value: uintptr(unsafe.Pointer(sid))}
		case ServiceListenerFieldLocalIPv4, ServiceListenerFieldRemoteIPv4:
			address, parseErr := netip.ParseAddr(condition.Value)
			if parseErr != nil || address.String() != "127.0.0.1" || address.Is4In6() {
				return nil, ErrInvalidAppContainerServiceListener
			}
			field = wfpConditionLocalIP
			bytes := address.As4()
			if condition.Field == ServiceListenerFieldRemoteIPv4 {
				field = wfpConditionRemoteIP
				request.remoteV4 = fwpV4AddressAndMask{Address: binary.LittleEndian.Uint32(bytes[:]), Mask: binary.LittleEndian.Uint32([]byte{255, 255, 255, 255})}
				value = fwpConditionValue0{Type: fwpDataV4AddrMask, Value: uintptr(unsafe.Pointer(&request.remoteV4))}
			} else {
				request.localV4 = fwpV4AddressAndMask{Address: binary.LittleEndian.Uint32(bytes[:]), Mask: binary.LittleEndian.Uint32([]byte{255, 255, 255, 255})}
				value = fwpConditionValue0{Type: fwpDataV4AddrMask, Value: uintptr(unsafe.Pointer(&request.localV4))}
			}
		case ServiceListenerFieldLocalIPv6, ServiceListenerFieldRemoteIPv6:
			address, parseErr := netip.ParseAddr(condition.Value)
			if parseErr != nil || address.String() != "::1" || address.Zone() != "" || address.Is4In6() {
				return nil, ErrInvalidAppContainerServiceListener
			}
			field = wfpConditionLocalIP
			if condition.Field == ServiceListenerFieldRemoteIPv6 {
				field = wfpConditionRemoteIP
				request.remoteV6 = fwpV6AddressAndMask{Address: address.As16(), PrefixLength: 128}
				value = fwpConditionValue0{Type: fwpDataV6AddrMask, Value: uintptr(unsafe.Pointer(&request.remoteV6))}
			} else {
				request.localV6 = fwpV6AddressAndMask{Address: address.As16(), PrefixLength: 128}
				value = fwpConditionValue0{Type: fwpDataV6AddrMask, Value: uintptr(unsafe.Pointer(&request.localV6))}
			}
		case ServiceListenerFieldProtocol:
			if condition.Value != "tcp" {
				return nil, errors.New("service listener WFP protocol must be TCP")
			}
			field, value = wfpConditionProtocol, fwpConditionValue0{Type: fwpDataUint8, Value: 6}
		case ServiceListenerFieldLocalPort:
			if condition.Value != strconv.Itoa(int(port)) {
				return nil, ErrInvalidAppContainerServiceListener
			}
			field, value = wfpConditionLocalPort, fwpConditionValue0{Type: fwpDataUint16, Value: uintptr(port)}
		default:
			return nil, errors.New("service listener WFP plan contains an unsupported condition")
		}
		request.conditions = append(request.conditions, fwpmFilterCondition0{FieldKey: field, MatchType: fwpMatchEqual, ConditionValue: value})
	}
	if len(request.conditions) == 0 {
		return nil, errors.New("service listener WFP filter has no AppContainer SID condition")
	}
	request.filter = fwpmFilter0{
		FilterKey: filterKey, DisplayData: fwpmDisplayData0{Name: request.name}, LayerKey: layer, SubLayerKey: wfpSubLayerUniversal,
		Weight:              fwpValue0{Type: fwpDataUint8, Value: uintptr(spec.Weight)},
		NumFilterConditions: uint32(len(request.conditions)), FilterCondition: &request.conditions[0],
		Action: fwpmAction0{Type: action},
	}
	return request, nil
}

func wfpLayerForServiceListenerFilter(layer ServiceListenerFilterLayer) (windows.GUID, bool) {
	switch layer {
	case ServiceListenerLayerIPv4ResourceAssignment:
		return wfpLayerALEResourceAssignmentV4, true
	case ServiceListenerLayerIPv6ResourceAssignment:
		return wfpLayerALEResourceAssignmentV6, true
	case ServiceListenerLayerIPv4Listen:
		return wfpLayerALEAuthListenV4, true
	case ServiceListenerLayerIPv6Listen:
		return wfpLayerALEAuthListenV6, true
	case ServiceListenerLayerIPv4Receive:
		return wfpLayerALEAuthReceiveV4, true
	case ServiceListenerLayerIPv6Receive:
		return wfpLayerALEAuthReceiveV6, true
	case ServiceListenerLayerIPv4Connect:
		return wfpLayerALEAuthConnectV4, true
	case ServiceListenerLayerIPv6Connect:
		return wfpLayerALEAuthConnectV6, true
	default:
		return windows.GUID{}, false
	}
}
