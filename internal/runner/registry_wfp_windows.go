// pattern: Imperative Shell
//go:build windows

package runner

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fwpmSessionFlagDynamic = 0x00000001
	rpcAuthnWinNT          = 10

	fwpActionBlock  = 0x00001001
	fwpActionPermit = 0x00001002
	fwpMatchEqual   = 0

	fwpDataUint8      = 1
	fwpDataUint16     = 2
	fwpDataSID        = 13
	fwpDataV4AddrMask = 0x100
	fwpDataV6AddrMask = 0x101
)

var (
	wfpLayerALEResourceAssignmentV4 = windows.GUID{Data1: 0x1247d66d, Data2: 0x0b60, Data3: 0x4a15, Data4: [8]byte{0x8d, 0x44, 0x71, 0x55, 0xd0, 0xf5, 0x3a, 0x0c}}
	wfpLayerALEResourceAssignmentV6 = windows.GUID{Data1: 0x55a650e1, Data2: 0x5f0a, Data3: 0x4eca, Data4: [8]byte{0xa6, 0x53, 0x88, 0xf5, 0x3b, 0x26, 0xaa, 0x8c}}
	wfpLayerALEAuthListenV4         = windows.GUID{Data1: 0x88bb5dad, Data2: 0x76d7, Data3: 0x4227, Data4: [8]byte{0x9c, 0x71, 0xdf, 0x0a, 0x3e, 0xd7, 0xbe, 0x7e}}
	wfpLayerALEAuthListenV6         = windows.GUID{Data1: 0x7ac9de24, Data2: 0x17dd, Data3: 0x4814, Data4: [8]byte{0xb4, 0xbd, 0xa9, 0xfb, 0xc9, 0x5a, 0x32, 0x1b}}
	wfpLayerALEAuthReceiveV4        = windows.GUID{Data1: 0xe1cd9fe7, Data2: 0xf4b5, Data3: 0x4273, Data4: [8]byte{0x96, 0xc0, 0x59, 0x2e, 0x48, 0x7b, 0x86, 0x50}}
	wfpLayerALEAuthReceiveV6        = windows.GUID{Data1: 0xa3b42c97, Data2: 0x9f04, Data3: 0x4672, Data4: [8]byte{0xb8, 0x7e, 0xce, 0xe9, 0xc4, 0x83, 0x25, 0x7f}}
	wfpLayerALEAuthConnectV4        = windows.GUID{Data1: 0xc38d57d1, Data2: 0x05a7, Data3: 0x4c33, Data4: [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82}}
	wfpLayerALEAuthConnectV6        = windows.GUID{Data1: 0x4a72393b, Data2: 0x319f, Data3: 0x44bc, Data4: [8]byte{0x84, 0xc3, 0xba, 0x54, 0xdc, 0xb3, 0xb6, 0xb4}}
	wfpSubLayerUniversal            = windows.GUID{Data1: 0xeebecc03, Data2: 0xced4, Data3: 0x4380, Data4: [8]byte{0x81, 0x9a, 0x27, 0x34, 0x39, 0x7b, 0x2b, 0x74}}
	wfpConditionALEPackageID        = windows.GUID{Data1: 0x71bc78fa, Data2: 0xf17c, Data3: 0x4997, Data4: [8]byte{0xa6, 0x02, 0x6a, 0xbb, 0x26, 0x1f, 0x35, 0x1c}}
	wfpConditionLocalIP             = windows.GUID{Data1: 0xd9ee00de, Data2: 0xc1ef, Data3: 0x4617, Data4: [8]byte{0xbf, 0xe3, 0xff, 0xd8, 0xf5, 0xa0, 0x89, 0x57}}
	wfpConditionRemoteIP            = windows.GUID{Data1: 0xb235ae9a, Data2: 0x1d64, Data3: 0x49b8, Data4: [8]byte{0xa4, 0x4c, 0x5f, 0xf3, 0xd9, 0x09, 0x50, 0x45}}
	wfpConditionProtocol            = windows.GUID{Data1: 0x3971ef2b, Data2: 0x623e, Data3: 0x4f9a, Data4: [8]byte{0x8c, 0xb1, 0x6e, 0x79, 0xb8, 0x06, 0xb9, 0xa7}}
	wfpConditionLocalPort           = windows.GUID{Data1: 0x0c1ba1af, Data2: 0x5765, Data3: 0x453f, Data4: [8]byte{0xaf, 0x22, 0xa8, 0xf7, 0x91, 0xac, 0x77, 0x5b}}
	wfpConditionRemotePort          = windows.GUID{Data1: 0xc35a604d, Data2: 0xd22b, Data3: 0x4e1a, Data4: [8]byte{0x91, 0xb4, 0x68, 0xf6, 0x74, 0xee, 0x67, 0x4b}}
)

