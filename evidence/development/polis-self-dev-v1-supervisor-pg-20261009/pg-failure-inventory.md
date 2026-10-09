# PG baseline failure inventory — 2026-10-09

Starting HEAD: `3e333100a22ee01e38879a90a614434717e1b896`.

Source: previous `20261008T090040Z-303/go-postgres-tests.log` (unfiltered three-package run). Historical evidence is unchanged. Non-verbose output identifies failed tests only; passed/skipped tests cannot be reconstructed from it. Parent failures may include nested failed subtests in the original log. A fresh JSON event run will enumerate all executed/pass/fail/skip outcomes.

| Test | Original error | Initial classification |
|---|---|---|
| TestPeerWorkspaceCheckReturnsActionableSemanticCriteria | behavioral_hardening_test.go:82: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProgressCheckpointIsHandoverStateButNotArtifactAcceptance | behavioral_hardening_test.go:156: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestContractBoundObligationIsSupersededAndReplacementIsCurrent | behavioral_hardening_test.go:220: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerSendMissingTaskTargetReturnsActionableFailureAndNoRows | behavioral_hardening_test.go:259: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilityApprovalBindingAndRevocationStayVersionPinned | capability_governance_integration_test.go:35: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilitySkillConcurrentSameRevisionImportPreflightsCASWrites | capability_governance_integration_test.go:220: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilitySkillDistinctRevisionImportsDoNotExhaustDatabasePool | capability_governance_integration_test.go:309: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilitySkillLoadRequiresCurrentEmployeeBindingAndFlowsThroughHandover | capability_governance_integration_test.go:354: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestControlledStdioMCPApprovalAndEmployeeUnbindAreAudited | capability_governance_integration_test.go:446: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPRuntimeQualificationRejectsUnsealedObservation | capability_governance_integration_test.go:506: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPRuntimeAuthorizationRequiresCurrentApprovalBindingAndSchema | capability_governance_integration_test.go:562: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStreamableHTTPMCPUsesPinned20260728ProfileWithoutNetworkProbe | capability_governance_integration_test.go:869: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestQualifiedCheckpointFreshness | --- FAIL: TestQualifiedCheckpointFreshness/current (0.23s) | undetermined |
| TestCheckpointFinalizationConcurrentContractAcceptance | --- FAIL: TestCheckpointFinalizationConcurrentContractAcceptance/0 (0.23s) | undetermined |
| TestCrossBackendHandoverRequiresPauseAndKnownTerminalSourceAndIsSingleUse | cross_backend_handover_integration_test.go:28: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCreateDailyRoutineRequiresTaskInstruction | daily_routine_integration_test.go:19: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMigratedActiveLegacyRoutineRequiresInstructionThenDelivers | daily_routine_integration_test.go:37: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDailyRoutineMaterializationPersistsAndWakesOwner | daily_routine_integration_test.go:70: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPausedMissionPersistsDailyRoutineWithoutWakingUntilResume | daily_routine_integration_test.go:192: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCancelledMissionRoutineOccurrenceDoesNotRewakeEmployee | daily_routine_integration_test.go:241: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestLegacyRoutineWithoutInstructionWaitsThenDeliversOnceSupplemented | daily_routine_integration_test.go:277: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDomainEvidenceSubmissionIsScopedAppendOnlyAndNeverQualifies | domain_evidence_integration_test.go:27: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDomainEvidencePreviewLoadsOnlyReferencedMissionInputAndVerifiesCAS | domain_evidence_preview_integration_test.go:28: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDomainEvidenceProfileQualificationIsCompanyScopedAndNeverEnablesExecution | domain_profile_qualification_integration_test.go:24: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestNewCompanyCreatesSleepingEmployeeSchedules | employee_schedule_integration_test.go:21: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestActionablePeerWorkCannotBeLostAcrossEmployeeSleepReconciliation | employee_schedule_integration_test.go:35: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFYIPeerMessageDoesNotWakeEmployeeSchedule | employee_schedule_integration_test.go:121: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionPauseBlocksPeerWorkAndResumeRestoresPendingWake | employee_schedule_integration_test.go:153: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestObservedPeerObligationKeepsStoppedEmployeeWakePending | employee_schedule_integration_test.go:201: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestReadyTasksAdvanceEmployeeScheduleOnMissionStartAndResume | employee_schedule_integration_test.go:240: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestNextProductWorkerDispatchCandidateIsAgeOrderedAndHonorsBarriers | employee_schedule_integration_test.go:301: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestKernelStartupScanRestoresReadyTaskWakeAfterLostHint | employee_schedule_integration_test.go:364: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCancellingPausedMissionClearsScheduleBarrierForNextMission | employee_schedule_integration_test.go:415: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestEmployeeScheduleReconcilerListensReconnectsAndScans | employee_schedule_listener_integration_test.go:13: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestEmployeeScheduleWakeHintStaysWithinCompanyScope | employee_schedule_listener_integration_test.go:49: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestEmployeeScheduleReconcilerMaterializesDueDailyRoutine | employee_schedule_listener_integration_test.go:72: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestEnvironmentExecutorQualificationRequiresCurrentCASBoundEvidence | environment_executor_qualification_integration_test.go:24: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently | environment_lifecycle_integration_test.go:30: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFaultBoundaries | fault_test.go:28: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProcessCrashRecovery | fault_test.go:179: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFrontendCollabApplyRejectsCandidateContentAndRequiresStructuredReferences | frontend_interaction_contract_test.go:12: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFrontendCollabApplyAndProgressCheckpointUseReceiptReferences | frontend_interaction_contract_test.go:55: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFrontendCollabApplyReportsUnsupportedCheckpointReceipt | frontend_interaction_contract_test.go:108: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestFrontendInteractionRejectsStaleAndAmbiguousReferences | frontend_interaction_contract_test.go:154: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestGitHubFeedbackRequiresSourceApprovalAndOnlyCompleteScansAdvanceCursor | github_feedback_integration_test.go:27: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestGitHubFeedbackBacklogKeepsInternalStateAndReopensOnSourceRevision | github_feedback_integration_test.go:150: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestGitHubFeedbackScheduleRequiresCurrentKernelLeaseForReserveAndDispatch | github_feedback_integration_test.go:221: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestHumanInterventionCreatesSafeDeduplicatedNotificationIntent | human_interventions_test.go:24: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestQQOutboxRequiresQualifiedRouteAndNeverRetriesUnknownOutcome | human_interventions_test.go:99: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPostgresSlice | kernel_test.go:25: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestLostLeaseCannotRecoverOverSuccessor | lifecycle_test.go:21: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCloseReleasesLeaseMutexBeforeWaitingForTransactions | lifecycle_test.go:45: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageImportPinsCompanyCASAndReplaysByRequestID | mcp_package_import_integration_test.go:29: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPObservationReservationPreventsRequestReplay | mcp_package_import_integration_test.go:75: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageImportSharesServerLockWithRuntimeObservation | mcp_package_import_integration_test.go:171: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageUpdateRevokesObservedRuntimeQualification | mcp_package_import_integration_test.go:216: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageImportSerializesSameRevisionBeforeCASWrites | mcp_package_import_integration_test.go:347: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageImportSerializesReusedRequestIDAcrossTargetsBeforeCASWrites | mcp_package_import_integration_test.go:404: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilitySkillAndMCPImportsShareRequestIDLockBeforeCASWrites | mcp_package_import_integration_test.go:465: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCapabilitySourceRequestLockBlocksUncoordinatedTXWrite | mcp_package_import_integration_test.go:515: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestStdioMCPPackageImportRequiresDescriptorAndManifestToMatch | mcp_package_import_integration_test.go:603: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionChangeRequestAppliesAsSuccessorOnlyAfterStoppedBoundary | mission_change_request_test.go:28: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionCommandGoalAndTerminalTransitions | mission_command_test.go:20: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionCommandCreateIsIdempotentByRequestKey | mission_command_test.go:63: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionInputUploadCreatesImmutableIdempotentRevisions | mission_input_test.go:25: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionDirectorySnapshotIsStoredAsOneVerifiedRevision | mission_input_test.go:118: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductTaskFreezesVersionedModelInputManifest | mission_input_test.go:172: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestMissionPauseResumeCommandsRequireStateAndReplayIdempotently | mission_lifecycle_test.go:20: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestOperatorInstructionIsPersistedForActiveMission | operator_instructions_test.go:24: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCompanyOrganizationRoundTrip | organization_test.go:20: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductWorkerDirectMessageToolSurfaceCompletesAnActionableRequest | peer_bidirectional_messaging_test.go:24: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductDirectSendRacingCandidateSubmissionNeverCommitsAfterSubmission | peer_bidirectional_messaging_test.go:114: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductDirectSendRacingMissionPauseHasSerializedOutcome | peer_bidirectional_messaging_test.go:170: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductDirectFYIsHaveNoObligationAndAdvanceInEventOrder | peer_bidirectional_messaging_test.go:224: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductDirectMessageSurfaceIsExplicitAndReadOnlyCannotWrite | peer_bidirectional_messaging_test.go:281: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductDirectMessageRejectsTargetsOutsideTheCurrentMission | peer_bidirectional_messaging_test.go:299: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerDirectMessageCanBeSentAndResolvedInBothDirections | peer_bidirectional_messaging_test.go:337: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerFYIMessageCanBeReadAndAcknowledgedByEitherPeer | peer_bidirectional_messaging_test.go:401: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerFYIInboxAdvancesAfterAcknowledgement | peer_bidirectional_messaging_test.go:437: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerDirectMessageCannotCrossMissionWithinCompany | peer_bidirectional_messaging_test.go:476: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerDirectMessageCannotTargetFinalizedTask | peer_bidirectional_messaging_test.go:494: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerResolveRequiresObligationTaskBoundToSession | peer_bidirectional_messaging_test.go:509: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerHandoverBoundarySnapshotReadsStoppedInitialSession | peer_collaboration_test.go:16: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerSendDeniesFinalizedCandidateTask | peer_collaboration_test.go:64: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerIntegrationUsesRuntimePaginationVerifierAndSeparateCoverage | peer_collaboration_test.go:83: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerInboxMakesMessageObservableBeforeAck | peer_collaboration_test.go:139: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestR03APeerSendCommitVisibleToIndependentSession | peer_collaboration_test.go:216: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestR03APeerCollaborationPositiveAndNegative | peer_collaboration_test.go:257: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestR03AFrontendHandoverPreservesPeerObligation | peer_collaboration_test.go:388: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestR03AT5BackendToolSurfaceInvokesEveryHandlerOnce | peer_employee_ops_test.go:21: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPeerWorkspaceCheckPersistsActionableBindingFailureReceipt | peer_employee_ops_test.go:86: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestProductCandidateFencesPostPublicationWorkspaceAndCheckpointWrites | product_finalization_fence_test.go:26: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestQQRouteCannotEnableBeforeQualificationAndDisableAdvancesRevision | qq_route_integration_test.go:22: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestBackendTerminalRecoveryAnchorForensicsReproducesErrNoRows | recovery_anchor_forensics_test.go:21: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestReviewerGrantPersistsReviewSubmitWithoutBehaviorVerifier | reviewer_h3_integration_test.go:70: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestT21CQualifiedCheckpointMustBindEffectiveContract | t21c_policy_binding_test.go:12: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestTakeoverLeaseFreezesWorkspaceAndReturnsHumanSnapshotToSuccessor | takeover_lease_test.go:27: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestWorkerHostContainmentBindingScopesRecoveryCandidates | worker_process_recovery_test.go:44: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestTaskWorkspaceCASRehydratesForNewSessionAfterKernelRestart | worker_process_recovery_test.go:134: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestLegacyLinuxWorkerSessionProcessGroupRecoveryRemainsUnresolved | worker_process_recovery_test.go:183: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestTXFinalizeWorkerBeforeProcessIsIdempotentForSameFailure | worker_process_recovery_test.go:242: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestWorkerLifecycleAndMediation | worker_test.go:23: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestTXNewWorkerHonorsEmployeeAdmissionBarriers | worker_test.go:98: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestTXNewWorkerMarksEmployeeScheduleAdmitted | worker_test.go:138: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestCrossBackendHandoverFlowsThroughPauseResumeAndJobRunWithoutProviderTurn | cross_backend_handover_integration_test.go:213: start target JobRun through Control without a provider WorkerSession: WORKER_SLOTS_UNCONFIGURED: an installation owner must set maxActiveSlots and protectedSlots at /api/workbench/installation/worker-slots before WorkerSessions can be admitted | admission fixture/config prerequisite |
| TestDeterministicWorkerRetainsOwnershipWhenStopPersistenceFails | deterministic_worker_test.go:46: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestDeterministicStopRetriesProcessAttachmentBeforeConfirmation | deterministic_worker_test.go:119: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDeterministicStopReplaysConfirmationAfterAmbiguousCommit | deterministic_worker_test.go:181: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestDeterministicStopRetainsWindowsHostReconciliationOwner | deterministic_worker_test.go:237: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestRealProviderWorkerRetainsOwnershipWhenStopPersistenceFails | real_provider_worker_test.go:326: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestRealProviderStartRetainsSessionWhenUnactivatedCleanupFails | real_provider_worker_test.go:402: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestRealProviderStartRetainsReservationWhenNoProcessCleanupFails | real_provider_worker_test.go:460: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestRealProviderStopReplaysConfirmationAfterAmbiguousCommit | real_provider_worker_test.go:516: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestBusinessPreflightRetainsOwnerWhenProcessStopFails | real_provider_worker_test.go:588: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesFrozenImageBytesAsVisionInput | task_input_delivery_test.go:99: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerLoadsOnlyApprovedBoundReadOnlySkill | task_input_delivery_test.go:224: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesFrozenGitSnapshotFromSelectedCommit | task_input_delivery_test.go:305: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesFrozenTextInputsInTurnPrompt | task_input_delivery_test.go:429: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesFrozenImageBytesInTurnInput | task_input_delivery_test.go:509: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesExtractedTextFromBoundPDFInput | task_input_delivery_test.go:599: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesTextFilesFromFrozenDirectoryInput | task_input_delivery_test.go:722: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesFrozenCSVInputInTurnPrompt | task_input_delivery_test.go:806: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestProductWorkerReceivesOnlySupportedTextFromFrozenZIPInput | task_input_delivery_test.go:892: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestPostgresReadStoreProjectsEmployeeSchedule | employee_schedule_projection_integration_test.go:45: ERROR: column role.role_name does not exist (SQLSTATE 42703) | product SQL defect |
| TestOpenHumanInterventionAppearsInWorkbenchAttention | human_intervention_integration_test.go:49: ERROR: column role.role_name does not exist (SQLSTATE 42703) | product SQL defect |
| TestOperatorInstructionResponseIsVisibleInWorkbench | operator_instruction_response_integration_test.go:42: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestRestartRecoveryProjectionShowsReconciliationRequired | recovery_ux_integration_test.go:45: POLICY_DENIED | undetermined: distinguish lease, admission and fixture setup |
| TestPostgresWorkspacePreviewReadsAuthorizedCASBlob | workspace_integration_test.go:36: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestPostgresReadStoreProjectsQualifiedCheckpointWithoutLifecycleFilters | checkpoint_projection_integration_test.go:53: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates | environment_job_lifecycle_integration_test.go:123: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestWorkbenchDirectoryInputIsScopedAndStoredAsOneSnapshot | mission_directory_inputs_integration_test.go:117: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestProductCandidateLifecycleProjectsThroughWorkbench | product_lifecycle_integration_test.go:44: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestTaskInputManifestReadShowsFinalWorkerDeliveryStatus | task_input_delivery_read_integration_test.go:64: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestEmptyTaskInputDeliveryIsRecordedAndReadAsNotRequired | task_input_delivery_read_integration_test.go:139: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestQualifiedArtifactDeliveryManifestAndPackageReadFromPostgresAndCAS | task_input_delivery_read_integration_test.go:224: MALFORMED_INPUT | undetermined: inspect submitted contract |
| TestTaskInputManifestReadProjectsPerFileDirectoryDeliveryReceipt | task_input_delivery_read_integration_test.go:424: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |
| TestZIPInputIsDeliveredToWorkerAndReadBackPerFile | task_input_delivery_read_integration_test.go:538: ERROR: could not determine data type of parameter $2 (SQLSTATE 42P18) | product SQL defect |

