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
	companyID  string
	endpoint   string
	missionID  string
	resourceID string
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
	case "notifications.test":
		notificationService, ok := service.(control.NotificationCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("notification command service is unavailable"))
			return
		}
		var input control.TestNotificationRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := notificationService.TestNotification(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "notifications.dispatch":
		notificationService, ok := service.(control.NotificationCommandService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("notification command service is unavailable"))
			return
		}
		var input control.DispatchNotificationsRequest
		if err := decodeJSON(response, request, &input); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := notificationService.DispatchNotifications(ctx, path.companyID, input)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, receipt)
	case "capabilities.skills.manifest":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var document control.ImportSkillDocumentRequest
		if err := decodeJSON(response, request, &document); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.ImportSkillDocument(ctx, path.companyID, document)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.skills.library":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var library control.ImportSkillLibraryRequest
		if err := decodeJSON(response, request, &library); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		result, err := capabilityService.ImportSkillLibrary(ctx, path.companyID, library)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, result)
	case "capabilities.tools.describe":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var described control.DescribeVisibleToolRequest
		if err := decodeJSON(response, request, &described); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		toolView, err := capabilityService.DescribeVisibleTool(ctx, path.companyID, described)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, toolView)
	case "capabilities.tools.list":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var tools control.ListVisibleToolsRequest
		if err := decodeJSON(response, request, &tools); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		toolPage, err := capabilityService.ListVisibleTools(ctx, path.companyID, tools)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, toolPage)
	case "capabilities.tools.call":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var call control.CallMCPToolRequest
		if err := decodeJSON(response, request, &call); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		receipt, err := capabilityService.CallMCPTool(ctx, path.companyID, call)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, receipt)
	case "capabilities.tools.call-qualification":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var qual control.UpsertMCPToolCallQualificationRequest
		if err := decodeJSON(response, request, &qual); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.UpsertMCPToolCallQualification(ctx, path.companyID, qual)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, item)
	case "capabilities.tools.call-qualifications":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var list control.ListMCPToolCallQualificationsRequest
		if err := decodeJSON(response, request, &list); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		items, err := capabilityService.ListMCPToolCallQualifications(ctx, path.companyID, list)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, items)
	case "capabilities.tools.call-receipts":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var list control.ListMCPToolCallReceiptsRequest
		if err := decodeJSON(response, request, &list); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		items, err := capabilityService.ListMCPToolCallReceipts(ctx, path.companyID, list)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, items)
	case "capabilities.tools.call-receipt":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var get control.GetMCPToolCallReceiptRequest
		if err := decodeJSON(response, request, &get); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.GetMCPToolCallReceipt(ctx, path.companyID, get)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, item)
	case "capabilities.resources.list":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var resources control.ListVisibleResourcesRequest
		if err := decodeJSON(response, request, &resources); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		resourcePage, err := capabilityService.ListVisibleResources(ctx, path.companyID, resources)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, resourcePage)
	case "capabilities.skills.list":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var list control.ListVisibleSkillsRequest
		if err := decodeJSON(response, request, &list); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		page, err := capabilityService.ListVisibleSkills(ctx, path.companyID, list)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, page)
	case "capabilities.skills.load":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var load control.LoadSkillRequest
		if err := decodeJSON(response, request, &load); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.LoadSkill(ctx, path.companyID, load)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.skills.reference":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var read control.ReadSkillReferenceRequest
		if err := decodeJSON(response, request, &read); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.ReadSkillReference(ctx, path.companyID, read)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusOK, item)
	case "capabilities.skills.approve", "capabilities.skills.revoke":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var statusRequest control.SkillRevisionStatusRequest
		if err := decodeJSON(response, request, &statusRequest); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		action := "approve"
		if path.endpoint == "capabilities.skills.revoke" {
			action = "revoke"
		}
		item, err := capabilityService.SetSkillRevisionStatus(ctx, path.companyID, statusRequest, action)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.mcp.qualify":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var probe control.QualifyMCPRequest
		if err := decodeJSON(response, request, &probe); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.QualifyMCP(ctx, path.companyID, probe)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.bindings":
		capabilityService, ok := service.(control.CapabilityService)
		if !ok {
			writeCommandErrorForTarget(response, http.StatusNotImplemented, path.companyID, path.companyID, "company", errors.New("capability catalog is unavailable"))
			return
		}
		var binding control.UpsertCapabilityBindingRequest
		if err := decodeJSON(response, request, &binding); err != nil {
			writeCommandErrorForTarget(response, http.StatusBadRequest, path.companyID, path.companyID, "company", err)
			return
		}
		item, err := capabilityService.UpsertCapabilityBinding(ctx, path.companyID, binding)
		if err != nil {
			writeCommandErrorForTarget(response, commandStatus(err), path.companyID, path.companyID, "company", err)
			return
		}
		writeJSON(response, http.StatusAccepted, item)
	case "capabilities.skills":
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
	if len(parts) == 2 && (parts[1] == "overview" || parts[1] == "activity" || parts[1] == "stream" || parts[1] == "missions" || parts[1] == "organization" || parts[1] == "archive" || parts[1] == "settings" || parts[1] == "instructions" || parts[1] == "collaboration" || parts[1] == "operations" || parts[1] == "notifications" || parts[1] == "capabilities") {
		if parts[1] == "missions" {
			return parsedPath{companyID: companyID, endpoint: "missions.create"}, true
		}
		return parsedPath{companyID: companyID, endpoint: parts[1]}, true
	}
	if len(parts) == 3 && parts[1] == "capabilities" && (parts[2] == "skills" || parts[2] == "mcp" || parts[2] == "bindings") {
		return parsedPath{companyID: companyID, endpoint: "capabilities." + parts[2]}, true
	}
	if len(parts) == 4 && parts[1] == "capabilities" && parts[2] == "skills" && (parts[3] == "manifest" || parts[3] == "library" || parts[3] == "approve" || parts[3] == "revoke" || parts[3] == "load" || parts[3] == "reference" || parts[3] == "list") {
		return parsedPath{companyID: companyID, endpoint: "capabilities.skills." + parts[3]}, true
	}
	if len(parts) == 4 && parts[1] == "capabilities" && parts[2] == "tools" && (parts[3] == "list" || parts[3] == "describe" || parts[3] == "call" || parts[3] == "call-qualification" || parts[3] == "call-qualifications" || parts[3] == "call-receipts" || parts[3] == "call-receipt") {
		return parsedPath{companyID: companyID, endpoint: "capabilities.tools." + parts[3]}, true
	}
	if len(parts) == 4 && parts[1] == "capabilities" && parts[2] == "resources" && parts[3] == "list" {
		return parsedPath{companyID: companyID, endpoint: "capabilities.resources.list"}, true
	}
	if len(parts) == 4 && parts[1] == "capabilities" && parts[2] == "mcp" && parts[3] == "qualify" {
		return parsedPath{companyID: companyID, endpoint: "capabilities.mcp.qualify"}, true
	}
	if len(parts) == 3 && parts[1] == "notifications" && (parts[2] == "route" || parts[2] == "test" || parts[2] == "dispatch") {
		return parsedPath{companyID: companyID, endpoint: "notifications." + parts[2]}, true
	}
	if len(parts) != 4 || parts[1] != "missions" || parts[2] == "" || parts[3] == "" {
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
	if err != nil || (parts[3] != "start" && parts[3] != "cancel") {
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
