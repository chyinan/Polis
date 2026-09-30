// pattern: Imperative Shell
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"os"
	"time"

	"polis/internal/desktop"
	"polis/internal/workbench"
)

const (
	fixtureCompanyID  = "browser-company-01"
	fixtureTaskID     = "task-browser-01"
	fixtureArtifactID = "artifact-browser-01"
	fixtureToken      = "slice26-browser-session"
)

func main() {
	manifest, manifestBytes, manifestDigest, artifactBytes := fixtureManifest()
	archiveBytes, err := fixtureArchive(manifestBytes, manifestDigest, artifactBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to build browser fixture package:", err)
		os.Exit(1)
	}
	packageHash := sha256.Sum256(archiveBytes)
	mux := http.NewServeMux()
	shutdownRequested := make(chan struct{}, 1)
	mux.HandleFunc("/__fixture/shutdown", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeFixtureJSON(response, http.StatusAccepted, map[string]bool{"accepted": true})
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	})
	mux.HandleFunc("/api/workbench/companies/"+fixtureCompanyID+"/overview", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeFixtureJSON(response, http.StatusOK, fixtureOverview(manifest))
	})
	mux.HandleFunc("/api/workbench/companies/"+fixtureCompanyID+"/tasks/"+fixtureTaskID+"/workspace", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeFixtureJSON(response, http.StatusOK, map[string]any{
			"taskId": fixtureTaskID, "revision": "1", "digest": string(repeatByte('b', 64)),
			"files": []any{}, "changedFiles": []string{}, "checkpoints": []any{},
		})
	})
	mux.HandleFunc("/api/workbench/companies/"+fixtureCompanyID+"/artifacts/"+fixtureArtifactID+"/detail", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeFixtureJSON(response, http.StatusOK, map[string]any{
			"artifactId": fixtureArtifactID, "taskId": fixtureTaskID, "digest": manifest.Content.SHA256,
			"bytes": manifest.Content.ByteSize, "state": "ready", "verdict": "passed",
			"content": "POLIS_BROWSER_DELIVERY_ARTIFACT_SENTINEL\n", "contentAvailable": true,
		})
	})
	mux.HandleFunc("/api/workbench/companies/"+fixtureCompanyID+"/artifacts/"+fixtureArtifactID+"/manifest", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeFixtureJSON(response, http.StatusOK, workbench.ArtifactDeliveryManifestResponse{Manifest: manifest, ManifestSHA256: manifestDigest})
	})
	mux.HandleFunc("/api/workbench/companies/"+fixtureCompanyID+"/artifacts/"+fixtureArtifactID+"/download", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Content-Type", "application/zip")
		response.Header().Set("Content-Disposition", `attachment; filename="polis-delivery.zip"`)
		response.Header().Set("Content-Length", fmt.Sprint(len(archiveBytes)))
		response.Header().Set("X-Content-SHA256", hex.EncodeToString(packageHash[:]))
		response.Header().Set("X-Polis-Manifest-SHA256", manifestDigest)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(archiveBytes)
	})
	address := os.Getenv("POLIS_WORKBENCH_FIXTURE_ADDR")
	if address == "" {
		address = "127.0.0.1:18084"
	}
	if address != "127.0.0.1:18084" {
		fmt.Fprintln(os.Stderr, "POLIS_WORKBENCH_FIXTURE_ADDR must use the local browser fixture address 127.0.0.1:18084")
		os.Exit(1)
	}
	token := os.Getenv("POLIS_DESKTOP_SESSION_TOKEN")
	if token != fixtureToken {
		fmt.Fprintln(os.Stderr, "POLIS_DESKTOP_SESSION_TOKEN must use the local browser fixture token")
		os.Exit(1)
	}
	server := &http.Server{Addr: address, Handler: desktop.Middleware(token, mux), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() {
		<-shutdownRequested
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "artifact browser fixture server failed:", err)
		os.Exit(1)
	}
}