The blanket claim that all failures were stale fixtures is not supported. In particular, undefined SQL aliases and untyped SQL parameters are production read-query defects. Grants will remain SELECT/INSERT/UPDATE plus sequence usage; no permission expansion is planned.

## Resolution categories established by fresh reproductions

The table above preserves the initial classification, not a retrospective claim that every early denial had one unique cause. The original non-verbose shared-DB run cannot disambiguate cascaded runtime lease/admission failures. Fresh per-test databases in the same disposable cluster and explicit current owner/coverage/budget setup removed that ambiguity. All intermediate failure output is retained in `postgres-runtime`; `postgres-run-inventory.json` indexes each completed diagnostic run and its original JSON events.

| Category | Reproduced issue and correction |
|---|---|
| Product SQL | Workbench employee lateral query omitted fields it projected; delivery disposition query supplied an untyped unused parameter; revocation aggregate omitted a grouped field. Corrected queries without changing access policy. |
| Product transactions | Read-only APIs attempted `FOR SHARE` on runtime_control. Read-only lease checks now use the same snapshot without a write lock; mutation transactions retain their lock/fence. |
| Product lock isolation | Nested capability source/CAS lifecycle locks exhausted one shared two-connection lock pool. Separate bounded lock pools preserve both lock families; no runtime grant increase. |
| Migration contract | Schema 125 removed supported historical Artifact contract values. Forward 130 restores those values, without changing Artifact qualification/publication policy. |
| Migration trigger | The MCP permit trigger shadowed its unassigned RECORD with SQL alias `p`, then referenced fields not selected under those names. Forward 131 corrects the alias/field references while retaining all epoch, grant and consumed-permit checks. Tests use no external MCP transport. |
| Product grant mismatch | Workspace snapshots and takeover handback attempted DELETE under a role intentionally lacking DELETE. Staging journals are retained. Forward 132 allows only terminal takeover pointer reuse; immutable lease events remain the active-state authority. No grants were expanded. |
| Product representation | The single-file takeover guard recognized workspace.txt but not the existing formatter.go mirror. Both require one exact file, digest and source revision; additional/mismatched paths remain denied. |
| Product normalization | Explicit empty assessment ID arrays normalized to nil and were then rejected. The new unit regression failed first and passed after preserving explicit empty slices; nil/missing remains rejected. |
| Continuation regression | Conditional Git seeding initially retained ErrNoRows from the absent workspace root. Fixed before final verification. Git snapshots seed only fixed Go bindings; legacy text-only Worker contracts retain read-only Git context. |
| Fixture contracts | Owner confirmation, company/Mission budget and installation slot policy are configured through real APIs. Missing acceptance and required Planning analysis are supplied explicitly. Planning transaction fixtures seed a ready review Task and then use real admission/assessment/stop APIs; this does not qualify production Planning dispatch. |
| Fixture projections | Input identities no longer assume random-ID ordering or treat supported PNG as unsupported; excluded .bin input remains tested. Fulfilled peer state is checked in authoritative rows and excluded from pending projection. Company cursor ordering is retained. |
| Test synchronization | Terminal provider observation is awaited together with stopped sessions, not inferred from the first turn event. Trusted sleeper ownership is stopped/confirmed before Mission pause. |
| Harness/config | Per-test clones use reconstructed DSNs (pgx ConnString otherwise retains the original database). POSIX temp directories replace DrvFS permission semantics. Actual migration/recovery DSNs and the historical POLIS_R0_5B16_TEST_DSN alias are enabled; missing DSNs are not a PG PASS. |
| Historical upgrade fixtures | Real migrations build the required old schema/specimen before upgrade; migration identity expectations follow embedded current migrations. Recovery uses real dump/restore and dedicated roles/databases. No successful result rows are hardcoded. |

Runtime SQL grants remain SELECT/INSERT/UPDATE plus sequence USAGE/SELECT. No assertion was removed to suppress a failure, no failed case was skipped, and no test result was substituted for host execution qualification. Final executed/skipped outcomes are reported separately after the complete suite finishes.