type fwpmDisplayData0 struct {
	Name        *uint16
	Description *uint16
}

type fwpmSession0 struct {
	SessionKey         windows.GUID
	DisplayData        fwpmDisplayData0
	Flags              uint32
	TxnWaitTimeoutInMS uint32
	ProcessID          uint32
	SID                *windows.SID
	Username           *uint16
	KernelMode         int32
}

type fwpValue0 struct {
	Type  uint32
	_     uint32
	Value uintptr
}

type fwpConditionValue0 struct {
	Type  uint32
	_     uint32
	Value uintptr
}

type fwpmByteBlob struct {
	Size uint32
	_    uint32
	Data *byte
}

type fwpmAction0 struct {
	Type uint32
	Data windows.GUID
}

type fwpmFilterCondition0 struct {
	FieldKey       windows.GUID
	MatchType      uint32
	ConditionValue fwpConditionValue0
}

type fwpV4AddressAndMask struct {
	Address uint32
	Mask    uint32
}

type fwpmFilterContext struct {
	Raw uint64
	Pad uint64
}

type fwpmFilter0 struct {
	FilterKey           windows.GUID
	DisplayData         fwpmDisplayData0
	Flags               uint32
	ProviderKey         *windows.GUID
	ProviderData        fwpmByteBlob
	LayerKey            windows.GUID
	SubLayerKey         windows.GUID
	Weight              fwpValue0
	NumFilterConditions uint32
	FilterCondition     *fwpmFilterCondition0
	Action              fwpmAction0
	Context             fwpmFilterContext
	Reserved            *windows.GUID
	FilterID            uint64
	EffectiveWeight     fwpValue0
}

type windowsRegistryEgressFilterLease struct {
	mu             sync.Mutex
	engine         windows.Handle
	api            registryWFPFilterAPI
	permitFilterID uint64
}

func installWindowsRegistryEgressFilters(appContainerSID *windows.SID, plan RegistryEgressPlan) (*windowsRegistryEgressFilterLease, error) {
	return installWindowsRegistryEgressFiltersWithAPI(systemRegistryWFPFilterAPI{}, appContainerSID, plan)
}

type registryWFPFilterAPI interface {
	openDynamicSession() (windows.Handle, error)
	addFilter(windows.Handle, *windows.SID, uint16, RegistryEgressFilter) (uint64, error)
	deleteFilter(windows.Handle, uint64) error
	closeDynamicSession(windows.Handle) error
}

type systemRegistryWFPFilterAPI struct{}

func installWindowsRegistryEgressFiltersWithAPI(api registryWFPFilterAPI, appContainerSID *windows.SID, plan RegistryEgressPlan) (*windowsRegistryEgressFilterLease, error) {
	if appContainerSID == nil || plan.ProxyPort == 0 || plan.ProxyAddress != "127.0.0.1" || len(plan.Filters) != 3 {
		return nil, errInvalidRegistryEgressEndpoint
	}
	if api == nil {
		return nil, errors.New("WFP filter API is unavailable")
	}
	engine, err := api.openDynamicSession()
	if err != nil {
		return nil, err
	}
	lease := &windowsRegistryEgressFilterLease{engine: engine, api: api}
	for index, filter := range plan.Filters {
		var filterID uint64
		filterID, err = api.addFilter(engine, appContainerSID, plan.ProxyPort, filter)
		if err == nil && filterID == 0 {
			err = errors.New("WFP returned an empty filter identifier")
		}
		if err != nil {
			closeErr := lease.Close()
			if closeErr != nil {
				return nil, fmt.Errorf("%w; failed to close partial WFP session: %v", err, closeErr)
			}
			return nil, err
		}
		if index == 0 {
			if filter.Action != RegistryFilterPermit {
				_ = lease.Close()
				return nil, errors.New("registry WFP plan must install its scoped permit first")
			}
			lease.permitFilterID = filterID
		}
	}
	return lease, nil
}

