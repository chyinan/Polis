package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"polis/db"
)

type desktopExecutableIdentity struct {
	Service                 string `json:"service"`
	ExecutableSHA256        string `json:"executable_sha256"`
	MigrationManifestSHA256 string `json:"migration_manifest_sha256"`
}

type polisSidecarInfo struct {
	Service                 string `json:"service"`
	MigrationManifestSHA256 string `json:"migration_manifest_sha256"`
}

func currentPolisSidecarInfo() polisSidecarInfo {
	return polisSidecarInfo{
		Service:                 "polis_backend",
		MigrationManifestSHA256: db.MigrationManifestSHA256(),
	}
}

func writeDesktopSidecarInfo(writer io.Writer) error {
	return json.NewEncoder(writer).Encode(currentPolisSidecarInfo())
}

func desktopIdentityHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		executable, err := os.Executable()
		if err != nil {
			writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "failed to inspect Polis executable identity"})
			return
		}
		identity, err := identifyDesktopExecutable(executable)
		if err != nil {
			writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "failed to inspect Polis executable identity"})
			return
		}
		writeJSON(response, http.StatusOK, identity)
	})
}

func identifyDesktopExecutable(path string) (desktopExecutableIdentity, error) {
	metadata, err := os.Lstat(path)
	if err != nil {
		return desktopExecutableIdentity{}, fmt.Errorf("inspect Polis executable: %w", err)
	}
	if !metadata.Mode().IsRegular() {
		return desktopExecutableIdentity{}, fmt.Errorf("Polis executable is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return desktopExecutableIdentity{}, fmt.Errorf("open Polis executable: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(metadata, opened) {
		return desktopExecutableIdentity{}, fmt.Errorf("Polis executable changed while it was inspected")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return desktopExecutableIdentity{}, fmt.Errorf("hash Polis executable: %w", err)
	}
	return desktopExecutableIdentity{
		Service:                 "polis_backend",
		ExecutableSHA256:        hex.EncodeToString(hasher.Sum(nil)),
		MigrationManifestSHA256: db.MigrationManifestSHA256(),
	}, nil
}
