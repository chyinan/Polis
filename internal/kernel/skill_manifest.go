// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
)

const (
	maxManifestDocument      = 8 * 1024
	skillManifestFormat      = "agent-skills-skill-md@1"
	maxSkillDisplayNameBytes = 160
	maxSkillDescriptionBytes = 2000
	maxManifestListItems     = 32
	maxManifestTokenBytes    = 80
)

type ManifestQualification struct {
	FormatStatus     string   `json:"formatStatus"`
	ExecutionStatus  string   `json:"executionStatus"`
	ConnectionStatus string   `json:"connectionStatus"`
	ScriptsExecuted  bool     `json:"scriptsExecuted"`
	NetworkAccess    bool     `json:"networkAccess"`
	ScriptsPresent   bool     `json:"scriptsPresent"`
	Issues           []string `json:"issues"`
}

type ParsedSkillDocument struct {
	PackageID     string
	DisplayName   string
	ContentDigest string
	Manifest      json.RawMessage
}

type skillManifestDocument struct {
	Format             string                   `json:"format"`
	Name               string                   `json:"name"`
	Description        string                   `json:"description"`
	License            string                   `json:"license,omitempty"`
	AllowedTools       []string                 `json:"allowedTools"`
	Body               string                   `json:"body"`
	References         []string                 `json:"references"`
	ReferenceSnapshots []skillReferenceSnapshot `json:"referenceSnapshots"`
	ScriptsPresent     bool                     `json:"scriptsPresent"`
	Qualification      ManifestQualification    `json:"qualification"`
}

func ParseSkillDocument(document []byte, scriptsPresent bool) (ParsedSkillDocument, error) {
	return parseSkillDocument(document, scriptsPresent, nil)
}

func parseSkillDocument(document []byte, scriptsPresent bool, references []SkillReferenceFile) (ParsedSkillDocument, error) {
	if len(document) == 0 {
		return ParsedSkillDocument{}, core.Malformed
	}
	if len(document) > maxManifestDocument {
		return ParsedSkillDocument{}, core.TooLarge
	}
	if !utf8.Valid(document) || strings.ContainsRune(string(document), '\x00') {
		return ParsedSkillDocument{}, core.Malformed
	}
	front, body, err := splitFrontMatter(string(document))
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	fields, err := parseManifestFields(front)
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	for key := range fields {
		if !allowedSkillKey(key) {
			return ParsedSkillDocument{}, core.Malformed
		}
	}
	name := strings.TrimSpace(fields["name"])
	if _, ok := fields["name"]; !ok || name == "" || len(name) > maxSkillDisplayNameBytes || strings.ContainsAny(name, "\r\n") {
		return ParsedSkillDocument{}, core.Malformed
	}
	description := fields["description"]
	if len(description) > maxSkillDescriptionBytes {
		return ParsedSkillDocument{}, core.TooLarge
	}
	license := fields["license"]
	if len(license) > maxSkillDisplayNameBytes {
		return ParsedSkillDocument{}, core.Malformed
	}
	allowed, err := parseTokenList(fields["allowed-tools"])
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	packageID, err := packageIDFromSkillName(name)
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	names, snapshots, err := snapshotReferences(references)
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	manifest, err := json.Marshal(skillManifestDocument{
		Format: skillManifestFormat, Name: name, Description: description, License: license,
		AllowedTools: allowed, Body: body, References: names, ReferenceSnapshots: snapshots, ScriptsPresent: scriptsPresent,
		Qualification: offlineQualification(scriptsPresent, "supported"),
	})
	if err != nil {
		return ParsedSkillDocument{}, err
	}
	return ParsedSkillDocument{PackageID: packageID, DisplayName: name, ContentDigest: contentDigest(document), Manifest: manifest}, nil
}

type SkillReferenceFile struct {
	Name string
	Body []byte
}

type skillReferenceSnapshot struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Body   string `json:"body"`
}