func (systemRegistryWFPFilterAPI) openDynamicSession() (windows.Handle, error) {
	if err := loadWFPProcedures(); err != nil {
		return 0, err
	}
	session := fwpmSession0{Flags: fwpmSessionFlagDynamic}
	var engine windows.Handle
	var pinner runtime.Pinner
	pinner.Pin(&session)
	pinner.Pin(&engine)
	status, _, _ := procFwpmEngineOpen0.Call(0, rpcAuthnWinNT, 0, uintptr(unsafe.Pointer(&session)), uintptr(unsafe.Pointer(&engine)))
	pinner.Unpin()
	runtime.KeepAlive(&session)
	runtime.KeepAlive(&engine)
	if status != 0 {
		return 0, wfpStatusError("open dynamic WFP session", status)
	}
	return engine, nil
}

func (systemRegistryWFPFilterAPI) addFilter(engine windows.Handle, sid *windows.SID, port uint16, filter RegistryEgressFilter) (uint64, error) {
	return addWindowsRegistryEgressFilter(engine, sid, port, filter)
}

func (systemRegistryWFPFilterAPI) deleteFilter(engine windows.Handle, filterID uint64) error {
	if err := loadWFPProcedures(); err != nil {
		return err
	}
	status, _, _ := procFwpmFilterDeleteByID0.Call(uintptr(engine), uintptr(filterID))
	if status != 0 {
		return wfpStatusError("delete AppContainer registry permit filter", status)
	}
	return nil
}

func (systemRegistryWFPFilterAPI) closeDynamicSession(engine windows.Handle) error {
	if err := loadWFPProcedures(); err != nil {
		return err
	}
	status, _, _ := procFwpmEngineClose0.Call(uintptr(engine))
	if status != 0 {
		return wfpStatusError("close dynamic WFP session", status)
	}
	return nil
}

func (lease *windowsRegistryEgressFilterLease) Close() error {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.engine == 0 {
		return nil
	}
	if lease.api == nil {
		return errors.New("WFP session close API is unavailable")
	}
	if err := lease.api.closeDynamicSession(lease.engine); err != nil {
		return err
	}
	lease.engine = 0
	return nil
}

func (lease *windowsRegistryEgressFilterLease) RevokePermit() error {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.engine == 0 || lease.permitFilterID == 0 {
		return nil
	}
	if lease.api == nil {
		return errors.New("WFP permit filter deletion API is unavailable")
	}
	if err := lease.api.deleteFilter(lease.engine, lease.permitFilterID); err != nil {
		return err
	}
	lease.permitFilterID = 0
	return nil
}

