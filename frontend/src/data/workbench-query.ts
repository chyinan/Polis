// pattern: Imperative Shell

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useEffect, useState} from 'react';
import type {ImportStdioMCPPackageOptions, ObserveStdioMCPRuntimeOptions, ObserveStreamableHTTPMCPRuntimeOptions} from './workbench-api';
import type {BindEmployeeCapabilityOptions, CompanyDraftOptions, ConfigureNotificationRouteOptions, CreateMissionOptions, CreateMissionChangeRequestOptions, CreateTaskEnvironmentHandoverOptions, CreateOperatorInstructionOptions, CreateTaskTakeoverLeaseOptions, DecideCapabilityOptions, DecideGitHubFeedbackSourceOptions, EnvironmentExecutorQualificationOptions, EnvironmentPolicyDecisionOptions, EnsureEnvironmentOptions, ImportSkillOptions, ImportReadOnlySkillPackageOptions, MissionChangeRequestCommandOptions, MissionChangeRequestQueryOptions, MissionCommandOptions, PollGitHubFeedbackSourceOptions, ProbeGitHubFeedbackSourceOptions, QualifyCapabilityOptions, RecordDomainEvidenceOptions, RecordDomainEvidenceReviewOptions, RecordDomainEvidenceSubstantiveAssessmentOptions, RecordDomainProfileQualificationOptions, RegisterGitHubFeedbackSourceOptions, ReleaseTaskTakeoverLeaseOptions, ReviewCapabilityRevocationOptions, SetGitHubFeedbackBacklogStatusOptions, SetHumanInterventionStateOptions, TaskCrossBackendHandoversQueryOptions, TaskInputManifestQueryOptions, TaskJobLogsQueryOptions, TaskJobRunsQueryOptions, StartTaskJobRunOptions, StopTaskJobRunOptions, TaskTakeoverLeaseQueryOptions, TaskTakeoverSnapshotOptions, UploadMissionDirectoryInputOptions, UploadMissionInputOptions, UpdateCompanyOptions, ArchiveCompanyOptions, RegisterMCPOptions, TestNotificationOptions, UpdateRuntimeSettingsOptions, WorkbenchApi} from './workbench-api';
import type {ActivityStreamStatus} from './workbench-api';
import type {MissionCloseoutOptions} from './workbench-api';
import type {CreateProjectJobBrowserSessionOptions} from './workbench-api';
import type {SetGitHubFeedbackCollectionPolicyOptions} from './workbench-api';
import type {RunResearchSimulationOptions} from './workbench-api';
import type {RecordContentReviewOptions, RegisterContentDraftOptions, SetContentSourceAuthorizationOptions} from './workbench-api';
import type {RecordContentCorrectionOptions, RecordContentFeedbackOptions, SimulateContentPublicationOptions} from './workbench-api';
import type {AllocateProblemToolCallsOptions, AllocateTaskToolCallsOptions, CloseTaskToolBudgetIncompleteOptions, CreateDailyRoutineOptions, SetDailyRoutineTaskInstructionOptions, SetProblemToolCallClosingReserveOptions} from './workbench-api';
import type {RevalidateMemoryTaskOptions} from './workbench-api';
import type {ChangeMissionToolCallBudgetOptions, MissionToolCallBudgetQueryOptions} from './workbench-api';
import type {SetMissionToolCallClosingReserveOptions} from './workbench-api';
import type {ChangeCompanyToolCallBudgetOptions} from './workbench-api';
import type {SetCompanyToolCallClosingReserveOptions} from './workbench-api';
import type {ProposeMemoryCorrectionOptions, ReviewMemoryCorrectionOptions} from './workbench-api';

export function useCompanyOverview(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'company-overview', companyId],
    queryFn: () => api.getCompanyOverview({companyId}),
    staleTime: 30_000,
    refetchOnMount: 'always',
  });
}

export function useCompanyList(api: WorkbenchApi) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'companies'],
    queryFn: () => api.listCompanies(),
    staleTime: 10_000,
    refetchOnMount: 'always',
  });
}

export function useCapabilityCatalog(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'capability-catalog', companyId],
    queryFn: () => api.listCapabilityCatalog({companyId}),
    staleTime: 10_000,
  });
}

