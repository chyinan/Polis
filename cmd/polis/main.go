// pattern: Imperative Shell
// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"polis/db"
	"polis/internal/codex"
	"polis/internal/control"
	"polis/internal/desktop"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/workbench"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) >= 2 && os.Args[1] == "deterministic-worker" {
		select {}
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: polis migrate | serve | create COMPANY MISSION | start COMPANY MISSION | status COMPANY MISSION")
	}
	if os.Args[1] == "serve" {
		return serveWorkbench()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("POLIS_DSN")
	if os.Args[1] == "migrate" {
		return db.Migrate(ctx, dsn)
	}
	if len(os.Args) != 4 {
		return fmt.Errorf("company and mission required")
	}
	company, mission := os.Args[2], os.Args[3]
	if os.Args[1] == "status" {
		s, e := kernel.ReadSnapshot(ctx, dsn, company, mission)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(s)
	}
	root := os.Getenv("POLIS_BLOB_ROOT")
	if root == "" {
		return fmt.Errorf("POLIS_BLOB_ROOT required")
	}
	k, e := kernel.Open(ctx, dsn, root)
	if e != nil {
		return e
	}
	defer k.Close()
	scope := k.LocalScope(company)
	switch os.Args[1] {
	case "create":
		scope, e = k.TXCreateCompany(ctx, company)
		if e == nil {
			e = k.TXCreateMission(ctx, scope, mission)
		}
		if e != nil {
			return e
		}
	case "start":
		r, e := k.TXStartMission(ctx, scope, mission, "start-"+mission)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(r)
	default:
		return fmt.Errorf("unknown command")
	}
	return nil
}

func serveWorkbench() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, os.Getenv("POLIS_DSN"), os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		return fmt.Errorf("failed to start workbench read store: %w", err)
	}
	defer store.Close()
	root := os.Getenv("POLIS_BLOB_ROOT")
	if root == "" {
		return fmt.Errorf("failed to start command surface: POLIS_BLOB_ROOT required")
	}
	kernelRuntime, err := kernel.Open(ctx, os.Getenv("POLIS_DSN"), root)
	if err != nil {
		return fmt.Errorf("failed to start command surface: %w", err)
	}
	defer kernelRuntime.Close()
	workerAdapter, err := buildWorkerAdapter(kernelRuntime)
	if err != nil {
		return fmt.Errorf("failed to select worker adapter: %w", err)
	}
	if err = workerAdapter.Readiness(ctx); err != nil {
		return fmt.Errorf("worker adapter readiness failed: %w", err)
	}
	modelCatalog := provider.NewCodexModelCatalogReader(codexRuntimeConfigFromEnvironment())
	commandService := control.NewService(kernelRuntime, workerAdapter, modelCatalog)
	defer commandService.Close()
	address := os.Getenv("POLIS_WORKBENCH_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	if err := desktop.ValidateServerBinding(address, os.Getenv("POLIS_DESKTOP_SESSION_TOKEN")); err != nil {
		return err
	}
	shutdownRequested := make(chan struct{}, 1)
	router := http.NewServeMux()
	router.Handle("/api/workbench/", workbench.NewHandler(store, commandService))
	router.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"service": "polis_backend", "status": "ready", "version": "r0.7"})
	})
	router.HandleFunc("/api/desktop/active-work", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		active, readErr := store.HasActiveWork(request.Context())
		if readErr != nil {
			writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "failed to inspect active work"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]bool{"active": active})
	})
	router.HandleFunc("/api/desktop/shutdown", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
		writeJSON(response, http.StatusAccepted, map[string]bool{"accepted": true})
	})
	server := &http.Server{
		Addr:              address,
		Handler:           desktop.Middleware(os.Getenv("POLIS_DESKTOP_SESSION_TOKEN"), router),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("workbench server failed: %w", err)
	case <-signals:
		return shutdownWorkbench(server)
	case <-shutdownRequested:
		return shutdownWorkbench(server)
	}
}

