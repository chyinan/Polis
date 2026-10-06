// pattern: Functional Core
package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProductEmployeeToolsExposeGenericAuthorizedTaskSemantics(t *testing.T) {
	tools := ProductEmployeeTools()
	want := []string{
		"polis_work_current",
		"polis_context_read",
		"polis_workspace_read",
		"polis_workspace_replace",
		"polis_workspace_check",
		"polis_work_checkpoint",
		"polis_task_submit",
	}
	if len(tools) != len(want) {
		t.Fatalf("tool count=%d, want %d", len(tools), len(want))
	}
	var encoded []byte
	seen := make(map[string]bool, len(tools))
	for i, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool %d has type %T", i, raw)
		}
		if name, _ := tool["name"].(string); name != want[i] || seen[name] {
			t.Fatalf("tool %d name=%q; want %q and no duplicates", i, name, want[i])
		} else {
			seen[name] = true
		}
		description, _ := tool["description"].(string)
		for _, forbidden := range []string{"formatter.go", "formatting-only", "pagination v3", "frozen checker", "probe evidence", "R0.3A"} {
			if strings.Contains(strings.ToLower(description), strings.ToLower(forbidden)) {
				t.Errorf("%s description contains task-specific term %q: %s", want[i], forbidden, description)
			}
		}
		if i == 2 && !strings.Contains(strings.ToLower(description), "current authorized task workspace") {
			t.Errorf("workspace_read description is not task-scoped: %s", description)
		}
		if i == 4 && !strings.Contains(strings.ToLower(description), "acceptance contract bound to this task") {
			t.Errorf("workspace_check description does not expose binding semantics: %s", description)
		}
	}
	encoded, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	var workspaceReplace map[string]any
	for _, raw := range tools {
		tool := raw.(map[string]any)
		if tool["name"] == "polis_workspace_replace" {
			workspaceReplace = tool
		}
	}
	schema := workspaceReplace["inputSchema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if properties["expected_revision"] == nil || properties["expected_digest"] == nil || properties["content"] == nil {
		t.Fatalf("workspace_replace lacks digest+revision CAS contract: %s", encoded)
	}
	if properties["path"] != nil || strings.Contains(strings.ToLower(string(encoded)), "formatter.go") {
		t.Fatalf("product workspace is falsely modeled as a file tree: %s", encoded)
	}
}

