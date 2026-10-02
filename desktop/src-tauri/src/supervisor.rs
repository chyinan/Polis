// pattern: Imperative Shell

use crate::core::{
    initial_snapshot, layout, DesktopLayout, ProviderSnapshot, RecoveryGenerationStageReceipt,
    RuntimeSnapshot, ServiceSnapshot, SidecarStageReceipt,
};
use crate::generation::{GenerationState, PreparedGenerationManifest, RuntimeGenerationRef};
use crate::generation_store::{
    create_generation_directory, read_generation_state, read_prepared_generation,
    resolve_generation_cas_root, resolve_generation_directory, write_generation_state,
    write_prepared_generation,
};
use crate::process::{assign_or_terminate, ChildOwnership};
use crate::provider_manifest::{parse_provider_manifest, validate_provider_auth};
use crate::sidecar::{
    validate_launch_identity, validate_migration_manifest_compatibility, SidecarRef, SidecarState,
    SidecarTransitionKind,
};
use crate::sidecar_store::{
    persist_state as persist_sidecar_state_file, read_state as read_sidecar_state,
    resolve_binary as resolve_sidecar_binary, retain_binary as retain_sidecar_binary,
    stage_binary as stage_sidecar_binary,
};
use fs2::FileExt;
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::ffi::OsString;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tauri::{AppHandle, Manager};
use uuid::Uuid;

const DATABASE_NAME: &str = "polis_r0_desktop";
const DATABASE_USER: &str = "polis_runtime";

#[derive(Clone)]
pub struct SupervisorState {
    pub inner: Arc<Mutex<Supervisor>>,
}

pub struct Supervisor {
    app: AppHandle,
    root: PathBuf,
    layout: DesktopLayout,
    config_path: PathBuf,
    database_path: PathBuf,
    generation_state_path: PathBuf,
    sidecar_state_path: PathBuf,
    _lock_file: File,
    ownership: ChildOwnership,
    postgres: Option<Child>,
    backend: Option<Child>,
    postgres_root: Option<PathBuf>,
    postgres_data: Option<PathBuf>,
    postgres_port: Option<u16>,
    pending_database_setup: Option<(PathBuf, String, DatabaseConfig)>,
    token: Option<String>,
    api_base_url: Option<String>,
    dsn: Option<String>,
    generation_state: Option<GenerationState>,
    sidecar_state: Option<SidecarState>,
    active_migration_manifest_sha256: Option<String>,
    active_cas_root: Option<PathBuf>,
    config: DesktopConfig,
    snapshot: RuntimeSnapshot,
}

