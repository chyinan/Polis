// pattern: Functional Core
package codex

func Tools() []any {
	definitions := []struct {
		name, description string
		properties        map[string]any
	}{
		{"work_current", "Read your authoritative task, responsibility and neutral handover context.", map[string]any{}},
		{"context_read", "Read approved context, facts and decisions.", map[string]any{}},
		{"workspace_read", "Read the sole allowed file formatter.go and its current digest.", map[string]any{}},
		{"workspace_replace", "Replace formatter.go with complete content using its expected digest. Only pure formatting functions and unaliased math/strconv imports are allowed; no init, globals or test-lifecycle hooks. No other file path is accepted.", map[string]any{"expected_digest": map[string]any{"type": "string"}, "content": map[string]any{"type": "string", "maxLength": 4096}}},
		{"workspace_check", "Compile and run the frozen task checks against the current file; returns an evidence receipt.", map[string]any{}},
		{"work_checkpoint", "Persist partial progress, compatibility facts, decisions, rejected alternatives and evidence receipt IDs from successful workspace_check calls on the current content.", map[string]any{"summary": map[string]any{"type": "string"}, "facts": stringArray(), "decisions": stringArray(), "rejected": stringArray(), "evidence": stringArray()}},
		{"artifact_submit", "Submit the current fixed file as a candidate. Independent acceptance is performed by the controller, not you.", map[string]any{}},
	}
	var tools []any
	for _, d := range definitions {
		required := []string{}
		for _, key := range []string{"expected_digest", "content", "summary", "facts", "decisions", "rejected", "evidence"} {
			if _, ok := d.properties[key]; ok {
				required = append(required, key)
			}
		}
		tools = append(tools, map[string]any{"type": "function", "name": "polis_" + d.name, "description": d.description, "inputSchema": map[string]any{"type": "object", "properties": d.properties, "required": required, "additionalProperties": false}})
	}
	return tools
}
func stringArray() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "maxItems": 8, "items": map[string]any{"type": "string", "maxLength": 512}}
}