export function useImportSkill(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ImportSkillOptions, 'companyId'>) => api.importSkill({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useRegisterMCP(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RegisterMCPOptions, 'companyId'>) => api.registerMCP({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useRuntimeSettings(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'runtime-settings', companyId],
    queryFn: () => api.getRuntimeSettings({companyId}),
    staleTime: 10_000,
    refetchOnMount: 'always',
  });
}

export function useCodexModelCatalog(api: WorkbenchApi, companyId: string, enabled: boolean) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'codex-model-catalog', companyId],
    queryFn: () => api.getCodexModelCatalog({companyId}),
    enabled,
    staleTime: 60_000,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

export function useActivityEvents(api: WorkbenchApi, companyId: string, cursor: string | null, limit: number, snapshotCursor: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'activity', companyId, {cursor, limit, snapshotCursor}],
    queryFn: () => api.listActivityEvents({companyId, cursor, limit, snapshotCursor}),
    enabled: api.mode === 'simulated' || snapshotCursor !== null,
    staleTime: 10_000,
  });
}

export function useActivityStream(api: WorkbenchApi, companyId: string, snapshotCursor: string | null): ActivityStreamStatus {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<ActivityStreamStatus>('closed');

  useEffect(() => {
    if (api.mode !== 'real' || snapshotCursor === null) {
      setStatus('closed');
      return undefined;
    }
    return api.subscribeToActivityEvents(
      {companyId, cursor: snapshotCursor},
      () => {
        void Promise.all([
          queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
          queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
        ]);
      },
      status => {
        setStatus(status);
        if (status === 'incompatible') {
          void Promise.all([
            queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
            queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
          ]);
        }
      },
    );
  }, [api, companyId, queryClient, snapshotCursor]);

  return status;
}

export function useCreateMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CreateMissionOptions, 'companyId'>) => api.createMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCreateCompany(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: CompanyDraftOptions) => api.createCompany(options),
    onSettled: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'companies']}),
  });
}

export function useUpdateCompany(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: UpdateCompanyOptions) => api.updateCompany(options),
    onSettled: async (_receipt, _error, options) => {
      await Promise.all([
        queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'companies']}),
        queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', options.companyId]}),
      ]);
    },
  });
}

export function useArchiveCompany(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: ArchiveCompanyOptions) => api.archiveCompany(options),
    onSettled: async (_receipt, _error, options) => {
      await Promise.all(archiveInvalidationKeys(api.mode, options.companyId).map(queryKey => queryClient.invalidateQueries({queryKey})));
    },
  });
}

export function archiveInvalidationKeys(apiMode: WorkbenchApi['mode'], companyId: string): ReadonlyArray<ReadonlyArray<unknown>> {
  return [
    ['workbench', apiMode, 'companies'],
    ['workbench', apiMode, 'company-overview', companyId],
  ];
}

export function useUpdateRuntimeSettings(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: UpdateRuntimeSettingsOptions) => api.updateRuntimeSettings(options),
    onSettled: async (_receipt, _error, options) => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'runtime-settings', options.companyId]}),
  });
}

export function useOperatorInstructions(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'operator-instructions', companyId],
    queryFn: () => api.listOperatorInstructions({companyId}),
    staleTime: 5_000,
  });
}

export function useSendOperatorInstruction(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CreateOperatorInstructionOptions, 'companyId'>) => api.sendOperatorInstruction({...options, companyId}),
    onSettled: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'operator-instructions', companyId]}),
  });
}

export function useCollaboration(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'collaboration', companyId],
    queryFn: () => api.listCollaboration({companyId}),
    staleTime: 5_000,
  });
}

export function useMissionInputs(api: WorkbenchApi, companyId: string, missionId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'mission-inputs', companyId, missionId],
    queryFn: () => api.listMissionInputs({companyId, missionId}),
    enabled: missionId.trim() !== '',
    staleTime: 30_000,
  });
}

export function useUploadMissionInput(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<UploadMissionInputOptions, 'companyId' | 'missionId'>) => api.uploadMissionInput({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-inputs', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useDailyRoutines(api: WorkbenchApi, companyId: string, missionId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'daily-routines', companyId, missionId],
    queryFn: () => api.listDailyRoutines({companyId, missionId}),
    staleTime: 10_000,
    refetchOnMount: 'always',
  });
}