#[derive(Clone, Debug, Default, Deserialize, Serialize)]
struct DesktopConfig {
    initialized: bool,
    last_company_id: Option<String>,
    runtime_profile: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct DatabaseConfig {
    username: String,
    password: String,
    database: String,
    #[serde(default)]
    postgres_admin_password: Option<String>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RecoveryBackupVerification {
    status: String,
    generation_id: String,
    manifest_sha256: String,
    runtime_role_name: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RestoredGenerationVerification {
    status: String,
    generation_id: String,
    manifest_sha256: String,
    database_name: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RecoveryRestoreResult {
    status: String,
    generation_id: String,
    database_name: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct DesktopBackendIdentity {
    service: String,
    executable_sha256: String,
    migration_manifest_sha256: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct DesktopSidecarInfo {
    service: String,
    migration_manifest_sha256: String,
}

#[derive(Clone, Debug)]
struct ResolvedSidecar {
    reference: SidecarRef,
    path: PathBuf,
}

impl SupervisorState {
    pub fn new(app: AppHandle, root: PathBuf) -> Result<Self, String> {
        let layout = layout(&root);
        let config_dir = root.join("config");
        fs::create_dir_all(&config_dir)
            .map_err(|err| format!("failed to create desktop config directory: {err}"))?;
        let lock_path = config_dir.join("desktop.lock");
        let lock_file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(&lock_path)
            .map_err(|err| format!("failed to open desktop single-instance lock: {err}"))?;
        lock_file
            .try_lock_exclusive()
            .map_err(|_| "another Polis desktop instance is already running".to_string())?;
        let config_path = config_dir.join("desktop.json");
        let database_path = config_dir.join("database.json");
        let generation_state_path = config_dir.join("generation-state.json");
        let sidecar_state_path = config_dir.join("sidecar-state.json");
        let config: DesktopConfig = read_json(&config_path).unwrap_or_default();
        let mut snapshot = initial_snapshot(&root);
        snapshot.first_run_required = !config.initialized;
        snapshot.last_company_id = config.last_company_id.clone();
        let ownership = ChildOwnership::new()?;
        Ok(Self {
            inner: Arc::new(Mutex::new(Supervisor {
                app,
                root,
                layout,
                config_path,
                database_path,
                generation_state_path,
                sidecar_state_path,
                _lock_file: lock_file,
                ownership,
                postgres: None,
                backend: None,
                postgres_root: None,
                postgres_data: None,
                postgres_port: None,
                pending_database_setup: None,
                token: None,
                api_base_url: None,
                dsn: None,
                generation_state: None,
                sidecar_state: None,
                active_migration_manifest_sha256: None,
                active_cas_root: None,
                config,
                snapshot,
            })),
        })
    }

    pub fn start_background(&self) {
        let inner = Arc::clone(&self.inner);
        thread::spawn(move || {
            if let Ok(mut supervisor) = inner.lock() {
                supervisor.bootstrap();
            }
        });
    }
}

impl Supervisor {
    fn bootstrap(&mut self) {
        if let Err(err) = self.bootstrap_with_generation_recovery() {
            let cleanup_error = self.stop_owned_services_for_generation_recovery().err();
            let error = match cleanup_error {
                Some(cleanup_error) => {
                    format!("{err}; startup cleanup could not be confirmed: {cleanup_error}")
                }
                None => err,
            };
            self.fail_current_stage(error);
        }
    }

    fn bootstrap_with_generation_recovery(&mut self) -> Result<(), String> {
        if self.sidecar_state.is_none() {
            match fs::symlink_metadata(&self.sidecar_state_path) {
                Ok(_) => self.sidecar_state = Some(read_sidecar_state(&self.sidecar_state_path)?),
                Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
                Err(_) => return Err("failed to inspect desktop sidecar state".to_string()),
            }
        }
        if std::env::var_os("POLIS_DESKTOP_POLIS_EXE").is_some() {
            if let Some(pending) = self.sidecar_state.clone().filter(|state| state.pending) {
                let operation = sidecar_transition_label(&pending);
                let restored = pending.rollback_pending()?;
                self.persist_sidecar_state(&restored)?;
                self.snapshot.diagnostics = Some(format!(
                    "pending sidecar {operation} was rolled back because POLIS_DESKTOP_POLIS_EXE is set"
                ));
            }
        }
        match self.bootstrap_inner() {
            Ok(()) => Ok(()),
            Err(candidate_error) => {
                let sidecar_pending = self.sidecar_state.clone().filter(|sidecar| sidecar.pending);
                if let Some(pending) = sidecar_pending {
                    let operation = sidecar_transition_label(&pending);
                    if self
                        .generation_state
                        .as_ref()
                        .map(|generation| generation.pending)
                        .unwrap_or(false)
                    {
                        return Err(format!(
                            "desktop startup found simultaneous pending sidecar and data generation transitions: {candidate_error}"
                        ));
                    }
                    self.stop_owned_services_for_generation_recovery()?;
                    let restored = pending.rollback_pending()?;
                    self.persist_sidecar_state(&restored)?;
                    self.reset_runtime_snapshot();
                    if let Err(rollback_error) = self.bootstrap_inner() {
                        let cleanup_error =
                            self.stop_owned_services_for_generation_recovery().err();
                        return Err(match cleanup_error {
                            Some(cleanup_error) => format!(
                                "sidecar {operation} failed ({candidate_error}); restored binary startup failed ({rollback_error}); cleanup failed ({cleanup_error})"
                            ),
                            None => format!(
                                "sidecar {operation} failed ({candidate_error}); restored binary startup failed ({rollback_error})"
                            ),
                        });
                    }
                    self.snapshot.diagnostics = Some(format!(
                        "{}: {candidate_error}",
                        sidecar_failure_summary(&pending)
                    ));
                    return Ok(());
                }
                let pending = self
                    .generation_state
                    .clone()
                    .or_else(|| {
                        read_generation_state(&self.generation_state_path, DATABASE_NAME).ok()
                    })
                    .filter(|generation| generation.pending);
                let Some(pending) = pending else {
                    return Err(candidate_error);
                };
                self.stop_owned_services_for_generation_recovery()?;
                let restored = pending.rollback_pending()?;
                write_generation_state(&self.generation_state_path, &restored)?;
                self.generation_state = Some(restored);
                self.active_cas_root = None;
                self.reset_runtime_snapshot();
                if let Err(rollback_error) = self.bootstrap_inner() {
                    let cleanup_error = self.stop_owned_services_for_generation_recovery().err();
                    return Err(match cleanup_error {
                        Some(cleanup_error) => format!(
                            "candidate generation startup failed ({candidate_error}); previous generation startup failed ({rollback_error}); cleanup failed ({cleanup_error})"
                        ),
                        None => format!(
                            "candidate generation startup failed ({candidate_error}); previous generation startup failed ({rollback_error})"
                        ),
                    });
                }
                self.snapshot.diagnostics = Some(format!(
                    "pending generation startup failed and the previous generation was restored: {candidate_error}"
                ));
                Ok(())
            }
        }
    }

    fn stop_owned_services_for_generation_recovery(&mut self) -> Result<(), String> {
        if let (Some(endpoint), Some(token)) = (self.api_base_url.as_deref(), self.token.as_deref())
        {
            let response = request_raw(
                endpoint,
                "POST",
                "/api/desktop/maintenance/quiesce",
                token,
                &[],
            )?;
            require_accepted_receipt(&response, "desktop maintenance quiescence")?;
            let _ = request_raw(endpoint, "POST", "/api/desktop/shutdown", token, &[]);
        }
        stop_child(&mut self.backend, "backend", Path::new(&self.layout.logs))?;
        self.stop_postgres()?;
        self.api_base_url = None;
        self.token = None;
        self.dsn = None;
        self.active_migration_manifest_sha256 = None;
        Ok(())
    }

    fn reset_runtime_snapshot(&mut self) {
        self.snapshot = initial_snapshot(&self.root);
        self.snapshot.first_run_required = !self.config.initialized;
        self.snapshot.last_company_id = self.config.last_company_id.clone();
        self.active_migration_manifest_sha256 = None;
        self.refresh_sidecar_snapshot();
    }

    fn load_sidecar_state_from_disk(&mut self) -> Result<(), String> {
        if self.sidecar_state.is_some() {
            return Ok(());
        }
        match fs::symlink_metadata(&self.sidecar_state_path) {
            Ok(_) => {
                self.sidecar_state = Some(read_sidecar_state(&self.sidecar_state_path)?);
                self.refresh_sidecar_snapshot();
                Ok(())
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(_) => Err("failed to inspect desktop sidecar state".to_string()),
        }
    }

    fn ensure_sidecar_state(&mut self) -> Result<SidecarState, String> {
        self.load_sidecar_state_from_disk()?;
        if let Some(state) = &self.sidecar_state {
            return Ok(state.clone());
        }
        let installed = self.resolve_default_backend_executable()?;
        let active =
            SidecarRef::installed(installed.display().to_string(), file_sha256(&installed)?);
        let state = SidecarState::initial(active);
        state.validate()?;
        self.sidecar_state = Some(state.clone());
        self.refresh_sidecar_snapshot();
        Ok(state)
    }

    fn persist_sidecar_state(&mut self, state: &SidecarState) -> Result<(), String> {
        let cleanup_warning =
            persist_sidecar_state_file(&self.root, &self.sidecar_state_path, state)?;
        self.sidecar_state = Some(state.clone());
        self.refresh_sidecar_snapshot();
        if let Some(warning) = cleanup_warning {
            let _ = append_log(
                &PathBuf::from(&self.layout.logs).join("desktop-supervisor.log"),
                &format!("unreferenced sidecar cleanup was skipped: {warning}\n"),
            );
            if self.snapshot.diagnostics.is_none() {
                self.snapshot.diagnostics = Some(format!(
                    "sidecar state is durable; stale managed-file cleanup was skipped: {warning}"
                ));
            }
        }
        Ok(())
    }

    fn refresh_sidecar_snapshot(&mut self) {
        if let Some(state) = &self.sidecar_state {
            self.snapshot.active_sidecar_sha256 = Some(state.active.sha256.clone());
            self.snapshot.previous_sidecar_sha256 = state
                .previous
                .as_ref()
                .map(|sidecar| sidecar.sha256.clone());
            self.snapshot.staged_sidecar_sha256 =
                state.staged.as_ref().map(|sidecar| sidecar.sha256.clone());
            self.snapshot.sidecar_update_pending = state.pending;
        }
        let override_path = std::env::var_os("POLIS_DESKTOP_POLIS_EXE");
        self.snapshot.sidecar_update_blocked = override_path.is_some();
        if let Some(path) = override_path {
            if let Ok(digest) = file_sha256(&normalize_windows_path(PathBuf::from(path))) {
                self.snapshot.active_sidecar_sha256 = Some(digest);
            }
        }
    }

    fn require_sidecar_update_allowed(&self) -> Result<(), String> {
        if std::env::var_os("POLIS_DESKTOP_POLIS_EXE").is_some() {
            return Err(
                "sidecar updates are unavailable while POLIS_DESKTOP_POLIS_EXE is set".to_string(),
            );
        }
        Ok(())
    }

    fn resolve_pinned_sidecar(&self, sidecar: &ResolvedSidecar) -> Result<PathBuf, String> {
        let path = resolve_sidecar_binary(&self.root, &sidecar.reference)?;
        if path != sidecar.path {
            return Err("selected Polis sidecar path changed after resolution".to_string());
        }
        Ok(path)
    }

    fn read_sidecar_info(&self, sidecar: &ResolvedSidecar) -> Result<DesktopSidecarInfo, String> {
        let path = self.resolve_pinned_sidecar(sidecar)?;
        let output = Command::new(&path)
            .arg("sidecar-info")
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .output()
            .map_err(|_| "failed to inspect selected Polis sidecar information".to_string())?;
        let still_pinned = self.resolve_pinned_sidecar(sidecar)?;
        if still_pinned != path {
            return Err(
                "selected Polis sidecar changed while its migration manifest was inspected"
                    .to_string(),
            );
        }
        if !output.status.success() || output.stdout.len() > 16 * 1024 {
            return Err(
                "selected Polis sidecar did not return a valid sidecar-info report".to_string(),
            );
        }
        let info: DesktopSidecarInfo = serde_json::from_slice(&output.stdout).map_err(|_| {
            "selected Polis sidecar returned malformed sidecar-info JSON".to_string()
        })?;
        if info.service != "polis_backend" {
            return Err(
                "selected executable did not identify itself as the Polis backend".to_string(),
            );
        }
        validate_migration_manifest_compatibility(
            &info.migration_manifest_sha256,
            &info.migration_manifest_sha256,
        )?;
        if let Some(expected) = sidecar.reference.migration_manifest_sha256.as_deref() {
            validate_migration_manifest_compatibility(expected, &info.migration_manifest_sha256)?;
        }
        Ok(info)
    }

    fn bootstrap_inner(&mut self) -> Result<(), String> {
        self.ensure_layout()?;
        self.load_sidecar_state_from_disk()?;
        self.set_stage("starting_database");
        let dsn = self.start_database()?;
        self.dsn = Some(dsn.clone());
        self.snapshot.database.state = "ready".to_string();
        self.set_stage("migrating_database");
        let mut backend = self.resolve_backend_executable()?;
        let sidecar_info = self.read_sidecar_info(&backend)?;
        if let Some(manifest) = backend.reference.migration_manifest_sha256.as_deref() {
            validate_migration_manifest_compatibility(
                manifest,
                &sidecar_info.migration_manifest_sha256,
            )?;
        } else {
            backend.reference.migration_manifest_sha256 =
                Some(sidecar_info.migration_manifest_sha256.clone());
        }
        let development_override = std::env::var_os("POLIS_DESKTOP_POLIS_EXE").is_some();
        if !development_override {
            if let Some(mut state) = self.sidecar_state.clone() {
                if state.active.sha256 == backend.reference.sha256 && !state.pending {
                    state.active.migration_manifest_sha256 =
                        backend.reference.migration_manifest_sha256.clone();
                    self.sidecar_state = Some(state.clone());
                    if fs::symlink_metadata(&self.sidecar_state_path).is_ok() {
                        self.persist_sidecar_state(&state)?;
                    }
                }
            }
        }
        if let Some(state) = self
            .generation_state
            .clone()
            .filter(|generation| generation.pending)
        {
            if let Some(package_path) = state.pending_package_path.as_deref() {
                self.verify_generation_target(&backend, Path::new(package_path), &state.active)?;
            }
        }
        self.run_migrations(&backend, &dsn)?;
        self.set_stage("starting_polis_runtime");
        let (endpoint, token, migration_manifest_sha256) = self.start_backend(&backend, &dsn)?;
        self.active_migration_manifest_sha256 = Some(migration_manifest_sha256);
        self.api_base_url = Some(endpoint.clone());
        self.token = Some(token.clone());
        self.snapshot.api_base_url = Some(endpoint);
        self.snapshot.session_token = Some(token);
        self.snapshot.backend.state = "ready".to_string();
        self.set_stage("checking_ai_runtime");
        self.snapshot.provider = inspect_provider();
        self.snapshot.event_stream = ServiceSnapshot {
            state: "ready".to_string(),
            pid: None,
            started_at: Some(now()),
            exit_status: None,
            last_failure: None,
        };
        if let Some(generation) = self
            .generation_state
            .clone()
            .filter(|generation| generation.pending)
        {
            let committed = generation.commit()?;
            write_generation_state(&self.generation_state_path, &committed)?;
            self.generation_state = Some(committed);
        }
        if let Some(sidecar) = self.sidecar_state.clone().filter(|state| state.pending) {
            let committed = sidecar.commit_pending()?;
            self.persist_sidecar_state(&committed)?;
        }
        self.refresh_sidecar_snapshot();
        self.set_stage("ready");
        Ok(())
    }

    fn ensure_layout(&self) -> Result<(), String> {
        let paths = [
            self.root.clone(),
            self.root.join("config"),
            self.root.join("data"),
            self.root.join("postgres"),
            self.root.join("cas"),
            self.root.join("logs"),
            self.root.join("runtime"),
            self.root.join("temp"),
        ];
        for path in paths {
            fs::create_dir_all(&path).map_err(|err| {
                format!(
                    "failed to create desktop data directory {}: {err}",
                    path.display()
                )
            })?;
        }
        Ok(())
    }

    fn start_database(&mut self) -> Result<String, String> {
        let postgres_root = self.resolve_postgres_root()?;
        let initdb = find_binary(&postgres_root, "initdb").ok_or_else(|| {
            "managed PostgreSQL runtime is missing; configure the bundled PostgreSQL 18 runtime"
                .to_string()
        })?;
        let postgres = find_binary(&postgres_root, "postgres")
            .ok_or_else(|| "managed PostgreSQL server binary is missing".to_string())?;
        let psql = find_binary(&postgres_root, "psql")
            .ok_or_else(|| "managed PostgreSQL client binary is missing".to_string())?;
        let data_dir = PathBuf::from(&self.layout.postgres).join("data");
        self.postgres_root = Some(postgres_root.clone());
        self.postgres_data = Some(data_dir.clone());
        let port = free_port()?;
        let database_config: DatabaseConfig = if data_dir.join("PG_VERSION").exists() {
            read_json(&self.database_path).ok_or_else(|| "desktop database credentials are missing; refusing to guess or reset the private cluster".to_string())?
        } else {
            let admin_password = secret();
            let app_password = secret();
            fs::create_dir_all(&data_dir)
                .map_err(|err| format!("failed to create PostgreSQL data directory: {err}"))?;
            let password_file =
                PathBuf::from(&self.layout.temp).join(format!("initdb-{}.pwd", Uuid::new_v4()));
            write_private_file(&password_file, admin_password.as_bytes())?;
            let init_log_path = PathBuf::from(&self.layout.logs).join("postgres-initdb.log");
            rotate_log(&init_log_path)?;
            let init_log = OpenOptions::new()
                .create(true)
                .append(true)
                .open(&init_log_path)
                .map_err(|err| format!("failed to open initdb log: {err}"))?;
            let init_status = Command::new(&initdb)
                .args([
                    "-D",
                    data_dir.to_string_lossy().as_ref(),
                    "-U",
                    "postgres",
                    "--pwfile",
                ])
                .arg(&password_file)
                .args([
                    "--auth-host=scram-sha-256",
                    "--auth-local=scram-sha-256",
                    "--encoding=UTF8",
                    "--no-locale",
                ])
                .stdout(Stdio::from(init_log.try_clone().map_err(|err| {
                    format!("failed to duplicate initdb log: {err}")
                })?))
                .stderr(Stdio::from(init_log))
                .status()
                .map_err(|err| format!("failed to initialize PostgreSQL cluster: {err}"))?;
            let _ = fs::remove_file(&password_file);
            if !init_status.success() {
                return Err(format!(
                    "PostgreSQL initialization failed with exit status {}",
                    init_status.code().unwrap_or(-1)
                ));
            }
            let config = DatabaseConfig {
                username: DATABASE_USER.to_string(),
                password: app_password,
                database: DATABASE_NAME.to_string(),
                postgres_admin_password: Some(admin_password.clone()),
            };
            self.write_database_config(&config)?;
            self.pending_database_setup = Some((psql.clone(), admin_password, config.clone()));
            config
        };
        let generation_state =
            read_generation_state(&self.generation_state_path, &database_config.database)?;
        self.generation_state = Some(generation_state.clone());
        let active_cas_root = resolve_generation_cas_root(&self.root, &generation_state.active)?;
        self.active_cas_root = Some(active_cas_root);
        let log_path = PathBuf::from(&self.layout.logs).join("postgres.log");
        rotate_log(&log_path)?;
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)
            .map_err(|err| format!("failed to open PostgreSQL log: {err}"))?;
        let child = Command::new(&postgres)
            .args([
                "-D",
                data_dir.to_string_lossy().as_ref(),
                "-h",
                "127.0.0.1",
                "-p",
                &port.to_string(),
            ])
            .stdout(Stdio::from(log.try_clone().map_err(|err| {
                format!("failed to duplicate PostgreSQL log: {err}")
            })?))
            .stderr(Stdio::from(log))
            .stdin(Stdio::null())
            .spawn()
            .map_err(|err| format!("failed to start PostgreSQL: {err}"))?;
        self.postgres = Some(child);
        self.postgres_port = Some(port);
        if let Err(error) =
            assign_or_terminate(&mut self.postgres, |child| self.ownership.assign(child))
        {
            if self.postgres.is_none() {
                self.postgres_port = None;
            }
            return Err(error);
        }
        if let Err(error) = wait_for_tcp(port, Duration::from_secs(15)) {
            let stop_error = stop_child(
                &mut self.postgres,
                "PostgreSQL",
                Path::new(&self.layout.logs),
            )
            .err();
            if self.postgres.is_none() {
                self.postgres_port = None;
            }
            return Err(match stop_error {
                Some(stop_error) => {
                    format!("{error}; PostgreSQL process stop is unconfirmed: {stop_error}")
                }
                None => error,
            });
        }
        if let Some((setup_psql, admin_password, config)) = self.pending_database_setup.take() {
            wait_for_postgres_query_ready(
                &setup_psql,
                port,
                "postgres",
                &admin_password,
                "postgres",
                Duration::from_secs(15),
            )?;
            self.create_role_database(&setup_psql, port, &admin_password, &config)?;
        }
        wait_for_postgres_query_ready(
            &psql,
            port,
            &database_config.username,
            &database_config.password,
            &generation_state.active.database_name,
            Duration::from_secs(15),
        )?;
        let dsn = format!(
            "postgres://{}:{}@127.0.0.1:{}/{}?sslmode=disable",
            database_config.username,
            database_config.password,
            port,
            generation_state.active.database_name
        );
        self.snapshot.database.pid = self.postgres.as_ref().map(Child::id);
        self.snapshot.database.started_at = Some(now());
        Ok(dsn)
    }

    fn create_role_database(
        &self,
        psql: &Path,
        port: u16,
        admin_password: &str,
        config: &DatabaseConfig,
    ) -> Result<(), String> {
        let sql = format!("CREATE ROLE {} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD {}; CREATE DATABASE {} OWNER {};", quote_identifier(&config.username), quote_literal(&config.password), quote_identifier(&config.database), quote_identifier(&config.username));
        let mut child = Command::new(psql)
            .args([
                "-h",
                "127.0.0.1",
                "-p",
                &port.to_string(),
                "-U",
                "postgres",
                "-d",
                "postgres",
                "-v",
                "ON_ERROR_STOP=1",
            ])
            .env("PGPASSWORD", admin_password)
            .stdin(Stdio::piped())
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|err| format!("failed to create private PostgreSQL role: {err}"))?;
        let mut stdin = child
            .stdin
            .take()
            .ok_or_else(|| "failed to open PostgreSQL setup input".to_string())?;
        stdin
            .write_all(sql.as_bytes())
            .map_err(|err| format!("failed to send PostgreSQL setup command: {err}"))?;
        drop(stdin);
        let output = child
            .wait_with_output()
            .map_err(|err| format!("failed to finish PostgreSQL setup: {err}"))?;
        let setup_log = PathBuf::from(&self.layout.logs).join("postgres-setup.log");
        let _ = append_log(&setup_log, &String::from_utf8_lossy(&output.stderr));
        if !output.status.success() {
            return Err(format!(
                "failed to create private PostgreSQL database (exit {})",
                output.status.code().unwrap_or(-1)
            ));
        }
        Ok(())
    }

    pub fn stage_sidecar_update(
        &mut self,
        source_path: String,
    ) -> Result<SidecarStageReceipt, String> {
        self.require_sidecar_update_allowed()?;
        if source_path.trim().is_empty() {
            return Err("select a local Polis sidecar binary".to_string());
        }
        self.ensure_layout()?;
        let current = self.ensure_sidecar_state()?;
        if current.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        let candidate = stage_sidecar_binary(&self.root, Path::new(source_path.trim()))?;
        let selected = current.select_candidate(candidate.clone())?;
        let candidate_path = resolve_sidecar_binary(&self.root, &candidate)?;
        let size_bytes = fs::metadata(&candidate_path)
            .map_err(|_| "staged sidecar binary is unavailable".to_string())?
            .len();
        self.persist_sidecar_state(&selected)?;
        Ok(SidecarStageReceipt {
            status: "STAGED".to_string(),
            sha256: candidate.sha256,
            size_bytes,
        })
    }

    pub fn activate_staged_sidecar(&mut self, sha256: String) -> Result<RuntimeSnapshot, String> {
        self.require_sidecar_update_allowed()?;
        let active_manifest = self
            .active_migration_manifest_sha256
            .clone()
            .ok_or_else(|| {
                "the running Polis sidecar identity is unavailable; restart the desktop before updating"
                    .to_string()
            })?;
        if self.request_quit()? {
            return Err(
                "active work must stop before updating the local Polis sidecar".to_string(),
            );
        }
        let mut current = self.ensure_sidecar_state()?;
        if current.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        if current
            .active
            .migration_manifest_sha256
            .as_deref()
            .map(|manifest| manifest != active_manifest)
            .unwrap_or(false)
        {
            return Err(
                "running Polis identity differs from the persisted active sidecar manifest; restart before updating"
                    .to_string(),
            );
        }
        let mut candidate = current
            .staged
            .as_ref()
            .filter(|candidate| candidate.sha256 == sha256)
            .cloned()
            .ok_or_else(|| "requested SHA-256 is not the staged sidecar candidate".to_string())?;
        let candidate_path = resolve_sidecar_binary(&self.root, &candidate)?;
        let candidate_info = self.read_sidecar_info(&ResolvedSidecar {
            reference: candidate.clone(),
            path: candidate_path,
        })?;
        if validate_migration_manifest_compatibility(
            &active_manifest,
            &candidate_info.migration_manifest_sha256,
        )
        .is_err()
        {
            return Err(
                "candidate embeds a different migration manifest. This local updater accepts same-migration-set patch binaries only; no database rollback will be attempted. Select a candidate built with the current migration manifest."
                    .to_string(),
            );
        }
        current.active.migration_manifest_sha256 = Some(active_manifest);
        candidate.migration_manifest_sha256 = Some(candidate_info.migration_manifest_sha256);
        current.staged = Some(candidate.clone());
        self.persist_sidecar_state(&current)?;
        let retained_active = retain_sidecar_binary(&self.root, &current.active)?;
        let pending = current.begin_update(candidate, retained_active)?;

        self.shutdown(false)?;
        if let Err(error) = self.persist_sidecar_state(&pending) {
            self.sidecar_state = Some(current.clone());
            self.reset_runtime_snapshot();
            let restart = self.bootstrap_with_generation_recovery();
            return Err(match restart {
                Ok(()) => format!("failed to persist pending sidecar update: {error}"),
                Err(restart_error) => format!(
                    "failed to persist pending sidecar update ({error}); current sidecar restart failed ({restart_error})"
                ),
            });
        }
        self.reset_runtime_snapshot();
        self.bootstrap_with_generation_recovery()?;
        if self
            .snapshot
            .diagnostics
            .as_deref()
            .map(|diagnostic| diagnostic.starts_with("sidecar update failed to start"))
            .unwrap_or(false)
        {
            return Err(self.snapshot.diagnostics.clone().unwrap_or_else(|| {
                "sidecar update failed to start and previous binary restored".to_string()
            }));
        }
        Ok(self.snapshot())
    }

    pub fn rollback_previous_sidecar(
        &mut self,
        confirm_rollback: bool,
    ) -> Result<RuntimeSnapshot, String> {
        self.require_sidecar_update_allowed()?;
        if !confirm_rollback {
            return Err(
                "confirm that the retained previous Polis sidecar should become active".to_string(),
            );
        }
        if self.request_quit()? {
            return Err(
                "active work must stop before rolling back the local Polis sidecar".to_string(),
            );
        }
        let current = self.ensure_sidecar_state()?;
        if current.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        let active_manifest = self
            .active_migration_manifest_sha256
            .as_deref()
            .ok_or_else(|| {
                "the running Polis sidecar identity is unavailable; restart the desktop before rolling back"
                    .to_string()
            })?;
        if current.active.migration_manifest_sha256.as_deref() != Some(active_manifest) {
            return Err(
                "running Polis identity differs from the persisted active sidecar manifest; restart before rollback"
                    .to_string(),
            );
        }
        let pending = current.begin_previous_rollback()?;
        let rollback_path = resolve_sidecar_binary(&self.root, &pending.active)?;
        let rollback_info = self.read_sidecar_info(&ResolvedSidecar {
            reference: pending.active.clone(),
            path: rollback_path,
        })?;
        validate_migration_manifest_compatibility(
            active_manifest,
            &rollback_info.migration_manifest_sha256,
        )?;

        self.shutdown(false)?;
        if let Err(error) = self.persist_sidecar_state(&pending) {
            self.sidecar_state = Some(current.clone());
            self.reset_runtime_snapshot();
            let restart = self.bootstrap_with_generation_recovery();
            return Err(match restart {
                Ok(()) => format!("failed to persist pending manual sidecar rollback: {error}"),
                Err(restart_error) => format!(
                    "failed to persist pending manual sidecar rollback ({error}); current sidecar restart failed ({restart_error})"
                ),
            });
        }
        self.reset_runtime_snapshot();
        self.bootstrap_with_generation_recovery()?;
        if self
            .snapshot
            .diagnostics
            .as_deref()
            .map(|diagnostic| diagnostic.starts_with("sidecar manual rollback failed to start"))
            .unwrap_or(false)
        {
            return Err(self.snapshot.diagnostics.clone().unwrap_or_else(|| {
                "sidecar manual rollback failed to start and current binary restored".to_string()
            }));
        }
        Ok(self.snapshot())
    }

    pub fn stage_recovery_generation(
        &mut self,
        package_root: String,
        administrator_password: Option<String>,
    ) -> Result<RecoveryGenerationStageReceipt, String> {
        if self.request_quit()? {
            return Err("active work must stop before staging a recovery generation".to_string());
        }
        let package_path = PathBuf::from(package_root);
        if !package_path.is_absolute() {
            return Err("recovery package path must be absolute".to_string());
        }
        let package_path = fs::canonicalize(&package_path)
            .map_err(|_| "recovery package directory is unavailable".to_string())?;
        let backend = self.resolve_backend_executable()?;
        let package_report: RecoveryBackupVerification = self.run_sidecar_json(
            &backend,
            "recovery package verification",
            &[
                OsString::from("recovery-backup-verify"),
                package_path.as_os_str().to_os_string(),
            ],
            &[],
        )?;
        if package_report.status != "PASSED" {
            return Err("recovery package did not pass verification".to_string());
        }
        let generation_id = package_report.generation_id;
        let generation = RuntimeGenerationRef {
            generation_id: generation_id.clone(),
            database_name: format!("polis_recovery_{generation_id}"),
            cas_relative_path: format!("generations/{generation_id}/cas"),
            recovery_manifest_sha256: Some(package_report.manifest_sha256),
        };
        generation.validate()?;
        if package_report.runtime_role_name != DATABASE_USER {
            return Err(
                "recovery package runtime role differs from the desktop runtime role".to_string(),
            );
        }
        if self
            .generation_state
            .as_ref()
            .map(|state| {
                state.active.generation_id == generation.generation_id
                    || state
                        .previous
                        .as_ref()
                        .map(|previous| previous.generation_id == generation.generation_id)
                        .unwrap_or(false)
            })
            .unwrap_or(false)
        {
            return Err(
                "recovery generation is already active or retained for rollback".to_string(),
            );
        }

        let generation_directory = self
            .root
            .join("generations")
            .join(&generation.generation_id);
        if fs::symlink_metadata(&generation_directory).is_ok() {
            let generation_directory =
                resolve_generation_directory(&self.root, &generation.generation_id)?;
            let manifest_path = generation_directory.join("generation.json");
            if let Ok(prepared) = read_prepared_generation(&manifest_path) {
                if prepared.generation != generation
                    || Path::new(&prepared.recovery_package_path) != package_path.as_path()
                {
                    return Err(
                        "generation directory is already bound to another recovery package"
                            .to_string(),
                    );
                }
            }
            self.verify_generation_target(&backend, &package_path, &generation)?;
            if read_prepared_generation(&manifest_path).is_err() {
                let recovered = PreparedGenerationManifest {
                    schema_version: "polis-desktop-prepared-generation@1".to_string(),
                    generation: generation.clone(),
                    recovery_package_path: package_path.to_string_lossy().to_string(),
                };
                write_prepared_generation(&manifest_path, &recovered)?;
            }
            return Ok(RecoveryGenerationStageReceipt {
                status: "ALREADY_STAGED".to_string(),
                generation_id: generation.generation_id,
                database_name: generation.database_name,
                manifest_sha256: generation.recovery_manifest_sha256.unwrap_or_default(),
            });
        }

        let database_config: DatabaseConfig = read_json(&self.database_path)
            .ok_or_else(|| "desktop database credentials are unavailable".to_string())?;
        let admin_password = database_config
            .postgres_admin_password
            .clone()
            .or(administrator_password)
            .ok_or_else(|| {
                "local PostgreSQL administrator password is required to restore a generation"
                    .to_string()
            })?;
        let port = self
            .postgres_port
            .ok_or_else(|| "managed PostgreSQL server is not running".to_string())?;
        let postgres_root = self
            .postgres_root
            .as_ref()
            .ok_or_else(|| "managed PostgreSQL runtime is unavailable".to_string())?;
        let psql = find_binary(postgres_root, "psql")
            .ok_or_else(|| "managed PostgreSQL client binary is missing".to_string())?;
        if self.generation_database_exists(
            &psql,
            port,
            &admin_password,
            &generation.database_name,
        )? {
            return Err("target database already exists without a matching prepared generation; refusing to overwrite it".to_string());
        }
        let generation_directory = create_generation_directory(&self.root, &generation)?;
        if database_config.postgres_admin_password.is_none() {
            let mut updated = database_config.clone();
            updated.postgres_admin_password = Some(admin_password.clone());
            self.write_database_config(&updated)?;
        }
        if let Err(error) = self.create_generation_database(
            &psql,
            port,
            &admin_password,
            &database_config.username,
            &generation.database_name,
        ) {
            let _ = fs::remove_dir(&generation_directory);
            return Err(error);
        }

        let target_dsn = Self::database_dsn(&database_config, port, &generation.database_name);
        let cas_root = generation_directory.join("cas");
        let pg_restore = find_binary(postgres_root, "pg_restore")
            .ok_or_else(|| "managed PostgreSQL restore binary is missing".to_string())?;
        let restore: RecoveryRestoreResult = self.run_sidecar_json(
            &backend,
            "recovery package restore",
            &[
                OsString::from("recovery-backup-restore"),
                package_path.as_os_str().to_os_string(),
                cas_root.as_os_str().to_os_string(),
            ],
            &[
                ("POLIS_RESTORE_DSN", OsString::from(&target_dsn)),
                (
                    "POLIS_RUNTIME_ROLE",
                    OsString::from(&database_config.username),
                ),
                (
                    "POLIS_PG_RESTORE_PATH",
                    pg_restore.as_os_str().to_os_string(),
                ),
            ],
        )?;
        if (restore.status != "RESTORED" && restore.status != "ALREADY_RESTORED")
            || restore.generation_id != generation.generation_id
            || restore.database_name != generation.database_name
        {
            return Err(
                "recovery restore result does not match the requested generation".to_string(),
            );
        }
        self.verify_generation_target(&backend, &package_path, &generation)?;
        let prepared = PreparedGenerationManifest {
            schema_version: "polis-desktop-prepared-generation@1".to_string(),
            generation: generation.clone(),
            recovery_package_path: package_path.to_string_lossy().to_string(),
        };
        write_prepared_generation(&generation_directory.join("generation.json"), &prepared)?;
        Ok(RecoveryGenerationStageReceipt {
            status: "STAGED".to_string(),
            generation_id: generation.generation_id,
            database_name: generation.database_name,
            manifest_sha256: generation.recovery_manifest_sha256.unwrap_or_default(),
        })
    }

    fn verify_generation_target(
        &self,
        backend: &ResolvedSidecar,
        package_path: &Path,
        generation: &RuntimeGenerationRef,
    ) -> Result<RestoredGenerationVerification, String> {
        let cas_root = resolve_generation_cas_root(&self.root, generation)?;
        let database_config: DatabaseConfig = read_json(&self.database_path)
            .ok_or_else(|| "desktop database credentials are unavailable".to_string())?;
        let port = self
            .postgres_port
            .ok_or_else(|| "managed PostgreSQL server is not running".to_string())?;
        let dsn = Self::database_dsn(&database_config, port, &generation.database_name);
        let verified: RestoredGenerationVerification = self.run_sidecar_json(
            backend,
            "restored generation verification",
            &[
                OsString::from("recovery-generation-verify"),
                package_path.as_os_str().to_os_string(),
                cas_root.as_os_str().to_os_string(),
            ],
            &[("POLIS_GENERATION_DSN", OsString::from(&dsn))],
        )?;
        if verified.status != "PASSED"
            || verified.generation_id != generation.generation_id
            || Some(verified.manifest_sha256.as_str())
                != generation.recovery_manifest_sha256.as_deref()
            || verified.database_name != generation.database_name
        {
            return Err(
                "restored generation verification does not match its prepared descriptor"
                    .to_string(),
            );
        }
        Ok(verified)
    }

    fn generation_database_exists(
        &self,
        psql: &Path,
        port: u16,
        admin_password: &str,
        database_name: &str,
    ) -> Result<bool, String> {
        let query = format!(
            "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname={})",
            quote_literal(database_name)
        );
        let output = Command::new(psql)
            .args([
                "-h",
                "127.0.0.1",
                "-p",
                &port.to_string(),
                "-U",
                "postgres",
                "-d",
                "postgres",
                "-A",
                "-t",
                "-v",
                "ON_ERROR_STOP=1",
                "-c",
                &query,
            ])
            .env("PGPASSWORD", admin_password)
            .output()
            .map_err(|_| "failed to inspect recovery database name".to_string())?;
        if !output.status.success() {
            return Err("failed to inspect recovery database name".to_string());
        }
        match String::from_utf8_lossy(&output.stdout).trim() {
            "t" => Ok(true),
            "f" => Ok(false),
            _ => Err("PostgreSQL returned an invalid database existence result".to_string()),
        }
    }

    fn create_generation_database(
        &self,
        psql: &Path,
        port: u16,
        admin_password: &str,
        runtime_role: &str,
        database_name: &str,
    ) -> Result<(), String> {
        let statement = format!(
            "CREATE DATABASE {} OWNER {}",
            quote_identifier(database_name),
            quote_identifier(runtime_role)
        );
        let status = Command::new(psql)
            .args([
                "-h",
                "127.0.0.1",
                "-p",
                &port.to_string(),
                "-U",
                "postgres",
                "-d",
                "postgres",
                "-v",
                "ON_ERROR_STOP=1",
                "-c",
                &statement,
            ])
            .env("PGPASSWORD", admin_password)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .map_err(|_| "failed to create recovery target database".to_string())?;
        if !status.success() {
            return Err("failed to create recovery target database".to_string());
        }
        Ok(())
    }

    fn database_dsn(config: &DatabaseConfig, port: u16, database_name: &str) -> String {
        format!(
            "postgres://{}:{}@127.0.0.1:{}/{}?sslmode=disable",
            config.username, config.password, port, database_name
        )
    }

    fn run_sidecar_json<T: DeserializeOwned>(
        &self,
        backend: &ResolvedSidecar,
        operation: &str,
        args: &[OsString],
        environment: &[(&str, OsString)],
    ) -> Result<T, String> {
        let path = self.resolve_pinned_sidecar(backend)?;
        let mut command = Command::new(path);
        command.args(args);
        for (key, value) in environment {
            command.env(key, value);
        }
        let output = command
            .output()
            .map_err(|_| format!("failed to start {operation}"))?;
        if !output.status.success() {
            return Err(format!(
                "{operation} failed with exit status {}",
                output.status.code().unwrap_or(-1)
            ));
        }
        if output.stdout.len() > 64 * 1024 {
            return Err(format!("{operation} returned an oversized result"));
        }
        serde_json::from_slice(&output.stdout)
            .map_err(|_| format!("{operation} returned an invalid result"))
    }

    pub fn activate_staged_generation(
        &mut self,
        generation_id: String,
    ) -> Result<RuntimeSnapshot, String> {
        if self.request_quit()? {
            return Err(
                "active work must stop before changing the desktop data generation".to_string(),
            );
        }
        let generation_directory = self.root.join("generations").join(&generation_id);
        let prepared = read_prepared_generation(&generation_directory.join("generation.json"))?;
        if prepared.generation.generation_id != generation_id {
            return Err(
                "prepared generation ID does not match the requested generation".to_string(),
            );
        }
        let current = self
            .generation_state
            .clone()
            .unwrap_or(read_generation_state(
                &self.generation_state_path,
                DATABASE_NAME,
            )?);
        if current.pending {
            return Err("another generation switch is already pending".to_string());
        }
        if current.active.generation_id == generation_id {
            return Err("requested generation is already active".to_string());
        }
        let backend = self.resolve_backend_executable()?;
        self.verify_generation_target(
            &backend,
            Path::new(&prepared.recovery_package_path),
            &prepared.generation,
        )?;
        let mut pending = current.clone().begin_switch(prepared.generation.clone())?;
        pending.pending_package_path = Some(prepared.recovery_package_path.clone());
        self.apply_generation_transition(current, pending, generation_id)
    }

    pub fn rollback_previous_generation(
        &mut self,
        confirm_data_branch: bool,
    ) -> Result<RuntimeSnapshot, String> {
        if !confirm_data_branch {
            return Err(
                "confirm that the retained previous data generation should become active"
                    .to_string(),
            );
        }
        if self.request_quit()? {
            return Err(
                "active work must stop before changing the desktop data generation".to_string(),
            );
        }
        let current = self
            .generation_state
            .clone()
            .unwrap_or(read_generation_state(
                &self.generation_state_path,
                DATABASE_NAME,
            )?);
        let previous = current
            .previous
            .as_ref()
            .ok_or_else(|| "no previous data generation is retained".to_string())?;
        resolve_generation_cas_root(&self.root, previous)?;
        let previous_id = previous.generation_id.clone();
        let pending = current.clone().begin_previous_rollback()?;
        self.apply_generation_transition(current, pending, previous_id)
    }

    fn apply_generation_transition(
        &mut self,
        previous: GenerationState,
        pending: GenerationState,
        expected_active_id: String,
    ) -> Result<RuntimeSnapshot, String> {
        if let Err(shutdown_error) = self.shutdown(false) {
            self.snapshot.diagnostics = Some(format!(
                "generation switch did not change the active pointer because shutdown was not confirmed: {shutdown_error}"
            ));
            return Err(shutdown_error);
        }

        if let Err(write_error) = write_generation_state(&self.generation_state_path, &pending) {
            let rollback_write = write_generation_state(&self.generation_state_path, &previous);
            self.generation_state = Some(previous);
            self.active_cas_root = None;
            self.reset_runtime_snapshot();
            if let Err(rollback_error) = rollback_write {
                return Err(format!(
                    "generation state write failed ({write_error}); previous pointer restore failed ({rollback_error}); services remain stopped"
                ));
            }
            if let Err(restart_error) = self.bootstrap_inner() {
                return Err(format!(
                    "generation state write failed ({write_error}); previous runtime restart failed ({restart_error})"
                ));
            }
            return Err(format!(
                "generation state could not be switched; previous generation was restarted: {write_error}"
            ));
        }

        self.generation_state = Some(pending);
        self.active_cas_root = None;
        self.reset_runtime_snapshot();
        if let Err(start_error) = self.bootstrap_with_generation_recovery() {
            self.fail_current_stage(start_error.clone());
            return Err(start_error);
        }
        if self
            .generation_state
            .as_ref()
            .map(|state| state.active.generation_id.as_str())
            != Some(expected_active_id.as_str())
        {
            return Err(
                "candidate startup failed; the previous generation was restored".to_string(),
            );
        }
        Ok(self.snapshot())
    }

    fn run_migrations(&self, backend: &ResolvedSidecar, dsn: &str) -> Result<(), String> {
        let blob_root = self
            .active_cas_root
            .as_ref()
            .ok_or_else(|| "active generation CAS root is unavailable".to_string())?;
        let log_path = PathBuf::from(&self.layout.logs).join("backend.log");
        rotate_log(&log_path)?;
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)
            .map_err(|err| format!("failed to open backend log: {err}"))?;
        if backend.reference.migration_manifest_sha256.is_none() {
            return Err(
                "selected sidecar has no pinned migration manifest before migration".to_string(),
            );
        }
        let pinned_path = self.resolve_pinned_sidecar(backend)?;
        let status = Command::new(pinned_path)
            .arg("migrate")
            .env("POLIS_DSN", dsn)
            .env("POLIS_BLOB_ROOT", blob_root)
            .stdout(Stdio::from(log.try_clone().map_err(|err| {
                format!("failed to duplicate backend log: {err}")
            })?))
            .stderr(Stdio::from(log))
            .status()
            .map_err(|err| format!("failed to run database migrations: {err}"))?;
        if !status.success() {
            return Err(format!(
                "database migration failed with exit status {}",
                status.code().unwrap_or(-1)
            ));
        }
        Ok(())
    }

    fn start_backend(
        &mut self,
        backend: &ResolvedSidecar,
        dsn: &str,
    ) -> Result<(String, String, String), String> {
        let blob_root = self
            .active_cas_root
            .as_ref()
            .ok_or_else(|| "active generation CAS root is unavailable".to_string())?;
        let port = free_port()?;
        let token = secret();
        let endpoint = format!("http://127.0.0.1:{port}");
        let log_path = PathBuf::from(&self.layout.logs).join("backend.log");
        rotate_log(&log_path)?;
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)
            .map_err(|err| format!("failed to open backend log: {err}"))?;
        let pinned_path = self.resolve_pinned_sidecar(backend)?;
        let child = Command::new(pinned_path)
            .arg("serve")
            .env("POLIS_DSN", dsn)
            .env("POLIS_BLOB_ROOT", blob_root)
            .env(
                "POLIS_MEMORY_REVOCATION_ROOT",
                self.root.join("memory-revocations"),
            )
            .env("POLIS_WORKBENCH_ADDR", format!("127.0.0.1:{port}"))
            .env("POLIS_WORKER_MODE", "deterministic")
            .env("POLIS_DESKTOP_SESSION_TOKEN", &token)
            .stdout(Stdio::from(log.try_clone().map_err(|err| {
                format!("failed to duplicate backend log: {err}")
            })?))
            .stderr(Stdio::from(log))
            .stdin(Stdio::null())
            .spawn()
            .map_err(|err| format!("failed to start Polis backend: {err}"))?;
        self.backend = Some(child);
        assign_or_terminate(&mut self.backend, |child| self.ownership.assign(child))?;
        wait_for_http(&endpoint, "/healthz", &token, Duration::from_secs(20))?;
        let identity_response =
            request_raw(&endpoint, "GET", "/api/desktop/identity", &token, &[])?;
        let identity: DesktopBackendIdentity = serde_json::from_slice(&identity_response)
            .map_err(|_| "Polis backend identity response is invalid".to_string())?;
        validate_launch_identity(
            &backend.reference,
            &identity.executable_sha256,
            &identity.migration_manifest_sha256,
        )?;
        if identity.service != "polis_backend" {
            return Err("Polis backend identity reports the wrong service".to_string());
        }
        self.snapshot.backend.pid = self.backend.as_ref().map(Child::id);
        self.snapshot.backend.started_at = Some(now());
        Ok((endpoint, token, identity.migration_manifest_sha256))
    }

    fn resolve_backend_executable(&mut self) -> Result<ResolvedSidecar, String> {
        if let Some(value) = std::env::var_os("POLIS_DESKTOP_POLIS_EXE") {
            let path = normalize_windows_path(PathBuf::from(value));
            let reference = SidecarRef::installed(path.display().to_string(), file_sha256(&path)?);
            let resolved = resolve_sidecar_binary(&self.root, &reference)?;
            self.snapshot.active_sidecar_sha256 = Some(reference.sha256.clone());
            self.snapshot.sidecar_update_blocked = true;
            return Ok(ResolvedSidecar {
                reference,
                path: resolved,
            });
        }
        self.snapshot.sidecar_update_blocked = false;
        let state = self.ensure_sidecar_state()?;
        let resolved = resolve_sidecar_binary(&self.root, &state.active)?;
        self.snapshot.active_sidecar_sha256 = Some(state.active.sha256.clone());
        Ok(ResolvedSidecar {
            reference: state.active,
            path: resolved,
        })
    }

    fn resolve_default_backend_executable(&self) -> Result<PathBuf, String> {
        let mut candidates = Vec::new();
        if let Ok(resource) = self.app.path().resource_dir() {
            candidates.push(normalize_windows_path(
                resource.join("binaries").join("polis.exe"),
            ));
            candidates.push(normalize_windows_path(resource.join("polis.exe")));
        }
        if let Ok(current) = std::env::current_exe() {
            if let Some(parent) = current.parent() {
                candidates.push(normalize_windows_path(parent.join("polis.exe")));
            }
        }
        candidates
            .into_iter()
            .find(|path| path.is_file())
            .ok_or_else(|| {
                "Polis backend sidecar is missing; install a package containing polis.exe"
                    .to_string()
            })
    }

    fn resolve_postgres_root(&self) -> Result<PathBuf, String> {
        if let Ok(value) = std::env::var("POLIS_DESKTOP_POSTGRES_ROOT") {
            let root = normalize_windows_path(PathBuf::from(value));
            if root.exists() {
                return Ok(root);
            }
        }
        let resource = self
            .app
            .path()
            .resource_dir()
            .map_err(|err| format!("failed to resolve packaged resources: {err}"))?;
        let mut candidates = vec![
            resource.join("postgres"),
            resource.join("resources").join("postgres"),
        ];
        if let Ok(current) = std::env::current_exe() {
            if let Some(parent) = current.parent() {
                candidates.push(parent.join("postgres"));
                candidates.push(parent.join("resources").join("postgres"));
            }
        }
        candidates
            .into_iter()
            .map(normalize_windows_path)
            .find(|path| find_binary(path, "postgres").is_some())
            .ok_or_else(|| {
                "managed PostgreSQL runtime is missing from packaged resources".to_string()
            })
    }

    fn write_database_config(&self, config: &DatabaseConfig) -> Result<(), String> {
        let bytes = serde_json::to_vec_pretty(config)
            .map_err(|err| format!("failed to encode database config: {err}"))?;
        write_private_file(&self.database_path, &bytes)
    }

    fn set_stage(&mut self, stage: &str) {
        self.snapshot.startup_stage = stage.to_string();
    }

    fn fail_current_stage(&mut self, error: String) {
        self.snapshot.diagnostics = Some(error.clone());
        let _ = append_log(
            &PathBuf::from(&self.layout.logs).join("desktop-supervisor.log"),
            &format!(
                "startup stage {} failed: {error}\n",
                self.snapshot.startup_stage
            ),
        );
        if self.snapshot.startup_stage.contains("database")
            || self.snapshot.startup_stage == "migrating_database"
        {
            self.snapshot.database = ServiceSnapshot::failed(error);
        } else {
            self.snapshot.backend = ServiceSnapshot::failed(error);
        }
        self.snapshot.startup_stage = "failed".to_string();
    }

    pub fn snapshot(&mut self) -> RuntimeSnapshot {
        self.refresh_processes();
        self.refresh_sidecar_snapshot();
        if let Some(state) = &self.generation_state {
            self.snapshot.active_generation_id = Some(state.active.generation_id.clone());
            self.snapshot.previous_generation_id = state
                .previous
                .as_ref()
                .map(|generation| generation.generation_id.clone());
            self.snapshot.generation_pending = state.pending;
        }
        self.snapshot.clone()
    }

    pub fn recheck_provider(&mut self) -> RuntimeSnapshot {
        self.snapshot.provider = inspect_provider();
        self.snapshot()
    }

    pub fn set_last_company(&mut self, company_id: String) -> Result<(), String> {
        if company_id.trim().is_empty() {
            return Err("company id cannot be empty".to_string());
        }
        self.config.initialized = true;
        self.config.last_company_id = Some(company_id);
        self.snapshot.first_run_required = false;
        self.snapshot.last_company_id = self.config.last_company_id.clone();
        let bytes = serde_json::to_vec_pretty(&self.config)
            .map_err(|err| format!("failed to encode desktop config: {err}"))?;
        write_private_file(&self.config_path, &bytes)
    }

    pub fn request_quit(&mut self) -> Result<bool, String> {
        if self.api_base_url.is_none() || self.token.is_none() {
            return Ok(false);
        }
        let response = match self.request_backend("GET", "/api/desktop/active-work") {
            Ok(response) => response,
            Err(error) if self.snapshot.backend.state == "failed" => {
                let _ = error;
                return Ok(false);
            }
            Err(error) => return Err(error),
        };
        let parsed: Value = serde_json::from_slice(&response)
            .map_err(|_| "failed to read active-work status".to_string())?;
        Ok(parsed
            .get("active")
            .and_then(Value::as_bool)
            .unwrap_or(false))
    }

    pub fn shutdown(&mut self, force_active_work: bool) -> Result<(), String> {
        if self.api_base_url.is_some() {
            self.quiesce_backend_for_maintenance(force_active_work)?;
        }
        if let (Some(endpoint), Some(token)) = (self.api_base_url.as_deref(), self.token.as_deref())
        {
            let _ = request_raw(endpoint, "POST", "/api/desktop/shutdown", token, &[]);
        }
        stop_child(&mut self.backend, "backend", Path::new(&self.layout.logs))?;
        self.stop_postgres()?;
        self.api_base_url = None;
        self.token = None;
        self.dsn = None;
        self.active_migration_manifest_sha256 = None;
        self.snapshot.database.state = "stopped".to_string();
        self.snapshot.backend.state = "stopped".to_string();
        self.snapshot.event_stream.state = "stopped".to_string();
        self.snapshot.startup_stage = "stopped".to_string();
        Ok(())
    }

    fn stop_postgres(&mut self) -> Result<(), String> {
        let mut stop_confirmed = false;
        if let (Some(root), Some(data)) = (self.postgres_root.as_ref(), self.postgres_data.as_ref())
        {
            if let Some(pg_ctl) = find_binary(root, "pg_ctl") {
                let status = Command::new(pg_ctl)
                    .args([
                        "-D",
                        data.to_string_lossy().as_ref(),
                        "-m",
                        "fast",
                        "-w",
                        "stop",
                    ])
                    .stdout(Stdio::null())
                    .stderr(Stdio::null())
                    .status();
                if status.map(|value| value.success()).unwrap_or(false) {
                    stop_confirmed = true;
                }
            }
        }
        if self.postgres.is_some() {
            stop_child(
                &mut self.postgres,
                "PostgreSQL",
                Path::new(&self.layout.logs),
            )?;
        } else if self.postgres_port.is_some() && !stop_confirmed {
            return Err(
                "PostgreSQL stop is unconfirmed and no child handle remains for recovery"
                    .to_string(),
            );
        }
        self.postgres_port = None;
        Ok(())
    }

    fn quiesce_backend_for_maintenance(&self, allow_active_work: bool) -> Result<(), String> {
        let path = if allow_active_work {
            "/api/desktop/maintenance/quiesce?allow-active=1"
        } else {
            "/api/desktop/maintenance/quiesce"
        };
        let response = self.request_backend("POST", path)?;
        require_accepted_receipt(&response, "desktop maintenance quiescence")
    }

    pub fn restart(&mut self) -> Result<RuntimeSnapshot, String> {
        self.shutdown(false)?;
        self.reset_runtime_snapshot();
        self.bootstrap_with_generation_recovery()?;
        Ok(self.snapshot())
    }

    pub fn open_logs(&self) -> Result<(), String> {
        #[cfg(windows)]
        {
            Command::new("explorer.exe")
                .arg(&self.layout.logs)
                .spawn()
                .map_err(|err| format!("failed to open logs directory: {err}"))?;
        }
        #[cfg(not(windows))]
        {
            Command::new("xdg-open")
                .arg(&self.layout.logs)
                .spawn()
                .map_err(|err| format!("failed to open logs directory: {err}"))?;
        }
        Ok(())
    }

    pub fn diagnostics(&mut self) -> String {
        let snapshot = self.snapshot();
        format!("Polis desktop diagnostics\nstartup_stage={}\ndata_root={}\nlogs_path={}\ndatabase_state={}\nbackend_state={}\nprovider_state={}\nprovider_auth_state={}\nprovider_qualification_state={}\napi_endpoint={}\ndiagnostic={}\n", snapshot.startup_stage, snapshot.data_root, snapshot.logs_path, snapshot.database.state, snapshot.backend.state, snapshot.provider.state, snapshot.provider.auth_state, snapshot.provider.qualification_state, snapshot.api_base_url.as_deref().unwrap_or("unavailable"), snapshot.diagnostics.as_deref().unwrap_or("none"))
    }

    fn request_backend(&self, method: &str, path: &str) -> Result<Vec<u8>, String> {
        let endpoint = self
            .api_base_url
            .as_deref()
            .ok_or_else(|| "Polis backend is not ready".to_string())?;
        let token = self
            .token
            .as_deref()
            .ok_or_else(|| "desktop session token is unavailable".to_string())?;
        request_raw(endpoint, method, path, token, &[])
    }

    fn refresh_processes(&mut self) {
        refresh_child(&mut self.postgres, &mut self.snapshot.database);
        refresh_child(&mut self.backend, &mut self.snapshot.backend);
    }
}

fn sidecar_transition_label(state: &SidecarState) -> &'static str {
    match &state.pending_operation {
        Some(SidecarTransitionKind::Update) => "update",
        Some(SidecarTransitionKind::ManualRollback) => "manual rollback",
        None => "unknown transition",
    }
}

fn sidecar_failure_summary(state: &SidecarState) -> &'static str {
    match &state.pending_operation {
        Some(SidecarTransitionKind::Update) => {
            "sidecar update failed to start; previous binary restored"
        }
        Some(SidecarTransitionKind::ManualRollback) => {
            "sidecar manual rollback failed to start; current binary restored"
        }
        None => "sidecar transition failed to start; active binary restored",
    }
}

fn inspect_provider() -> ProviderSnapshot {
    let manifest = std::env::var("POLIS_PROVIDER_RUNTIME_MANIFEST")
        .ok()
        .map(PathBuf::from);
    let binary = std::env::var("POLIS_PROVIDER_BINARY")
        .ok()
        .map(PathBuf::from);
    let helper = std::env::var("POLIS_PROVIDER_HELPER_BINARY")
        .ok()
        .map(PathBuf::from);
    let auth = std::env::var("POLIS_PROVIDER_AUTH_FILE")
        .ok()
        .map(PathBuf::from);
    if manifest.is_none() && binary.is_none() && helper.is_none() && auth.is_none() {
        return ProviderSnapshot::setup_required("controlled provider runtime is not configured; no provider binary was discovered from PATH");
    }
    if missing_path(&manifest) || missing_path(&binary) || missing_path(&helper) {
        return ProviderSnapshot {
            state: "degraded".to_string(),
            runtime_version: None,
            auth_state: "runtime_missing".to_string(),
            qualification_state: "unknown".to_string(),
            reason: Some("controlled provider manifest or binary is missing".to_string()),
        };
    }
    if missing_path(&auth) {
        return ProviderSnapshot {
            state: "degraded".to_string(),
            runtime_version: None,
            auth_state: "authentication_required".to_string(),
            qualification_state: "unknown".to_string(),
            reason: Some("controlled provider authentication is not ready".to_string()),
        };
    }
    let manifest_path = manifest.expect("manifest path checked above");
    let binary_path = binary.expect("binary path checked above");
    let helper_path = helper.expect("helper path checked above");
    let auth_path = auth.expect("auth path checked above");
    let manifest_raw = match fs::read_to_string(&manifest_path) {
        Ok(raw) => raw,
        Err(_) => {
            return degraded_provider(
                "runtime_manifest_unreadable",
                "controlled provider manifest cannot be read",
            )
        }
    };
    let manifest = match parse_provider_manifest(&manifest_raw) {
        Ok(manifest) => manifest,
        Err(reason) => return degraded_provider("runtime_manifest_invalid", &reason),
    };
    let binary_hash = match file_sha256(&binary_path) {
        Ok(hash) => hash,
        Err(_) => {
            return degraded_provider(
                "runtime_binary_unreadable",
                "controlled provider binary cannot be hashed",
            )
        }
    };
    let helper_hash = match file_sha256(&helper_path) {
        Ok(hash) => hash,
        Err(_) => {
            return degraded_provider(
                "runtime_helper_unreadable",
                "controlled provider helper cannot be hashed",
            )
        }
    };
    if binary_hash != manifest.binary_sha256 || helper_hash != manifest.helper_sha256 {
        return degraded_provider(
            "runtime_artifact_mismatch",
            "controlled provider artifact hash does not match its manifest",
        );
    }
    let auth_raw = match fs::read_to_string(&auth_path) {
        Ok(raw) => raw,
        Err(_) => {
            return degraded_provider(
                "authentication_unreadable",
                "controlled provider authentication cannot be read",
            )
        }
    };
    if let Err(reason) = validate_provider_auth(&auth_raw) {
        return degraded_provider("authentication_invalid", &reason);
    }
    ProviderSnapshot {
        state: "ready".to_string(),
        runtime_version: Some(manifest.version),
        auth_state: "ready".to_string(),
        qualification_state: "verified".to_string(),
        reason: None,
    }
}

fn degraded_provider(auth_state: &str, reason: &str) -> ProviderSnapshot {
    ProviderSnapshot {
        state: "degraded".to_string(),
        runtime_version: None,
        auth_state: auth_state.to_string(),
        qualification_state: "unknown".to_string(),
        reason: Some(reason.to_string()),
    }
}

fn file_sha256(path: &Path) -> Result<String, String> {
    let mut file = File::open(path).map_err(|err| err.to_string())?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 64 * 1024];
    loop {
        let count = file.read(&mut buffer).map_err(|err| err.to_string())?;
        if count == 0 {
            break;
        }
        hasher.update(&buffer[..count]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

fn missing_path(path: &Option<PathBuf>) -> bool {
    path.as_ref().map(|value| !value.is_file()).unwrap_or(true)
}

fn find_binary(root: &Path, name: &str) -> Option<PathBuf> {
    let executable = if cfg!(windows) {
        format!("{name}.exe")
    } else {
        name.to_string()
    };
    [root.join(&executable), root.join("bin").join(&executable)]
        .into_iter()
        .find(|path| path.is_file())
}

fn normalize_windows_path(path: PathBuf) -> PathBuf {
    if cfg!(windows) {
        let value = path.to_string_lossy();
        if let Some(stripped) = value.strip_prefix(r"\\?\") {
            return PathBuf::from(stripped);
        }
        if let Some(stripped) = value.strip_prefix("//?/") {
            return PathBuf::from(stripped);
        }
    }
    path
}

fn free_port() -> Result<u16, String> {
    let listener = TcpListener::bind("127.0.0.1:0")
        .map_err(|err| format!("failed to allocate local port: {err}"))?;
    listener
        .local_addr()
        .map(|address| address.port())
        .map_err(|err| format!("failed to read allocated local port: {err}"))
}

fn wait_for_tcp(port: u16, timeout: Duration) -> Result<(), String> {
    let deadline = std::time::Instant::now() + timeout;
    loop {
        if TcpStream::connect(("127.0.0.1", port)).is_ok() {
            return Ok(());
        }
        if std::time::Instant::now() >= deadline {
            return Err(format!("PostgreSQL readiness timeout on 127.0.0.1:{port}"));
        }
        thread::sleep(Duration::from_millis(100));
    }
}

fn wait_for_postgres_query_ready(
    psql: &Path,
    port: u16,
    username: &str,
    password: &str,
    database: &str,
    timeout: Duration,
) -> Result<(), String> {
    let deadline = std::time::Instant::now() + timeout;
    let mut last_exit_status: Option<i32>;
    loop {
        let status = Command::new(psql)
            .args([
                "-h",
                "127.0.0.1",
                "-p",
                &port.to_string(),
                "-U",
                username,
                "-d",
                database,
                "-v",
                "ON_ERROR_STOP=1",
                "-At",
                "-c",
                "SELECT 1",
            ])
            .env("PGPASSWORD", password)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .map_err(|err| format!("failed to probe PostgreSQL SQL readiness: {err}"))?;
        if status.success() {
            return Ok(());
        }
        last_exit_status = status.code();
        if std::time::Instant::now() >= deadline {
            return Err(format!(
                "PostgreSQL SQL readiness timeout for database {database} (last exit status {})",
                last_exit_status.unwrap_or(-1)
            ));
        }
        thread::sleep(Duration::from_millis(100));
    }
}

fn wait_for_http(endpoint: &str, path: &str, token: &str, timeout: Duration) -> Result<(), String> {
    let port = endpoint
        .rsplit(':')
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| "invalid backend endpoint".to_string())?;
    let deadline = std::time::Instant::now() + timeout;
    loop {
        if request_raw(endpoint, "GET", path, token, &[]).is_ok() {
            return Ok(());
        }
        if std::time::Instant::now() >= deadline {
            return Err(format!(
                "Polis backend readiness timeout on 127.0.0.1:{port}"
            ));
        }
        thread::sleep(Duration::from_millis(100));
    }
}

fn request_raw(
    endpoint: &str,
    method: &str,
    path: &str,
    token: &str,
    body: &[u8],
) -> Result<Vec<u8>, String> {
    let port = endpoint
        .rsplit(':')
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| "invalid backend endpoint".to_string())?;
    let address = format!("127.0.0.1:{port}")
        .parse()
        .map_err(|err| format!("invalid backend address: {err}"))?;
    let mut stream = TcpStream::connect_timeout(&address, Duration::from_millis(500))
        .map_err(|err| format!("backend connection failed: {err}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .map_err(|err| format!("failed to configure backend probe: {err}"))?;
    let request = format!("{method} {path} HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\nX-Polis-Desktop-Token: {token}\r\nContent-Length: {}\r\n\r\n", body.len());
    stream
        .write_all(request.as_bytes())
        .map_err(|err| format!("failed to send backend probe: {err}"))?;
    stream
        .write_all(body)
        .map_err(|err| format!("failed to send backend request body: {err}"))?;
    let mut response = Vec::new();
    stream
        .read_to_end(&mut response)
        .map_err(|err| format!("failed to read backend probe: {err}"))?;
    if !response.starts_with(b"HTTP/1.1 200") && !response.starts_with(b"HTTP/1.1 202") {
        return Err("backend returned a non-ready response".to_string());
    }
    let body_start = response
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .map(|position| position + 4)
        .unwrap_or(response.len());
    Ok(response[body_start..].to_vec())
}

fn require_accepted_receipt(body: &[u8], operation: &str) -> Result<(), String> {
    let receipt: Value = serde_json::from_slice(body)
        .map_err(|_| format!("{operation} returned an invalid receipt"))?;
    if receipt.get("accepted").and_then(Value::as_bool) == Some(true) {
        Ok(())
    } else {
        Err(format!("{operation} was not accepted"))
    }
}

fn refresh_child(child: &mut Option<Child>, snapshot: &mut ServiceSnapshot) {
    let exited = child
        .as_mut()
        .and_then(|process| process.try_wait().ok().flatten());
    if let Some(status) = exited {
        snapshot.state = "failed".to_string();
        snapshot.exit_status = status.code();
        snapshot.last_failure = Some(format!(
            "child exited with status {}",
            status.code().unwrap_or(-1)
        ));
        *child = None;
    }
}

fn stop_child(child: &mut Option<Child>, name: &str, logs: &Path) -> Result<(), String> {
    let Some(process) = child.as_mut() else {
        return Ok(());
    };
    let mut exited = process
        .try_wait()
        .map_err(|err| format!("failed to inspect {name} process: {err}"))?
        .is_some();
    for _ in 0..50 {
        if exited {
            break;
        }
        thread::sleep(Duration::from_millis(100));
        exited = process
            .try_wait()
            .map_err(|err| format!("failed to inspect {name} process: {err}"))?
            .is_some();
    }
    if !exited {
        if let Err(kill_error) = process.kill() {
            if process
                .try_wait()
                .map_err(|err| format!("failed to confirm {name} process exit: {err}"))?
                .is_none()
            {
                return Err(format!("failed to stop {name}: {kill_error}"));
            }
        } else {
            process
                .wait()
                .map_err(|err| format!("failed to confirm {name} process exit: {err}"))?;
        }
        let path = logs.join("desktop-supervisor.log");
        let _ = append_log(
            &path,
            &format!("forced termination fallback used for {name}\n"),
        );
    }
    *child = None;
    Ok(())
}

fn append_log(path: &Path, text: &str) -> Result<(), String> {
    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .map_err(|err| err.to_string())?;
    file.write_all(text.as_bytes())
        .map_err(|err| err.to_string())
}

fn rotate_log(path: &Path) -> Result<(), String> {
    if let Ok(metadata) = fs::metadata(path) {
        if metadata.len() > 5 * 1024 * 1024 {
            let rotated = path.with_extension("log.1");
            let _ = fs::remove_file(&rotated);
            fs::rename(path, rotated)
                .map_err(|err| format!("failed to rotate log {}: {err}", path.display()))?;
        }
    }
    Ok(())
}

fn read_json<T: for<'de> Deserialize<'de>>(path: &Path) -> Option<T> {
    fs::read(path)
        .ok()
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
}

fn write_private_file(path: &Path, bytes: &[u8]) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|err| format!("failed to create private file directory: {err}"))?;
    }
    fs::write(path, bytes).map_err(|err| format!("failed to write private desktop file: {err}"))?;
    // The app-data root is user-scoped. Keep its inherited ACL intact rather
    // than issuing a partial icacls rule that could remove the current user's
    // own read access to the one-shot initdb password handoff.
    Ok(())
}