func fixtureManifest() (workbench.ArtifactDeliveryManifestView, []byte, string, []byte) {
	artifactBytes := []byte("POLIS_BROWSER_DELIVERY_ARTIFACT_SENTINEL\n")
	artifactHash := sha256.Sum256(artifactBytes)
	manifest := workbench.ArtifactDeliveryManifestView{
		SchemaVersion: workbench.ArtifactDeliveryManifestSchema,
		CompanyID:     fixtureCompanyID,
		ArtifactID:    fixtureArtifactID,
		TaskID:        fixtureTaskID,
		Content: workbench.ArtifactDeliveryContent{
			FileName: "artifact.bin", ContentType: "application/octet-stream",
			ByteSize: fmt.Sprint(len(artifactBytes)), SHA256: hex.EncodeToString(artifactHash[:]),
		},
		State: "ready", Verdict: "passed",
		Qualification: workbench.ArtifactDeliveryQualification{
			CheckpointID: "checkpoint-browser-01", ValidationReceiptID: "receipt-browser-01",
			TaskValidationBindingDigest: string(repeatByte('a', 64)), WorkspaceDigest: string(repeatByte('b', 64)),
			WorkspaceRevision: "1", RunnerRevision: "runner-browser@1",
		},
		CreatedAt: "2026-09-25T00:00:00Z",
	}
	manifestBytes, _ := json.Marshal(manifest)
	manifestHash := sha256.Sum256(manifestBytes)
	return manifest, manifestBytes, hex.EncodeToString(manifestHash[:]), artifactBytes
}

func fixtureArchive(manifestBytes []byte, manifestDigest string, artifactBytes []byte) ([]byte, error) {
	artifactHash := sha256.Sum256(artifactBytes)
	checksums := []byte(fmt.Sprintf("%s  manifest.json\n%s  artifact.bin\n", manifestDigest, hex.EncodeToString(artifactHash[:])))
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"artifact.bin", artifactBytes}, {"manifest.json", manifestBytes}, {"SHA256SUMS", checksums}} {
		header := &zip.FileHeader{
			Name: entry.name, Method: zip.Store,
			UncompressedSize64: uint64(len(entry.data)), CompressedSize64: uint64(len(entry.data)),
		}
		header.Flags = 0
		header.CRC32 = crc32.ChecksumIEEE(entry.data)
		header.SetMode(0o644)
		header.SetModTime(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC))
		file, err := writer.CreateRaw(header)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write(entry.data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func fixtureOverview(manifest workbench.ArtifactDeliveryManifestView) map[string]any {
	return map[string]any{
		"meta": map[string]any{
			"schemaVersion": 2, "companyId": fixtureCompanyID, "entityRevision": "1", "snapshotCursor": "cursor-browser-1",
			"observedAt": "2026-09-25T00:00:00Z", "dataMode": "real", "freshness": "fresh",
			"sourceLabel": "local browser delivery fixture", "recoveryState": "operational",
		},
		"company": map[string]any{"companyId": fixtureCompanyID, "name": "Delivery fixture", "description": "Local browser smoke fixture."},
		"mission": map[string]any{
			"missionId": "mission-browser-01", "title": "Artifact browser download", "goal": "Verify authenticated ZIP delivery.",
			"state": "active", "contract": "fixture@1", "acceptanceContract": nil, "currentContractRevision": nil,
			"nextMilestone": "none", "verifiedMilestones": "0", "milestoneTotal": "0", "milestones": []any{},
		},
		"team":      map[string]any{"total": "1", "working": "0", "sleeping": "0", "waiting": "0", "stopped": "1"},
		"employees": []any{},
		"tasks": []any{map[string]any{
			"taskId": fixtureTaskID, "title": "Download verification", "kind": "compat", "state": "candidate",
			"ownerEmployeeId": "emp-backend", "generation": "1", "contractRevisionId": nil,
			"workspaceRevision": "1", "acceptance": "passed", "dependencyLabel": nil,
		}},
		"obligations": []any{},
		"artifacts": []any{map[string]any{
			"artifactId": manifest.ArtifactID, "taskId": manifest.TaskID, "authorEmployeeId": "emp-backend",
			"digest": manifest.Content.SHA256, "bytes": manifest.Content.ByteSize, "state": manifest.State,
			"verdict": manifest.Verdict, "contractRevisionId": nil, "qualificationId": nil, "checkpointId": manifest.Qualification.CheckpointID,
		}},
		"checkpoints": []any{map[string]any{
			"checkpointId": manifest.Qualification.CheckpointID, "taskId": manifest.TaskID, "employeeId": "emp-backend",
			"sessionId": "session-browser-01", "sessionState": "stopped", "sessionEpoch": "1", "kind": "qualified",
			"state": "current", "qualificationState": "qualified", "workspaceRevision": "1",
			"workspaceDigest": string(repeatByte('b', 64)), "artifactId": manifest.ArtifactID,
		}},
		"resources": map[string]any{
			"toolCallsUsed": "0", "toolCallsLimit": "10", "toolBudgetQuality": "reported",
			"moneyQuality": "unavailable", "moneyAmount": nil, "currency": nil,
			"asOf": "2026-09-25T00:00:00Z", "note": "local browser fixture",
		},
		"attention": []any{}, "recentActivity": []any{},
	}
}

func writeFixtureJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func repeatByte(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}