export function useCreateDailyRoutine(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CreateDailyRoutineOptions, 'companyId' | 'missionId'>) => api.createDailyRoutine({...options, companyId, missionId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'daily-routines', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useSetDailyRoutineTaskInstruction(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetDailyRoutineTaskInstructionOptions, 'companyId' | 'missionId'>) => api.setDailyRoutineTaskInstruction({...options, companyId, missionId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'daily-routines', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useMemoryTaskStatus(api: WorkbenchApi, companyId: string, taskId: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-memory-impact', companyId, taskId],
    queryFn: () => api.getMemoryTaskStatus({companyId, taskId: taskId ?? ''}),
    enabled: taskId !== null && taskId.trim() !== '',
    staleTime: 5_000,
    refetchOnMount: 'always',
  });
}

export function useMemoryCorrectionQueue(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'memory-corrections', companyId],
    queryFn: () => api.listMemoryCorrections({companyId}),
    enabled: api.mode === 'real' && companyId.trim() !== '',
    staleTime: 0,
    refetchOnMount: 'always',
  });
}

export function useProposeMemoryCorrection(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ProposeMemoryCorrectionOptions, 'companyId'>) => api.proposeMemoryCorrection({...options, companyId}),
    onSuccess: async () => invalidateMemoryCorrectionQueries(queryClient, api, companyId),
  });
}

export function useReviewMemoryCorrection(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ReviewMemoryCorrectionOptions, 'companyId'>) => api.reviewMemoryCorrection({...options, companyId}),
    onSuccess: async () => invalidateMemoryCorrectionQueries(queryClient, api, companyId),
  });
}

async function invalidateMemoryCorrectionQueries(queryClient: ReturnType<typeof useQueryClient>, api: WorkbenchApi, companyId: string): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'memory-corrections', companyId]}),
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
  ]);
}

export function useProblemToolCallBudgets(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId],
    queryFn: () => api.listProblemToolCallBudgets({companyId}),
    enabled: api.mode === 'real' && companyId.trim() !== '',
    staleTime: 0,
    refetchOnMount: 'always',
  });
}

export function useCompanyToolCallBudget(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'company-tool-call-budget', companyId],
    queryFn: () => api.getCompanyToolCallBudget({companyId}),
    enabled: api.mode === 'real' && companyId.trim() !== '',
    staleTime: 0,
    refetchOnMount: 'always',
  });
}

export function useChangeCompanyToolCallBudget(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ChangeCompanyToolCallBudgetOptions, 'companyId'>) => api.changeCompanyToolCallBudget({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-tool-call-budget', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useSetCompanyToolCallClosingReserve(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetCompanyToolCallClosingReserveOptions, 'companyId'>) => api.setCompanyToolCallClosingReserve({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-tool-call-budget', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useMissionToolCallBudgets(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'mission-tool-budgets', companyId],
    queryFn: () => api.listMissionToolCallBudgets({companyId} satisfies MissionToolCallBudgetQueryOptions),
    enabled: api.mode === 'real' && companyId.trim() !== '',
    staleTime: 0,
    refetchOnMount: 'always',
  });
}

export function useChangeMissionToolCallBudget(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ChangeMissionToolCallBudgetOptions, 'companyId'>) => api.changeMissionToolCallBudget({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useSetMissionToolCallClosingReserve(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetMissionToolCallClosingReserveOptions, 'companyId'>) => api.setMissionToolCallClosingReserve({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useAllocateProblemToolCalls(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<AllocateProblemToolCallsOptions, 'companyId'>) => api.allocateProblemToolCalls({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useAllocateTaskToolCalls(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<AllocateTaskToolCallsOptions, 'companyId'>) => api.allocateTaskToolCalls({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useCloseTaskToolBudgetIncomplete(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CloseTaskToolBudgetIncompleteOptions, 'companyId'>) => api.closeTaskToolBudgetIncomplete({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useSetProblemToolCallClosingReserve(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetProblemToolCallClosingReserveOptions, 'companyId'>) => api.setProblemToolCallClosingReserve({...options, companyId}),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'problem-tool-budgets', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useRevalidateMemoryTask(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RevalidateMemoryTaskOptions, 'companyId'>) => api.revalidateMemoryTask({...options, companyId}),
    onSuccess: async (_receipt, options) => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-memory-impact', companyId, options.taskId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useMissionChangeRequests(api: WorkbenchApi, companyId: string, missionId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId],
    queryFn: () => api.listMissionChangeRequests({companyId, missionId} satisfies MissionChangeRequestQueryOptions),
    staleTime: 5_000,
    refetchOnMount: 'always',
  });
}

export function useCreateMissionChangeRequest(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CreateMissionChangeRequestOptions, 'companyId' | 'missionId'>) => api.createMissionChangeRequest({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
    ]),
  });
}

export function useConsiderMissionChangeRequest(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionChangeRequestCommandOptions, 'companyId' | 'missionId'>) => api.considerMissionChangeRequest({...options, companyId, missionId}),
    onSettled: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId]}),
  });
}

