// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"polis/internal/browser"
	"polis/internal/kernel"
	"polis/internal/research"
)

type PlaywrightOperationAdapter struct {
	Kernel           *kernel.Kernel
	Runner           browser.PlaywrightRunner
	TimeoutMS        int
	MaxRequests      int
	MaxResponseBytes int64
}

func (adapter *PlaywrightOperationAdapter) ExecuteBrowserRun(ctx context.Context, binding kernel.Binding, request kernel.BrowserRunRequest, record kernel.BrowserRunRecord) (kernel.BrowserRunSuccessInput, error) {
	if adapter == nil || adapter.Kernel == nil {
		return kernel.BrowserRunSuccessInput{}, errors.New("BrowserRun adapter is not configured")
	}
	timeoutMS := adapter.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 30000
	}
	maxRequests := adapter.MaxRequests
	if maxRequests == 0 {
		maxRequests = 128
	}
	maxResponseBytes := adapter.MaxResponseBytes
	if maxResponseBytes == 0 {
		maxResponseBytes = 8 << 20
	}
	outcome, err := adapter.Runner.Run(ctx, browser.BrowserRunPlan{TargetOrigin: request.TargetOrigin, TimeoutMS: timeoutMS, MaxRequests: maxRequests, MaxResponseBytes: maxResponseBytes})
	if err != nil {
		return kernel.BrowserRunSuccessInput{}, err
	}
	payload, err := json.Marshal(outcome)
	if err != nil {
		return kernel.BrowserRunSuccessInput{}, fmt.Errorf("encode BrowserRun page snapshot: %w", err)
	}
	artifactRequestID := operationAdapterRequestID("browser-evidence", record.RunID, hexDigest(payload))
	artifact, err := adapter.Kernel.TXStoreOperationEvidenceArtifact(ctx, binding, kernel.OperationEvidenceArtifactInput{OperationID: record.RunID, Kind: "page_snapshot", Content: payload, RequestID: artifactRequestID})
	if err != nil {
		return kernel.BrowserRunSuccessInput{}, err
	}
	manifest := kernel.OperationEvidenceManifest{SchemaVersion: kernel.OperationEvidenceManifestSchema, CompanyID: record.CompanyID, MissionID: record.MissionID, TaskID: record.TaskID, OperationID: record.RunID, Artifacts: []kernel.OperationEvidenceArtifactRef{artifact}}
	return kernel.BrowserRunSuccessInput{RunID: record.RunID, Manifest: manifest, RequestID: operationAdapterRequestID("browser-success", record.RunID, artifact.Digest)}, nil
}

type ResearchFetchOperationAdapter struct {
	Kernel    *kernel.Kernel
	Fetcher   *research.Fetcher
	TimeoutMS int
	MaxBytes  int64
}

type ResearchSearchOperationAdapter struct {
	Kernel      *kernel.Kernel
	Backend     research.SearchBackend
	HTTPClient  research.HTTPDoer
	Credentials research.CredentialProvider
}

type ResearchOperationAdapterMux struct {
	Fetch  *ResearchFetchOperationAdapter
	Search *ResearchSearchOperationAdapter
}

func (adapter *ResearchOperationAdapterMux) ExecuteResearchOperation(ctx context.Context, binding kernel.Binding, request kernel.ResearchOperationRequest, record kernel.ResearchOperationRecord) (kernel.ResearchOperationSuccessInput, error) {
	if adapter == nil {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research operation adapter is not configured")
	}
	if request.Kind == kernel.ResearchOperationKindFetch && adapter.Fetch != nil {
		return adapter.Fetch.ExecuteResearchOperation(ctx, binding, request, record)
	}
	if request.Kind == kernel.ResearchOperationKindSearch && adapter.Search != nil {
		return adapter.Search.ExecuteResearchOperation(ctx, binding, request, record)
	}
	return kernel.ResearchOperationSuccessInput{}, errors.New("research operation adapter does not support this operation")
}

