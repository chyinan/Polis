// pattern: Imperative Shell
//go:build windows

package runner

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	maxLoopbackAppContainerSIDEntries = 4096
	loopbackConfigMutexName           = `Global\Polis-AppContainerLoopbackConfig`
	loopbackConfigMutexTimeout        = 30 * time.Second
	waitObject0                       = 0
	waitAbandoned                     = 0x80
	waitTimeout                       = 0x102
)

var (
	loopbackConfigMu        sync.Mutex
	firewallAPIDLL          = windows.NewLazySystemDLL("FirewallAPI.dll")
	procNetworkIsolationGet = firewallAPIDLL.NewProc("NetworkIsolationGetAppContainerConfig")
	procNetworkIsolationSet = firewallAPIDLL.NewProc("NetworkIsolationSetAppContainerConfig")
	kernel32DLL             = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessHeap      = kernel32DLL.NewProc("GetProcessHeap")
	procHeapFree            = kernel32DLL.NewProc("HeapFree")
	procCreateMutexW        = kernel32DLL.NewProc("CreateMutexW")
	procWaitForSingleObject = kernel32DLL.NewProc("WaitForSingleObject")
	procReleaseMutex        = kernel32DLL.NewProc("ReleaseMutex")
)

func setAppContainerLoopbackSID(target *windows.SID, enabled bool) (bool, error) {
	if target == nil {
		return false, errors.New("AppContainer loopback SID is missing")
	}
	loopbackConfigMu.Lock()
	defer loopbackConfigMu.Unlock()
	committed := false
	err := withNamedWindowsMutex(loopbackConfigMutexName, loopbackConfigMutexTimeout, func() error {
		var updateErr error
		committed, updateErr = setAppContainerLoopbackSIDLocked(target, enabled)
		return updateErr
	})
	return committed, err
}

func setAppContainerLoopbackSIDLocked(target *windows.SID, enabled bool) (committed bool, returnErr error) {
	for _, procedure := range []*windows.LazyProc{procNetworkIsolationGet, procNetworkIsolationSet, procGetProcessHeap, procHeapFree} {
		if err := procedure.Find(); err != nil {
			return false, errors.New("Windows AppContainer loopback configuration API is unavailable")
		}
	}
	var count uint32
	var entries *windows.SIDAndAttributes
	var outputPinner runtime.Pinner
	outputPinner.Pin(&count)
	outputPinner.Pin(&entries)
	status, _, _ := procNetworkIsolationGet.Call(uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&entries)))
	outputPinner.Unpin()
	runtime.KeepAlive(&count)
	runtime.KeepAlive(&entries)
	if status != 0 {
		if entries != nil && count <= maxLoopbackAppContainerSIDEntries {
			_ = freeLoopbackSIDEntries(count, entries)
		}
		return false, loopbackConfigStatusError("read current AppContainer loopback list", status)
	}
	defer func() {
		if err := freeLoopbackSIDEntries(count, entries); returnErr == nil && err != nil {
			returnErr = err
		}
	}()
	if count > maxLoopbackAppContainerSIDEntries {
		return false, errors.New("current AppContainer loopback list exceeds its accepted bound")
	}

	current := make([]AppContainerLoopbackSID, count)
	currentWindows := make([]windows.SIDAndAttributes, count)
	if count > 0 {
		for index, entry := range unsafe.Slice(entries, int(count)) {
			if entry.Sid == nil {
				return false, errors.New("current AppContainer loopback list contains an invalid SID")
			}
			sidText := entry.Sid.String()
			if sidText == "" {
				return false, errors.New("current AppContainer loopback list contains an unreadable SID")
			}
			current[index] = AppContainerLoopbackSID{SID: sidText, Attributes: entry.Attributes}
			currentWindows[index] = entry
		}
	}
	targetString := target.String()
	if targetString == "" {
		return false, errors.New("AppContainer loopback SID could not be rendered")
	}
	merged := MergeLoopbackAppContainerSIDEntries(current, targetString, enabled)
	if EqualLoopbackAppContainerSIDEntries(current, merged) {
		return false, nil
	}
	bySID := make(map[string][]windows.SIDAndAttributes, len(currentWindows))
	for index, entry := range current {
		key := strings.ToUpper(entry.SID)
		bySID[key] = append(bySID[key], currentWindows[index])
	}
	mergedWindows := make([]windows.SIDAndAttributes, 0, len(merged))
	for _, entry := range merged {
		key := strings.ToUpper(entry.SID)
		if owned := bySID[key]; len(owned) > 0 {
			mergedWindows = append(mergedWindows, owned[0])
			bySID[key] = owned[1:]
			continue
		}
		if strings.EqualFold(entry.SID, targetString) && enabled {
			mergedWindows = append(mergedWindows, windows.SIDAndAttributes{Sid: target, Attributes: entry.Attributes})
			continue
		}
		return false, errors.New("merged AppContainer loopback list lost an existing SID")
	}
	var mergedPointer uintptr
	var pinner runtime.Pinner
	if len(mergedWindows) > 0 {
		pinner.Pin(&mergedWindows[0])
		mergedPointer = uintptr(unsafe.Pointer(&mergedWindows[0]))
	}
	status, _, _ = procNetworkIsolationSet.Call(uintptr(len(mergedWindows)), mergedPointer)
	pinner.Unpin()
	if status != 0 {
		return false, loopbackConfigStatusError("update AppContainer loopback list", status)
	}
	committed = true
	return true, nil
}