func TestProductEmployeeSkillSurfaceAddsOnlyBoundedReadOnlyLoad(t *testing.T) {
	legacy := ProductEmployeeTools()
	tools := ProductEmployeeToolsWithReadOnlySkill()
	if len(legacy) != 7 || len(tools) != 8 {
		t.Fatalf("legacy/current skill tool counts=%d/%d, want 7/8", len(legacy), len(tools))
	}
	for index, tool := range legacy {
		legacyJSON, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		currentJSON, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(legacyJSON) != string(currentJSON) {
			t.Fatalf("legacy tool %d changed while adding the separately versioned Skill surface", index)
		}
	}
	tool, ok := tools[7].(map[string]any)
	if !ok || tool["name"] != "polis_skills_load" {
		t.Fatalf("Skill tool=%v, want polis_skills_load", tools[7])
	}
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok || schema["additionalProperties"] != false {
		t.Fatalf("Skill load schema is not closed: %v", tool["inputSchema"])
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["skill_id"] == nil || properties["relative_path"] == nil {
		t.Fatalf("Skill load must name an exact bound revision and relative path: %v", schema)
	}
	definition, _ := json.Marshal(tool)
	for _, forbidden := range []string{"install", "shell", "network"} {
		if strings.Contains(strings.ToLower(string(definition)), forbidden) {
			t.Fatalf("Skill load tool suggests executable capability %q: %s", forbidden, definition)
		}
	}
}

func TestProductEmployeeReadOnlyJobsSurfaceAddsOnlyBoundedStatusAndLogs(t *testing.T) {
	legacy := ProductEmployeeTools()
	tools := ProductEmployeeToolsWithReadOnlyJobs()
	if len(legacy) != 7 || len(tools) != 9 {
		t.Fatalf("legacy/read-only-jobs tool counts=%d/%d, want 7/9", len(legacy), len(tools))
	}
	for index, tool := range legacy {
		legacyJSON, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		currentJSON, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(legacyJSON) != string(currentJSON) {
			t.Fatalf("legacy tool %d changed while adding the separately versioned jobs-read surface", index)
		}
	}
	for index, want := range []string{"polis_jobs_status", "polis_jobs_logs"} {
		tool, ok := tools[7+index].(map[string]any)
		if !ok || tool["name"] != want {
			t.Fatalf("read-only jobs tool %d=%v, want %s", index, tools[7+index], want)
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not closed: %v", want, tool["inputSchema"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok || properties["job_id"] == nil {
			t.Fatalf("%s must require an exact job_id: %v", want, schema)
		}
		definition, _ := json.Marshal(tool)
		for _, forbidden := range []string{"start", "stop", "borrower", "lease", "process"} {
			if strings.Contains(strings.ToLower(string(definition)), forbidden) {
				t.Fatalf("%s suggests non-read-only capability %q: %s", want, forbidden, definition)
			}
		}
	}
}

func TestProductEmployeeBorrowerLeaseSurfaceExtendsReadOnlyJobsWithBoundedLeaseCalls(t *testing.T) {
	readOnly := ProductEmployeeToolsWithReadOnlyJobs()
	tools := ProductEmployeeToolsWithBorrowerLeases()
	if len(readOnly) != 9 || len(tools) != 12 {
		t.Fatalf("read-only-jobs/borrower-lease tool counts=%d/%d, want 9/12", len(readOnly), len(tools))
	}
	for index, tool := range readOnly {
		readOnlyJSON, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		currentJSON, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(readOnlyJSON) != string(currentJSON) {
			t.Fatalf("read-only job tool %d changed while adding the borrower-lease surface", index)
		}
	}
	for index, want := range []string{"polis_jobs_borrow", "polis_jobs_touch", "polis_jobs_release"} {
		tool, ok := tools[9+index].(map[string]any)
		if !ok || tool["name"] != want {
			t.Fatalf("borrower lease tool %d=%v, want %s", index, tools[9+index], want)
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not closed: %v", want, tool["inputSchema"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s schema properties missing: %v", want, schema)
		}
		if want == "polis_jobs_borrow" && (properties["job_id"] == nil || properties["generation"] == nil) {
			t.Fatalf("%s must require exact job_id and generation: %v", want, schema)
		}
		if want != "polis_jobs_borrow" && properties["lease_id"] == nil {
			t.Fatalf("%s must require an exact lease_id: %v", want, schema)
		}
		definition, _ := json.Marshal(tool)
		for _, forbidden := range []string{"command", "process", "cookie", "origin"} {
			if strings.Contains(strings.ToLower(string(definition)), forbidden) {
				t.Fatalf("%s suggests an out-of-scope capability %q: %s", want, forbidden, definition)
			}
		}
	}
}

func TestProductEmployeeBrowserRunSurfaceAddsDefaultDeniedControlPlaneTools(t *testing.T) {
	borrower := ProductEmployeeToolsWithBorrowerLeases()
	tools := ProductEmployeeToolsWithBrowserRun()
	if len(borrower) != 12 || len(tools) != 14 {
		t.Fatalf("borrower/browser tool counts=%d/%d, want 12/14", len(borrower), len(tools))
	}
	for index, tool := range borrower {
		left, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		right, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(left) != string(right) {
			t.Fatalf("borrower tool %d changed while adding BrowserRun", index)
		}
	}
	for index, want := range []string{"polis_browser_run", "polis_browser_results"} {
		tool, ok := tools[12+index].(map[string]any)
		if !ok || tool["name"] != want {
			t.Fatalf("BrowserRun tool %d=%v, want %s", index, tools[12+index], want)
		}
		definition, _ := json.Marshal(tool)
		for _, forbidden := range []string{"cookie", "storageState", "bearer", "download", "websocket"} {
			if strings.Contains(strings.ToLower(string(definition)), strings.ToLower(forbidden)) {
				t.Fatalf("%s exposes forbidden browser secret/egress concept %q: %s", want, forbidden, definition)
			}
		}
	}
}

