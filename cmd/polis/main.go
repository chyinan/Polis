// pattern: Imperative Shell
// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"polis/db"
	"polis/internal/codex"
	"polis/internal/control"
	"polis/internal/desktop"
	"polis/internal/environment"
	githubfeedback "polis/internal/feedback/github"
	"polis/internal/installationauth"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/recovery"
	"polis/internal/runner"
	"polis/internal/workbench"
	"runtime"
	"strconv"
	"strings"
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
		return fmt.Errorf("usage: polis migrate | migrate-to VERSION | migration-status | migration-attempt-status | sidecar-info | serve | cas-collect [flags] | recovery-backup OUTPUT_DIRECTORY | recovery-backup-verify PACKAGE_DIRECTORY | recovery-backup-restore PACKAGE_DIRECTORY BLOB_ROOT | recovery-generation-verify PACKAGE_DIRECTORY BLOB_ROOT | linux-node-toolchain-sha256 | import-git COMPANY MISSION REPOSITORY_PATH COMMIT_ID REQUEST_ID [INPUT_ID] | create COMPANY MISSION | start COMPANY MISSION | status COMPANY MISSION")
	}
	if os.Args[1] == "sidecar-info" {
		if len(os.Args) != 2 {
			return fmt.Errorf("usage: polis sidecar-info")
		}
		return writeDesktopSidecarInfo(os.Stdout)
	}
	if os.Args[1] == "cas-collect" {
		return runCASCollection()
	}
	if os.Args[1] == "recovery-backup" {
		return createRecoveryBackup()
	}
	if os.Args[1] == "recovery-backup-verify" {
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: polis recovery-backup-verify PACKAGE_DIRECTORY")
		}
		report, err := recovery.VerifyRecoveryBackupPackage(os.Args[2])
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	if os.Args[1] == "recovery-backup-restore" {
		return restoreRecoveryBackup()
	}
	if os.Args[1] == "recovery-generation-verify" {
		return verifyRestoredRecoveryGeneration()
	}
	if os.Args[1] == "linux-node-toolchain-sha256" {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("Linux Node/npm toolchain fingerprints are unavailable on %s", runtime.GOOS)
		}
		paths, err := linuxNodeSandboxPathsFromEnvironment()
		if err != nil {
			return fmt.Errorf("Linux Node/npm toolchain configuration is invalid: %w", err)
		}
		if err = environment.VerifyLinuxNodeWorkspaceFilesystem(paths); err != nil {
			return fmt.Errorf("Linux Node/npm workspace disk bound is not enforced: %w", err)
		}
		digest, err := environment.LinuxNodeToolchainSHA256(paths)
		if err != nil {
			return fmt.Errorf("Linux Node/npm toolchain configuration is invalid: %w", err)
		}
		fmt.Println(digest)
		return nil
	}
	if os.Args[1] == "serve" {
		return serveWorkbench()
	}
	if os.Args[1] == "owner-bootstrap" {
		return runOwnerBootstrap()
	}
	if os.Args[1] == "import-git" {
		return importGitMissionInput()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("POLIS_DSN")
	if os.Args[1] == "migration-attempt-status" {
		if len(os.Args) != 2 {
			return fmt.Errorf("usage: polis migration-attempt-status")
		}
		root := migrationAttemptRoot()
		if root == "" {
			return fmt.Errorf("POLIS_MIGRATION_ATTEMPT_ROOT or POLIS_BLOB_ROOT is required")
		}
		attempts, statusErr := readMigrationAttemptStatuses(root)
		if statusErr != nil {
			return statusErr
		}
		return json.NewEncoder(os.Stdout).Encode(attempts)
	}
	if os.Args[1] == "migrate" {
		if len(os.Args) != 2 {
			return fmt.Errorf("usage: polis migrate")
		}
		return runMigrationWithAttemptJournal(ctx, dsn, 0)
	}
	if os.Args[1] == "migrate-to" {
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: polis migrate-to VERSION")
		}
		version, parseErr := strconv.ParseInt(os.Args[2], 10, 64)
		if parseErr != nil || version <= 0 {
			return fmt.Errorf("migration version must be a positive integer")
		}
		return runMigrationWithAttemptJournal(ctx, dsn, version)
	}
	if os.Args[1] == "migration-status" {
		if len(os.Args) != 2 {
			return fmt.Errorf("usage: polis migration-status")
		}
		records, statusErr := db.ReadMigrationExecutionEvidence(ctx, dsn)
		if statusErr != nil {
			return statusErr
		}
		return json.NewEncoder(os.Stdout).Encode(records)
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

func runCASCollection() error {
	flags := flag.NewFlagSet("cas-collect", flag.ContinueOnError)
	companyID := flags.String("company", "", "company whose local CAS directory to inspect")
	afterDigest := flags.String("after", "", "resume after this lowercase CAS SHA-256")
	limit := flags.Int("limit", 32, "maximum CAS objects to inspect in this page (1-128)")
	apply := flags.Bool("apply", false, "delete only objects proven unreferenced; omitted means dry-run")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *companyID == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: polis cas-collect --company COMPANY_ID [--after SHA256] [--limit 1-128] [--apply]")
	}
	dsn, root := os.Getenv("POLIS_DSN"), os.Getenv("POLIS_BLOB_ROOT")
	if dsn == "" || root == "" {
		return fmt.Errorf("POLIS_DSN and POLIS_BLOB_ROOT are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	runtime, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		return err
	}
	defer runtime.Close()
	report, err := runtime.CollectOrphanCASBlobs(ctx, *companyID, *afterDigest, *limit, *apply)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func createRecoveryBackup() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: polis recovery-backup OUTPUT_DIRECTORY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	databaseDSN := os.Getenv("POLIS_BACKUP_DSN")
	if databaseDSN == "" {
		databaseDSN = os.Getenv("POLIS_DSN")
	}
	runtimeRole := os.Getenv("POLIS_RUNTIME_ROLE")
	if runtimeRole == "" {
		runtimeRole = "polis_runtime"
	}
	result, err := recovery.CreateRecoveryBackupPackage(ctx, recovery.CreateRecoveryBackupOptions{
		DatabaseDSN:     databaseDSN,
		BlobRoot:        os.Getenv("POLIS_BLOB_ROOT"),
		OutputRoot:      os.Args[2],
		PGDumpPath:      os.Getenv("POLIS_PG_DUMP_PATH"),
		RuntimeRoleName: runtimeRole,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func restoreRecoveryBackup() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: polis recovery-backup-restore PACKAGE_DIRECTORY BLOB_ROOT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	runtimeRole := os.Getenv("POLIS_RUNTIME_ROLE")
	if runtimeRole == "" {
		runtimeRole = "polis_runtime"
	}
	result, err := recovery.RestoreRecoveryBackupPackage(ctx, recovery.RestoreRecoveryBackupOptions{
		PackageRoot:       os.Args[2],
		TargetDatabaseDSN: os.Getenv("POLIS_RESTORE_DSN"),
		TargetBlobRoot:    os.Args[3],
		RuntimeRoleName:   runtimeRole,
		PGRestorePath:     os.Getenv("POLIS_PG_RESTORE_PATH"),
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func importGitMissionInput() error {
	if len(os.Args) != 7 && len(os.Args) != 8 {
		return fmt.Errorf("usage: polis import-git COMPANY MISSION REPOSITORY_PATH COMMIT_ID REQUEST_ID [INPUT_ID]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dsn := os.Getenv("POLIS_DSN")
	root := os.Getenv("POLIS_BLOB_ROOT")
	if dsn == "" || root == "" {
		return fmt.Errorf("POLIS_DSN and POLIS_BLOB_ROOT are required for Git snapshot import")
	}
	prepared, err := intake.ImportGitCommit(ctx, os.Args[4], os.Args[5])
	if err != nil {
		return fmt.Errorf("failed to import Git snapshot: %w", err)
	}
	k, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		return err
	}
	defer k.Close()
	inputID := ""
	if len(os.Args) == 8 {
		inputID = os.Args[7]
	}
	revision, err := k.TXAddMissionInput(ctx, k.LocalScope(os.Args[2]), os.Args[3], inputID, os.Args[6], prepared.Upload, prepared.Archive)
	if err != nil {
		return err
	}
	if revision.State == string(intake.StatePartial) {
		fmt.Fprintln(os.Stderr, "Git snapshot is partial; only the selected commit was included without network fetch. The Worker source note lists submodule, LFS, and bounded exclusions and states that working-tree files were not read or included. Add a separate directory snapshot to include local changes.")
	}
	return json.NewEncoder(os.Stdout).Encode(revision)
}

func serveWorkbench() (returnErr error) {
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
	startupReconciliationID := fmt.Sprintf("mcp-runtime-startup-%d", time.Now().UnixNano())
	if err = kernelRuntime.TXReconcileStdioMCPRuntimeObservationIntents(ctx, startupReconciliationID); err != nil {
		return fmt.Errorf("failed to reconcile interrupted MCP runtime observations: %w", err)
	}
	var linuxCgroupManager environment.LinuxNodeCgroupManager
	var linuxInstanceIdentity string
	if runtime.GOOS == "linux" {
		hasUnrestoredLinuxWork, recoveryCheckErr := kernelRuntime.HasUnrestoredLinuxNodeHostWork(ctx)
		if recoveryCheckErr != nil {
			return fmt.Errorf("failed to inspect Linux Node process recovery state: %w", recoveryCheckErr)
		}
		runtimeRoot := strings.TrimSpace(os.Getenv("POLIS_LINUX_NODE_RUNTIME_ROOT"))
		cgroupRoot := strings.TrimSpace(os.Getenv("POLIS_LINUX_NODE_CGROUP_ROOT"))
		if recoveryErr := validateLinuxNodeStartupRecoveryRoot(runtime.GOOS, hasUnrestoredLinuxWork, runtimeRoot, cgroupRoot); recoveryErr != nil {
			return recoveryErr
		}
		if cgroupRoot != "" {
			linuxInstanceIdentity, err = linuxNodeInstanceIdentity(os.Getenv("POLIS_DSN"), root)
			if err != nil {
				return err
			}
			linuxCgroupManager, err = environment.NewLinuxNodeCgroupV2Manager(runtimeRoot, cgroupRoot, linuxInstanceIdentity, environment.DefaultLinuxNodeResourceLimits(), strings.TrimSpace(os.Getenv("POLIS_LINUX_NODE_BWRAP_PATH")))
			if err != nil {
				return fmt.Errorf("failed to reconcile Linux Node cgroups: %w", err)
			}
			defer func() {
				if closer, ok := linuxCgroupManager.(environment.LinuxNodeCgroupManagerCloser); ok {
					if closeErr := closer.Close(); closeErr != nil {
						returnErr = errors.Join(returnErr, fmt.Errorf("failed to release Linux Node cgroup root lease: %w", closeErr))
					}
				}
			}()
		}
	}
	var linuxWorkerCgroupManager environment.LinuxWorkerCgroupManager
	if manager, ok := linuxCgroupManager.(environment.LinuxWorkerCgroupManager); ok {
		linuxWorkerCgroupManager = manager
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		summary, reconcileErr := kernelRuntime.ReconcileWorkerSessions(ctx, linuxWorkerCgroupManager)
		if runtime.GOOS == "linux" && (errors.Is(reconcileErr, environment.ErrLinuxWorkerCgroupOrphanPopulated) || errors.Is(reconcileErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed)) {
			return fmt.Errorf("refusing to open the Linux control listener before Worker cgroup cleanup is confirmed: %w", reconcileErr)
		}
		if summary.Candidates > 0 || reconcileErr != nil {
			fmt.Fprintf(os.Stderr, "WorkerSession host reconciliation: candidates=%d stopped=%d unresolved=%d\n", summary.Candidates, summary.Stopped, summary.Unresolved)
		}
	}
	workerAdapter, err := buildWorkerAdapter(kernelRuntime, linuxWorkerCgroupManager)
	if err != nil {
		return fmt.Errorf("failed to select worker adapter: %w", err)
	}
	if err = workerAdapter.Readiness(ctx); err != nil {
		workerAdapter.Close()
		return fmt.Errorf("worker adapter readiness failed: %w", err)
	}
	mcpSandboxRoot := ""
	if sandboxProvider, ok := workerAdapter.(interface {
		ControlledMCPSandbox() *runner.AppContainerSandbox
	}); ok {
		if sandbox := sandboxProvider.ControlledMCPSandbox(); sandbox != nil {
			mcpSandboxRoot = sandbox.WorkspaceRoot()
		}
	}
	if err = kernelRuntime.TXRevokeStdioMCPRuntimeQualificationsOutsideSandbox(ctx, mcpSandboxRoot, startupReconciliationID); err != nil {
		workerAdapter.Close()
		return fmt.Errorf("failed to reconcile prior MCP AppContainer runtime qualifications: %w", err)
	}
	modelCatalog := provider.NewCodexModelCatalogReader(codexRuntimeConfigFromEnvironment())
	commandService := control.NewService(kernelRuntime, workerAdapter, modelCatalog)
	defer func() {
		if closeErr := closeCommandServiceWithRetry(commandService); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("failed to close command service resources: %w", closeErr))
		}
	}()
	if os.Getenv("POLIS_MCP_RUNTIME_OBSERVATION_ENABLED") == "1" {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("stdio MCP runtime observation is unavailable on %s", runtime.GOOS)
		}
		var observationSandbox *runner.AppContainerSandbox
		ownsSandbox := true
		if provider, ok := workerAdapter.(interface {
			ControlledMCPSandbox() *runner.AppContainerSandbox
		}); ok {
			observationSandbox = provider.ControlledMCPSandbox()
			ownsSandbox = observationSandbox == nil
		}
		if observationSandbox == nil {
			identity := strings.TrimSpace(os.Getenv("POLIS_MCP_APP_CONTAINER_ID"))
			if identity == "" {
				return fmt.Errorf("stdio MCP runtime observation requires POLIS_MCP_APP_CONTAINER_ID")
			}
			observationSandbox, err = runner.NewAppContainerSandbox(identity)
			if err != nil {
				return fmt.Errorf("stdio MCP runtime observation AppContainer creation failed: %w", err)
			}
		}
		observer, observerErr := control.NewAppContainerStdioMCPRuntimeObserver(kernelRuntime, observationSandbox, os.Getenv("SystemRoot"), ownsSandbox)
		if observerErr != nil {
			if ownsSandbox {
				_ = observationSandbox.Close()
			}
			return fmt.Errorf("stdio MCP runtime observation configuration failed: %w", observerErr)
		}
		commandService.SetStdioMCPRuntimeObserver(observer)
	}
	if os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") == "1" {
		commandService.SetStreamableHTTPMCPRuntimeObserver(control.NewStreamableHTTPMCPRuntimeObserver())
	}
	if os.Getenv("POLIS_LINUX_NODE_PREPARATION_ENABLED") == "1" {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("Linux Node/npm environment preparation is unavailable on %s", runtime.GOOS)
		}
		paths, pathErr := linuxNodeSandboxPathsFromEnvironment()
		if pathErr != nil {
			return fmt.Errorf("Linux Node/npm environment preparation configuration is invalid: %w", pathErr)
		}
		if linuxInstanceIdentity == "" {
			linuxInstanceIdentity, pathErr = linuxNodeInstanceIdentity(os.Getenv("POLIS_DSN"), root)
			if pathErr != nil {
				return pathErr
			}
		}
		paths.InstanceIdentity = linuxInstanceIdentity
		if linuxCgroupManager == nil {
			return fmt.Errorf("Linux Node/npm cgroup root lease is not available")
		}
		executor, executorErr := control.NewLinuxNodeNPMPreparationExecutorWithCgroupManager(paths, linuxCgroupManager)
		if executorErr != nil {
			return fmt.Errorf("Linux Node/npm environment preparation configuration is invalid: %w", executorErr)
		}
		commandService.SetProjectEnvironmentPreparationExecutor(executor)
		commandService.SetProjectJobExecutor(executor)
	}
	if os.Getenv("POLIS_WINDOWS_NODE_PREPARATION_ENABLED") == "1" {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("Windows Node/npm environment preparation is unavailable on %s", runtime.GOOS)
		}
		executor, executorErr := control.NewWindowsNodeNPMPreparationExecutor(os.Getenv("POLIS_WINDOWS_NODE_PATH"), os.Getenv("POLIS_WINDOWS_NPM_CLI_PATH"), os.Getenv("SystemRoot"), os.Getenv("POLIS_WINDOWS_NODE_WORKSPACE_ROOT"))
		if executorErr != nil {
			return fmt.Errorf("Windows Node/npm environment preparation configuration is invalid: %w", executorErr)
		}
		commandService.SetProjectEnvironmentPreparationExecutor(executor)
		commandService.SetProjectJobExecutor(executor)
	}
	if _, err = commandService.ReconcileEnvironmentPreparationsAfterRestart(ctx); err != nil {
		return fmt.Errorf("failed to reconcile project environments after restart: %w", err)
	}
	githubCredentialStore, githubCredentialStoreErr := githubfeedback.NewProtectedGitHubCredentialStore()
	if githubCredentialStoreErr == nil {
		commandService.SetGitHubFeedbackCredentialStore(githubCredentialStore)
	}
	if os.Getenv("POLIS_GITHUB_READONLY_ENABLED") == "1" {
		if githubCredentialStoreErr != nil {
			return fmt.Errorf("GitHub read-only collection requires protected credential storage")
		}
		commandService.SetGitHubFeedbackTokenSource(githubfeedback.ProtectedGitHubTokenSource(githubCredentialStore, control.GitHubFeedbackCredentialRef))
	}
	githubSchedulerRequested, schedulerConfigErr := githubFeedbackSchedulerConfig(
		os.Getenv("POLIS_GITHUB_READONLY_ENABLED"),
		os.Getenv("POLIS_GITHUB_FEEDBACK_SCHEDULER_ENABLED"),
		githubCredentialStoreErr == nil,
	)
	if schedulerConfigErr != nil {
		return schedulerConfigErr
	}
	commandService.SetGitHubFeedbackSchedulerEnabled(githubSchedulerRequested)
	if os.Getenv("POLIS_QQ_ENABLED") == "1" {
		ipcKey, decodeErr := base64.RawURLEncoding.DecodeString(os.Getenv("POLIS_SENDER_IPC_KEY"))
		if decodeErr != nil || len(ipcKey) < 32 {
			return fmt.Errorf("QQ sender IPC is enabled but its authenticated session key is invalid")
		}
		ipcURL := os.Getenv("POLIS_SENDER_IPC_URL")
		if ipcURL == "" {
			ipcURL = "http://127.0.0.1:8099"
		}
		sender, senderErr := control.NewLoopbackQQNotificationSender(ipcURL, ipcKey, nil)
		if senderErr != nil {
			return fmt.Errorf("QQ sender IPC configuration is invalid")
		}
		commandService.SetQQNotificationSender(sender)
	}
	address := os.Getenv("POLIS_WORKBENCH_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	sessionToken := os.Getenv("POLIS_DESKTOP_SESSION_TOKEN")
	remoteOrigin := os.Getenv("POLIS_REMOTE_WORKBENCH_ORIGIN")
	if err := desktop.ValidateServerBinding(address, sessionToken); err != nil {
		return err
	}
	if err := desktop.ValidateRemoteManagementConfig(address, sessionToken, remoteOrigin); err != nil {
		return err
	}
	shutdownRequested := make(chan struct{}, 1)
	maintenance := newDesktopMaintenanceGate()
	router := http.NewServeMux()
	ownerAuthStore, err := installationauth.OpenStore(ctx, os.Getenv("POLIS_DSN"))
	if err != nil {
		return fmt.Errorf("failed to start installation owner setup store: %w", err)
	}
	defer ownerAuthStore.Close()
	router.Handle("/api/installation/owner/", desktop.OwnerSetupHandler(ownerAuthStore, remoteOrigin != ""))
	router.Handle("/api/workbench/", workbench.NewHandler(store, commandService))
	router.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(response, http.StatusOK, map[string]string{"service": "polis_backend", "status": "ready", "version": "r0.7"})
	})
	router.Handle("/api/desktop/identity", desktopIdentityHandler())
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
	router.HandleFunc("/api/desktop/maintenance/quiesce", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		quiesced, quiesceErr := maintenance.Quiesce(
			request.Context(),
			store.HasActiveWork,
			request.URL.Query().Get("allow-active") == "1",
		)
		if quiesceErr != nil {
			writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "failed to inspect active work"})
			return
		}
		if !quiesced {
			writeJSON(response, http.StatusConflict, map[string]string{"error": "Polis still has active work"})
			return
		}
		writeJSON(response, http.StatusAccepted, map[string]bool{"accepted": true})
	})
	router.HandleFunc("/api/desktop/shutdown", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !maintenance.IsQuiesced() {
			writeJSON(response, http.StatusConflict, map[string]string{"error": "desktop maintenance must quiesce work before shutdown"})
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
		Handler:           ownerAuthStore.SessionMiddleware(desktop.MiddlewareWithRemoteOrigin(sessionToken, remoteOrigin, maintenance.Middleware(router))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}
	scheduleCtx, cancelScheduleReconciler := context.WithCancel(context.Background())
	stopScheduleReconciler, err := kernelRuntime.StartEmployeeScheduleReconciler(scheduleCtx, func(error) {
		fmt.Fprintln(os.Stderr, "employee schedule reconciliation iteration failed")
	})
	if err != nil {
		cancelScheduleReconciler()
		return fmt.Errorf("failed to start employee schedule reconciliation: %w", err)
	}
	defer func() {
		cancelScheduleReconciler()
		stopScheduleReconciler()
	}()
	revocationStopCtx, cancelRevocationStop := context.WithCancel(context.Background())
	stopRevocationStop, err := commandService.StartCapabilityRevocationWorkerStopper(revocationStopCtx, func(error) {
		fmt.Fprintln(os.Stderr, "capability revocation Worker stop iteration failed")
	})
	if err != nil {
		cancelRevocationStop()
		return fmt.Errorf("failed to start capability revocation Worker stopper: %w", err)
	}
	defer func() {
		cancelRevocationStop()
		stopRevocationStop()
	}()
	automaticProductDispatchEnabled, err := environmentFlag("POLIS_AUTO_WORKER_DISPATCH_ENABLED")
	if err != nil {
		return err
	}
	if automaticProductDispatchEnabled {
		dispatchCtx, cancelDispatch := context.WithCancel(context.Background())
		stopDispatch, dispatchErr := commandService.StartAutomaticProductWorkerDispatcher(dispatchCtx, func(error) {
			fmt.Fprintln(os.Stderr, "automatic product Worker dispatch iteration failed")
		})
		if dispatchErr != nil {
			cancelDispatch()
			return dispatchErr
		}
		defer func() {
			cancelDispatch()
			stopDispatch()
		}()
	}
	schedulerCtx, cancelGitHubScheduler := context.WithCancel(context.Background())
	schedulerDone := make(chan struct{})
	if githubSchedulerRequested {
		go func() {
			defer close(schedulerDone)
			commandService.RunGitHubFeedbackScheduler(schedulerCtx, func(error) {
				fmt.Fprintln(os.Stderr, "GitHub scheduled collection iteration failed")
			})
		}()
	} else {
		close(schedulerDone)
	}
	defer func() {
		cancelGitHubScheduler()
		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
			returnErr = errors.Join(returnErr, errors.New("GitHub collection scheduler did not stop before shutdown"))
		}
	}()
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

func githubFeedbackSchedulerConfig(readOnlyFlag, schedulerFlag string, protectedCredentialStore bool) (bool, error) {
	if schedulerFlag != "1" {
		return false, nil
	}
	if readOnlyFlag != "1" {
		return false, fmt.Errorf("GitHub feedback scheduler requires POLIS_GITHUB_READONLY_ENABLED=1")
	}
	if !protectedCredentialStore {
		return false, fmt.Errorf("GitHub feedback scheduler requires protected credential storage")
	}
	return true, nil
}

func closeCommandServiceWithRetry(service interface{ Close() error }) error {
	if service == nil {
		return nil
	}
	firstErr := service.Close()
	if firstErr == nil {
		return nil
	}
	retryErr := service.Close()
	if retryErr == nil {
		return nil
	}
	return errors.Join(fmt.Errorf("first close attempt: %w", firstErr), fmt.Errorf("retry close attempt: %w", retryErr))
}

func linuxNodeSandboxPathsFromEnvironment() (environment.LinuxNodeSandboxPaths, error) {
	workspaceDiskLimit := environment.DefaultLinuxNodeWorkspaceDiskLimitBytes
	if raw := strings.TrimSpace(os.Getenv("POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			return environment.LinuxNodeSandboxPaths{}, fmt.Errorf("POLIS_LINUX_NODE_WORKSPACE_MAX_BYTES must be a positive integer")
		}
		workspaceDiskLimit = parsed
	}
	paths := environment.LinuxNodeSandboxPaths{
		BubblewrapPath:          os.Getenv("POLIS_LINUX_NODE_BWRAP_PATH"),
		RuntimeRoot:             os.Getenv("POLIS_LINUX_NODE_RUNTIME_ROOT"),
		SystemImageRoot:         os.Getenv("POLIS_LINUX_NODE_SYSTEM_IMAGE_ROOT"),
		WorkspaceRoot:           os.Getenv("POLIS_LINUX_NODE_WORKSPACE_ROOT"),
		WorkspaceDiskLimitBytes: workspaceDiskLimit,
		NPMCacheRoot:            os.Getenv("POLIS_LINUX_NODE_NPM_CACHE_ROOT"),
		CgroupRoot:              os.Getenv("POLIS_LINUX_NODE_CGROUP_ROOT"),
		NodeExecutable:          os.Getenv("POLIS_LINUX_NODE_PATH"),
		NPMCLIScript:            os.Getenv("POLIS_LINUX_NODE_NPM_CLI_PATH"),
	}
	if paths.NodeExecutable == "" {
		paths.NodeExecutable = "/usr/bin/node"
	}
	if paths.NPMCLIScript == "" {
		paths.NPMCLIScript = "/usr/lib/node_modules/npm/bin/npm-cli.js"
	}
	return paths, nil
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

func buildWorkerAdapter(kernelRuntime *kernel.Kernel, workerCgroupManagers ...environment.LinuxWorkerCgroupManager) (control.WorkerAdapter, error) {
	if len(workerCgroupManagers) > 1 {
		return nil, errors.New("only one Linux Worker cgroup manager may be configured")
	}
	var workerCgroupManager environment.LinuxWorkerCgroupManager
	if len(workerCgroupManagers) == 1 {
		workerCgroupManager = workerCgroupManagers[0]
	}
	var workerCgroupArgs []environment.LinuxWorkerCgroupManager
	if workerCgroupManager != nil {
		workerCgroupArgs = []environment.LinuxWorkerCgroupManager{workerCgroupManager}
	}
	mode := os.Getenv("POLIS_WORKER_MODE")
	if mode == "" {
		mode = "deterministic"
	}
	offlineDirectMessaging, err := environmentFlag("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED")
	if err != nil {
		return nil, err
	}
	offlineSharedMissionArtifacts, err := environmentFlag("POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED")
	if err != nil {
		return nil, err
	}
	offlineSkillDirectory, err := environmentFlag("POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED")
	if err != nil {
		return nil, err
	}
	offlineWorkspaceTree, err := environmentFlag("POLIS_OFFLINE_WORKSPACE_TREE_ENABLED")
	if err != nil {
		return nil, err
	}
	offlineMissionChangeAssessment, err := environmentFlag("POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED")
	if err != nil {
		return nil, err
	}
	if offlineMissionChangeAssessment && (offlineDirectMessaging || offlineSharedMissionArtifacts || offlineSkillDirectory || offlineWorkspaceTree) {
		return nil, fmt.Errorf("POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED=1 requires its isolated Fake @12 tool surface")
	}
	if offlineWorkspaceTree && (offlineDirectMessaging || offlineSharedMissionArtifacts || offlineSkillDirectory) {
		return nil, fmt.Errorf("POLIS_OFFLINE_WORKSPACE_TREE_ENABLED=1 requires its isolated Fake @10 tool surface")
	}
	if offlineSharedMissionArtifacts && offlineDirectMessaging {
		return nil, fmt.Errorf("POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED=1 requires its isolated Fake @8 tool surface")
	}
	if offlineSkillDirectory && (offlineDirectMessaging || offlineSharedMissionArtifacts) {
		return nil, fmt.Errorf("POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED=1 requires its isolated Fake @9 tool surface")
	}
	automaticDispatch, err := environmentFlag("POLIS_AUTO_WORKER_DISPATCH_ENABLED")
	if err != nil {
		return nil, err
	}
	if automaticDispatch && !offlineDirectMessaging {
		return nil, fmt.Errorf("POLIS_AUTO_WORKER_DISPATCH_ENABLED=1 requires POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED=1")
	}
	if offlineDirectMessaging && (mode != "real" || os.Getenv("POLIS_PROVIDER_TRANSPORT") != "fake") {
		return nil, fmt.Errorf("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED=1 requires POLIS_WORKER_MODE=real and POLIS_PROVIDER_TRANSPORT=fake")
	}
	if offlineSharedMissionArtifacts && (mode != "real" || os.Getenv("POLIS_PROVIDER_TRANSPORT") != "fake") {
		return nil, fmt.Errorf("POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED=1 requires POLIS_WORKER_MODE=real and POLIS_PROVIDER_TRANSPORT=fake")
	}
	if offlineSkillDirectory && (mode != "real" || os.Getenv("POLIS_PROVIDER_TRANSPORT") != "fake") {
		return nil, fmt.Errorf("POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED=1 requires POLIS_WORKER_MODE=real and POLIS_PROVIDER_TRANSPORT=fake")
	}
	if offlineWorkspaceTree && (mode != "real" || os.Getenv("POLIS_PROVIDER_TRANSPORT") != "fake") {
		return nil, fmt.Errorf("POLIS_OFFLINE_WORKSPACE_TREE_ENABLED=1 requires POLIS_WORKER_MODE=real and POLIS_PROVIDER_TRANSPORT=fake")
	}
	if offlineMissionChangeAssessment && (mode != "real" || os.Getenv("POLIS_PROVIDER_TRANSPORT") != "fake") {
		return nil, fmt.Errorf("POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED=1 requires POLIS_WORKER_MODE=real and POLIS_PROVIDER_TRANSPORT=fake")
	}
	switch mode {
	case "deterministic":
		return control.NewDeterministicWorkerAdapter(kernelRuntime, workerCgroupArgs...), nil
	case "real":
		transport := os.Getenv("POLIS_PROVIDER_TRANSPORT")
		controlledMCPV1 := os.Getenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE") == "1"
		controlledMCPV2 := os.Getenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2") == "1"
		if controlledMCPV1 && controlledMCPV2 {
			return nil, fmt.Errorf("select only one versioned controlled MCP tool surface")
		}
		controlledMCP := controlledMCPV1 || controlledMCPV2
		if controlledMCP && (offlineDirectMessaging || offlineSharedMissionArtifacts || offlineSkillDirectory || offlineWorkspaceTree || offlineMissionChangeAssessment) {
			return nil, fmt.Errorf("offline product extensions require isolated Fake tool surfaces")
		}
		var providerRuntime provider.Runtime
		if transport == "fake" {
			providerRuntime = provider.NewFakeRuntime(provider.FakeRuntimeConfig{Model: os.Getenv("POLIS_PROVIDER_MODEL"), Effort: os.Getenv("POLIS_PROVIDER_EFFORT"), Profile: os.Getenv("POLIS_PROVIDER_PROFILE"), Purpose: os.Getenv("POLIS_PROVIDER_PURPOSE"), ToolCallLimit: envIntOrDefault("POLIS_PROVIDER_TOOL_CALL_LIMIT", 16), TurnDelay: 100 * time.Millisecond, DirectMessagingSurface: offlineDirectMessaging, SharedMissionArtifactSurface: offlineSharedMissionArtifacts, ReadOnlySkillDirectorySurface: offlineSkillDirectory, WorkspaceTreeSurface: offlineWorkspaceTree, MissionChangeAssessmentSurface: offlineMissionChangeAssessment, ControlledMCPToolSurface: controlledMCPV1, ControlledMCPToolSurfaceV2: controlledMCPV2})
		} else if transport == "codex" {
			if controlledMCP {
				return nil, fmt.Errorf("real-provider controlled MCP surfaces are not qualified")
			}
			config := codexRuntimeConfigFromEnvironment()
			config.RequireWorkerCgroup = runtime.GOOS == "linux"
			config.ToolSurface = provider.ProductToolSurface()
			bound, err := provider.BindCodexRuntimeConfig(config)
			if err != nil {
				return nil, fmt.Errorf("provider runtime binding failed: %w", err)
			}
			providerRuntime = provider.NewCodexRuntime(bound)
		} else {
			return nil, fmt.Errorf("real worker mode requires POLIS_PROVIDER_TRANSPORT=fake or codex")
		}
		if controlledMCPV1 {
			identity := strings.TrimSpace(os.Getenv("POLIS_MCP_APP_CONTAINER_ID"))
			if identity == "" {
				return nil, fmt.Errorf("controlled MCP requires POLIS_MCP_APP_CONTAINER_ID")
			}
			sandbox, err := runner.NewAppContainerSandbox(identity)
			if err != nil {
				return nil, fmt.Errorf("controlled MCP AppContainer creation failed: %w", err)
			}
			adapter, adapterErr := control.NewRealProviderWorkerAdapterWithMCPAppContainer(kernelRuntime, providerRuntime, sandbox, workerCgroupArgs...)
			if adapterErr != nil {
				_ = sandbox.Close()
				return nil, adapterErr
			}
			return adapter, nil
		}
		if controlledMCPV2 {
			identity := strings.TrimSpace(os.Getenv("POLIS_MCP_APP_CONTAINER_ID"))
			if runtime.GOOS == "windows" && identity != "" {
				sandbox, err := runner.NewAppContainerSandbox(identity)
				if err != nil {
					return nil, fmt.Errorf("controlled MCP v2 AppContainer creation failed: %w", err)
				}
				adapter, adapterErr := control.NewRealProviderWorkerAdapterWithMCPAppContainer(kernelRuntime, providerRuntime, sandbox, workerCgroupArgs...)
				if adapterErr != nil {
					_ = sandbox.Close()
					return nil, adapterErr
				}
				return adapter, nil
			}
			return control.NewRealProviderWorkerAdapter(kernelRuntime, providerRuntime, workerCgroupArgs...)
		}
		return control.NewRealProviderWorkerAdapter(kernelRuntime, providerRuntime, workerCgroupArgs...)
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

func environmentFlag(name string) (bool, error) {
	switch strings.TrimSpace(os.Getenv(name)) {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be unset, 0, or 1", name)
	}
}