func (adapter *ResearchSearchOperationAdapter) ExecuteResearchOperation(ctx context.Context, binding kernel.Binding, request kernel.ResearchOperationRequest, record kernel.ResearchOperationRecord) (kernel.ResearchOperationSuccessInput, error) {
	if adapter == nil || adapter.Kernel == nil {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research search adapter is not configured")
	}
	if request.Kind != kernel.ResearchOperationKindSearch || record.SourceID == "" {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research search adapter requires a bound search source")
	}
	source, err := adapter.Kernel.GetAuthorizedResearchSource(ctx, record.CompanyID, record.MissionID, record.SourceID)
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	backend := adapter.Backend
	if backend == nil && adapter.HTTPClient != nil && source.SearchEndpoint != "" {
		backend = &research.HTTPJSONSearchBackend{Client: adapter.HTTPClient, Origin: source.Origin, Endpoint: source.SearchEndpoint, CredentialRef: source.SearchCredentialRef, Credentials: adapter.Credentials}
	}
	if backend == nil {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research search backend is not configured")
	}
	candidates, err := backend.Search(ctx, request.Query)
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	results, err := research.NormalizeSearchResults(source.Origin, candidates)
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	payload, err := json.Marshal(struct {
		Results []research.SearchResult `json:"results"`
	}{results})
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	artifact, err := adapter.Kernel.TXStoreOperationEvidenceArtifact(ctx, binding, kernel.OperationEvidenceArtifactInput{OperationID: record.OperationID, Kind: "search_result", Content: payload, RequestID: operationAdapterRequestID("research-search-evidence", record.OperationID, hexDigest(payload))})
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	manifest := kernel.OperationEvidenceManifest{SchemaVersion: kernel.OperationEvidenceManifestSchema, CompanyID: record.CompanyID, MissionID: record.MissionID, TaskID: record.TaskID, OperationID: record.OperationID, Artifacts: []kernel.OperationEvidenceArtifactRef{artifact}}
	manifestBytes, err := kernel.CanonicalOperationEvidenceJSON(mustJSON(manifest))
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	digest := sha256.Sum256(manifestBytes)
	envelope := kernel.ResearchOperationEvidenceEnvelope{EvidenceManifestSHA256: hex.EncodeToString(digest[:]), EvidenceManifest: manifest, Payload: payload}
	return kernel.ResearchOperationSuccessInput{OperationID: record.OperationID, Envelope: envelope, RequestID: operationAdapterRequestID("research-search-success", record.OperationID, artifact.Digest)}, nil
}

func (adapter *ResearchFetchOperationAdapter) ExecuteResearchOperation(ctx context.Context, binding kernel.Binding, request kernel.ResearchOperationRequest, record kernel.ResearchOperationRecord) (kernel.ResearchOperationSuccessInput, error) {
	if adapter == nil || adapter.Kernel == nil || adapter.Fetcher == nil {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research fetch adapter is not configured")
	}
	if request.Kind != kernel.ResearchOperationKindFetch || record.SourceID == "" {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research fetch adapter requires a bound fetch source")
	}
	source, err := adapter.Kernel.GetAuthorizedResearchSource(ctx, record.CompanyID, record.MissionID, record.SourceID)
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	timeoutMS := adapter.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 30000
	}
	maxBytes := adapter.MaxBytes
	if maxBytes == 0 {
		maxBytes = 8 << 20
	}
	outcome, err := adapter.Fetcher.Fetch(ctx, research.FetchPlan{Origin: source.Origin, TargetURL: record.TargetURL, TimeoutMS: timeoutMS, MaxBytes: maxBytes})
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	if len(outcome.Body) == 0 {
		return kernel.ResearchOperationSuccessInput{}, errors.New("research fetch returned an empty evidence body")
	}
	artifactRequestID := operationAdapterRequestID("research-evidence", record.OperationID, outcome.BodySHA256)
	artifact, err := adapter.Kernel.TXStoreOperationEvidenceArtifact(ctx, binding, kernel.OperationEvidenceArtifactInput{OperationID: record.OperationID, Kind: "page_snapshot", Content: outcome.Body, RequestID: artifactRequestID})
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	manifest := kernel.OperationEvidenceManifest{SchemaVersion: kernel.OperationEvidenceManifestSchema, CompanyID: record.CompanyID, MissionID: record.MissionID, TaskID: record.TaskID, OperationID: record.OperationID, Artifacts: []kernel.OperationEvidenceArtifactRef{artifact}}
	manifestBytes, err := kernel.CanonicalOperationEvidenceJSON(mustJSON(manifest))
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	payload, err := json.Marshal(struct {
		FinalURL    string `json:"finalUrl"`
		StatusCode  int    `json:"statusCode"`
		ContentType string `json:"contentType"`
		BodySHA256  string `json:"bodySha256"`
	}{outcome.FinalURL, outcome.StatusCode, outcome.ContentType, outcome.BodySHA256})
	if err != nil {
		return kernel.ResearchOperationSuccessInput{}, err
	}
	digest := sha256.Sum256(manifestBytes)
	envelope := kernel.ResearchOperationEvidenceEnvelope{EvidenceManifestSHA256: hex.EncodeToString(digest[:]), EvidenceManifest: manifest, Payload: payload}
	return kernel.ResearchOperationSuccessInput{OperationID: record.OperationID, Envelope: envelope, RequestID: operationAdapterRequestID("research-success", record.OperationID, artifact.Digest)}, nil
}

func operationAdapterRequestID(prefix, operationID, digest string) string {
	digestBytes := sha256.Sum256([]byte(prefix + "\n" + operationID + "\n" + digest))
	return prefix + "-" + hex.EncodeToString(digestBytes[:])[:48]
}

func hexDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func mustJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}
