//go:build linux

// pattern: Imperative Shell

package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const linuxProcessGroupRecoveryMaxEntries = 16384

func ReconcileLinuxWorkerProcessGroup(ctx context.Context, identity string, processGroupID int) (LinuxProcessGroupStopProof, error) {
	return reconcileLinuxProcessGroupAt(ctx, "/proc", identity, processGroupID)
}

func reconcileLinuxProcessGroupAt(ctx context.Context, procRoot, identity string, processGroupID int) (LinuxProcessGroupStopProof, error) {
	if ctx == nil || ctx.Err() != nil || identity == "" || len(identity) > 128 || processGroupID <= 0 || !filepath.IsAbs(procRoot) {
		return LinuxProcessGroupStopProof{}, errors.New("invalid Linux process-group reconciliation request")
	}
	members, err := linuxProcessGroupMemberCount(ctx, procRoot, processGroupID)
	if err != nil {
		return LinuxProcessGroupStopProof{}, err
	}
	if members != 0 {
		return LinuxProcessGroupStopProof{}, fmt.Errorf("%w: process group %d still contains %d process(es)", ErrLinuxProcessGroupStopUnconfirmed, processGroupID, members)
	}
	if err := syscall.Kill(-processGroupID, 0); err == nil {
		return LinuxProcessGroupStopProof{}, fmt.Errorf("%w: process group %d became active during procfs inspection", ErrLinuxProcessGroupStopUnconfirmed, processGroupID)
	} else if !errors.Is(err, syscall.ESRCH) {
		return LinuxProcessGroupStopProof{}, fmt.Errorf("failed to confirm Linux process group absence: %w", err)
	}
	return LinuxProcessGroupStopProof{identity: identity, processGroupID: processGroupID, stopped: true}, nil
}

func linuxProcessGroupMemberCount(ctx context.Context, procRoot string, processGroupID int) (int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect Linux process table: %w", err)
	}
	inspected := 0
	members := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || pid <= 0 {
			continue
		}
		inspected++
		if inspected > linuxProcessGroupRecoveryMaxEntries {
			return 0, errors.New("Linux process table exceeds the reconciliation bound")
		}
		stat, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "stat"))
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return 0, fmt.Errorf("failed to inspect Linux process %d: %w", pid, readErr)
		}
		observedGroup, parseErr := linuxProcessGroupIDFromStat(stat)
		if parseErr != nil {
			return 0, fmt.Errorf("failed to inspect Linux process %d identity: %w", pid, parseErr)
		}
		if observedGroup == processGroupID {
			members++
		}
	}
	return members, nil
}

func linuxProcessGroupIDFromStat(stat []byte) (int, error) {
	commandEnd := bytes.LastIndexByte(stat, ')')
	if commandEnd < 0 || commandEnd+1 >= len(stat) {
		return 0, errors.New("Linux process stat record is malformed")
	}
	fields := strings.Fields(string(stat[commandEnd+1:]))
	if len(fields) < 3 {
		return 0, errors.New("Linux process stat record is incomplete")
	}
	processGroupID, err := strconv.Atoi(fields[2])
	if err != nil || processGroupID < 0 {
		return 0, errors.New("Linux process group identifier is invalid")
	}
	return processGroupID, nil
}
