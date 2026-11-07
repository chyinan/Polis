// pattern: Imperative Shell
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

type CodexReasoningEffortOption struct {
	ReasoningEffort string `json:"reasoningEffort"`
	Description     string `json:"description"`
}

type CodexModelOption struct {
	Model                     string                       `json:"model"`
	DisplayName               string                       `json:"displayName"`
	Description               string                       `json:"description"`
	IsDefault                 bool                         `json:"isDefault"`
	DefaultReasoningEffort    string                       `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []CodexReasoningEffortOption `json:"supportedReasoningEfforts"`
}

type CodexModelCatalog struct {
	Models []CodexModelOption `json:"models"`
}

type ModelCatalogProvider interface {
	ListModels(context.Context) (CodexModelCatalog, error)
}

type CodexModelCatalogReader struct {
	config CodexRuntimeConfig
}

func NewCodexModelCatalogReader(config CodexRuntimeConfig) *CodexModelCatalogReader {
	return &CodexModelCatalogReader{config: config}
}

func (r *CodexModelCatalogReader) ListModels(ctx context.Context) (catalog CodexModelCatalog, err error) {
	if r == nil {
		return CodexModelCatalog{}, errors.New("Codex model catalog is not configured")
	}
	if err := ctx.Err(); err != nil {
		return CodexModelCatalog{}, err
	}
	root, err := os.MkdirTemp("", "polis-codex-model-catalog-")
	if err != nil {
		return CodexModelCatalog{}, fmt.Errorf("failed to create temporary Codex catalog workspace: %w", err)
	}
	safeToRemoveRoot := true
	defer func() {
		if !safeToRemoveRoot {
			return
		}
		if cleanupErr := os.RemoveAll(root); cleanupErr != nil && err == nil {
			catalog = CodexModelCatalog{}
			err = fmt.Errorf("failed to clean temporary Codex catalog workspace: %w", cleanupErr)
		}
	}()
	config, err := prepareCodexModelCatalogRuntime(ctx, r.config, filepath.Join(root, "controlled-runtime"))
	if err != nil {
		return CodexModelCatalog{}, err
	}
	if config.TransportPolicy.InitializeTimeout == 0 {
		config.TransportPolicy = codex.DefaultTransportPolicy()
	}
	config.Root = filepath.Join(root, "runtime")
	config.EvidenceRoot = filepath.Join(root, "evidence")
	runtime := NewCodexRuntime(config)
	sessionID := "catalog-" + filepath.Base(root)
	session, err := runtime.startModelCatalogSession(sessionID, "cmd/polis settings -> Codex model/list")
	if err != nil {
		return CodexModelCatalog{}, fmt.Errorf("failed to start Codex model catalog process: %w", err)
	}
	safeToRemoveRoot = false
	defer func() {
		if _, stopErr := session.Stop(context.Background()); stopErr != nil {
			catalog = CodexModelCatalog{}
			if err == nil {
				err = fmt.Errorf("failed to stop Codex model catalog process; temporary workspace retained: %w", stopErr)
			} else {
				err = fmt.Errorf("%v; failed to stop Codex model catalog process and retained its temporary workspace: %w", err, stopErr)
			}
			return
		}
		safeToRemoveRoot = true
	}()

	listCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err = session.Initialize(listCtx, config.TransportPolicy); err != nil {
		return CodexModelCatalog{}, fmt.Errorf("failed to initialize Codex model catalog: %w", err)
	}
	catalog, err = listCodexModels(listCtx, session.client)
	if err != nil {
		return CodexModelCatalog{}, err
	}
	return catalog, nil
}

func prepareCodexModelCatalogRuntime(ctx context.Context, config CodexRuntimeConfig, stagingRoot string) (CodexRuntimeConfig, error) {
	if config.AuthFile == "" {
		authFile, err := defaultCodexAuthFile()
		if err != nil {
			return CodexRuntimeConfig{}, err
		}
		config.AuthFile = authFile
	}
	if _, err := os.Stat(config.AuthFile); err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("Codex authentication file is unavailable: %w", err)
	}
	if config.RuntimeManifestPath != "" {
		bound, err := BindCodexRuntimeConfig(config)
		if err != nil {
			return CodexRuntimeConfig{}, fmt.Errorf("Codex runtime identity is unavailable: %w", err)
		}
		return bound, nil
	}
	if config.Binary == "" {
		binary, helper, err := discoverInstalledCodex()
		if err != nil {
			return CodexRuntimeConfig{}, err
		}
		config.Binary = binary
		config.HelperBinary = helper
	}
	if adjacentManifest := runtimeManifestAdjacentTo(config.Binary); adjacentManifest != "" {
		if _, err := os.Stat(adjacentManifest); err == nil {
			config.RuntimeManifestPath = adjacentManifest
			bound, bindErr := BindCodexRuntimeConfig(config)
			if bindErr != nil {
				return CodexRuntimeConfig{}, fmt.Errorf("Codex runtime identity is unavailable: %w", bindErr)
			}
			return bound, nil
		}
	}
	if runtime.GOOS != "windows" {
		return CodexRuntimeConfig{}, errors.New("Codex model discovery requires a controlled Windows runtime manifest")
	}
	if config.HelperBinary == "" {
		config.HelperBinary = runner.NativeCodeModeHostPath(config.Binary)
	}
	actualVersion, err := readCodexBinaryVersion(ctx, config.Binary)
	if err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("failed to identify the installed Codex runtime: %w", err)
	}
	if config.ExpectedVersion != "" && normalizeProviderRuntimeVersion(config.ExpectedVersion) != normalizeProviderRuntimeVersion(actualVersion) {
		return CodexRuntimeConfig{}, fmt.Errorf("configured Codex version %q does not match installed version %q", config.ExpectedVersion, actualVersion)
	}
	manifest, err := runner.StageWindowsRuntimeArtifact(config.Binary, config.HelperBinary, stagingRoot, actualVersion, "", "", "codex_model_catalog_only")
	if err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("failed to stage the installed Codex runtime for model discovery: %w", err)
	}
	config.RuntimeManifestPath = manifest.ManifestPath
	config.Binary = manifest.CodexBinaryStagedPath
	config.HelperBinary = manifest.CodeModeHostStagedPath
	config.BinarySHA256 = manifest.CodexBinarySHA256
	config.HelperSHA256 = manifest.CodeModeHostSHA256
	config.ExpectedVersion = manifest.CodexVersion
	bound, err := BindCodexRuntimeConfig(config)
	if err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("staged Codex runtime identity is unavailable: %w", err)
	}
	return bound, nil
}

func discoverInstalledCodex() (string, string, error) {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return "", "", errors.New("Codex CLI was not configured and LOCALAPPDATA is unavailable")
	}
	installRoot := filepath.Join(localAppData, "OpenAI", "Codex", "bin")
	entries, err := os.ReadDir(installRoot)
	if err != nil {
		return "", "", fmt.Errorf("Codex CLI was not configured and the OpenAI Codex install directory is unavailable: %w", err)
	}
	type codexInstall struct {
		binary  string
		helper  string
		updated time.Time
	}
	installs := make([]codexInstall, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		installDir := filepath.Join(installRoot, entry.Name())
		binary := filepath.Join(installDir, "codex.exe")
		helper := filepath.Join(installDir, "codex-code-mode-host.exe")
		binaryInfo, binaryErr := os.Stat(binary)
		if binaryErr != nil || !binaryInfo.Mode().IsRegular() {
			continue
		}
		if helperInfo, helperErr := os.Stat(helper); helperErr != nil || !helperInfo.Mode().IsRegular() {
			continue
		}
		installs = append(installs, codexInstall{binary: binary, helper: helper, updated: binaryInfo.ModTime()})
	}
	if len(installs) == 0 {
		return "", "", errors.New("Codex CLI and its code-mode host were not found in the OpenAI Codex installation directory")
	}
	sort.Slice(installs, func(left, right int) bool { return installs[left].updated.After(installs[right].updated) })
	return installs[0].binary, installs[0].helper, nil
}

func defaultCodexAuthFile() (string, error) {
	codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if codexHome == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to locate the current user's Codex home: %w", err)
		}
		codexHome = filepath.Join(userHome, ".codex")
	}
	authFile := filepath.Join(codexHome, "auth.json")
	if _, err := os.Stat(authFile); err != nil {
		return "", fmt.Errorf("Codex authentication is not configured at CODEX_HOME or the default user Codex home: %w", err)
	}
	return authFile, nil
}

type codexModelListPage struct {
	Data       []codexModelListItem `json:"data"`
	NextCursor *string              `json:"nextCursor"`
}

type codexModelListItem struct {
	ID                        string                       `json:"id"`
	Model                     string                       `json:"model"`
	DisplayName               string                       `json:"displayName"`
	Description               string                       `json:"description"`
	Hidden                    bool                         `json:"hidden"`
	IsDefault                 bool                         `json:"isDefault"`
	DefaultReasoningEffort    string                       `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []CodexReasoningEffortOption `json:"supportedReasoningEfforts"`
}

