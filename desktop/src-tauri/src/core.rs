// pattern: Functional Core

use serde::Serialize;
use std::path::Path;

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "snake_case")]
pub struct DesktopLayout {
    pub root: String,
    pub config: String,
    pub data: String,
    pub postgres: String,
    pub cas: String,
    pub logs: String,
    pub runtime: String,
    pub temp: String,
}

pub fn layout(root: &Path) -> DesktopLayout {
    DesktopLayout {
        root: root.display().to_string(),
        config: root.join("config").display().to_string(),
        data: root.join("data").display().to_string(),
        postgres: root.join("postgres").display().to_string(),
        cas: root.join("cas").display().to_string(),
        logs: root.join("logs").display().to_string(),
        runtime: root.join("runtime").display().to_string(),
        temp: root.join("temp").display().to_string(),
    }
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "snake_case")]
pub struct ServiceSnapshot {
    pub state: String,
    pub pid: Option<u32>,
    pub started_at: Option<String>,
    pub exit_status: Option<i32>,
    pub last_failure: Option<String>,
}

impl ServiceSnapshot {
    pub fn pending() -> Self {
        Self {
            state: "starting".to_string(),
            pid: None,
            started_at: None,
            exit_status: None,
            last_failure: None,
        }
    }

    pub fn failed(message: impl Into<String>) -> Self {
        Self {
            state: "failed".to_string(),
            pid: None,
            started_at: None,
            exit_status: None,
            last_failure: Some(message.into()),
        }
    }
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "snake_case")]
pub struct ProviderSnapshot {
    pub state: String,
    pub runtime_version: Option<String>,
    pub auth_state: String,
    pub qualification_state: String,
    pub reason: Option<String>,
}

impl ProviderSnapshot {
    pub fn setup_required(reason: impl Into<String>) -> Self {
        Self {
            state: "degraded".to_string(),
            runtime_version: None,
            auth_state: "setup_required".to_string(),
            qualification_state: "unknown".to_string(),
            reason: Some(reason.into()),
        }
    }
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "snake_case")]
pub struct RuntimeSnapshot {
    pub startup_stage: String,
    pub first_run_required: bool,
    pub last_company_id: Option<String>,
    pub data_root: String,
    pub api_base_url: Option<String>,
    pub session_token: Option<String>,
    pub database: ServiceSnapshot,
    pub backend: ServiceSnapshot,
    pub provider: ProviderSnapshot,
    pub event_stream: ServiceSnapshot,
    pub logs_path: String,
    pub diagnostics: Option<String>,
    pub active_generation_id: Option<String>,
    pub previous_generation_id: Option<String>,
    pub generation_pending: bool,
    pub active_sidecar_sha256: Option<String>,
    pub previous_sidecar_sha256: Option<String>,
    pub staged_sidecar_sha256: Option<String>,
    pub sidecar_update_pending: bool,
    pub sidecar_update_blocked: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RecoveryGenerationStageReceipt {
    pub status: String,
    pub generation_id: String,
    pub database_name: String,
    pub manifest_sha256: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SidecarStageReceipt {
    pub status: String,
    pub sha256: String,
    pub size_bytes: u64,
}

pub fn initial_snapshot(root: &Path) -> RuntimeSnapshot {
    RuntimeSnapshot {
        startup_stage: "starting_database".to_string(),
        first_run_required: true,
        last_company_id: None,
        data_root: root.display().to_string(),
        api_base_url: None,
        session_token: None,
        database: ServiceSnapshot::pending(),
        backend: ServiceSnapshot::pending(),
        provider: ProviderSnapshot::setup_required("AI runtime has not been configured"),
        event_stream: ServiceSnapshot::pending(),
        logs_path: root.join("logs").display().to_string(),
        diagnostics: None,
        active_generation_id: None,
        previous_generation_id: None,
        generation_pending: false,
        active_sidecar_sha256: None,
        previous_sidecar_sha256: None,
        staged_sidecar_sha256: None,
        sidecar_update_pending: false,
        sidecar_update_blocked: false,
    }
}

#[cfg(test)]
mod tests {
    use super::{initial_snapshot, layout};
    use std::path::Path;

    #[test]
    fn layout_is_deterministic_and_explicit() {
        let value = layout(Path::new("C:/Users/example/AppData/Local/Polis"));
        assert!(value.config.ends_with("config"));
        assert!(value.postgres.ends_with("postgres"));
        assert!(value.cas.ends_with("cas"));
        assert!(value.temp.ends_with("temp"));
    }

    #[test]
    fn initial_snapshot_does_not_expose_a_token_or_endpoint() {
        let value = initial_snapshot(Path::new("C:/Polis"));
        assert_eq!(value.startup_stage, "starting_database");
        assert!(value.api_base_url.is_none());
        assert!(value.session_token.is_none());
        assert!(value.first_run_required);
        assert!(value.active_generation_id.is_none());
        assert!(value.previous_generation_id.is_none());
        assert!(!value.generation_pending);
        assert!(value.active_sidecar_sha256.is_none());
        assert!(value.previous_sidecar_sha256.is_none());
        assert!(value.staged_sidecar_sha256.is_none());
        assert!(!value.sidecar_update_pending);
        assert!(!value.sidecar_update_blocked);
    }
}
