// pattern: Functional Core
package runner

import "strings"

type AppContainerLoopbackSID struct {
	SID        string
	Attributes uint32
}

func MergeLoopbackAppContainerSIDEntries(current []AppContainerLoopbackSID, target string, enabled bool) []AppContainerLoopbackSID {
	currentSIDs := make([]string, len(current))
	bySID := make(map[string][]AppContainerLoopbackSID, len(current))
	for index, entry := range current {
		currentSIDs[index] = entry.SID
		key := strings.ToUpper(entry.SID)
		bySID[key] = append(bySID[key], entry)
	}
	mergedSIDs := MergeLoopbackSIDStrings(currentSIDs, target, enabled)
	merged := make([]AppContainerLoopbackSID, 0, len(mergedSIDs))
	for _, sid := range mergedSIDs {
		key := strings.ToUpper(sid)
		if existing := bySID[key]; len(existing) > 0 {
			merged = append(merged, existing[0])
			bySID[key] = existing[1:]
			continue
		}
		merged = append(merged, AppContainerLoopbackSID{SID: sid})
	}
	return merged
}

func EqualLoopbackAppContainerSIDEntries(left, right []AppContainerLoopbackSID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func MergeLoopbackSIDStrings(current []string, target string, enabled bool) []string {
	result := append([]string(nil), current...)
	if target == "" {
		return result
	}
	filtered := result[:0]
	found := false
	for _, candidate := range result {
		if strings.EqualFold(candidate, target) {
			found = true
			if !enabled {
				continue
			}
		}
		filtered = append(filtered, candidate)
	}
	if enabled && !found {
		filtered = append(filtered, target)
	}
	return filtered
}