func shutdownWorkbench(server *http.Server) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	return server.Shutdown(shutdownCtx)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func buildWorkerAdapter(runtime *kernel.Kernel) (control.WorkerAdapter, error) {
	mode := os.Getenv("POLIS_WORKER_MODE")
	if mode == "" {
		mode = "deterministic"
	}
	switch mode {
	case "deterministic":
		return control.NewDeterministicWorkerAdapter(runtime), nil
	case "real":
		transport := os.Getenv("POLIS_PROVIDER_TRANSPORT")
		var providerRuntime provider.Runtime
		if transport == "fake" {
			providerRuntime = provider.NewFakeRuntime(provider.FakeRuntimeConfig{Model: os.Getenv("POLIS_PROVIDER_MODEL"), Effort: os.Getenv("POLIS_PROVIDER_EFFORT"), Profile: os.Getenv("POLIS_PROVIDER_PROFILE"), Purpose: os.Getenv("POLIS_PROVIDER_PURPOSE"), ToolCallLimit: envIntOrDefault("POLIS_PROVIDER_TOOL_CALL_LIMIT", 16), TurnDelay: 100 * time.Millisecond})
		} else if transport == "codex" {
			config := codexRuntimeConfigFromEnvironment()
			config.ToolSurface = provider.ProductToolSurface()
			bound, err := provider.BindCodexRuntimeConfig(config)
			if err != nil {
				return nil, fmt.Errorf("provider runtime binding failed: %w", err)
			}
			providerRuntime = provider.NewCodexRuntime(bound)
		} else {
			return nil, fmt.Errorf("real worker mode requires POLIS_PROVIDER_TRANSPORT=fake or codex")
		}
		return control.NewRealProviderWorkerAdapter(runtime, providerRuntime)
	default:
		return nil, fmt.Errorf("unsupported POLIS_WORKER_MODE %q", mode)
	}
}

func codexRuntimeConfigFromEnvironment() provider.CodexRuntimeConfig {
	return provider.CodexRuntimeConfig{
		Binary: os.Getenv("POLIS_PROVIDER_BINARY"), HelperBinary: os.Getenv("POLIS_PROVIDER_HELPER_BINARY"),
		RuntimeManifestPath: os.Getenv("POLIS_PROVIDER_RUNTIME_MANIFEST"), BinarySHA256: os.Getenv("POLIS_PROVIDER_BINARY_SHA256"),
		HelperSHA256: os.Getenv("POLIS_PROVIDER_HELPER_SHA256"), AuthFile: os.Getenv("POLIS_PROVIDER_AUTH_FILE"),
		Root: os.Getenv("POLIS_PROVIDER_ROOT"), EvidenceRoot: os.Getenv("POLIS_PROVIDER_EVIDENCE"),
		Model: os.Getenv("POLIS_PROVIDER_MODEL"), Effort: os.Getenv("POLIS_PROVIDER_EFFORT"),
		ExpectedVersion: os.Getenv("POLIS_PROVIDER_EXPECTED_VERSION"), TransportPolicy: codex.DefaultTransportPolicy(),
		ToolSurface: provider.ProductToolSurface(), Purpose: os.Getenv("POLIS_PROVIDER_PURPOSE"),
		ExactSurfaceExecutionFingerprint: os.Getenv("POLIS_PROVIDER_EXACT_SURFACE_EXECUTION_FINGERPRINT"),
		ProductProviderL2Fingerprint:     os.Getenv("POLIS_PROVIDER_L2_FINGERPRINT"),
		ExecutionEnvelope:                os.Getenv("POLIS_PROVIDER_EXECUTION_ENVELOPE"),
		ToolSurfaceQualification:         os.Getenv("POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION"),
		AllowancePath:                    os.Getenv("POLIS_PROVIDER_ALLOWANCE_PATH"),
		MediumLimit:                      1, HighLimit: 0, ToolCallLimit: envIntOrDefault("POLIS_PROVIDER_TOOL_CALL_LIMIT", 16),
	}
}

func envIntOrDefault(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
