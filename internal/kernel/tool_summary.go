package kernel

import (
	"encoding/json"
	"slices"
	"strings"

	"polis/internal/core"
)

type toolCatalogSource struct {
	ID               string
	Name             string
	Status           string
	DescriptorDigest string
	Qualification    json.RawMessage
}

type ToolSummary struct {
	ServerID         string `json:"serverId"`
	ServerName       string `json:"serverName"`
	ToolName         string `json:"toolName"`
	DescriptorDigest string `json:"descriptorDigest"`
	Handle           string `json:"handle"`
	ToolsCalled      int    `json:"toolsCalled"`
	Callable         bool   `json:"callable"`
}

type ToolSummaryPage struct {
	Items      []ToolSummary `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

func visibleToolPage(bindingStatus string, boundIDs []string, sources []toolCatalogSource, cursor string, limit int) (ToolSummaryPage, error) {
	if cursor != "" && strings.ContainsAny(cursor, " \t\r\n") {
		return ToolSummaryPage{}, core.Malformed
	}
	if limit < 1 || limit > maxVisibleSkillPage {
		return ToolSummaryPage{}, core.Malformed
	}
	page := ToolSummaryPage{Items: []ToolSummary{}}
	if bindingStatus != "approved" || len(boundIDs) == 0 {
		return page, nil
	}
	bound := map[string]struct{}{}
	for _, id := range boundIDs {
		if !core.ValidID(id) {
			return ToolSummaryPage{}, core.Malformed
		}
		bound[id] = struct{}{}
	}
	matched := []ToolSummary{}
	ordered := append([]toolCatalogSource(nil), sources...)
	slices.SortFunc(ordered, func(a, b toolCatalogSource) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	for _, source := range ordered {
		if _, ok := bound[source.ID]; !ok || source.Status == "revoked" || source.Status == "unverified" || source.Status == "" {
			continue
		}
		items, err := toolSummaries(source)
		if err != nil {
			return ToolSummaryPage{}, err
		}
		matched = append(matched, items...)
	}
	slices.SortFunc(matched, func(a, b ToolSummary) int {
		if a.Handle < b.Handle {
			return -1
		}
		if a.Handle > b.Handle {
			return 1
		}
		return 0
	})
	visible := make([]ToolSummary, 0, len(matched))
	for _, item := range matched {
		if item.Handle > cursor {
			visible = append(visible, item)
		}
	}
	if len(visible) > limit {
		page.Items = visible[:limit]
		next := page.Items[len(page.Items)-1].Handle
		page.NextCursor = &next
		return page, nil
	}
	page.Items = visible
	return page, nil
}

func toolSummaries(source toolCatalogSource) ([]ToolSummary, error) {
	if source.DescriptorDigest == "" || !core.ValidID(source.ID) {
		return nil, core.Malformed
	}
	var receipt struct {
		ProbeStatus string   `json:"probeStatus"`
		ToolNames   []string `json:"toolNames"`
		ToolsCalled int      `json:"toolsCalled"`
	}
	if len(source.Qualification) > 0 && json.Unmarshal(source.Qualification, &receipt) != nil {
		return nil, core.Malformed
	}
	if receipt.ProbeStatus != "passed" {
		return []ToolSummary{}, nil
	}
	if receipt.ToolsCalled != 0 {
		return nil, core.Integrity
	}
	items := []ToolSummary{}
	seen := map[string]struct{}{}
	for _, name := range receipt.ToolNames {
		if name == "" || strings.ContainsAny(name, ":/ \t\r\n") {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, ToolSummary{
			ServerID: source.ID, ServerName: source.Name, ToolName: name, DescriptorDigest: source.DescriptorDigest,
			Handle: source.ID + ":" + name, ToolsCalled: 0, Callable: false,
		})
	}
	return items, nil
}
