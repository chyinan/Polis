// pattern: Imperative Shell

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useEffect, useState} from 'react';
import type {CompanyDraftOptions, ConfigureNotificationRouteOptions, CreateMissionOptions, CreateOperatorInstructionOptions, DescribeVisibleToolOptions, GetMCPToolCallReceiptOptions, ImportSkillDocumentOptions, ImportSkillLibraryOptions, ImportSkillOptions, ListMCPToolCallQualificationsOptions, ListMCPToolCallReceiptsOptions, ListVisibleResourcesOptions, ListVisibleSkillsOptions, ListVisibleToolsOptions, LoadSkillOptions, MissionCommandOptions, QualifyMCPOptions, ReadSkillReferenceOptions, RegisterMCPOptions, SkillRevisionStatusOptions, TestNotificationOptions, UpdateCompanyOptions, ArchiveCompanyOptions, UpdateRuntimeSettingsOptions, UpsertCapabilityBindingOptions, UpsertMCPToolCallQualificationOptions, WorkbenchApi} from './workbench-api';
import type {ActivityStreamStatus} from './workbench-api';

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

export function useQualifyMCP(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<QualifyMCPOptions, 'companyId'>) => api.qualifyMCP({...options, companyId}),
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

export function useImportSkillDocument(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ImportSkillDocumentOptions, 'companyId'>) => api.importSkillDocument({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useImportSkillLibrary(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<ImportSkillLibraryOptions, 'companyId'>) => api.importSkillLibrary({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useReadSkillReference(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ReadSkillReferenceOptions, 'companyId'>) => api.readSkillReference({...options, companyId}),
  });
}

export function useDescribeVisibleTool(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<DescribeVisibleToolOptions, 'companyId'>) => api.describeVisibleTool({...options, companyId}),
  });
}

export function useListVisibleResources(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ListVisibleResourcesOptions, 'companyId'>) => api.listVisibleResources({...options, companyId}),
  });
}

export function useListVisibleTools(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ListVisibleToolsOptions, 'companyId'>) => api.listVisibleTools({...options, companyId}),
  });
}

export function useListVisibleSkills(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ListVisibleSkillsOptions, 'companyId'>) => api.listVisibleSkills({...options, companyId}),
  });
}

export function useLoadSkill(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<LoadSkillOptions, 'companyId'>) => api.loadSkill({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useApproveSkill(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SkillRevisionStatusOptions, 'companyId'>) => api.approveSkill({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useRevokeSkill(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<SkillRevisionStatusOptions, 'companyId'>) => api.revokeSkill({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useUpsertCapabilityBinding(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<UpsertCapabilityBindingOptions, 'companyId'>) => api.upsertCapabilityBinding({...options, companyId}),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'capability-catalog', companyId]}),
  });
}

export function useListMCPToolCallQualifications(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ListMCPToolCallQualificationsOptions, 'companyId'>) => api.listMCPToolCallQualifications({...options, companyId}),
  });
}

export function useListMCPToolCallReceipts(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<ListMCPToolCallReceiptsOptions, 'companyId'>) => api.listMCPToolCallReceipts({...options, companyId}),
  });
}

export function useGetMCPToolCallReceipt(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<GetMCPToolCallReceiptOptions, 'companyId'>) => api.getMCPToolCallReceipt({...options, companyId}),
  });
}

export function useUpsertMCPToolCallQualification(api: WorkbenchApi, companyId: string) {
  return useMutation({
    mutationFn: (options: Omit<UpsertMCPToolCallQualificationOptions, 'companyId'>) => api.upsertMCPToolCallQualification({...options, companyId}),
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
    onSuccess: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCreateCompany(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: CompanyDraftOptions) => api.createCompany(options),
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'companies']}),
  });
}

export function useUpdateCompany(api: WorkbenchApi) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: UpdateCompanyOptions) => api.updateCompany(options),
    onSuccess: async (_receipt, options) => {
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
    onSuccess: async (_receipt, options) => {
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
    onSuccess: async (_receipt, options) => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'runtime-settings', options.companyId]}),
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
    onSuccess: async () => queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'operator-instructions', companyId]}),
  });
}

export function useCollaboration(api: WorkbenchApi, companyId: string) {
  return useQuery({
    queryKey: ['workbench', api.mode, 'collaboration', companyId],
    queryFn: () => api.listCollaboration({companyId}),
    staleTime: 5_000,
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
    onSuccess: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

export function useCancelMission(api: WorkbenchApi, companyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (options: Omit<MissionCommandOptions, 'companyId'>) => api.cancelMission({...options, companyId}),
    onSuccess: async () => invalidateWorkbenchQueries(queryClient, api, companyId),
  });
}

async function invalidateWorkbenchQueries(queryClient: ReturnType<typeof useQueryClient>, api: WorkbenchApi, companyId: string): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'company-overview', companyId]}),
    queryClient.invalidateQueries({queryKey: ['workbench', api.mode, 'activity', companyId]}),
  ]);
}