func listCodexModels(ctx context.Context, client *codex.Client) (CodexModelCatalog, error) {
	const pageLimit = 100
	const maxPages = 100

	models := make([]CodexModelOption, 0)
	seenModels := make(map[string]struct{})
	seenCursors := make(map[string]struct{})
	var cursor string
	for pageNumber := 0; pageNumber < maxPages; pageNumber++ {
		params := map[string]any{"limit": pageLimit, "includeHidden": false}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := client.Request(ctx, "model/list", params)
		if err != nil {
			return CodexModelCatalog{}, fmt.Errorf("failed to query Codex model/list: %w", err)
		}
		var page codexModelListPage
		if err = json.Unmarshal(raw, &page); err != nil {
			return CodexModelCatalog{}, fmt.Errorf("failed to decode Codex model/list response: %w", err)
		}
		for _, item := range page.Data {
			modelID := strings.TrimSpace(item.Model)
			if modelID == "" {
				modelID = strings.TrimSpace(item.ID)
			}
			if item.Hidden || modelID == "" || strings.TrimSpace(item.DisplayName) == "" {
				continue
			}
			if _, exists := seenModels[modelID]; exists {
				continue
			}
			seenModels[modelID] = struct{}{}
			efforts := make([]CodexReasoningEffortOption, 0, len(item.SupportedReasoningEfforts))
			for _, effort := range item.SupportedReasoningEfforts {
				if value := strings.TrimSpace(effort.ReasoningEffort); value != "" {
					efforts = append(efforts, CodexReasoningEffortOption{ReasoningEffort: value, Description: strings.TrimSpace(effort.Description)})
				}
			}
			models = append(models, CodexModelOption{
				Model:                     modelID,
				DisplayName:               strings.TrimSpace(item.DisplayName),
				Description:               strings.TrimSpace(item.Description),
				IsDefault:                 item.IsDefault,
				DefaultReasoningEffort:    strings.TrimSpace(item.DefaultReasoningEffort),
				SupportedReasoningEfforts: efforts,
			})
		}
		if page.NextCursor == nil || strings.TrimSpace(*page.NextCursor) == "" {
			return CodexModelCatalog{Models: models}, nil
		}
		nextCursor := strings.TrimSpace(*page.NextCursor)
		if _, exists := seenCursors[nextCursor]; exists {
			return CodexModelCatalog{}, errors.New("Codex model/list repeated a pagination cursor")
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}
	return CodexModelCatalog{}, errors.New("Codex model/list exceeded the pagination limit")
}