func withNamedWindowsMutex(name string, timeout time.Duration, operation func() error) (returnErr error) {
	if name == "" || timeout <= 0 || operation == nil || timeout.Milliseconds() > int64(^uint32(0)-1) {
		return errors.New("invalid Windows mutex lease")
	}
	namePointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return errors.New("Windows mutex name is invalid")
	}
	for _, procedure := range []*windows.LazyProc{procCreateMutexW, procWaitForSingleObject, procReleaseMutex} {
		if err := procedure.Find(); err != nil {
			return errors.New("Windows named mutex API is unavailable")
		}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	handle, _, _ := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePointer)))
	if handle == 0 {
		return errors.New("Windows could not create the loopback configuration mutex")
	}
	locked := false
	defer func() {
		if locked {
			if result, _, _ := procReleaseMutex.Call(handle); result == 0 && returnErr == nil {
				returnErr = errors.New("Windows could not release the loopback configuration mutex")
			}
		}
		if err := windows.CloseHandle(windows.Handle(handle)); err != nil && returnErr == nil {
			returnErr = errors.New("Windows could not close the loopback configuration mutex")
		}
	}()
	waitMilliseconds := uintptr(timeout.Milliseconds())
	result, _, _ := procWaitForSingleObject.Call(handle, waitMilliseconds)
	if result != waitObject0 && result != waitAbandoned {
		if result == waitTimeout {
			return errors.New("timed out waiting for the loopback configuration mutex")
		}
		return errors.New("Windows could not acquire the loopback configuration mutex")
	}
	locked = true
	return operation()
}

func freeLoopbackSIDEntries(count uint32, entries *windows.SIDAndAttributes) error {
	if entries == nil {
		return nil
	}
	heap, _, _ := procGetProcessHeap.Call()
	if heap == 0 {
		return errors.New("process heap is unavailable while freeing loopback SIDs")
	}
	var firstError error
	for _, entry := range unsafe.Slice(entries, int(count)) {
		if entry.Sid == nil {
			continue
		}
		if result, _, _ := procHeapFree.Call(heap, 0, uintptr(unsafe.Pointer(entry.Sid))); result == 0 && firstError == nil {
			firstError = errors.New("failed to free a returned loopback SID")
		}
	}
	if result, _, _ := procHeapFree.Call(heap, 0, uintptr(unsafe.Pointer(entries))); result == 0 && firstError == nil {
		firstError = errors.New("failed to free the returned loopback SID list")
	}
	return firstError
}

func loopbackConfigStatusError(operation string, status uintptr) error {
	return fmt.Errorf("failed to %s: Windows network isolation status 0x%08x", operation, uint32(status))
}