fn quote_identifier(value: &str) -> String {
    format!("\"{}\"", value.replace('"', "\"\""))
}
fn quote_literal(value: &str) -> String {
    format!("'{}'", value.replace('\'', "''"))
}
fn secret() -> String {
    Uuid::new_v4().simple().to_string()
}
fn now() -> String {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs().to_string())
        .unwrap_or_else(|_| "0".to_string())
}

#[cfg(test)]
mod maintenance_tests {
    use super::{require_accepted_receipt, sidecar_failure_summary};
    use crate::sidecar::{SidecarRef, SidecarState, SidecarTransitionKind};

    #[test]
    fn maintenance_requires_an_explicit_accepted_receipt() {
        assert!(require_accepted_receipt(br#"{"accepted":true}"#, "maintenance").is_ok());
        assert!(require_accepted_receipt(br#"{"accepted":false}"#, "maintenance").is_err());
        assert!(require_accepted_receipt(b"{}", "maintenance").is_err());
        assert!(require_accepted_receipt(b"not json", "maintenance").is_err());
    }

    #[test]
    fn sidecar_recovery_diagnostics_distinguish_update_from_manual_rollback() {
        let active = SidecarRef::installed("C:/Polis/current.exe".to_string(), "a".repeat(64));
        let previous = SidecarRef::installed("C:/Polis/previous.exe".to_string(), "b".repeat(64));
        let update = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: active.clone(),
            previous: Some(previous.clone()),
            staged: None,
            pending: true,
            pending_operation: Some(SidecarTransitionKind::Update),
        };
        let rollback = SidecarState {
            pending_operation: Some(SidecarTransitionKind::ManualRollback),
            ..update.clone()
        };

        assert_eq!(
            sidecar_failure_summary(&update),
            "sidecar update failed to start; previous binary restored"
        );
        assert_eq!(
            sidecar_failure_summary(&rollback),
            "sidecar manual rollback failed to start; current binary restored"
        );
    }
}
