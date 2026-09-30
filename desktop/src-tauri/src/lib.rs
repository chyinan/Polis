// pattern: Imperative Shell

mod commands;
mod core;
mod generation;
mod generation_store;
mod process;
mod provider_manifest;
mod sidecar;
mod sidecar_store;
mod supervisor;
mod windows_startup;

use supervisor::SupervisorState;
use tauri::{Emitter, Manager, WindowEvent};

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .invoke_handler(tauri::generate_handler![
            commands::get_desktop_runtime,
            commands::recheck_provider,
            commands::set_last_company,
            commands::restart_local_services,
            commands::stage_recovery_generation,
            commands::activate_staged_generation,
            commands::rollback_previous_generation,
            commands::stage_sidecar_update,
            commands::activate_staged_sidecar,
            commands::rollback_previous_sidecar,
            commands::request_quit,
            commands::quit_app,
            commands::open_logs,
            commands::diagnostic_summary,
            commands::windows_startup_status,
            commands::set_windows_startup_enabled,
        ])
        .setup(|app| {
            let root = if let Ok(test_root) = std::env::var("POLIS_DESKTOP_DATA_ROOT") {
                std::path::PathBuf::from(test_root)
            } else {
                let app_local_root = app
                    .path()
                    .app_local_data_dir()
                    .map_err(|err| format!("failed to resolve Polis app-data directory: {err}"))?;
                app_local_root
                    .parent()
                    .map(|parent| parent.join("Polis"))
                    .unwrap_or(app_local_root)
            };
            let state = SupervisorState::new(app.handle().clone(), root)
                .map_err(|err| format!("failed to initialize Polis desktop: {err}"))?;
            state.start_background();
            app.manage(state);
            setup_tray(app)?;
            if let Some(window) = app.get_webview_window("main") {
                let close_window = window.clone();
                window.on_window_event(move |event| {
                    if let WindowEvent::CloseRequested { api, .. } = event {
                        api.prevent_close();
                        let _ = close_window.hide();
                    }
                });
            }
            Ok(())
        });
    builder
        .run(tauri::generate_context!())
        .expect("error while running Polis desktop");
}

fn setup_tray(app: &mut tauri::App) -> tauri::Result<()> {
    use tauri::menu::{MenuBuilder, MenuItemBuilder};
    use tauri::tray::TrayIconBuilder;
    let tray_icon = tauri::image::Image::from_bytes(include_bytes!("../icons/icon.png"))?;
    let open = MenuItemBuilder::with_id("open", "Open Polis").build(app)?;
    let status = MenuItemBuilder::with_id("status", "Runtime Status").build(app)?;
    let restart = MenuItemBuilder::with_id("restart", "Restart Local Services").build(app)?;
    let logs = MenuItemBuilder::with_id("logs", "Open Logs").build(app)?;
    let quit = MenuItemBuilder::with_id("quit", "Quit Polis").build(app)?;
    let menu = MenuBuilder::new(app)
        .items(&[&open, &status, &restart, &logs, &quit])
        .build()?;
    TrayIconBuilder::with_id("main")
        .icon(tray_icon)
        .menu(&menu)
        .tooltip("Polis — Starting")
        .on_menu_event(|app, event| {
            let Some(window) = app.get_webview_window("main") else {
                return;
            };
            match event.id.as_ref() {
                "open" => {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
                "status" => {
                    let _ = window.show();
                    let _ = window.set_focus();
                    let _ = app.emit("desktop-runtime-status", ());
                }
                "restart" => {
                    let _ = window.show();
                    let _ = window.set_focus();
                    let _ = app.emit("desktop-restart-requested", ());
                }
                "logs" => {
                    let _ = window.show();
                    let _ = window.set_focus();
                    let _ = app.emit("desktop-open-logs", ());
                }
                "quit" => {
                    let _ = window.show();
                    let _ = window.set_focus();
                    let _ = app.emit("desktop-quit-requested", ());
                }
                _ => {}
            }
        })
        .build(app)?;
    Ok(())
}
