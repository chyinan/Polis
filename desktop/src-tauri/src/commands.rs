// pattern: Imperative Shell

use crate::core::RuntimeSnapshot;
use crate::supervisor::SupervisorState;
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