export function useDeclineMissionChangeRequest(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionChangeRequestCommandOptions, 'companyId' | 'missionId'>) => api.declineMissionChangeRequest({...options, companyId, missionId}),
    onSettled: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId]}),
  });
}

export function useApplyMissionChangeRequest(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionChangeRequestCommandOptions, 'companyId' | 'missionId'>) => api.applyMissionChangeRequest({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'companies']}),
    ]),
  });
}

export function useTaskTakeoverLeases(api: WorkbenchApi, companyId: string, missionId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-takeover-leases', companyId, missionId],
    queryFn: () => api.listTaskTakeoverLeases({companyId, missionId} satisfies TaskTakeoverLeaseQueryOptions),
    staleTime: 5_000,
    refetchOnMount: 'always',
  });
}

export function useCreateTaskTakeoverLease(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<CreateTaskTakeoverLeaseOptions, 'companyId' | 'missionId'>) => api.createTaskTakeoverLease({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-takeover-leases', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
    ]),
  });
}

export function useSubmitTaskTakeoverSnapshot(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<TaskTakeoverSnapshotOptions, 'companyId' | 'missionId'>) => api.submitTaskTakeoverSnapshot({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-takeover-leases', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-change-requests', companyId, missionId]}),
    ]),
  });
}

export function useReleaseTaskTakeoverLease(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ReleaseTaskTakeoverLeaseOptions, 'companyId' | 'missionId'>) => api.releaseTaskTakeoverLease({...options, companyId, missionId}),
    onSettled: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-takeover-leases', companyId, missionId]}),
  });
}

export function useImportReadOnlySkillPackage(api: WorkbenchApi, companyId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (options: Omit<ImportReadOnlySkillPackageOptions, 'companyId'>) => api.importReadOnlySkillPackage({...options, companyId}),
		onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
	});
}

export function useImportStdioMCPPackage(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ImportStdioMCPPackageOptions, 'companyId'>) => api.importStdioMCPPackage({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useDomainEvidence(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'domain-evidence', companyId],
    queryFn: () => api.listDomainEvidence({companyId}),
    staleTime: 10_000,
  });
}

