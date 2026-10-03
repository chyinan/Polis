// pattern: Imperative Shell
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/kernel"
)

func NewHandler(model ReadModel, services ...control.CommandService) http.Handler {
	var service control.CommandService
	if len(services) > 0 {
		service = services[0]
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		serveRequest(model, service, response, request)
	})
}

type parsedPath struct {
	companyID    string
	endpoint     string
	missionID    string
	resourceID   string
	targetID     string
	evidenceArea string
}

func serveRequest(model ReadModel, service control.CommandService, response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/api/workbench/companies" {
		serveCompanyCollection(service, response, request)
		return
	}
	path, ok := parsePath(request.URL.Path)
	if !ok {
		writeError(response, http.StatusNotFound, "workbench route not found")
		return
	}
	if err := validateCompanyID(path.companyID); err != nil {
		writeError(response, http.StatusBadRequest, "invalid company scope")
		return
	}
	ctx := request.Context()
	if request.Method == http.MethodGet {
		switch path.endpoint {
		case "memory.corrections":
			response.Header().Set("Cache-Control", "no-store")
			memoryService, ok := service.(control.MemoryCorrectionQueueService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "memory correction queue is unavailable")
				return
			}
			queue, err := memoryService.ListMemoryCorrections(ctx, path.companyID)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, "corrections", "memory", err)
				return
			}
			writeJSON(response, http.StatusOK, queue)
		case "problem.budgets":
			response.Header().Set("Cache-Control", "no-store")
			budgetService, ok := service.(control.ProblemToolBudgetService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "ProblemKey budget service is unavailable")
				return
			}
			budgets, err := budgetService.ListProblemToolCallBudgets(ctx, path.companyID)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, "budgets", "company", err)
				return
			}
			writeJSON(response, http.StatusOK, budgets)
		case "mission.budgets":
			response.Header().Set("Cache-Control", "no-store")
			budgetService, ok := service.(control.ProblemToolBudgetService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "Mission tool-call budget service is unavailable")
				return
			}
			budgets, err := budgetService.ListMissionToolCallBudgets(ctx, path.companyID)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, "budgets", "company", err)
				return
			}
			writeJSON(response, http.StatusOK, budgets)
		case "missions.routines":
			routineReader, ok := model.(DailyRoutineReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "daily Routine reader is unavailable")
				return
			}
			items, err := routineReader.ListDailyRoutines(ctx, path.companyID, path.missionID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, items)
		case "missions.inputs":
			inputReader, ok := model.(MissionInputReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "mission input reader is unavailable")
				return
			}
			items, err := inputReader.ListMissionInputs(ctx, path.companyID, path.missionID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, items)
		case "missions.change_requests":
			changeRequestService, ok := service.(control.MissionChangeRequestService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "mission change request service is unavailable")
				return
			}
			items, err := changeRequestService.ListMissionChangeRequests(ctx, path.companyID, path.missionID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, items)
		case "missions.takeover_leases":
			takeoverService, ok := service.(control.TaskTakeoverLeaseService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "Task takeover service is unavailable")
				return
			}
			items, err := takeoverService.ListTaskTakeoverLeases(ctx, path.companyID, path.missionID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, items)
		case "tasks.input_manifest":
			inputManifestReader, ok := model.(TaskInputManifestReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "task input manifest reader is unavailable")
				return
			}
			manifest, err := inputManifestReader.GetTaskInputManifest(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, manifest)
		case "tasks.memory_impact":
			response.Header().Set("Cache-Control", "no-store")
			memoryService, ok := service.(control.MemoryRevalidationService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "memory impact service is unavailable")
				return
			}
			status, err := memoryService.GetMemoryTaskStatus(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "task", err)
				return
			}
			writeJSON(response, http.StatusOK, status)
		case "memory.record_revocation_preview":
			response.Header().Set("Cache-Control", "no-store")
			memoryService, ok := service.(control.MemoryRevocationService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "memory revocation service is unavailable")
				return
			}
			preview, err := memoryService.GetMemoryRecordRevocationPreview(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "memory", err)
				return
			}
			writeJSON(response, http.StatusOK, preview)
		case "tasks.memory_revalidation_preview":
			response.Header().Set("Cache-Control", "no-store")
			memoryService, ok := service.(control.MemoryRevalidationService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "memory revalidation service is unavailable")
				return
			}
			query := request.URL.Query()
			dependencyIDs, correctionIDs := query["dependencyId"], query["correctionId"]
			if len(query) != 2 || len(dependencyIDs) != 1 || len(correctionIDs) != 1 ||
				!core.ValidID(dependencyIDs[0]) || !core.ValidID(correctionIDs[0]) {
				writeError(response, http.StatusBadRequest, "one dependencyId and one correctionId are required")
				return
			}
			preview, err := memoryService.GetMemoryTaskRevalidationPreview(ctx, path.companyID, path.resourceID, dependencyIDs[0], correctionIDs[0])
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "task", err)
				return
			}
			writeJSON(response, http.StatusOK, preview)
		case "tasks.jobs":
			jobReader, ok := model.(EnvironmentJobLifecycleReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "job run reader is unavailable")
				return
			}
			jobs, err := jobReader.ListTaskJobRuns(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, jobs)
		case "tasks.handovers":
			handoverReader, ok := model.(EnvironmentJobLifecycleReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "cross-backend handover reader is unavailable")
				return
			}
			handovers, err := handoverReader.ListTaskCrossBackendHandovers(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, handovers)
		case "jobs.logs":
			jobService, ok := service.(control.ProjectJobLifecycleService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "project job log reader is unavailable")
				return
			}
			logs, err := jobService.GetProjectJobLogs(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, logs)
		case "jobs.browser_session":
			_, ok := service.(control.ProjectJobLifecycleService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "project job browser session service is unavailable")
				return
			}
			writeError(response, http.StatusMethodNotAllowed, "browser sessions must be created with POST")
		case "notifications":
			notificationReader, ok := model.(NotificationsReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "notification reader is unavailable")
				return
			}
			notifications, err := notificationReader.ListNotifications(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, notifications)
		case "feedback":
			feedbackReader, ok := model.(FeedbackReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "feedback read model is unavailable")
				return
			}
			feedback, err := feedbackReader.GetCompanyFeedback(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, feedback)
		case "capabilities":
			capabilityService, ok := service.(control.CapabilityService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "capability catalog is unavailable")
				return
			}
			catalog, err := capabilityService.ListCapabilityCatalog(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, catalog)
		case "domain_workflows":
			domainEvidenceService, ok := service.(control.DomainEvidenceService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "domain evidence service is unavailable")
				return
			}
			ledger, err := domainEvidenceService.ListDomainEvidence(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, ledger)
		case "domain.evidence.preview":
			domainEvidenceService, ok := service.(control.DomainEvidenceService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "domain evidence preview service is unavailable")
				return
			}
			query := request.URL.Query()
			previewPaths := query["path"]
			if len(query) != 1 || len(previewPaths) != 1 || strings.TrimSpace(previewPaths[0]) == "" || len(previewPaths[0]) > 1024 {
				writeError(response, http.StatusBadRequest, "a bounded evidence entry path is required")
				return
			}
			preview, err := domainEvidenceService.ReadDomainEvidenceArtifactPreview(ctx, path.companyID, path.resourceID, domainworkflow.DomainEvidenceArea(path.evidenceArea), previewPaths[0])
			if errors.Is(err, kernel.ErrDomainEvidencePreviewUnsupported) {
				writeError(response, http.StatusUnsupportedMediaType, "this evidence entry cannot be previewed in Workbench")
				return
			}
			if err != nil {
				writeModelError(response, err)
				return
			}
			response.Header().Set("Content-Type", preview.MediaType)
			response.Header().Set("Content-Disposition", "inline")
			response.Header().Set("Content-Length", strconv.Itoa(len(preview.Content)))
			response.Header().Set("Cache-Control", "no-store")
			response.Header().Set("X-Content-Type-Options", "nosniff")
			response.Header().Set("X-Polis-Preview-Filename", url.QueryEscape(preview.FileName))
			response.Header().Set("X-Polis-Input-ID", preview.InputID)
			response.Header().Set("X-Polis-Input-Revision", strconv.FormatInt(preview.InputRevision, 10))
			response.Header().Set("X-Content-SHA256", preview.ContentSHA256)
			response.Header().Set("X-Source-SHA256", preview.SourceDigest)
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(preview.Content)
		case "domain.evidence.preview_entries":
			domainEvidenceService, ok := service.(control.DomainEvidenceService)
			if !ok {
				writeError(response, http.StatusNotImplemented, "domain evidence preview service is unavailable")
				return
			}
			manifest, err := domainEvidenceService.ListDomainEvidenceArtifactPreviewEntries(ctx, path.companyID, path.resourceID, domainworkflow.DomainEvidenceArea(path.evidenceArea))
			if errors.Is(err, kernel.ErrDomainEvidencePreviewUnsupported) {
				writeError(response, http.StatusUnsupportedMediaType, "this evidence source cannot be inspected in Workbench")
				return
			}
			if err != nil {
				writeModelError(response, err)
				return
			}
			response.Header().Set("Cache-Control", "no-store")
			writeJSON(response, http.StatusOK, manifest)
		case "environments":
			environmentReader, ok := model.(EnvironmentJobLifecycleReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "environment reader is unavailable")
				return
			}
			environments, err := environmentReader.ListProjectEnvironments(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, environments)
		case "operations":
			operationsReader, ok := model.(OperationsReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "operations reader is unavailable")
				return
			}
			operations, err := operationsReader.GetOperations(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, operations)
		case "workspace":
			workspaceReader, ok := model.(WorkspaceReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "workspace reader is unavailable")
				return
			}
			workspace, err := workspaceReader.GetWorkspace(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, workspace)
		case "artifact":
			workspaceReader, ok := model.(WorkspaceReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "artifact reader is unavailable")
				return
			}
			artifact, err := workspaceReader.GetArtifact(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, artifact)
		case "artifacts.manifest":
			deliveryReader, ok := model.(ArtifactDeliveryReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "artifact delivery manifest reader is unavailable")
				return
			}
			manifest, err := deliveryReader.GetArtifactDeliveryManifest(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			response.Header().Set("Cache-Control", "no-store")
			response.Header().Set("X-Polis-Manifest-SHA256", manifest.ManifestSHA256)
			writeJSON(response, http.StatusOK, manifest)
		case "artifacts.download":
			deliveryReader, ok := model.(ArtifactDeliveryReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "artifact delivery package reader is unavailable")
				return
			}
			bundle, err := deliveryReader.GetArtifactDeliveryPackage(ctx, path.companyID, path.resourceID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			response.Header().Set("Content-Type", "application/zip")
			response.Header().Set("Content-Disposition", `attachment; filename="polis-delivery.zip"`)
			response.Header().Set("Content-Length", strconv.Itoa(len(bundle.Archive)))
			response.Header().Set("Cache-Control", "no-store")
			response.Header().Set("X-Content-SHA256", bundle.PackageSHA256)
			response.Header().Set("X-Polis-Manifest-SHA256", bundle.ManifestSHA256)
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(bundle.Archive)
		case "collaboration":
			collaborationReader, ok := model.(CollaborationReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "collaboration reader is unavailable")
				return
			}
			items, err := collaborationReader.ListCollaboration(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, items)
		case "instructions":
			instructionReader, ok := model.(InstructionReader)
			if !ok {
				writeError(response, http.StatusNotImplemented, "operator instruction reader is unavailable")
				return
			}
			instructions, err := instructionReader.ListOperatorInstructions(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, instructions)
		case "settings":
			settingsService, ok := asRuntimeSettingsService(service)
			if !ok {
				writeError(response, http.StatusNotImplemented, "runtime settings service is unavailable")
				return
			}
			settings, err := settingsService.GetRuntimeSettings(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, settings)
		case "settings.models":
			catalogService, ok := service.(control.CodexModelCatalogService)
			if !ok {
				writeError(response, http.StatusServiceUnavailable, "Codex model catalog is unavailable")
				return
			}
			catalog, err := catalogService.ListCodexModels(ctx, path.companyID)
			if err != nil {
				status := http.StatusServiceUnavailable
				if errors.Is(err, errCompanyNotFound) || errors.Is(err, core.OutOfScope) {
					status = http.StatusNotFound
				} else if errors.Is(err, core.Malformed) {
					status = http.StatusBadRequest
				}
				writeError(response, status, err.Error())
				return
			}
			writeJSON(response, http.StatusOK, catalog)
		case "organization":
			organizationService, ok := asOrganizationService(service)
			if !ok {
				writeError(response, http.StatusNotImplemented, "organization service is unavailable")
				return
			}
			company, err := organizationService.GetCompany(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, company)
		case "overview":
			view, err := model.GetCompanyOverview(ctx, path.companyID)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, view)
		case "activity":
			query, err := parseActivityQuery(request, path.companyID)
			if err != nil {
				writeError(response, http.StatusBadRequest, err.Error())
				return
			}
			view, err := model.ListActivity(ctx, query)
			if err != nil {
				writeModelError(response, err)
				return
			}
			writeJSON(response, http.StatusOK, view)
		case "stream":
			serveActivityStream(model, response, request, path.companyID)
		default:
			writeError(response, http.StatusNotFound, "workbench route not found")
		}
		return
	}
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if service == nil {
		writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("command surface is unavailable"))
		return
	}
	switch path.endpoint {
	case "mission.budget.allocate":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "Mission tool-call budget service is unavailable")
			return
		}
		var input control.MissionToolCallBudgetChangeRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "mission_budget", err)
			return
		}
		receipt, err := budgetService.ChangeMissionToolCallBudget(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "mission_budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "mission.budget.closing_reserve":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "Mission tool-call budget service is unavailable")
			return
		}
		var input control.MissionToolCallClosingReserveRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "mission_budget", err)
			return
		}
		receipt, err := budgetService.SetMissionToolCallClosingReserve(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "mission_budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "problem.budget.allocate":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "ProblemKey budget service is unavailable")
			return
		}
		var input control.ProblemToolCallAllocationRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "budget", err)
			return
		}
		receipt, err := budgetService.AllocateProblemToolCalls(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "problem.budget.task.allocate":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "ProblemKey budget service is unavailable")
			return
		}
		var input control.TaskToolCallAllocationRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.targetID, "task_budget", err)
			return
		}
		receipt, err := budgetService.AllocateTaskToolCalls(ctx, path.companyID, path.resourceID, path.targetID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.targetID, "task_budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "problem.budget.task.close_incomplete":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "ProblemKey budget service is unavailable")
			return
		}
		var input control.TaskToolCallIncompleteClosureRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.targetID, "task_budget", err)
			return
		}
		receipt, err := budgetService.CloseTaskToolBudgetIncomplete(ctx, path.companyID, path.resourceID, path.targetID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.targetID, "task_budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "problem.budget.closing_reserve":
		response.Header().Set("Cache-Control", "no-store")
		budgetService, ok := service.(control.ProblemToolBudgetService)
		if !ok {
			writeError(response, http.StatusNotImplemented, "ProblemKey budget service is unavailable")
			return
		}
		var input control.ProblemToolCallClosingReserveRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "budget", err)
			return
		}
		receipt, err := budgetService.SetProblemToolCallClosingReserve(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "budget", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "tasks.handovers":
		handoverService, ok := service.(control.ProjectEnvironmentHandoverCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "task", errors.New("project environment handover command service is unavailable"))
			return
		}
		var input control.CreateProjectEnvironmentHandoverRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "task", err)
			return
		}
		record, err := handoverService.CreateTaskEnvironmentHandover(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "task", err)
			return
		}
		writeJSON(response, http.StatusAccepted, record)
	case "tasks.jobs":
		jobService, ok := service.(control.ProjectJobLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "task", errors.New("project job command service is unavailable"))
			return
		}
		var input control.StartProjectJobRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "task", err)
			return
		}
		input.TaskID = path.resourceID
		job, err := jobService.StartProjectJob(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "task", err)
			return
		}
		writeJSON(response, http.StatusAccepted, job)
	case "jobs.stop":
		jobService, ok := service.(control.ProjectJobLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "job", errors.New("project job command service is unavailable"))
			return
		}
		var input control.StopProjectJobRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "job", err)
			return
		}
		job, err := jobService.StopProjectJob(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "job", err)
			return
		}
		writeJSON(response, http.StatusAccepted, job)
	case "jobs.browser_session":
		jobService, ok := service.(control.ProjectJobLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "job", errors.New("project job browser session service is unavailable"))
			return
		}
		var input control.CreateProjectJobBrowserSessionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "job", err)
			return
		}
		session, err := jobService.CreateProjectJobBrowserSession(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "job", err)
			return
		}
		writeJSON(response, http.StatusCreated, session)
	case "feedback.credentials":
		credentialService, ok := service.(control.GitHubFeedbackCredentialCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("GitHub credential command service is unavailable"))
			return
		}
		var input struct {
			Token string `json:"token"`
		}
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := credentialService.StoreGitHubFeedbackCredential(ctx, input.Token)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "feedback.credentials.delete":
		credentialService, ok := service.(control.GitHubFeedbackCredentialCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("GitHub credential command service is unavailable"))
			return
		}
		var input struct{}
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := credentialService.DeleteGitHubFeedbackCredential(ctx)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "human_interventions.acknowledge", "human_interventions.resolve":
		interventionService, ok := service.(control.HumanInterventionCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "human_intervention", errors.New("human intervention command service is unavailable"))
			return
		}
		var input control.SetHumanInterventionStateRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "human_intervention", err)
			return
		}
		input.InterventionID = path.resourceID
		if path.endpoint == "human_interventions.acknowledge" {
			input.State = "acknowledged"
		} else {
			input.State = "resolved"
		}
		receipt, err := interventionService.SetHumanInterventionState(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "human_intervention", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "notifications.route":
		notificationService, ok := service.(control.NotificationCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("notification command service is unavailable"))
			return
		}
		var input control.ConfigureNotificationRouteRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := notificationService.ConfigureNotificationRoute(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "environments":
		environmentService, ok := service.(control.EnvironmentLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("environment command service is unavailable"))
			return
		}
		var input control.RegisterProjectEnvironmentRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		revision, err := environmentService.RegisterProjectEnvironment(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, revision)
	case "environments.policy":
		environmentService, ok := service.(control.EnvironmentLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "environment", errors.New("environment command service is unavailable"))
			return
		}
		var input control.EnvironmentPolicyDecisionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "environment", err)
			return
		}
		receipt, err := environmentService.DecideProjectEnvironmentPolicy(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "environment", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "environments.executor_qualification":
		environmentService, ok := service.(control.EnvironmentLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "environment", errors.New("environment command service is unavailable"))
			return
		}
		var input control.EnvironmentExecutorQualificationRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "environment", err)
			return
		}
		receipt, err := environmentService.DecideProjectEnvironmentExecutorQualification(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "environment", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "environments.preparation":
		environmentService, ok := service.(control.EnvironmentLifecycleService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "environment", errors.New("environment command service is unavailable"))
			return
		}
		var input control.EnsureEnvironmentRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "environment", err)
			return
		}
		run, err := environmentService.EnsureProjectEnvironment(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "environment", err)
			return
		}
		writeJSON(response, http.StatusAccepted, run)
	case "notifications.test":
		notificationService, ok := service.(control.NotificationCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("notification command service is unavailable"))
			return
		}
		var input control.MissionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := notificationService.TestNotification(ctx, path.companyID, input.RequestID)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "capabilities.skills":
		if contentType, _, parseErr := mime.ParseMediaType(request.Header.Get("Content-Type")); parseErr == nil && contentType == "multipart/form-data" {
			sourceService, sourceOK := service.(control.SkillPackageImportService)
			if !sourceOK {
				writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("Skill package import is unavailable"))
				return
			}
			input, err := parseReadOnlySkillPackageUpload(response, request)
			if errors.Is(err, errSkillPackageTooLarge) {
				writeError(response, http.StatusRequestEntityTooLarge, err.Error())
				return
			}
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
				return
			}
			item, err := sourceService.ImportReadOnlySkillPackage(ctx, path.companyID, input)
			if err != nil {
				writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
				return
			}
			writeJSON(response, http.StatusAccepted, item)
			return
		}
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var input control.ImportSkillRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.ImportSkill(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "domain.evidence":
		domainEvidenceService, ok := service.(control.DomainEvidenceService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("domain evidence service is unavailable"))
			return
		}
		var input control.DomainEvidenceCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		record, err := domainEvidenceService.RecordDomainEvidence(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "domain_evidence", err)
			return
		}
		writeJSON(response, http.StatusAccepted, record)
	case "domain.evidence.review":
		domainEvidenceService, ok := service.(control.DomainEvidenceService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "domain_evidence_review", errors.New("domain evidence review service is unavailable"))
			return
		}
		var input control.DomainEvidenceReviewCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "domain_evidence_review", err)
			return
		}
		review, err := domainEvidenceService.RecordDomainEvidenceReview(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "domain_evidence_review", err)
			return
		}
		writeJSON(response, http.StatusAccepted, review)
	case "domain.evidence.assessment":
		assessmentService, ok := service.(control.DomainEvidenceSubstantiveAssessmentService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "domain_evidence_assessment", errors.New("substantive domain evidence assessment service is unavailable"))
			return
		}
		var input control.DomainEvidenceSubstantiveAssessmentCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "domain_evidence_assessment", err)
			return
		}
		assessment, err := assessmentService.RecordDomainEvidenceSubstantiveAssessment(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "domain_evidence_assessment", err)
			return
		}
		writeJSON(response, http.StatusAccepted, assessment)
	case "domain.profile.qualification":
		qualificationService, ok := service.(control.DomainProfileQualificationService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "domain_profile_qualification", errors.New("domain profile qualification service is unavailable"))
			return
		}
		var input control.DomainProfileQualificationCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "domain_profile_qualification", err)
			return
		}
		decision, err := qualificationService.RecordDomainProfileQualification(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "domain_profile_qualification", err)
			return
		}
		writeJSON(response, http.StatusAccepted, decision)
	case "domain.research.simulation":
		simulationService, ok := service.(control.ResearchSimulationCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "research_simulation", errors.New("research simulation service is unavailable"))
			return
		}
		var input control.ResearchSimulationCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "research_simulation", err)
			return
		}
		run, err := simulationService.RunResearchSimulation(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "research_simulation", err)
			return
		}
		writeJSON(response, http.StatusAccepted, run)
	case "domain.content.source.authorization":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_source_authorization", errors.New("content operations service is unavailable"))
			return
		}
		var input control.DomainContentSourceAuthorizationCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_source_authorization", err)
			return
		}
		event, err := contentService.SetContentSourceAuthorization(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_source_authorization", err)
			return
		}
		writeJSON(response, http.StatusAccepted, event)
	case "domain.content.draft":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_draft", errors.New("content operations service is unavailable"))
			return
		}
		var input control.DomainContentDraftCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_draft", err)
			return
		}
		draft, err := contentService.RegisterContentDraft(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_draft", err)
			return
		}
		writeJSON(response, http.StatusAccepted, draft)
	case "domain.content.review":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_review", errors.New("content operations service is unavailable"))
			return
		}
		var input struct {
			DraftInputID  string `json:"draftInputId"`
			DraftRevision int64  `json:"draftRevision,string"`
			control.DomainContentReviewCommandRequest
		}
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_review", err)
			return
		}
		review, err := contentService.RecordContentReview(ctx, path.companyID, input.DraftInputID, input.DraftRevision, input.DomainContentReviewCommandRequest)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_review", err)
			return
		}
		writeJSON(response, http.StatusAccepted, review)
	case "domain.content.publication":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_publication", errors.New("content operations service is unavailable"))
			return
		}
		var input control.DomainContentPublicationCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_publication", err)
			return
		}
		publication, err := contentService.SimulateContentPublication(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_publication", err)
			return
		}
		writeJSON(response, http.StatusAccepted, publication)
	case "domain.content.correction":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_correction", errors.New("content operations service is unavailable"))
			return
		}
		var input control.DomainContentCorrectionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_correction", err)
			return
		}
		correction, err := contentService.RecordContentCorrection(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_correction", err)
			return
		}
		writeJSON(response, http.StatusAccepted, correction)
	case "domain.content.feedback":
		contentService, ok := service.(control.ContentOperationsCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "content_feedback", errors.New("content operations service is unavailable"))
			return
		}
		var input control.DomainContentFeedbackCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "content_feedback", err)
			return
		}
		feedback, err := contentService.RecordContentFeedback(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RequestID, "content_feedback", err)
			return
		}
		writeJSON(response, http.StatusAccepted, feedback)
	case "feedback.sources":
		feedbackService, ok := service.(control.GitHubFeedbackCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("GitHub feedback command service is unavailable"))
			return
		}
		var input control.RegisterGitHubFeedbackSourceRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		source, err := feedbackService.RegisterGitHubFeedbackSource(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, source)
	case "feedback.source.probe":
		feedbackService, ok := service.(control.GitHubFeedbackCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "github_feedback_source", errors.New("GitHub feedback command service is unavailable"))
			return
		}
		var input control.GitHubFeedbackProbeRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		receipt, err := feedbackService.ProbeGitHubFeedbackSource(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "feedback.source.decision":
		feedbackService, ok := service.(control.GitHubFeedbackCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "github_feedback_source", errors.New("GitHub feedback command service is unavailable"))
			return
		}
		var input control.GitHubFeedbackDecisionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		source, err := feedbackService.DecideGitHubFeedbackSource(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		writeJSON(response, http.StatusAccepted, source)
	case "feedback.source.scan":
		feedbackService, ok := service.(control.GitHubFeedbackCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.resourceID, "github_feedback_source", errors.New("GitHub feedback command service is unavailable"))
			return
		}
		var input control.GitHubFeedbackPollRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		receipt, err := feedbackService.PollGitHubFeedbackSource(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.resourceID, "github_feedback_source", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "feedback.backlog":
		feedbackService, ok := service.(control.GitHubFeedbackBacklogCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("GitHub feedback backlog command service is unavailable"))
			return
		}
		var input control.GitHubFeedbackBacklogStatusRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := feedbackService.SetGitHubFeedbackBacklogStatus(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "feedback.collection-policy":
		policyService, ok := service.(control.GitHubFeedbackCollectionPolicyCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("GitHub collection policy service is unavailable"))
			return
		}
		var input control.GitHubFeedbackCollectionPolicyRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		policy, err := policyService.SetGitHubFeedbackCollectionPolicy(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, policy)
	case "capabilities.mcp-packages":
		contentType, _, parseErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if parseErr != nil || contentType != "multipart/form-data" {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", core.Malformed)
			return
		}
		packageService, ok := service.(control.StdioMCPPackageImportService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("stdio MCP package import is unavailable"))
			return
		}
		input, err := parseStdioMCPPackageUpload(response, request)
		if errors.Is(err, errStdioMCPPackageTooLarge) {
			writeError(response, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		item, err := packageService.ImportStdioMCPPackage(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.ServerID, "mcp_server_package", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.mcp":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var input control.RegisterMCPRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.RegisterMCP(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.qualify":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var input control.QualifyCapabilityRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.QualifyCapability(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.CapabilityID, "capability", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.decide":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var input control.DecideCapabilityRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := capabilityService.DecideCapability(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.CapabilityID, "capability", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "capabilities.runtime-approve":
		qualificationService, ok := service.(control.MCPRuntimeQualificationService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("MCP runtime qualification is unavailable"))
			return
		}
		var input control.ApproveStdioMCPRuntimeQualificationRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := qualificationService.ApproveStdioMCPRuntimeQualification(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.RuntimeQualificationID, "mcp_runtime_qualification", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "capabilities.runtime-observe":
		observationService, ok := service.(control.StdioMCPRuntimeObservationService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("MCP runtime observation is unavailable"))
			return
		}
		var input control.ObserveStdioMCPRuntimeRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		qualification, err := observationService.ObserveStdioMCPRuntime(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.PackageRevisionID, "mcp_runtime_observation", err)
			return
		}
		writeJSON(response, http.StatusAccepted, qualification)
	case "capabilities.runtime-observe-http":
		observationService, ok := service.(control.StreamableHTTPMCPRuntimeObservationService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("Streamable HTTP MCP runtime observation is unavailable"))
			return
		}
		var input control.ObserveStreamableHTTPMCPRuntimeRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		qualification, err := observationService.ObserveStreamableHTTPMCPRuntime(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.CapabilityID, "mcp_runtime_observation", err)
			return
		}
		writeJSON(response, http.StatusAccepted, qualification)
	case "capabilities.bind", "capabilities.unbind":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var input control.BindEmployeeCapabilityRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		var receipt kernel.Receipt
		var err error
		if path.endpoint == "capabilities.bind" {
			receipt, err = capabilityService.BindEmployeeCapability(ctx, path.companyID, input)
		} else {
			receipt, err = capabilityService.RevokeEmployeeCapability(ctx, path.companyID, input)
		}
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, input.EmployeeID, "employee", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "instructions":
		instructionService, ok := service.(control.OperatorInstructionService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("operator instruction service is unavailable"))
			return
		}
		var input control.CreateOperatorInstructionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		instruction, err := instructionService.CreateOperatorInstruction(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, instruction)
	case "settings":
		settingsService, ok := asRuntimeSettingsService(service)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("runtime settings service is unavailable"))
			return
		}
		var input control.UpdateRuntimeSettingsRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := settingsService.UpdateRuntimeSettings(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "organization":
		organizationService, ok := asOrganizationService(service)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("organization service is unavailable"))
			return
		}
		var input control.UpdateCompanyRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		input.ID = path.companyID
		receipt, err := organizationService.UpdateCompany(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "archive":
		organizationService, ok := asOrganizationService(service)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("organization service is unavailable"))
			return
		}
		var input control.MissionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := organizationService.ArchiveCompany(ctx, path.companyID, input.RequestID)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.inputs":
		inputService, ok := service.(control.MissionInputService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("mission input service is unavailable"))
			return
		}
		input, filename, mediaType, content, err := parseMissionInputUpload(response, request, path.missionID)
		if errors.Is(err, errMissionInputTooLarge) {
			writeError(response, http.StatusRequestEntityTooLarge, "mission input exceeds the 8 MB limit")
			return
		}
		if err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		receipt, err := inputService.UploadMissionInput(ctx, path.companyID, input, filename, mediaType, content)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.change_requests":
		changeRequestService, ok := service.(control.MissionChangeRequestService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("mission change request service is unavailable"))
			return
		}
		var input control.CreateMissionChangeRequestRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		item, err := changeRequestService.CreateMissionChangeRequest(ctx, path.companyID, path.missionID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "mission_change_requests.consider", "mission_change_requests.decline", "mission_change_requests.apply":
		changeRequestService, ok := service.(control.MissionChangeRequestService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("mission change request service is unavailable"))
			return
		}
		var input control.MissionChangeRequestCommand
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		var item kernel.MissionChangeRequest
		var err error
		switch path.endpoint {
		case "mission_change_requests.consider":
			item, err = changeRequestService.ConsiderMissionChangeRequest(ctx, path.companyID, path.missionID, path.resourceID, input)
		case "mission_change_requests.decline":
			item, err = changeRequestService.DeclineMissionChangeRequest(ctx, path.companyID, path.missionID, path.resourceID, input)
		default:
			item, err = changeRequestService.ApplyMissionChangeRequest(ctx, path.companyID, path.missionID, path.resourceID, input)
		}
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "tasks.takeover_lease":
		takeoverService, ok := service.(control.TaskTakeoverLeaseService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("Task takeover service is unavailable"))
			return
		}
		var input control.TaskTakeoverLeaseCommand
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		item, err := takeoverService.CreateTaskTakeoverLease(ctx, path.companyID, path.missionID, path.resourceID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "tasks.memory_revalidation":
		memoryService, ok := service.(control.MemoryRevalidationService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.resourceID, errors.New("memory revalidation service is unavailable"))
			return
		}
		var input control.RevalidateMemoryTaskRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.resourceID, err)
			return
		}
		receipt, err := memoryService.RevalidateMemoryTask(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.resourceID, err)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSON(response, http.StatusAccepted, receipt)
	case "memory.record_revoke":
		response.Header().Set("Cache-Control", "no-store")
		memoryService, ok := service.(control.MemoryRevocationService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.resourceID, errors.New("memory revocation service is unavailable"))
			return
		}
		var input control.RevokeMemoryRecordRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.resourceID, err)
			return
		}
		receipt, err := memoryService.RevokeMemoryRecord(ctx, path.companyID, path.resourceID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.resourceID, err)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSON(response, http.StatusAccepted, receipt)
	case "takeover_leases.snapshot", "takeover_leases.release":
		takeoverService, ok := service.(control.TaskTakeoverLeaseService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("Task takeover service is unavailable"))
			return
		}
		var err error
		var item kernel.TaskTakeoverLease
		if path.endpoint == "takeover_leases.snapshot" {
			var input control.TaskTakeoverSnapshotCommand
			if err = decodeJSON(response, request, &input); err == nil {
				item, err = takeoverService.SubmitTaskTakeoverSnapshot(ctx, path.companyID, path.missionID, path.resourceID, input)
			}
		} else {
			var input control.TaskTakeoverLeaseCommand
			if err = decodeJSON(response, request, &input); err == nil {
				item, err = takeoverService.ReleaseTaskTakeoverLease(ctx, path.companyID, path.missionID, path.resourceID, input)
			}
		}
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "missions.inputs.directory":
		inputService, ok := service.(control.MissionDirectoryInputService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("mission directory input service is unavailable"))
			return
		}
		input, files, err := parseMissionDirectoryUpload(response, request, path.missionID)
		if errors.Is(err, errMissionDirectoryInputTooLarge) {
			writeError(response, http.StatusRequestEntityTooLarge, "mission directory input exceeds the 7 MB file limit")
			return
		}
		if err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		receipt, err := inputService.UploadMissionDirectoryInput(ctx, path.companyID, input, files)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.create":
		var input control.CreateMissionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, "", err)
			return
		}
		receipt, err := service.CreateMission(ctx, path.companyID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, "", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.routines":
		routineService, ok := service.(control.DailyRoutineCommandService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("daily Routine service is unavailable"))
			return
		}
		var input control.CreateDailyRoutineRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		receipt, err := routineService.CreateDailyRoutine(ctx, path.companyID, path.missionID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.routine.instruction":
		routineService, ok := service.(control.DailyRoutineCommandService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("daily Routine service is unavailable"))
			return
		}
		var input control.SetDailyRoutineTaskInstructionRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		receipt, err := routineService.SetDailyRoutineTaskInstruction(ctx, path.companyID, path.missionID, path.resourceID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.start":
		var input control.MissionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		input.MissionID = path.missionID
		receipt, err := service.StartMission(ctx, path.companyID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.cancel":
		var input control.MissionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		input.MissionID = path.missionID
		receipt, err := service.CancelMission(ctx, path.companyID, input)
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "missions.pause", "missions.resume":
		lifecycleService, ok := service.(control.MissionLifecycleService)
		if !ok {
			writeCommandError(response, http.StatusNotImplemented, path.companyID, path.missionID, errors.New("mission lifecycle commands are unavailable"))
			return
		}
		var input control.MissionCommandRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandError(response, http.StatusBadRequest, path.companyID, path.missionID, err)
			return
		}
		input.MissionID = path.missionID
		var receipt control.CommandReceipt
		var err error
		if path.endpoint == "missions.pause" {
			receipt, err = lifecycleService.PauseMission(ctx, path.companyID, input)
		} else {
			receipt, err = lifecycleService.ResumeMission(ctx, path.companyID, input)
		}
		if err != nil {
			writeCommandError(response, commandStatus(err), path.companyID, path.missionID, err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	default:
		writeError(response, http.StatusNotFound, "workbench route not found")
	}
}

func serveActivityStream(model ReadModel, response http.ResponseWriter, request *http.Request, companyID string) {
	streamer, ok := model.(ActivityStreamer)
	if !ok {
		writeJSON(response, http.StatusNotImplemented, map[string]string{"error": "live activity stream is unavailable"})
		return
	}
	cursor, err := parseStreamCursor(request)
	if err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	flusher, ok := response.(http.Flusher)
	if !ok {
		writeError(response, http.StatusInternalServerError, "live activity stream is unsupported by this server")
		return
	}
	response.Header().Set("Cache-Control", "no-cache, no-store")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	flusher.Flush()
	err = streamer.StreamActivity(request.Context(), companyID, cursor, func(event ActivityEvent) error {
		payload, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(response, "id: %s\nevent: activity\ndata: %s\n\n", event.CompanySeq, payload); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return
	}
}

func parseStreamCursor(request *http.Request) (int64, error) {
	value := request.Header.Get("Last-Event-ID")
	if value == "" {
		value = request.URL.Query().Get("cursor")
	}
	if strings.HasPrefix(value, "company-seq:") {
		return parseSnapshotCursor(value)
	}
	if value == "" {
		return 0, fmt.Errorf("stream cursor is required")
	}
	sequence, err := strconv.ParseInt(value, 10, 64)
	if err != nil || sequence < 0 {
		return 0, fmt.Errorf("invalid stream cursor")
	}
	return sequence, nil
}

func parsePath(path string) (parsedPath, bool) {
	const prefix = "/api/workbench/companies/"
	if !strings.HasPrefix(path, prefix) {
		return parsedPath{}, false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return parsedPath{}, false
	}
	companyID, err := url.PathUnescape(parts[0])
	if err != nil {
		return parsedPath{}, false
	}
	if len(parts) == 3 && parts[1] == "settings" && parts[2] == "models" {
		return parsedPath{companyID: companyID, endpoint: "settings.models"}, true
	}
	if len(parts) == 3 && parts[1] == "memory" && parts[2] == "corrections" {
		return parsedPath{companyID: companyID, endpoint: "memory.corrections"}, true
	}
	if len(parts) == 2 && parts[1] == "problem-budgets" {
		return parsedPath{companyID: companyID, endpoint: "problem.budgets"}, true
	}
	if len(parts) == 2 && parts[1] == "mission-budgets" {
		return parsedPath{companyID: companyID, endpoint: "mission.budgets"}, true
	}
	if len(parts) == 4 && parts[1] == "mission-budgets" && parts[3] == "allocations" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: missionID, endpoint: "mission.budget.allocate"}, true
	}
	if len(parts) == 4 && parts[1] == "mission-budgets" && parts[3] == "closing-reserve" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: missionID, endpoint: "mission.budget.closing_reserve"}, true
	}
	if len(parts) == 4 && parts[1] == "problem-budgets" && parts[3] == "allocations" {
		problemKey, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: problemKey, endpoint: "problem.budget.allocate"}, true
	}
	if len(parts) == 6 && parts[1] == "problem-budgets" && parts[3] == "tasks" && parts[5] == "allocations" {
		problemKey, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		taskID, err := url.PathUnescape(parts[4])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: problemKey, targetID: taskID, endpoint: "problem.budget.task.allocate"}, true
	}
	if len(parts) == 6 && parts[1] == "problem-budgets" && parts[3] == "tasks" && parts[5] == "budget-closure" {
		problemKey, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		taskID, err := url.PathUnescape(parts[4])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: problemKey, targetID: taskID, endpoint: "problem.budget.task.close_incomplete"}, true
	}
	if len(parts) == 4 && parts[1] == "problem-budgets" && parts[3] == "closing-reserve" {
		problemKey, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: problemKey, endpoint: "problem.budget.closing_reserve"}, true
	}
	if len(parts) == 2 && (parts[1] == "overview" || parts[1] == "activity" || parts[1] == "stream" || parts[1] == "missions" || parts[1] == "organization" || parts[1] == "archive" || parts[1] == "settings" || parts[1] == "instructions" || parts[1] == "collaboration" || parts[1] == "operations" || parts[1] == "notifications" || parts[1] == "capabilities" || parts[1] == "environments" || parts[1] == "feedback" || parts[1] == "domain-workflows") {
		if parts[1] == "missions" {
			return parsedPath{companyID: companyID, endpoint: "missions.create"}, true
		}
		if parts[1] == "domain-workflows" {
			return parsedPath{companyID: companyID, endpoint: "domain_workflows"}, true
		}
		return parsedPath{companyID: companyID, endpoint: parts[1]}, true
	}
	if len(parts) == 3 && parts[1] == "domain-workflows" && parts[2] == "research-simulations" {
		return parsedPath{companyID: companyID, endpoint: "domain.research.simulation"}, true
	}
	if len(parts) == 3 && parts[1] == "domain-workflows" {
		switch parts[2] {
		case "content-sources":
			return parsedPath{companyID: companyID, endpoint: "domain.content.source.authorization"}, true
		case "content-drafts":
			return parsedPath{companyID: companyID, endpoint: "domain.content.draft"}, true
		case "content-reviews":
			return parsedPath{companyID: companyID, endpoint: "domain.content.review"}, true
		case "content-publications":
			return parsedPath{companyID: companyID, endpoint: "domain.content.publication"}, true
		case "content-corrections":
			return parsedPath{companyID: companyID, endpoint: "domain.content.correction"}, true
		case "content-feedback":
			return parsedPath{companyID: companyID, endpoint: "domain.content.feedback"}, true
		}
	}
	if len(parts) == 4 && parts[1] == "domain-workflows" && parts[2] != "" && parts[3] == "qualification" {
		profileID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: profileID, endpoint: "domain.profile.qualification"}, true
	}
	if len(parts) == 2 && parts[1] == "domain-evidence" {
		return parsedPath{companyID: companyID, endpoint: "domain.evidence"}, true
	}
	if len(parts) == 4 && parts[1] == "domain-evidence" && parts[2] != "" && parts[3] == "review" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, endpoint: "domain.evidence.review"}, true
	}
	if len(parts) == 4 && parts[1] == "domain-evidence" && parts[2] != "" && parts[3] == "assessment" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, endpoint: "domain.evidence.assessment"}, true
	}
	if len(parts) == 6 && parts[1] == "domain-evidence" && parts[2] != "" && parts[3] == "evidence" && parts[4] != "" && parts[5] == "preview" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		area, err := url.PathUnescape(parts[4])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, evidenceArea: area, endpoint: "domain.evidence.preview"}, true
	}
	if len(parts) == 7 && parts[1] == "domain-evidence" && parts[2] != "" && parts[3] == "evidence" && parts[4] != "" && parts[5] == "preview" && parts[6] == "entries" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		area, err := url.PathUnescape(parts[4])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, evidenceArea: area, endpoint: "domain.evidence.preview_entries"}, true
	}
	if len(parts) == 3 && parts[1] == "capabilities" && (parts[2] == "skills" || parts[2] == "mcp" || parts[2] == "mcp-packages" || parts[2] == "qualify" || parts[2] == "decide" || parts[2] == "bind" || parts[2] == "unbind" || parts[2] == "runtime-approve" || parts[2] == "runtime-observe" || parts[2] == "runtime-observe-http") {
		return parsedPath{companyID: companyID, endpoint: "capabilities." + parts[2]}, true
	}
	if len(parts) == 3 && parts[1] == "feedback" && parts[2] == "sources" {
		return parsedPath{companyID: companyID, endpoint: "feedback.sources"}, true
	}
	if len(parts) == 3 && parts[1] == "feedback" && parts[2] == "backlog" {
		return parsedPath{companyID: companyID, endpoint: "feedback.backlog"}, true
	}
	if len(parts) == 3 && parts[1] == "feedback" && parts[2] == "collection-policy" {
		return parsedPath{companyID: companyID, endpoint: "feedback.collection-policy"}, true
	}
	if len(parts) == 3 && parts[1] == "feedback" && parts[2] == "credentials" {
		return parsedPath{companyID: companyID, endpoint: "feedback.credentials"}, true
	}
	if len(parts) == 4 && parts[1] == "feedback" && parts[2] == "credentials" && parts[3] == "delete" {
		return parsedPath{companyID: companyID, endpoint: "feedback.credentials.delete"}, true
	}
	if len(parts) == 5 && parts[1] == "feedback" && parts[2] == "sources" && parts[3] != "" && (parts[4] == "probe" || parts[4] == "decision" || parts[4] == "scan") {
		sourceID, err := url.PathUnescape(parts[3])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: sourceID, endpoint: "feedback.source." + parts[4]}, true
	}
	if len(parts) == 4 && parts[1] == "environments" && parts[2] != "" && parts[3] == "policy" {
		revisionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: revisionID, endpoint: "environments.policy"}, true
	}
	if len(parts) == 4 && parts[1] == "environments" && parts[2] != "" && parts[3] == "executor-qualification" {
		revisionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: revisionID, endpoint: "environments.executor_qualification"}, true
	}
	if len(parts) == 4 && parts[1] == "environments" && parts[2] != "" && parts[3] == "preparation" {
		revisionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: revisionID, endpoint: "environments.preparation"}, true
	}
	if len(parts) == 3 && parts[1] == "notifications" && (parts[2] == "route" || parts[2] == "test") {
		return parsedPath{companyID: companyID, endpoint: "notifications." + parts[2]}, true
	}
	if len(parts) == 4 && parts[1] == "human-interventions" && parts[2] != "" && (parts[3] == "acknowledge" || parts[3] == "resolve") {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "human_interventions." + parts[3]}, true
	}
	if len(parts) == 5 && parts[1] == "missions" && parts[2] != "" && parts[3] == "inputs" && parts[4] == "directory" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions.inputs.directory"}, true
	}
	if len(parts) == 4 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "input-manifest" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.input_manifest"}, true
	}
	if len(parts) == 4 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "memory-impact" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.memory_impact"}, true
	}
	if len(parts) == 4 && parts[1] == "memory" && parts[2] != "" && parts[3] == "revoke" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, endpoint: "memory.record_revoke"}, true
	}
	if len(parts) == 4 && parts[1] == "memory" && parts[2] != "" && parts[3] == "revocation" {
		recordID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: recordID, endpoint: "memory.record_revocation_preview"}, true
	}
	if len(parts) == 4 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "memory-revalidation" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.memory_revalidation"}, true
	}
	if len(parts) == 5 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "memory-revalidation" && parts[4] == "preview" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.memory_revalidation_preview"}, true
	}
	if len(parts) == 4 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "jobs" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.jobs"}, true
	}
	if len(parts) == 4 && parts[1] == "tasks" && parts[2] != "" && parts[3] == "environment-handovers" {
		resourceID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "tasks.handovers"}, true
	}
	if len(parts) == 4 && parts[1] == "jobs" && parts[2] != "" && (parts[3] == "logs" || parts[3] == "stop" || parts[3] == "browser-session") {
		jobID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		endpoint := "jobs." + strings.ReplaceAll(parts[3], "-", "_")
		return parsedPath{companyID: companyID, resourceID: jobID, endpoint: endpoint}, true
	}
	if len(parts) == 4 && parts[1] == "missions" && parts[2] != "" && parts[3] == "inputs" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions.inputs"}, true
	}
	if len(parts) == 4 && parts[1] == "missions" && parts[2] != "" && parts[3] == "routines" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions.routines"}, true
	}
	if len(parts) == 6 && parts[1] == "missions" && parts[2] != "" && parts[3] == "routines" && parts[4] != "" && parts[5] == "instruction" {
		missionID, missionErr := url.PathUnescape(parts[2])
		routineID, routineErr := url.PathUnescape(parts[4])
		if missionErr != nil || routineErr != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, resourceID: routineID, endpoint: "missions.routine.instruction"}, true
	}
	if len(parts) == 4 && parts[1] == "missions" && parts[2] != "" && parts[3] == "change-requests" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions.change_requests"}, true
	}
	if len(parts) == 4 && parts[1] == "missions" && parts[2] != "" && parts[3] == "takeover-leases" {
		missionID, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions.takeover_leases"}, true
	}
	if len(parts) == 6 && parts[1] == "missions" && parts[2] != "" && parts[3] == "tasks" && parts[4] != "" && parts[5] == "takeover-lease" {
		missionID, missionErr := url.PathUnescape(parts[2])
		taskID, taskErr := url.PathUnescape(parts[4])
		if missionErr != nil || taskErr != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, resourceID: taskID, endpoint: "tasks.takeover_lease"}, true
	}
	if len(parts) == 6 && parts[1] == "missions" && parts[2] != "" && parts[3] == "takeover-leases" && parts[4] != "" && (parts[5] == "snapshot" || parts[5] == "release") {
		missionID, missionErr := url.PathUnescape(parts[2])
		leaseID, leaseErr := url.PathUnescape(parts[4])
		if missionErr != nil || leaseErr != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, resourceID: leaseID, endpoint: "takeover_leases." + parts[5]}, true
	}
	if len(parts) == 6 && parts[1] == "missions" && parts[2] != "" && parts[3] == "change-requests" && parts[4] != "" && (parts[5] == "consider" || parts[5] == "decline" || parts[5] == "apply") {
		missionID, missionErr := url.PathUnescape(parts[2])
		requestID, requestErr := url.PathUnescape(parts[4])
		if missionErr != nil || requestErr != nil {
			return parsedPath{}, false
		}
		return parsedPath{companyID: companyID, missionID: missionID, resourceID: requestID, endpoint: "mission_change_requests." + parts[5]}, true
	}
	if len(parts) != 4 || parts[1] != "missions" || parts[2] == "" || parts[3] == "" {
		if len(parts) == 4 && parts[2] != "" && parts[1] == "artifacts" && (parts[3] == "manifest" || parts[3] == "download") {
			resourceID, err := url.PathUnescape(parts[2])
			if err != nil {
				return parsedPath{}, false
			}
			return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: "artifacts." + parts[3]}, true
		}
		if len(parts) == 4 && parts[2] != "" && ((parts[1] == "tasks" && (parts[3] == "workspace" || parts[3] == "checkpoints")) || (parts[1] == "artifacts" && parts[3] == "detail")) {
			resourceID, err := url.PathUnescape(parts[2])
			if err != nil {
				return parsedPath{}, false
			}
			endpoint := "workspace"
			if parts[1] == "artifacts" {
				endpoint = "artifact"
			}
			return parsedPath{companyID: companyID, resourceID: resourceID, endpoint: endpoint}, true
		}
		return parsedPath{}, false
	}
	missionID, err := url.PathUnescape(parts[2])
	if err != nil || (parts[3] != "start" && parts[3] != "cancel" && parts[3] != "pause" && parts[3] != "resume") {
		return parsedPath{}, false
	}
	return parsedPath{companyID: companyID, missionID: missionID, endpoint: "missions." + parts[3]}, true
}

