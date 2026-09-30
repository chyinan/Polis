// pattern: Imperative Shell

import {invoke} from '@tauri-apps/api/core';
import {listen, type UnlistenFn} from '@tauri-apps/api/event';

export type DesktopServiceSnapshot = Readonly<{
  state: string;
  pid: number | null;
  started_at: string | null;
  exit_status: number | null;
  last_failure: string | null;
}>;

export type DesktopProviderSnapshot = Readonly<{
  state: string;
  runtime_version: string | null;
  auth_state: string;
  qualification_state: string;
  reason: string | null;
}>;

export type DesktopRuntimeSnapshot = Readonly<{
  startup_stage: string;
  first_run_required: boolean;
  last_company_id: string | null;
  data_root: string;
  api_base_url: string | null;
  session_token: string | null;
  database: DesktopServiceSnapshot;
  backend: DesktopServiceSnapshot;
  provider: DesktopProviderSnapshot;
  event_stream: DesktopServiceSnapshot;
  logs_path: string;
  diagnostics: string | null;
  active_generation_id: string | null;
  previous_generation_id: string | null;
  generation_pending: boolean;
  active_sidecar_sha256: string | null;
  previous_sidecar_sha256: string | null;
  staged_sidecar_sha256: string | null;
  sidecar_update_pending: boolean;
  sidecar_update_blocked: boolean;
}>;

export type RecoveryGenerationStageReceipt = Readonly<{
  status: string;
  generationId: string;
  databaseName: string;
  manifestSha256: string;
}>;

export type SidecarStageReceipt = Readonly<{
  status: string;
  sha256: string;
  sizeBytes: number;
}>;

export type WindowsStartupStatus = Readonly<{
  supported: boolean;
  enabled: boolean;
}>;

export function isDesktopHost(): boolean {
  return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;
}

export async function getDesktopRuntime(): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('get_desktop_runtime');
}

export async function recheckDesktopProvider(): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('recheck_provider');
}

export async function setLastCompany(companyId: string): Promise<void> {
  return invoke('set_last_company', {companyId});
}

export async function restartLocalServices(): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('restart_local_services');
}

export async function stageRecoveryGeneration(packageRoot: string, administratorPassword?: string): Promise<RecoveryGenerationStageReceipt> {
  return invoke<RecoveryGenerationStageReceipt>('stage_recovery_generation', {packageRoot, administratorPassword: administratorPassword ?? null});
}

export async function activateStagedGeneration(generationId: string): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('activate_staged_generation', {generationId});
}

export async function rollbackPreviousGeneration(confirmDataBranch: boolean): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('rollback_previous_generation', {confirmDataBranch});
}

export async function stageSidecarUpdate(sourcePath: string): Promise<SidecarStageReceipt> {
  return invoke<SidecarStageReceipt>('stage_sidecar_update', {sourcePath});
}

export async function activateStagedSidecar(sha256: string): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('activate_staged_sidecar', {sha256});
}

export async function rollbackPreviousSidecar(confirmRollback: boolean): Promise<DesktopRuntimeSnapshot> {
  return invoke<DesktopRuntimeSnapshot>('rollback_previous_sidecar', {confirmRollback});
}

export async function requestQuit(): Promise<boolean> {
  return invoke<boolean>('request_quit');
}

export async function quitDesktop(forceActiveWork: boolean): Promise<void> {
  return invoke('quit_app', {forceActiveWork});
}

export async function openDesktopLogs(): Promise<void> {
  return invoke('open_logs');
}

export async function getDiagnosticSummary(): Promise<string> {
  return invoke<string>('diagnostic_summary');
}

export async function getWindowsStartupStatus(): Promise<WindowsStartupStatus> {
  return invoke<WindowsStartupStatus>('windows_startup_status');
}

export async function setWindowsStartupEnabled(enabled: boolean): Promise<WindowsStartupStatus> {
  return invoke<WindowsStartupStatus>('set_windows_startup_enabled', {enabled});
}

export function listenForDesktopEvent(eventName: string, callback: () => void): Promise<UnlistenFn> {
  return listen(eventName, callback);
}