export function useRunResearchSimulation(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RunResearchSimulationOptions, 'companyId'>) => api.runResearchSimulation({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useSetContentSourceAuthorization(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetContentSourceAuthorizationOptions, 'companyId'>) => api.setContentSourceAuthorization({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRegisterContentDraft(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RegisterContentDraftOptions, 'companyId'>) => api.registerContentDraft({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordContentReview(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordContentReviewOptions, 'companyId'>) => api.recordContentReview({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useSimulateContentPublication(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SimulateContentPublicationOptions, 'companyId'>) => api.simulateContentPublication({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordContentCorrection(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordContentCorrectionOptions, 'companyId'>) => api.recordContentCorrection({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordContentFeedback(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordContentFeedbackOptions, 'companyId'>) => api.recordContentFeedback({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordDomainEvidence(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordDomainEvidenceOptions, 'companyId'>) => api.recordDomainEvidence({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordDomainEvidenceReview(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordDomainEvidenceReviewOptions, 'companyId'>) => api.recordDomainEvidenceReview({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordDomainEvidenceSubstantiveAssessment(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordDomainEvidenceSubstantiveAssessmentOptions, 'companyId'>) => api.recordDomainEvidenceSubstantiveAssessment({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useRecordDomainProfileQualification(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RecordDomainProfileQualificationOptions, 'companyId'>) => api.recordDomainProfileQualification({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'domain-evidence', companyId]}),
  });
}

export function useQualifyCapability(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<QualifyCapabilityOptions, 'companyId'>) => api.qualifyCapability({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useDecideCapability(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<DecideCapabilityOptions, 'companyId'>) => api.decideCapability({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useReviewIncompleteCapabilityRevocation(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ReviewCapabilityRevocationOptions, 'companyId'>) => api.reviewIncompleteCapabilityRevocation({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useApproveStdioMCPRuntimeQualification(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<Parameters<WorkbenchApi['approveStdioMCPRuntimeQualification']>[0], 'companyId'>) => api.approveStdioMCPRuntimeQualification({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useObserveStdioMCPRuntime(api: WorkbenchApi, companyId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (options: Omit<ObserveStdioMCPRuntimeOptions, 'companyId'>) => api.observeStdioMCPRuntime({...options, companyId}),
		onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
	});
}

export function useObserveStreamableHTTPMCPRuntime(api: WorkbenchApi, companyId: string) {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (options: Omit<ObserveStreamableHTTPMCPRuntimeOptions, 'companyId'>) => api.observeStreamableHTTPMCPRuntime({...options, companyId}),
		onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
	});
}

export function useBindEmployeeCapability(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<BindEmployeeCapabilityOptions, 'companyId'>) => api.bindEmployeeCapability({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useRevokeEmployeeCapability(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<BindEmployeeCapabilityOptions, 'companyId'>) => api.revokeEmployeeCapability({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useSetHumanInterventionState(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetHumanInterventionStateOptions, 'companyId'>) => api.setHumanInterventionState({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
  });
}

export function useTaskInputManifest(api: WorkbenchApi, companyId: string, taskId: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-input-manifest', companyId, taskId],
    queryFn: () => api.getTaskInputManifest({companyId, taskId: taskId as string} satisfies TaskInputManifestQueryOptions),
    enabled: taskId !== null && taskId.trim() !== '',
    staleTime: 30_000,
  });
}

export function useProjectEnvironments(api: WorkbenchApi, companyId: string, enabled = true) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'project-environments', companyId],
    queryFn: () => api.listProjectEnvironments({companyId}),
    enabled,
    staleTime: 5_000,
  });
}

export function useTaskJobRuns(api: WorkbenchApi, companyId: string, taskId: string | null, enabled = true) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-job-runs', companyId, taskId],
    queryFn: () => api.listTaskJobRuns({companyId, taskId: taskId as string} satisfies TaskJobRunsQueryOptions),
    enabled: enabled && taskId !== null && taskId.trim() !== '',
    staleTime: 5_000,
  });
}

export function useTaskCrossBackendHandovers(api: WorkbenchApi, companyId: string, taskId: string | null, enabled = true) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-cross-backend-handovers', companyId, taskId],
    queryFn: () => api.listTaskCrossBackendHandovers({companyId, taskId: taskId as string} satisfies TaskCrossBackendHandoversQueryOptions),
    enabled: enabled && taskId !== null && taskId.trim() !== '',
    staleTime: 5_000,
  });
}

export function useCreateTaskEnvironmentHandover(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: CreateTaskEnvironmentHandoverOptions) => api.createTaskEnvironmentHandover(options),
    onSettled: async (_handover, _error, options) => {
      await Promise.all([
        queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-cross-backend-handovers', options.companyId, options.taskId]}),
        queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-job-runs', options.companyId, options.taskId]}),
      ]);
    },
  });
}

export function useTaskJobLogs(api: WorkbenchApi, companyId: string, jobId: string | null, enabled = true) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'task-job-logs', companyId, jobId],
    queryFn: () => api.getTaskJobLogs({companyId, jobId: jobId as string} satisfies TaskJobLogsQueryOptions),
    enabled: enabled && jobId !== null && jobId.trim() !== '',
    staleTime: 30_000,
  });
}

export function useStartTaskJobRun(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: StartTaskJobRunOptions) => api.startTaskJobRun(options),
    onSettled: (_receipt, _error, options) => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-job-runs', options.companyId, options.taskId]}),
  });
}

export function useStopTaskJobRun(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: StopTaskJobRunOptions) => api.stopTaskJobRun(options),
    onSettled: (_receipt, _error, options) => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'task-job-runs', options.companyId]}),
  });
}

export function useCreateProjectJobBrowserSession(api: WorkbenchApi) {
  return useMutation({
    mutationFn: (options: CreateProjectJobBrowserSessionOptions) => api.createProjectJobBrowserSession(options),
  });
}

export function useDecideProjectEnvironmentPolicy(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<EnvironmentPolicyDecisionOptions, 'companyId'>) => api.decideProjectEnvironmentPolicy({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'project-environments', companyId]}),
  });
}

