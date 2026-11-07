package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestSkillLoadPersistsApprovedBodyWithoutExecution(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "skill-load-co")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, "mission-1", "start-skill-load"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,$5,'ready')`, scope.company, "compat-task-1", "mission-1", "emp-backend", core.TaskKindCompat); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO missions(company_id,id,title,goal,contract) VALUES($1,'mission-2','mission-2','Mission mission-2',$2)`, scope.company, core.Contract); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,$5,'ready')`, scope.company, "frontend-task-1", "mission-2", "emp-frontend", core.TaskKindCompat); err != nil {
		t.Fatal(err)
	}
	document := []byte("---\nname: brand-guidelines\ndescription: colors\nallowed-tools: [shell]\n---\nUse the palette.\n")
	skill, err := k.TXImportSkillDocument(ctx, scope.company, SkillDocumentInput{PublisherScope: "company", Revision: "1", SourceRef: "library://skills/brand-guidelines/SKILL.md", Document: document, References: []SkillReferenceFile{{Name: "references/palette.md", Body: []byte("Use blue.\n")}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXLoadSkill(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID); !errors.Is(err, core.Denied) {
		t.Fatalf("candidate load err = %v", err)
	}
	approved, err := k.TXSetSkillRevisionStatus(ctx, scope.company, skill.ID, "approve")
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approve = %+v %v", approved, err)
	}
	command := "polis-not-executed"
	if _, err = k.TXRegisterMCPServerDefinition(ctx, scope.company, MCPServerDefinitionInput{ID: "memory-server", Name: "memory", Transport: "stdio", Command: &command, Args: []string{}, DescriptorDigest: "sha256:" + strings.Repeat("ab", 32)}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXUpsertEmployeeCapabilityBinding(ctx, scope.company, CapabilityBindingInput{EmployeeID: "emp-backend", SkillRevisionIDs: []string{skill.ID}, MCPDefinitionIDs: []string{"memory-server"}, Status: "approved"}); err != nil {
		t.Fatal(err)
	}
	beforeProbe, err := k.TXListVisibleTools(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(beforeProbe.Items) != 0 {
		t.Fatalf("unprobed tools = %+v %v", beforeProbe, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_server_definitions SET status='candidate', qualification=$1 WHERE company_id=$2 AND id=$3`, []byte(`{"probeStatus":"passed","toolNames":["polis_probe_ping"],"toolsCalled":0}`), scope.company, "memory-server"); err != nil {
		t.Fatal(err)
	}
	tools, err := k.TXListVisibleTools(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(tools.Items) != 1 || tools.Items[0].ToolName != "polis_probe_ping" || tools.Items[0].ToolsCalled != 0 || tools.Items[0].Callable || tools.NextCursor != nil {
		t.Fatalf("visible tools = %+v %v", tools, err)
	}
	if _, err = k.TXDescribeVisibleTool(ctx, scope.company, "emp-backend", tools.Items[0].Handle); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("names-only describe err = %v", err)
	}
	unnamed, err := k.TXListVisibleResources(ctx, scope.company, "emp-backend", "memory-server", "", 32)
	if err != nil || len(unnamed.Items) != 0 {
		t.Fatalf("names-only resources = %+v %v", unnamed, err)
	}
	if _, err = k.TXUpsertEmployeeCapabilityBinding(ctx, scope.company, CapabilityBindingInput{EmployeeID: "emp-frontend", SkillRevisionIDs: []string{skill.ID}, Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXLoadSkill(ctx, scope.company, "frontend-task-1", "emp-frontend", skill.ID); !errors.Is(err, core.Denied) {
		t.Fatalf("draft binding load err = %v", err)
	}
	visible, err := k.TXListVisibleSkills(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(visible.Items) != 1 || visible.Items[0].ID != skill.ID || visible.Items[0].Purpose != "colors" || visible.Items[0].ScriptsExecuted || visible.NextCursor != nil {
		t.Fatalf("visible skills = %+v %v", visible, err)
	}
	encoded, err := json.Marshal(visible.Items[0])
	if err != nil || strings.Contains(string(encoded), "Use the palette") || strings.Contains(string(encoded), `"body"`) {
		t.Fatalf("summary leaked body: %s %v", encoded, err)
	}
	hidden, err := k.TXListVisibleSkills(ctx, scope.company, "emp-frontend", "", 32)
	if err != nil || len(hidden.Items) != 0 {
		t.Fatalf("draft visible skills = %+v %v", hidden, err)
	}
	if _, err = k.TXReadSkillReference(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID, "references/palette.md"); !errors.Is(err, core.Denied) {
		t.Fatalf("unread reference err = %v", err)
	}
	first, err := k.TXLoadSkill(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Repeated || first.ScriptsExecuted || first.Body != "Use the palette.\n" || first.ContentDigest != skill.ContentDigest || len(first.References) != 1 || first.References[0] != "references/palette.md" {
		t.Fatalf("first load = %+v", first)
	}
	second, err := k.TXLoadSkill(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Repeated || second.ID != first.ID || second.ContentDigest != first.ContentDigest || second.Body != first.Body || second.ScriptsExecuted {
		t.Fatalf("repeat load = %+v", second)
	}
	var loads int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM skill_loads WHERE company_id=$1`, scope.company).Scan(&loads); err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("load rows = %d", loads)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2`, scope.company, "compat-task-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXLoadSkill(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID); !errors.Is(err, core.Denied) {
		t.Fatalf("completed task load err = %v", err)
	}
	if _, err = k.TXSetSkillRevisionStatus(ctx, scope.company, skill.ID, "revoke"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE tasks SET state='working' WHERE company_id=$1 AND id=$2`, scope.company, "compat-task-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXLoadSkill(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID); !errors.Is(err, core.Denied) {
		t.Fatalf("revoked skill load err = %v", err)
	}
	afterRevoke, err := k.TXListVisibleSkills(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(afterRevoke.Items) != 0 {
		t.Fatalf("revoked visible skills = %+v %v", afterRevoke, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_server_definitions SET status='revoked' WHERE company_id=$1 AND id=$2`, scope.company, "memory-server"); err != nil {
		t.Fatal(err)
	}
	revokedTools, err := k.TXListVisibleTools(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(revokedTools.Items) != 0 {
		t.Fatalf("revoked tools = %+v %v", revokedTools, err)
	}
	if _, err = k.TXDescribeVisibleTool(ctx, scope.company, "emp-backend", "memory-server:polis_probe_ping"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("revoked describe err = %v", err)
	}
	frozen, err := freezeToolDescriptor("polis_probe_ping", "Ping the probe.", json.RawMessage(`{"type":"object"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := json.Marshal(map[string]any{
		"probeStatus": "passed", "toolNames": []string{"polis_probe_ping"}, "toolsCalled": 0, "resourcesRead": 0,
		"toolDescriptors":   []CachedToolDescriptor{frozen},
		"resourceSummaries": []CachedResourceSummary{{URI: "polis://probe/ping", Name: "Probe ping"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_server_definitions SET qualification=$1 WHERE company_id=$2 AND id=$3`, qualification, scope.company, "memory-server"); err != nil {
		t.Fatal(err)
	}
	revokedResources, err := k.TXListVisibleResources(ctx, scope.company, "emp-backend", "memory-server", "", 32)
	if err != nil || len(revokedResources.Items) != 0 {
		t.Fatalf("revoked resources = %+v %v", revokedResources, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE mcp_server_definitions SET status='candidate' WHERE company_id=$1 AND id=$2`, scope.company, "memory-server"); err != nil {
		t.Fatal(err)
	}
	read, err := k.TXReadSkillReference(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID, "references/palette.md")
	if err != nil || read.ScriptsExecuted || read.Body != "Use blue.\n" || read.ContentDigest != contentDigest([]byte("Use blue.\n")) {
		t.Fatalf("reference read = %+v %v", read, err)
	}
	if _, err = k.TXReadSkillReference(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID, "scripts/run.sh"); !errors.Is(err, core.Malformed) {
		t.Fatalf("script reference err = %v", err)
	}
	if _, err = k.TXReadSkillReference(ctx, scope.company, "compat-task-1", "emp-backend", skill.ID, "references/missing.md"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("missing reference err = %v", err)
	}
	catalog, err := k.ListCapabilityCatalog(ctx, scope.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Loads) != 1 || catalog.Loads[0].ID != first.ID || catalog.Loads[0].Body != first.Body || catalog.Loads[0].ScriptsExecuted {
		t.Fatalf("catalog loads = %+v", catalog.Loads)
	}
	var mcpCount int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM mcp_server_definitions WHERE company_id=$1`, scope.company).Scan(&mcpCount); err != nil {
		t.Fatal(err)
	}
	if mcpCount != 1 {
		t.Fatalf("mcp definitions = %d", mcpCount)
	}
	cached, err := k.TXListVisibleTools(ctx, scope.company, "emp-backend", "", 32)
	if err != nil || len(cached.Items) != 1 || cached.Items[0].ToolsCalled != 0 || cached.Items[0].Callable {
		t.Fatalf("cached tools = %+v %v", cached, err)
	}
	described, err := k.TXDescribeVisibleTool(ctx, scope.company, "emp-backend", cached.Items[0].Handle)
	if err != nil || described.Description != "Ping the probe." || described.ToolsCalled != 0 || described.Callable || string(described.OutputSchema) != "null" || described.DescriptorDigest != frozen.Digest {
		t.Fatalf("described = %+v %v", described, err)
	}
	describedBytes, err := json.Marshal(described)
	if err != nil || strings.Contains(string(describedBytes), "polis-not-executed") || strings.Contains(string(describedBytes), `"command"`) {
		t.Fatalf("described leaked command: %s %v", describedBytes, err)
	}
	var storedCommand string
	var storedCalled int
	if err = k.pool.QueryRow(ctx, `SELECT command, (qualification->>'toolsCalled')::int FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, scope.company, "memory-server").Scan(&storedCommand, &storedCalled); err != nil {
		t.Fatal(err)
	}
	if storedCommand != "polis-not-executed" || storedCalled != 0 {
		t.Fatalf("command=%s toolsCalled=%d", storedCommand, storedCalled)
	}
	resources, err := k.TXListVisibleResources(ctx, scope.company, "emp-backend", "memory-server", "", 32)
	if err != nil || len(resources.Items) != 1 || resources.Items[0].URI != "polis://probe/ping" || resources.Items[0].Name != "Probe ping" || resources.Items[0].ResourcesRead != 0 || resources.Items[0].Readable || resources.NextCursor != nil {
		t.Fatalf("resources = %+v %v", resources, err)
	}
	resourceBytes, err := json.Marshal(resources.Items[0])
	if err != nil || strings.Contains(string(resourceBytes), "polis-not-executed") || strings.Contains(string(resourceBytes), `"command"`) {
		t.Fatalf("resources leaked command: %s %v", resourceBytes, err)
	}
}