func addWindowsRegistryEgressFilter(engine windows.Handle, appContainerSID *windows.SID, proxyPort uint16, spec RegistryEgressFilter) (uint64, error) {
	layer, ok := wfpLayerForRegistryFilter(spec.Layer)
	if !ok {
		return 0, errors.New("registry WFP filter plan contains an unsupported layer")
	}
	action, ok := wfpActionForRegistryFilter(spec.Action)
	if !ok {
		return 0, errors.New("registry WFP filter plan contains an unsupported action")
	}
	filterKey, err := newWFPFilterGUID()
	if err != nil {
		return 0, errors.New("could not allocate a WFP filter key")
	}
	displayName := fmt.Sprintf("Polis AppContainer registry egress %s %s", spec.Action, spec.Layer)
	name, err := windows.UTF16PtrFromString(displayName)
	if err != nil {
		return 0, errors.New("could not encode the registry WFP filter name")
	}
	conditions := make([]fwpmFilterCondition0, 0, len(spec.Conditions))
	var proxyAddress fwpV4AddressAndMask
	for _, condition := range spec.Conditions {
		var field windows.GUID
		var value fwpConditionValue0
		switch condition.Field {
		case RegistryFilterAppContainerSID:
			field, value = wfpConditionALEPackageID, fwpConditionValue0{Type: fwpDataSID, Value: uintptr(unsafe.Pointer(appContainerSID))}
		case RegistryFilterRemoteIPv4:
			if condition.Value != "127.0.0.1" {
				return 0, errInvalidRegistryEgressEndpoint
			}
			field = wfpConditionRemoteIP
			proxyAddress = fwpV4AddressAndMask{Address: binary.LittleEndian.Uint32([]byte{127, 0, 0, 1}), Mask: binary.LittleEndian.Uint32([]byte{255, 255, 255, 255})}
			value = fwpConditionValue0{Type: fwpDataV4AddrMask, Value: uintptr(unsafe.Pointer(&proxyAddress))}
		case RegistryFilterProtocol:
			if condition.Value != "tcp" {
				return 0, errors.New("registry WFP filter protocol is not TCP")
			}
			field, value = wfpConditionProtocol, fwpConditionValue0{Type: fwpDataUint8, Value: 6}
		case RegistryFilterRemotePort:
			if condition.Value != fmt.Sprintf("%d", proxyPort) {
				return 0, errInvalidRegistryEgressEndpoint
			}
			field, value = wfpConditionRemotePort, fwpConditionValue0{Type: fwpDataUint16, Value: uintptr(proxyPort)}
		default:
			return 0, errors.New("registry WFP filter plan contains an unsupported condition")
		}
		conditions = append(conditions, fwpmFilterCondition0{FieldKey: field, MatchType: fwpMatchEqual, ConditionValue: value})
	}
	if len(conditions) == 0 {
		return 0, errors.New("registry WFP filter has no package SID condition")
	}
	filter := fwpmFilter0{
		FilterKey: filterKey, DisplayData: fwpmDisplayData0{Name: name}, LayerKey: layer, SubLayerKey: wfpSubLayerUniversal,
		Weight:              fwpValue0{Type: fwpDataUint8, Value: uintptr(spec.Weight)},
		NumFilterConditions: uint32(len(conditions)), FilterCondition: &conditions[0],
		Action: fwpmAction0{Type: action},
	}
	var filterID uint64
	var pinner runtime.Pinner
	pinner.Pin(&filter)
	pinner.Pin(&conditions[0])
	pinner.Pin(name)
	pinner.Pin(&proxyAddress)
	pinner.Pin(&filterID)
	status, _, _ := procFwpmFilterAdd0.Call(uintptr(engine), uintptr(unsafe.Pointer(&filter)), 0, uintptr(unsafe.Pointer(&filterID)))
	pinner.Unpin()
	runtime.KeepAlive(appContainerSID)
	runtime.KeepAlive(name)
	runtime.KeepAlive(&filterID)
	if status != 0 {
		return 0, wfpStatusError("add AppContainer network filter", status)
	}
	return filterID, nil
}

func wfpLayerForRegistryFilter(layer RegistryFilterLayer) (windows.GUID, bool) {
	switch layer {
	case RegistryFilterLayerIPv4Connect:
		return wfpLayerALEAuthConnectV4, true
	case RegistryFilterLayerIPv6Connect:
		return wfpLayerALEAuthConnectV6, true
	default:
		return windows.GUID{}, false
	}
}

func wfpActionForRegistryFilter(action RegistryFilterAction) (uint32, bool) {
	switch action {
	case RegistryFilterPermit:
		return fwpActionPermit, true
	case RegistryFilterBlock:
		return fwpActionBlock, true
	default:
		return 0, false
	}
}

func newWFPFilterGUID() (windows.GUID, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return windows.GUID{}, err
	}
	return windows.GUID{
		Data1: binary.LittleEndian.Uint32(bytes[0:4]),
		Data2: binary.LittleEndian.Uint16(bytes[4:6]),
		Data3: binary.LittleEndian.Uint16(bytes[6:8]),
		Data4: [8]byte(bytes[8:16]),
	}, nil
}

func wfpStatusError(operation string, status uintptr) error {
	return fmt.Errorf("failed to %s: Windows Filtering Platform status 0x%08x", operation, uint32(status))
}

var (
	fwpuclnt                  = windows.NewLazySystemDLL("fwpuclnt.dll")
	procFwpmEngineOpen0       = fwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0      = fwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmFilterAdd0        = fwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmFilterDeleteByID0 = fwpuclnt.NewProc("FwpmFilterDeleteById0")
)

func loadWFPProcedures() error {
	for _, procedure := range []*windows.LazyProc{procFwpmEngineOpen0, procFwpmEngineClose0, procFwpmFilterAdd0, procFwpmFilterDeleteByID0} {
		if err := procedure.Find(); err != nil {
			return errors.New("Windows Filtering Platform API is unavailable")
		}
	}
	return nil
}