export function useDecideProjectEnvironmentExecutorQualification(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<EnvironmentExecutorQualificationOptions, 'companyId'>) => api.decideProjectEnvironmentExecutorQualification({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'project-environments', companyId]}),
  });
}

export function useEnsureProjectEnvironment(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<EnsureEnvironmentOptions, 'companyId'>) => api.ensureProjectEnvironment({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'project-environments', companyId]}),
  });
}

export function useUploadMissionDirectoryInput(api: WorkbenchApi, companyId: string, missionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<UploadMissionDirectoryInputOptions, 'companyId' | 'missionId'>) => api.uploadMissionDirectoryInput({...options, companyId, missionId}),
    onSettled: async () => Promise.all([
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'mission-inputs', companyId, missionId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
      queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
    ]),
  });
}

export function useTaskWorkspace(api: WorkbenchApi, companyId: string, taskId: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'workspace', companyId, taskId],
    queryFn: () => api.getWorkspace({companyId, taskId: taskId as string}),
    enabled: taskId !== null,
    staleTime: 5_000,
  });
}

export function useArtifactDetail(api: WorkbenchApi, companyId: string, artifactId: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'artifact-detail', companyId, artifactId],
    queryFn: () => api.getArtifact({companyId, artifactId: artifactId as string}),
    enabled: artifactId !== null,
    staleTime: 5_000,
  });
}

export function useArtifactDeliveryManifest(api: WorkbenchApi, companyId: string, artifactId: string | null) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'artifact-delivery-manifest', companyId, artifactId],
    queryFn: () => api.getArtifactDeliveryManifest({companyId, artifactId: artifactId as string}),
    enabled: artifactId !== null,
    staleTime: 5_000,
  });
}

export function useOperations(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'operations', companyId],
    queryFn: () => api.getOperations({companyId}),
    staleTime: 5_000,
  });
}

export function useNotifications(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'notifications', companyId],
    queryFn: () => api.listNotifications({companyId}),
    staleTime: 5_000,
  });
}

export function useConfigureNotificationRoute(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ConfigureNotificationRouteOptions, 'companyId'>) => api.configureNotificationRoute({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'notifications', companyId]}),
  });
}

export function useTestNotification(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<TestNotificationOptions, 'companyId'>) => api.testNotification({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'notifications', companyId]}),
  });
}

export function useStartMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCommandOptions, 'companyId'>) => api.startMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCompanyFeedback(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'feedback', companyId],
    queryFn: () => api.getCompanyFeedback({companyId}),
    staleTime: 5_000,
  });
}

export function useStoreGitHubFeedbackCredential(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Readonly<{token: string; requestId: string}>) => api.storeGitHubFeedbackCredential({...options, companyId}),
  });
}

export function useDeleteGitHubFeedbackCredential(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Readonly<{requestId: string}>) => api.deleteGitHubFeedbackCredential({...options, companyId}),
  });
}

export function useRegisterGitHubFeedbackSource(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<RegisterGitHubFeedbackSourceOptions, 'companyId'>) => api.registerGitHubFeedbackSource({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function useProbeGitHubFeedbackSource(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ProbeGitHubFeedbackSourceOptions, 'companyId'>) => api.probeGitHubFeedbackSource({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function useDecideGitHubFeedbackSource(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<DecideGitHubFeedbackSourceOptions, 'companyId'>) => api.decideGitHubFeedbackSource({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function usePollGitHubFeedbackSource(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<PollGitHubFeedbackSourceOptions, 'companyId'>) => api.pollGitHubFeedbackSource({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function useSetGitHubFeedbackBacklogStatus(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetGitHubFeedbackBacklogStatusOptions, 'companyId'>) => api.setGitHubFeedbackBacklogStatus({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function useSetGitHubFeedbackCollectionPolicy(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SetGitHubFeedbackCollectionPolicyOptions, 'companyId'>) => api.setGitHubFeedbackCollectionPolicy({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'feedback', companyId]}),
  });
}

export function usePauseMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCommandOptions, 'companyId'>) => api.pauseMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useResumeMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCommandOptions, 'companyId'>) => api.resumeMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCancelMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCommandOptions, 'companyId'>) => api.cancelMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCloseMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCloseoutOptions, 'companyId'>) => api.closeMission({...options, companyId}),
    onSettled: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

async function invalidateWorkbenchQueries(queryClient: ReturnType<typeof useQueryClient>, api: WorkbenchApi, companyId: string): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
  ]);
}
