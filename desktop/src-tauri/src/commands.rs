// pattern: Imperative Shell

use crate::core::{RecoveryGenerationStageReceipt, RuntimeSnapshot, SidecarStageReceipt};
use crate::supervisor::SupervisorState;
use crate::windows_startup::WindowsStartupStatus;
use tauri::{AppHandle, State};

#[tauri::command]
pub fn get_desktop_runtime(state: State<'_, SupervisorState>) -> Result<RuntimeSnapshot, String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())
        .map(|mut supervisor| supervisor.snapshot())
}

#[tauri::command]
pub fn recheck_provider(state: State<'_, SupervisorState>) -> Result<RuntimeSnapshot, String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())
        .map(|mut supervisor| supervisor.recheck_provider())
}

#[tauri::command]
pub fn set_last_company(
    state: State<'_, SupervisorState>,
    company_id: String,
) -> Result<(), String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
        .set_last_company(company_id)
}

#[tauri::command]
pub fn restart_local_services(
    state: State<'_, SupervisorState>,
) -> Result<RuntimeSnapshot, String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())
        .and_then(|mut supervisor| supervisor.restart())
}

#[tauri::command]
pub async fn stage_recovery_generation(
    state: State<'_, SupervisorState>,
    package_root: String,
    administrator_password: Option<String>,
) -> Result<RecoveryGenerationStageReceipt, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .stage_recovery_generation(package_root, administrator_password)
    })
    .await
    .map_err(|_| "recovery generation staging worker failed".to_string())?
}

#[tauri::command]
pub async fn activate_staged_generation(
    state: State<'_, SupervisorState>,
    generation_id: String,
) -> Result<RuntimeSnapshot, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .activate_staged_generation(generation_id)
    })
    .await
    .map_err(|_| "generation activation worker failed".to_string())?
}

#[tauri::command]
pub async fn rollback_previous_generation(
    state: State<'_, SupervisorState>,
    confirm_data_branch: bool,
) -> Result<RuntimeSnapshot, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .rollback_previous_generation(confirm_data_branch)
    })
    .await
    .map_err(|_| "generation rollback worker failed".to_string())?
}

#[tauri::command]
pub async fn stage_sidecar_update(
    state: State<'_, SupervisorState>,
    source_path: String,
) -> Result<SidecarStageReceipt, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .stage_sidecar_update(source_path)
    })
    .await
    .map_err(|_| "sidecar staging worker failed".to_string())?
}

#[tauri::command]
pub async fn activate_staged_sidecar(
    state: State<'_, SupervisorState>,
    sha256: String,
) -> Result<RuntimeSnapshot, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .activate_staged_sidecar(sha256)
    })
    .await
    .map_err(|_| "sidecar activation worker failed".to_string())?
}

#[tauri::command]
pub async fn rollback_previous_sidecar(
    state: State<'_, SupervisorState>,
    confirm_rollback: bool,
) -> Result<RuntimeSnapshot, String> {
    let inner = state.inner.clone();
    tauri::async_runtime::spawn_blocking(move || {
        inner
            .lock()
            .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
            .rollback_previous_sidecar(confirm_rollback)
    })
    .await
    .map_err(|_| "sidecar rollback worker failed".to_string())?
}

#[tauri::command]
pub fn request_quit(state: State<'_, SupervisorState>) -> Result<bool, String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())
        .and_then(|mut supervisor| supervisor.request_quit())
}

#[tauri::command]
pub fn quit_app(
    app: AppHandle,
    state: State<'_, SupervisorState>,
    force_active_work: bool,
) -> Result<(), String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
        .shutdown(force_active_work)?;
    app.exit(0);
    Ok(())
}

#[tauri::command]
pub fn open_logs(state: State<'_, SupervisorState>) -> Result<(), String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())?
        .open_logs()
}

#[tauri::command]
pub fn diagnostic_summary(state: State<'_, SupervisorState>) -> Result<String, String> {
    state
        .inner
        .lock()
        .map_err(|_| "desktop supervisor lock is poisoned".to_string())
        .map(|mut supervisor| supervisor.diagnostics())
}

#[tauri::command]
pub fn windows_startup_status() -> Result<WindowsStartupStatus, String> {
    crate::windows_startup::status()
}

#[tauri::command]
pub fn set_windows_startup_enabled(enabled: bool) -> Result<WindowsStartupStatus, String> {
    crate::windows_startup::set_enabled(enabled)
}