func TestProductEmployeeEnvironmentEnsureSurfaceExtendsEnvironmentStatusOnly(t *testing.T) {
	status := ProductEmployeeToolsWithEnvironmentStatus()
	tools := ProductEmployeeToolsWithEnvironmentEnsure()
	if len(status) != 8 || len(tools) != 9 {
		t.Fatalf("environment status/ensure tool counts=%d/%d, want 8/9", len(status), len(tools))
	}
	for index, tool := range status {
		statusJSON, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		currentJSON, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(statusJSON) != string(currentJSON) {
			t.Fatalf("environment status tool %d changed while adding ensure", index)
		}
	}
	tool, ok := tools[8].(map[string]any)
	if !ok || tool["name"] != "polis_environment_ensure" {
		t.Fatalf("environment ensure tool=%v, want polis_environment_ensure", tools[8])
	}
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok || schema["additionalProperties"] != false {
		t.Fatalf("environment ensure schema is not closed: %v", tool["inputSchema"])
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["revision_id"] == nil {
		t.Fatalf("environment ensure must require an exact revision_id: %v", schema)
	}
}

func TestControlledMCPToolSurfaceAddsOnlyBoundedCallContract(t *testing.T) {
	legacy := ProductEmployeeTools()
	tools := ProductEmployeeToolsWithControlledMCP()
	if len(legacy) != 7 || len(tools) != 8 {
		t.Fatalf("legacy/controlled-MCP tool counts=%d/%d, want 7/8", len(legacy), len(tools))
	}
	for index, tool := range legacy {
		legacyJSON, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		currentJSON, err := json.Marshal(tools[index])
		if err != nil {
			t.Fatal(err)
		}
		if string(legacyJSON) != string(currentJSON) {
			t.Fatalf("legacy tool %d changed when the controlled MCP extension was added", index)
		}
	}
	tool, ok := tools[7].(map[string]any)
	if !ok || tool["name"] != "polis_mcp_call" {
		t.Fatalf("controlled MCP tool=%v, want polis_mcp_call", tools[7])
	}
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok || schema["additionalProperties"] != false {
		t.Fatalf("MCP call schema is not closed: %v", tool["inputSchema"])
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("MCP call schema properties have unexpected type: %T", schema["properties"])
	}
	for _, field := range []string{"capability_id", "tool_name", "tool_schema_sha256", "arguments"} {
		if properties[field] == nil {
			t.Errorf("MCP call schema is missing %q", field)
		}
	}
	for _, forbidden := range []string{"command", "endpoint", "url", "transport"} {
		if properties[forbidden] != nil {
			t.Errorf("MCP call accepts caller-controlled process/transport field %q", forbidden)
		}
	}
	definition, _ := json.Marshal(tool)
	for _, requiredText := range []string{"employee-bound", "approved", "untrusted"} {
		if !strings.Contains(strings.ToLower(string(definition)), requiredText) {
			t.Errorf("MCP call description does not explain %q boundary", requiredText)
		}
	}
}

func TestProductGuidanceSummarySchemaRejectsWhitespaceOnly(t *testing.T) {
	tools := ProductEmployeeToolsWithGuidance()
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok || tool["name"] != "polis_guidance_respond" {
			continue
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("guidance response schema has unexpected type: %T", tool["inputSchema"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("guidance response properties have unexpected type: %T", schema["properties"])
		}
		summary, ok := properties["summary"].(map[string]any)
		if !ok || summary["pattern"] != "\\S" {
			t.Fatalf("guidance summary schema does not require non-whitespace text: %v", properties["summary"])
		}
		return
	}
	t.Fatal("guidance response tool is missing")
}
