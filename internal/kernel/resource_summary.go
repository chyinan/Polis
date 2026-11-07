package kernel

import (
	"encoding/json"
	"slices"
	"strings"

	"polis/internal/core"
)

type ResourceSummary struct {
	ServerID      string `json:"serverId"`
	ServerName    string `json:"serverName"`
	URI           string `json:"uri"`
	Name          string `json:"name"`
	ResourcesRead int    `json:"resourcesRead"`
	Readable      bool   `json:"readable"`
}

type ResourceSummaryPage struct {
	Items      []ResourceSummary `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

type CachedResourceSummary struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

func visibleResourcePage(bindingStatus string, boundIDs []string, sources []toolCatalogSource, serverID, cursor string, limit int) (ResourceSummaryPage, error) {
	if !core.ValidID(serverID) || (cursor != "" && strings.ContainsAny(cursor, " \t\r\n")) {
		return ResourceSummaryPage{}, core.Malformed
	}
	if limit < 1 || limit > maxVisibleSkillPage {
		return ResourceSummaryPage{}, core.Malformed
	}
	page := ResourceSummaryPage{Items: []ResourceSummary{}}
	if bindingStatus != "approved" {
		return page, nil
	}
	bound := false
	for _, id := range boundIDs {
		if !core.ValidID(id) {
			return ResourceSummaryPage{}, core.Malformed
		}
		if id == serverID {
			bound = true
		}
	}
	if !bound {
		return page, nil
	}
	var source *toolCatalogSource
	for i := range sources {
		if sources[i].ID == serverID {
			source = &sources[i]
			break
		}
	}
	if source == nil || source.Status == "revoked" || source.Status == "unverified" || source.Status == "" {
		return page, nil
	}
	if source.DescriptorDigest == "" || !core.ValidID(source.ID) {
		return ResourceSummaryPage{}, core.Malformed
	}
	var receipt struct {
		ProbeStatus       string                  `json:"probeStatus"`
		ToolsCalled       int                     `json:"toolsCalled"`
		ResourcesRead     int                     `json:"resourcesRead"`
		ResourceSummaries []CachedResourceSummary `json:"resourceSummaries"`
	}
	if len(source.Qualification) > 0 && json.Unmarshal(source.Qualification, &receipt) != nil {
		return ResourceSummaryPage{}, core.Malformed
	}
	if receipt.ProbeStatus != "passed" {
		return page, nil
	}
	if receipt.ToolsCalled != 0 || receipt.ResourcesRead != 0 {
		return ResourceSummaryPage{}, core.Integrity
	}
	items, err := resourceSummaries(*source, receipt.ResourceSummaries)
	if err != nil {
		return ResourceSummaryPage{}, err
	}
	visible := make([]ResourceSummary, 0, len(items))
	for _, item := range items {
		if item.URI > cursor {
			visible = append(visible, item)
		}
	}
	if len(visible) > limit {
		page.Items = visible[:limit]
		next := page.Items[len(page.Items)-1].URI
		page.NextCursor = &next
		return page, nil
	}
	page.Items = visible
	return page, nil
}

func resourceSummaries(source toolCatalogSource, cached []CachedResourceSummary) ([]ResourceSummary, error) {
	if len(cached) > maxManifestListItems {
		return nil, core.TooLarge
	}
	items := make([]ResourceSummary, 0, len(cached))
	seen := map[string]struct{}{}
	for _, item := range cached {
		if item.URI == "" || len(item.URI) > 1024 || strings.ContainsAny(item.URI, " \t\r\n") {
			return nil, core.Malformed
		}
		if len(item.Name) > maxManifestDocument {
			return nil, core.TooLarge
		}
		if _, ok := seen[item.URI]; ok {
			return nil, core.Malformed
		}
		seen[item.URI] = struct{}{}
		name := item.Name
		if name == "" {
			name = item.URI
		}
		items = append(items, ResourceSummary{
			ServerID: source.ID, ServerName: source.Name, URI: item.URI, Name: name, ResourcesRead: 0, Readable: false,
		})
	}
	slices.SortFunc(items, func(a, b ResourceSummary) int {
		if a.URI < b.URI {
			return -1
		}
		if a.URI > b.URI {
			return 1
		}
		return 0
	})
	return items, nil
}

func parseResourceList(raw json.RawMessage) ([]CachedResourceSummary, error) {
	if len(raw) == 0 {
		return []CachedResourceSummary{}, nil
	}
	var listed struct {
		Resources []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"resources"`
	}
	if json.Unmarshal(raw, &listed) != nil {
		return nil, core.Malformed
	}
	if len(listed.Resources) > maxManifestListItems {
		return nil, core.TooLarge
	}
	items := make([]CachedResourceSummary, 0, len(listed.Resources))
	seen := map[string]struct{}{}
	for _, item := range listed.Resources {
		if item.URI == "" || len(item.URI) > 1024 || strings.ContainsAny(item.URI, " \t\r\n") || len(item.Name) > maxManifestDocument {
			if len(item.Name) > maxManifestDocument {
				return nil, core.TooLarge
			}
			return nil, core.Malformed
		}
		if _, ok := seen[item.URI]; ok {
			return nil, core.Malformed
		}
		seen[item.URI] = struct{}{}
		items = append(items, CachedResourceSummary{URI: item.URI, Name: item.Name})
	}
	return items, nil
}
