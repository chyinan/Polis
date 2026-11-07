package kernel

import (
	"encoding/json"
	"slices"

	"polis/internal/core"
)

const maxVisibleSkillPage = 32

type skillSummarySource struct {
	ID            string
	PackageID     string
	Revision      string
	DisplayName   string
	SourceRef     string
	ContentDigest string
	Status        string
	Manifest      json.RawMessage
}

type SkillSummary struct {
	ID              string `json:"id"`
	PackageID       string `json:"packageId"`
	Revision        string `json:"revision"`
	DisplayName     string `json:"displayName"`
	Purpose         string `json:"purpose"`
	SourceRef       string `json:"sourceRef"`
	ContentDigest   string `json:"contentDigest"`
	ExecutionStatus string `json:"executionStatus"`
	ScriptsExecuted bool   `json:"scriptsExecuted"`
}

type SkillSummaryPage struct {
	Items      []SkillSummary `json:"items"`
	NextCursor *string        `json:"nextCursor"`
}

func visibleSkillPage(bindingStatus string, boundIDs []string, sources []skillSummarySource, cursor string, limit int) (SkillSummaryPage, error) {
	if cursor != "" && !core.ValidID(cursor) {
		return SkillSummaryPage{}, core.Malformed
	}
	if limit < 1 || limit > maxVisibleSkillPage {
		return SkillSummaryPage{}, core.Malformed
	}
	page := SkillSummaryPage{Items: []SkillSummary{}}
	if bindingStatus != "approved" || len(boundIDs) == 0 {
		return page, nil
	}
	bound := map[string]struct{}{}
	for _, id := range boundIDs {
		if !core.ValidID(id) {
			return SkillSummaryPage{}, core.Malformed
		}
		bound[id] = struct{}{}
	}
	ordered := append([]skillSummarySource(nil), sources...)
	slices.SortFunc(ordered, func(a, b skillSummarySource) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	matched := make([]SkillSummary, 0, len(ordered))
	for _, source := range ordered {
		if _, ok := bound[source.ID]; !ok || source.Status != "approved" || source.ID <= cursor {
			continue
		}
		item, err := skillSummary(source)
		if err != nil {
			return SkillSummaryPage{}, err
		}
		matched = append(matched, item)
	}
	if len(matched) > limit {
		page.Items = matched[:limit]
		next := page.Items[len(page.Items)-1].ID
		page.NextCursor = &next
		return page, nil
	}
	page.Items = matched
	return page, nil
}

func skillSummary(source skillSummarySource) (SkillSummary, error) {
	if source.ContentDigest == "" || !core.ValidID(source.ID) {
		return SkillSummary{}, core.Malformed
	}
	var document struct {
		Description string `json:"description"`
	}
	if len(source.Manifest) > 0 && json.Unmarshal(source.Manifest, &document) != nil {
		return SkillSummary{}, core.Malformed
	}
	return SkillSummary{
		ID: source.ID, PackageID: source.PackageID, Revision: source.Revision, DisplayName: source.DisplayName,
		Purpose: document.Description, SourceRef: source.SourceRef, ContentDigest: source.ContentDigest,
		ExecutionStatus: "unverified", ScriptsExecuted: false,
	}, nil
}