func snapshotReferences(files []SkillReferenceFile) ([]string, []skillReferenceSnapshot, error) {
	if len(files) > maxManifestListItems {
		return nil, nil, core.TooLarge
	}
	names := []string{}
	snapshots := []skillReferenceSnapshot{}
	seen := map[string]struct{}{}
	for _, file := range files {
		if !safeSkillReference(file.Name) {
			return nil, nil, core.Malformed
		}
		if _, ok := seen[file.Name]; ok {
			return nil, nil, core.Malformed
		}
		seen[file.Name] = struct{}{}
		if len(file.Body) > maxManifestDocument {
			return nil, nil, core.TooLarge
		}
		if !utf8.Valid(file.Body) || strings.ContainsRune(string(file.Body), '\x00') {
			return nil, nil, core.Malformed
		}
		names = append(names, file.Name)
		snapshots = append(snapshots, skillReferenceSnapshot{Name: file.Name, Digest: contentDigest(file.Body), Body: string(file.Body)})
	}
	return names, snapshots, nil
}

func offlineQualification(scriptsPresent bool, formatStatus string) ManifestQualification {
	return ManifestQualification{
		FormatStatus: formatStatus, ExecutionStatus: "unverified", ConnectionStatus: "unprovisioned",
		ScriptsExecuted: false, NetworkAccess: false, ScriptsPresent: scriptsPresent, Issues: []string{},
	}
}

func contentDigest(document []byte) string {
	sum := sha256.Sum256(document)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func packageIDFromSkillName(name string) (string, error) {
	if core.ValidID(name) {
		return strings.ToLower(name), nil
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-':
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if !core.ValidID(id) {
		return "", core.Malformed
	}
	return id, nil
}

func splitFrontMatter(document string) (string, string, error) {
	normalized := strings.ReplaceAll(document, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", core.Malformed
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		if strings.HasSuffix(rest, "\n---") {
			end = len(rest) - len("\n---")
			return rest[:end], "", nil
		}
		return "", "", core.Malformed
	}
	return rest[:end], strings.TrimPrefix(rest[end+len("\n---\n"):], "\n"), nil
}

func parseManifestFields(front string) (map[string]string, error) {
	fields := map[string]string{}
	for _, line := range strings.Split(front, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.ContainsAny(line, "\t&*!<>|") {
			return nil, core.Malformed
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, core.Malformed
		}
		key = strings.TrimSpace(key)
		if key == "allowed_tools" {
			key = "allowed-tools"
		}
		if !manifestKeyPattern(key) {
			return nil, core.Malformed
		}
		if _, exists := fields[key]; exists {
			return nil, core.Malformed
		}
		if isSecretManifestKey(key) {
			return nil, core.Denied
		}
		fields[key] = strings.TrimSpace(unquoteScalar(value))
	}
	return fields, nil
}

func allowedSkillKey(key string) bool {
	switch key {
	case "name", "description", "license", "allowed-tools":
		return true
	default:
		return false
	}
}

func isSecretManifestKey(key string) bool {
	switch strings.ToLower(key) {
	case "token", "secret", "password", "apikey", "api_key", "api-key", "authorization", "bearer", "credentials", "credential":
		return true
	default:
		return false
	}
}

func manifestKeyPattern(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for i, r := range key {
		if i == 0 && !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func unquoteScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'' {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseTokenList(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}, nil
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, core.Malformed
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return []string{}, nil
	}
	parts := strings.Split(inner, ",")
	if len(parts) > maxManifestListItems {
		return nil, core.TooLarge
	}
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		token := unquoteScalar(part)
		if token == "" || len(token) > maxManifestTokenBytes || strings.ContainsAny(token, " \t\r\n") {
			return nil, core.Malformed
		}
		for _, r := range token {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == ':' || r == '-') {
				return nil, core.Malformed
			}
		}
		items = append(items, token)
	}
	return items, nil
}

func validRevisionLabel(revision string) bool {
	revision = strings.TrimSpace(revision)
	if revision == "" || len(revision) > 64 {
		return false
	}
	for _, r := range revision {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == '+') {
			return false
		}
	}
	return true
}

func validSourceRef(source string) bool {
	source = strings.TrimSpace(source)
	return source != "" && len(source) <= 512 && !strings.ContainsAny(source, "\r\n\x00")
}
