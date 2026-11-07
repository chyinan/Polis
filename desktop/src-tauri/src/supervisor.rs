// pattern: Imperative Shell

use crate::core::{
    initial_snapshot, layout, DesktopLayout, ProviderSnapshot, RuntimeSnapshot, ServiceSnapshot,
};
use crate::process::ChildOwnership;
use crate::provider_manifest::{parse_provider_manifest, validate_provider_auth};
use fs2::FileExt;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
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
    _lock_file: File,
    ownership: ChildOwnership,
    postgres: Option<Child>,
    backend: Option<Child>,
    postgres_root: Option<PathBuf>,
    postgres_data: Option<PathBuf>,
    pending_database_setup: Option<(PathBuf, String, DatabaseConfig)>,
    token: Option<String>,
    api_base_url: Option<String>,
    dsn: Option<String>,
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
                _lock_file: lock_file,
                ownership,
                postgres: None,
                backend: None,
                postgres_root: None,
                postgres_data: None,
                pending_database_setup: None,
                token: None,
                api_base_url: None,
                dsn: None,
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
        if let Err(err) = self.bootstrap_inner() {
            self.fail_current_stage(err);
        }
    }

    fn bootstrap_inner(&mut self) -> Result<(), String> {
        self.ensure_layout()?;
        self.set_stage("starting_database");
        let (dsn, postgres) = self.start_database()?;
        self.dsn = Some(dsn.clone());
        self.postgres = Some(postgres);
        self.snapshot.database.state = "ready".to_string();
        self.set_stage("migrating_database");
        let backend_path = self.resolve_backend_executable()?;
        self.run_migrations(&backend_path, &dsn)?;
        self.set_stage("starting_polis_runtime");
        let (backend, endpoint, token) = self.start_backend(&backend_path, &dsn)?;
        self.backend = Some(backend);
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

    fn start_database(&mut self) -> Result<(String, Child), String> {
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
            };
            self.write_database_config(&config)?;
            self.pending_database_setup = Some((psql.clone(), admin_password, config.clone()));
            config
        };
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
        self.ownership.assign(&child)?;
        if let Err(error) = wait_for_tcp(port, Duration::from_secs(15)) {
            let mut failed_child = child;
            let _ = failed_child.kill();
            let _ = failed_child.wait();
            return Err(error);
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
            &database_config.database,
            Duration::from_secs(15),
        )?;
        let dsn = format!(
            "postgres://{}:{}@127.0.0.1:{}/{}?sslmode=disable",
            database_config.username, database_config.password, port, database_config.database
        );
        self.snapshot.database.pid = Some(child.id());
        self.snapshot.database.started_at = Some(now());
        Ok((dsn, child))
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

    fn run_migrations(&self, backend: &Path, dsn: &str) -> Result<(), String> {
        let log_path = PathBuf::from(&self.layout.logs).join("backend.log");
        rotate_log(&log_path)?;
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)
            .map_err(|err| format!("failed to open backend log: {err}"))?;
        let status = Command::new(backend)
            .arg("migrate")
            .env("POLIS_DSN", dsn)
            .env("POLIS_BLOB_ROOT", &self.layout.cas)
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
        backend: &Path,
        dsn: &str,
    ) -> Result<(Child, String, String), String> {
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
        let child = Command::new(backend)
            .arg("serve")
            .env("POLIS_DSN", dsn)
            .env("POLIS_BLOB_ROOT", &self.layout.cas)
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
        self.ownership.assign(&child)?;
        if let Err(error) = wait_for_http(&endpoint, "/healthz", &token, Duration::from_secs(20)) {
            let mut failed_child = child;
            let _ = failed_child.kill();
            let _ = failed_child.wait();
            return Err(error);
        }
        self.snapshot.backend.pid = Some(child.id());
        self.snapshot.backend.started_at = Some(now());
        Ok((child, endpoint, token))
    }

    fn resolve_backend_executable(&self) -> Result<PathBuf, String> {
        let mut candidates = Vec::new();
        if let Ok(value) = std::env::var("POLIS_DESKTOP_POLIS_EXE") {
            candidates.push(normalize_windows_path(PathBuf::from(value)));
        }
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
        if self.request_quit()? && !force_active_work {
            return Err(
                "Polis still has active work; confirm a safe stop before quitting".to_string(),
            );
        }
        if let (Some(endpoint), Some(token)) = (self.api_base_url.as_deref(), self.token.as_deref())
        {
            let _ = request_raw(endpoint, "POST", "/api/desktop/shutdown", token, &[]);
        }
        stop_child(&mut self.backend, "backend", Path::new(&self.layout.logs))?;
        self.stop_postgres()?;
        self.snapshot.database.state = "stopped".to_string();
        self.snapshot.backend.state = "stopped".to_string();
        self.snapshot.event_stream.state = "stopped".to_string();
        self.snapshot.startup_stage = "stopped".to_string();
        Ok(())
    }

    fn stop_postgres(&mut self) -> Result<(), String> {
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
                    if let Some(mut postgres) = self.postgres.take() {
                        let _ = postgres.wait();
                    }
                    return Ok(());
                }
            }
        }
        if let Some(mut postgres) = self.postgres.take() {
            if postgres
                .try_wait()
                .map_err(|err| format!("failed to inspect PostgreSQL process: {err}"))?
                .is_none()
            {
                postgres
                    .kill()
                    .map_err(|err| format!("failed to stop PostgreSQL: {err}"))?;
                let _ = postgres.wait();
                let _ = append_log(
                    &PathBuf::from(&self.layout.logs).join("desktop-supervisor.log"),
                    "forced termination fallback used for postgres\n",
                );
            }
        }
        Ok(())
    }

    pub fn restart(&mut self) -> Result<RuntimeSnapshot, String> {
        self.shutdown(false)?;
        self.snapshot = initial_snapshot(&self.root);
        self.snapshot.first_run_required = !self.config.initialized;
        self.snapshot.last_company_id = self.config.last_company_id.clone();
        self.bootstrap_inner()?;
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
    if let Some(mut process) = child.take() {
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
            process
                .kill()
                .map_err(|err| format!("failed to stop {name}: {err}"))?;
            let _ = process.wait();
            let path = logs.join("desktop-supervisor.log");
            let _ = append_log(
                &path,
                &format!("forced termination fallback used for {name}\n"),
            );
        }
    }
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