func parseActivityQuery(request *http.Request, companyID string) (ActivityQuery, error) {
	values := request.URL.Query()
	snapshotCursor := values.Get("snapshot_cursor")
	if snapshotCursor == "" {
		return ActivityQuery{}, fmt.Errorf("snapshot cursor is required")
	}
	if _, err := parseSnapshotCursor(snapshotCursor); err != nil {
		return ActivityQuery{}, err
	}
	limitText := values.Get("limit")
	if limitText == "" {
		return ActivityQuery{}, fmt.Errorf("activity limit is required")
	}
	limit, err := strconv.Atoi(limitText)
	if err != nil {
		return ActivityQuery{}, fmt.Errorf("activity limit must be an integer")
	}
	if err := validateActivityLimit(limit); err != nil {
		return ActivityQuery{}, err
	}
	var cursor *string
	if raw, exists := values["cursor"]; exists {
		if len(raw) != 1 {
			return ActivityQuery{}, fmt.Errorf("activity cursor must be singular")
		}
		value := raw[0]
		if _, err := parseActivityCursor(&value); err != nil {
			return ActivityQuery{}, err
		}
		cursor = &value
	}
	return ActivityQuery{CompanyID: companyID, SnapshotCursor: snapshotCursor, Cursor: cursor, Limit: limit}, nil
}

