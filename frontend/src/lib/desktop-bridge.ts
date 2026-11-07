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

export function listenForDesktopEvent(eventName: string, callback: () => void): Promise<UnlistenFn> {
  return listen(eventName, callback);
}
