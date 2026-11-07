// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"net/url"
	"strings"

	"polis/internal/core"
)

const mcpManifestFormat = "polis-mcp-definition@1"

type ParsedMCPDocument struct {
	ID               string
	Name             string
	Transport        string
	Endpoint         *string
	Command          *string
	Args             []string
	DescriptorDigest string
}

type mcpManifestDocument struct {
	Format    string   `json:"format"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Command   string   `json:"command,omitempty"`
	Endpoint  string   `json:"endpoint,omitempty"`
	Args      []string `json:"args"`
	Executed  bool     `json:"executed"`
	Connected bool     `json:"connected"`
}

func ParseMCPDocument(document []byte) (ParsedMCPDocument, error) {
	if len(document) == 0 {
		return ParsedMCPDocument{}, core.Malformed
	}
	if len(document) > maxManifestDocument {
		return ParsedMCPDocument{}, core.TooLarge
	}
	fields, err := parseManifestFields(string(document))
	if err != nil {
		return ParsedMCPDocument{}, err
	}
	for key := range fields {
		if !allowedMCPKey(key) {
			return ParsedMCPDocument{}, core.Malformed
		}
	}
	name := strings.TrimSpace(fields["name"])
	if name == "" || len(name) > maxSkillDisplayNameBytes || strings.ContainsAny(name, "\r\n") {
		return ParsedMCPDocument{}, core.Malformed
	}
	transport := fields["transport"]
	if transport != "stdio" && transport != "streamable_http" {
		return ParsedMCPDocument{}, core.Malformed
	}
	args, err := parseTokenList(fields["args"])
	if err != nil {
		return ParsedMCPDocument{}, err
	}
	command := strings.TrimSpace(fields["command"])
	endpoint := strings.TrimSpace(fields["endpoint"])
	if transport == "stdio" {
		if command == "" || len(command) > 512 || endpoint != "" || strings.ContainsAny(command, "\r\n") {
			return ParsedMCPDocument{}, core.Malformed
		}
	}
	if transport == "streamable_http" {
		if command != "" || !safeHTTPEndpoint(endpoint) {
			return ParsedMCPDocument{}, core.Malformed
		}
	}
	id, err := packageIDFromSkillName(name)
	if err != nil {
		return ParsedMCPDocument{}, err
	}
	canonical, err := json.Marshal(mcpManifestDocument{Format: mcpManifestFormat, Name: name, Transport: transport, Command: command, Endpoint: endpoint, Args: args, Executed: false, Connected: false})
	if err != nil {
		return ParsedMCPDocument{}, err
	}
	parsed := ParsedMCPDocument{ID: id, Name: name, Transport: transport, Args: args, DescriptorDigest: contentDigest(canonical)}
	if command != "" {
		parsed.Command = &command
	}
	if endpoint != "" {
		parsed.Endpoint = &endpoint
	}
	return parsed, nil
}

func allowedMCPKey(key string) bool {
	switch key {
	case "name", "transport", "command", "endpoint", "args":
		return true
	default:
		return false
	}
}

func safeHTTPEndpoint(raw string) bool {
	if raw == "" || len(raw) > 512 || strings.ContainsAny(raw, "\r\n ") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}