func writeModelError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, errCompanyNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, errInvalidActivityCursor) {
		status = http.StatusBadRequest
	} else if errors.Is(err, core.Conflict) {
		status = http.StatusConflict
	}
	writeError(response, status, err.Error())
}

type commandError struct {
	Error        string `json:"error"`
	Code         string `json:"code"`
	TargetType   string `json:"targetType,omitempty"`
	TargetID     string `json:"targetId,omitempty"`
	CurrentState string `json:"currentState,omitempty"`
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return fmt.Errorf("command content type must be application/json")
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16*1024)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid command body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("invalid command body: multiple JSON values")
		}
		return fmt.Errorf("invalid command body: %w", err)
	}
	return nil
}

func commandStatus(err error) int {
	switch {
	case errors.Is(err, control.ErrGitHubFeedbackUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, control.ErrGitHubFeedbackCredentialStoreUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, core.Malformed):
		return http.StatusBadRequest
	case errors.Is(err, core.OutOfScope):
		return http.StatusNotFound
	case errors.Is(err, core.Conflict), errors.Is(err, core.Denied), errors.Is(err, core.StaleEpoch):
		return http.StatusConflict
	case errors.Is(err, core.TooLarge):
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func commandCode(err error) string {
	if errors.Is(err, control.ErrGitHubFeedbackUnavailable) {
		return "GITHUB_FEEDBACK_UNAVAILABLE"
	}
	if errors.Is(err, control.ErrGitHubFeedbackCredentialStoreUnavailable) {
		return "GITHUB_CREDENTIAL_STORE_UNAVAILABLE"
	}
	for _, code := range []core.Code{core.Malformed, core.TooLarge, core.OutOfScope, core.StaleEpoch, core.Conflict, core.Denied, core.Integrity, core.ToolCallBudgetExceeded} {
		if errors.Is(err, code) {
			return string(code)
		}
	}
	return "COMMAND_FAILED"
}

func writeCommandError(response http.ResponseWriter, status int, companyID, missionID string, err error) {
	writeCommandErrorForTarget(response, status, companyID, missionID, "mission", err)
}

func writeCommandErrorForTarget(response http.ResponseWriter, status int, companyID, targetID, targetType string, err error) {
	if targetID == "" {
		targetID = companyID
	}
	currentState := ""
	var conflict core.ConflictError
	if errors.As(err, &conflict) {
		currentState = conflict.CurrentState
	}
	writeJSON(response, status, commandError{Error: err.Error(), Code: commandCode(err), TargetType: targetType, TargetID: targetID, CurrentState: currentState})
}

func serveCompanyCollection(service control.CommandService, response http.ResponseWriter, request *http.Request) {
	organizationService, ok := asOrganizationService(service)
	if !ok {
		writeError(response, http.StatusNotImplemented, "organization service is unavailable")
		return
	}
	ctx := request.Context()
	if request.Method == http.MethodGet {
		companies, err := organizationService.ListCompanies(ctx)
		if err != nil {
			writeModelError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, companies)
		return
	}
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var input control.CreateCompanyRequest
	if err := decodeJSON(response, request, &input); err != nil {
		writeCommandErrorForTarget(response, http.StatusBadRequest, input.ID, input.ID, "company", err)
		return
	}
	receipt, err := organizationService.CreateCompany(ctx, input)
	if err != nil {
		writeCommandErrorForTarget(response, commandStatus(err), input.ID, input.ID, "company", err)
		return
	}
	writeJSON(response, http.StatusAccepted, receipt)
}

func asOrganizationService(service control.CommandService) (control.OrganizationService, bool) {
	organizationService, ok := service.(control.OrganizationService)
	return organizationService, ok
}

func asRuntimeSettingsService(service control.CommandService) (control.RuntimeSettingsService, bool) {
	settingsService, ok := service.(control.RuntimeSettingsService)
	return settingsService, ok
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

var errCompanyNotFound = errors.New("company scope not found")
