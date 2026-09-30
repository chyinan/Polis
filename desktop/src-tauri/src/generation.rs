// pattern: Functional Core

use serde::{Deserialize, Serialize};
use std::path::Path;

const GENERATION_STATE_SCHEMA: &str = "polis-desktop-generation-state@1";
const PREPARED_GENERATION_SCHEMA: &str = "polis-desktop-prepared-generation@1";
const LEGACY_GENERATION_ID: &str = "legacy";

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct RuntimeGenerationRef {
    pub generation_id: String,
    pub database_name: String,
    pub cas_relative_path: String,
    pub recovery_manifest_sha256: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct PreparedGenerationManifest {
    pub schema_version: String,
    pub generation: RuntimeGenerationRef,
    pub recovery_package_path: String,
}

impl PreparedGenerationManifest {
    pub fn validate(&self) -> Result<(), String> {
        if self.schema_version != PREPARED_GENERATION_SCHEMA {
            return Err("prepared generation schema is unsupported".to_string());
        }
        self.generation.validate()?;
        if self.generation.generation_id == LEGACY_GENERATION_ID
            || !Path::new(&self.recovery_package_path).is_absolute()
        {
            return Err("prepared recovery generation identity is invalid".to_string());
        }
        Ok(())
    }
}

impl RuntimeGenerationRef {
    pub fn validate(&self) -> Result<(), String> {
        if !valid_database_name(&self.database_name) {
            return Err("generation database name is invalid".to_string());
        }
        if self.generation_id == LEGACY_GENERATION_ID {
            if self.cas_relative_path != "cas" || self.recovery_manifest_sha256.is_some() {
                return Err("legacy generation reference is invalid".to_string());
            }
            return Ok(());
        }
        if !valid_generation_id(&self.generation_id)
            || self.cas_relative_path != format!("generations/{}/cas", self.generation_id)
            || !self
                .recovery_manifest_sha256
                .as_deref()
                .map(valid_sha256)
                .unwrap_or(false)
        {
            return Err("recovery generation reference is invalid".to_string());
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct GenerationState {
    pub schema_version: String,
    pub active: RuntimeGenerationRef,
    pub previous: Option<RuntimeGenerationRef>,
    pub pending: bool,
    #[serde(default)]
    pub pending_package_path: Option<String>,
}

impl GenerationState {
    pub fn legacy(database_name: impl Into<String>) -> Self {
        Self {
            schema_version: GENERATION_STATE_SCHEMA.to_string(),
            active: RuntimeGenerationRef {
                generation_id: LEGACY_GENERATION_ID.to_string(),
                database_name: database_name.into(),
                cas_relative_path: "cas".to_string(),
                recovery_manifest_sha256: None,
            },
            previous: None,
            pending: false,
            pending_package_path: None,
        }
    }

    pub fn validate(&self) -> Result<(), String> {
        if self.schema_version != GENERATION_STATE_SCHEMA {
            return Err("generation state schema is unsupported".to_string());
        }
        self.active.validate()?;
        if let Some(previous) = &self.previous {
            previous.validate()?;
            if previous.generation_id == self.active.generation_id {
                return Err("active and previous generation IDs must differ".to_string());
            }
        }
        if self.pending && self.previous.is_none() {
            return Err("pending generation switch requires a rollback generation".to_string());
        }
        if self
            .pending_package_path
            .as_deref()
            .map(|path| !Path::new(path).is_absolute())
            .unwrap_or(false)
            || (!self.pending && self.pending_package_path.is_some())
        {
            return Err("pending generation package path is invalid".to_string());
        }
        Ok(())
    }

    pub fn begin_switch(&self, candidate: RuntimeGenerationRef) -> Result<Self, String> {
        self.validate()?;
        candidate.validate()?;
        if self.pending {
            return Err("another generation switch is already pending".to_string());
        }
        if candidate.generation_id == self.active.generation_id {
            return Err("candidate generation is already active".to_string());
        }
        Ok(Self {
            schema_version: GENERATION_STATE_SCHEMA.to_string(),
            active: candidate,
            previous: Some(self.active.clone()),
            pending: true,
            pending_package_path: None,
        })
    }

    pub fn begin_previous_rollback(&self) -> Result<Self, String> {
        self.validate()?;
        if self.pending {
            return Err("another generation switch is already pending".to_string());
        }
        let previous = self
            .previous
            .as_ref()
            .ok_or_else(|| "no previous generation is available".to_string())?;
        Ok(Self {
            schema_version: GENERATION_STATE_SCHEMA.to_string(),
            active: previous.clone(),
            previous: Some(self.active.clone()),
            pending: true,
            pending_package_path: None,
        })
    }

    pub fn commit(&self) -> Result<Self, String> {
        self.validate()?;
        if !self.pending {
            return Err("no pending generation switch can be committed".to_string());
        }
        let mut next = self.clone();
        next.pending = false;
        next.pending_package_path = None;
        Ok(next)
    }

    pub fn rollback_pending(&self) -> Result<Self, String> {
        self.validate()?;
        if !self.pending {
            return Err("no pending generation switch can be rolled back".to_string());
        }
        let previous = self
            .previous
            .as_ref()
            .ok_or_else(|| "pending generation switch has no rollback generation".to_string())?;
        Ok(Self {
            schema_version: GENERATION_STATE_SCHEMA.to_string(),
            active: previous.clone(),
            previous: None,
            pending: false,
            pending_package_path: None,
        })
    }
}

fn valid_database_name(value: &str) -> bool {
    let mut characters = value.chars();
    matches!(characters.next(), Some('a'..='z'))
        && value.len() <= 63
        && characters.all(|character| matches!(character, 'a'..='z' | '0'..='9' | '_'))
}

pub(crate) fn valid_generation_id(value: &str) -> bool {
    value.len() == 32
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

fn valid_sha256(value: &str) -> bool {
    value.len() == 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

#[cfg(test)]
mod tests {
    use super::{
        GenerationState, PreparedGenerationManifest, RuntimeGenerationRef, LEGACY_GENERATION_ID,
        PREPARED_GENERATION_SCHEMA,
    };

    fn legacy() -> GenerationState {
        GenerationState::legacy("polis_r0_desktop")
    }

    fn restored(id: &str, database: &str) -> RuntimeGenerationRef {
        RuntimeGenerationRef {
            generation_id: id.to_string(),
            database_name: database.to_string(),
            cas_relative_path: format!("generations/{id}/cas"),
            recovery_manifest_sha256: Some("a".repeat(64)),
        }
    }

    #[test]
    fn legacy_generation_has_no_recovery_package_claim() {
        let state = legacy();
        assert!(state.validate().is_ok());
        assert_eq!(state.active.generation_id, "legacy");
        assert!(state.previous.is_none());
        assert!(!state.pending);
    }

    #[test]
    fn successful_cutover_keeps_exactly_one_previous_generation() {
        let state = legacy();
        let pending = state
            .begin_switch(restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ))
            .expect("valid generation should stage");
        assert!(pending.pending);
        assert_eq!(
            pending.active.generation_id,
            "0123456789abcdef0123456789abcdef"
        );
        assert_eq!(pending.previous.as_ref().unwrap().generation_id, "legacy");

        let committed = pending.commit().expect("successful startup should commit");
        assert!(!committed.pending);
        assert_eq!(
            committed.active.generation_id,
            "0123456789abcdef0123456789abcdef"
        );
        assert_eq!(committed.previous.as_ref().unwrap().generation_id, "legacy");
    }

    #[test]
    fn failed_candidate_start_restores_previous_generation() {
        let pending = legacy()
            .begin_switch(restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ))
            .expect("valid generation should stage");

        let rolled_back = pending
            .rollback_pending()
            .expect("failed startup should roll back");
        assert!(!rolled_back.pending);
        assert_eq!(rolled_back.active.generation_id, "legacy");
        assert!(rolled_back.previous.is_none());
    }

    #[test]
    fn manual_rollback_is_pending_until_previous_services_start() {
        let active = legacy()
            .begin_switch(restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ))
            .unwrap()
            .commit()
            .unwrap();

        let rollback = active
            .begin_previous_rollback()
            .expect("previous generation should exist");
        assert!(rollback.pending);
        assert_eq!(rollback.active.generation_id, "legacy");
        assert_eq!(
            rollback.previous.as_ref().unwrap().generation_id,
            "0123456789abcdef0123456789abcdef"
        );

        let committed = rollback.commit().unwrap();
        assert!(!committed.pending);
        assert_eq!(committed.active.generation_id, "legacy");
        assert_eq!(
            committed.previous.as_ref().unwrap().generation_id,
            "0123456789abcdef0123456789abcdef"
        );
    }

    #[test]
    fn generation_rejects_path_escape_and_mismatched_identity() {
        let mut escaped = restored(
            "0123456789abcdef0123456789abcdef",
            "polis_recovery_0123456789abcdef0123456789abcdef",
        );
        escaped.cas_relative_path = "../../outside".to_string();
        assert!(GenerationState::legacy("polis_r0_desktop")
            .begin_switch(escaped)
            .is_err());

        let mut mismatched = restored(
            "0123456789abcdef0123456789abcdef",
            "polis_recovery_0123456789abcdef0123456789abcdef",
        );
        mismatched.cas_relative_path =
            "generations/fedcba9876543210fedcba9876543210/cas".to_string();
        assert!(GenerationState::legacy("polis_r0_desktop")
            .begin_switch(mismatched)
            .is_err());
    }

    #[test]
    fn generation_state_rejects_a_second_transition_while_pending() {
        let pending = legacy()
            .begin_switch(restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ))
            .unwrap();
        assert!(pending.begin_previous_rollback().is_err());
        assert!(pending
            .begin_switch(restored(
                "fedcba9876543210fedcba9876543210",
                "polis_recovery_fedcba9876543210fedcba9876543210"
            ))
            .is_err());
    }

    #[test]
    fn pending_restore_package_path_must_be_absolute_and_is_cleared_on_commit() {
        let mut pending = legacy()
            .begin_switch(restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ))
            .unwrap();
        pending.pending_package_path = Some("relative/package".to_string());
        assert!(pending.validate().is_err());

        pending.pending_package_path = Some(
            std::env::temp_dir()
                .join("polis-recovery-package")
                .display()
                .to_string(),
        );
        assert!(pending.validate().is_ok());
        assert!(pending.commit().unwrap().pending_package_path.is_none());
    }

    #[test]
    fn prepared_generation_manifest_requires_a_nonlegacy_generation_and_absolute_package() {
        let valid = PreparedGenerationManifest {
            schema_version: PREPARED_GENERATION_SCHEMA.to_string(),
            generation: restored(
                "0123456789abcdef0123456789abcdef",
                "polis_recovery_0123456789abcdef0123456789abcdef",
            ),
            recovery_package_path: std::env::temp_dir()
                .join("polis-recovery-package")
                .display()
                .to_string(),
        };
        assert!(valid.validate().is_ok());

        let mut invalid = valid.clone();
        invalid.recovery_package_path = "relative/package".to_string();
        assert!(invalid.validate().is_err());
        let mut invalid = valid;
        invalid.generation = RuntimeGenerationRef {
            generation_id: LEGACY_GENERATION_ID.to_string(),
            database_name: "polis_r0_desktop".to_string(),
            cas_relative_path: "cas".to_string(),
            recovery_manifest_sha256: None,
        };
        assert!(invalid.validate().is_err());
    }
}
