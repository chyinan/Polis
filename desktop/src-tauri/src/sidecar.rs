// pattern: Functional Core

use serde::{Deserialize, Serialize};

const SIDECAR_STATE_SCHEMA: &str = "polis-desktop-sidecar-state@2";

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(
    deny_unknown_fields,
    rename_all = "camelCase",
    tag = "kind",
    content = "value"
)]
pub enum SidecarLocation {
    Installed(String),
    Managed(String),
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct SidecarRef {
    pub sha256: String,
    #[serde(default)]
    pub migration_manifest_sha256: Option<String>,
    pub location: SidecarLocation,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SidecarTransitionKind {
    Update,
    ManualRollback,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct SidecarState {
    pub schema_version: String,
    pub active: SidecarRef,
    pub previous: Option<SidecarRef>,
    pub staged: Option<SidecarRef>,
    pub pending: bool,
    #[serde(default)]
    pub pending_operation: Option<SidecarTransitionKind>,
}

impl SidecarRef {
    pub fn installed(path: String, sha256: String) -> Self {
        Self {
            sha256,
            migration_manifest_sha256: None,
            location: SidecarLocation::Installed(path),
        }
    }

    pub fn managed(path: String, sha256: String) -> Self {
        Self {
            sha256,
            migration_manifest_sha256: None,
            location: SidecarLocation::Managed(path),
        }
    }

    pub fn validate(&self) -> Result<(), String> {
        if !valid_sha256(&self.sha256) {
            return Err("sidecar SHA-256 identity is invalid".to_string());
        }
        if self
            .migration_manifest_sha256
            .as_deref()
            .map(|digest| !valid_sha256(digest))
            .unwrap_or(false)
        {
            return Err("sidecar migration manifest SHA-256 is invalid".to_string());
        }
        match &self.location {
            SidecarLocation::Installed(path) if std::path::Path::new(path).is_absolute() => Ok(()),
            SidecarLocation::Installed(_) => {
                Err("installed sidecar path must be absolute".to_string())
            }
            SidecarLocation::Managed(path) if valid_managed_path(path, &self.sha256) => Ok(()),
            SidecarLocation::Managed(_) => {
                Err("managed sidecar path escapes the private sidecar directories".to_string())
            }
        }
    }
}

impl SidecarState {
    pub fn initial(active: SidecarRef) -> Self {
        Self {
            schema_version: SIDECAR_STATE_SCHEMA.to_string(),
            active,
            previous: None,
            staged: None,
            pending: false,
            pending_operation: None,
        }
    }

    pub fn validate(&self) -> Result<(), String> {
        if self.schema_version != SIDECAR_STATE_SCHEMA {
            return Err("sidecar state schema is unsupported".to_string());
        }
        self.active.validate()?;
        if let Some(previous) = &self.previous {
            previous.validate()?;
            if previous.sha256 == self.active.sha256 {
                return Err("active and previous sidecars must differ".to_string());
            }
        }
        if let Some(staged) = &self.staged {
            staged.validate()?;
            if staged.sha256 == self.active.sha256
                || !matches!(
                    &staged.location,
                    SidecarLocation::Managed(path)
                        if path.starts_with("config/polis-sidecar-candidate-")
                )
            {
                return Err("staged sidecar candidate is invalid".to_string());
            }
        }
        if self.pending {
            if self.previous.is_none() || self.staged.is_some() || self.pending_operation.is_none()
            {
                return Err(
                    "pending sidecar transition requires one rollback binary and operation"
                        .to_string(),
                );
            }
            let previous = self
                .previous
                .as_ref()
                .ok_or_else(|| "pending sidecar transition has no previous binary".to_string())?;
            let previous_manifest = previous.migration_manifest_sha256.as_deref();
            if previous_manifest.is_none()
                || self.active.migration_manifest_sha256.as_deref() != previous_manifest
            {
                return Err(
                    "pending sidecar transition must preserve the embedded migration manifest"
                        .to_string(),
                );
            }
            let roles_match = match self.pending_operation.as_ref() {
                Some(SidecarTransitionKind::Update) => {
                    is_candidate_sidecar(&self.active) && is_retained_sidecar(previous)
                }
                Some(SidecarTransitionKind::ManualRollback) => {
                    (is_retained_sidecar(&self.active)
                        && (is_candidate_sidecar(previous)
                            || matches!(previous.location, SidecarLocation::Installed(_))))
                        || (is_candidate_sidecar(&self.active) && is_retained_sidecar(previous))
                }
                None => false,
            };
            if !roles_match {
                return Err(
                    "pending sidecar operation does not match its active and previous binary roles"
                        .to_string(),
                );
            }
        } else if self.pending_operation.is_some() {
            return Err("stable sidecar state cannot retain a pending operation".to_string());
        }
        Ok(())
    }

    pub fn select_candidate(&self, candidate: SidecarRef) -> Result<Self, String> {
        self.validate()?;
        candidate.validate()?;
        if self.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        if candidate.sha256 == self.active.sha256 {
            return Err("candidate sidecar is already active".to_string());
        }
        if !matches!(
            &candidate.location,
            SidecarLocation::Managed(path)
                if path.starts_with("config/polis-sidecar-candidate-")
        ) {
            return Err("candidate sidecar must be staged in the private data root".to_string());
        }
        let mut next = self.clone();
        next.staged = Some(candidate);
        Ok(next)
    }

    pub fn begin_update(
        &self,
        candidate: SidecarRef,
        retained_active: SidecarRef,
    ) -> Result<Self, String> {
        self.validate()?;
        candidate.validate()?;
        retained_active.validate()?;
        if self.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        if self.staged.as_ref() != Some(&candidate) {
            return Err(
                "requested sidecar candidate is not the selected staged binary".to_string(),
            );
        }
        if retained_active.sha256 != self.active.sha256
            || !matches!(
                &retained_active.location,
                SidecarLocation::Managed(path)
                    if path.starts_with("config/polis-sidecar-retained-")
            )
        {
            return Err("previous sidecar is not a retained copy of the active binary".to_string());
        }
        let active_manifest = self.active.migration_manifest_sha256.as_deref();
        if active_manifest.is_none()
            || retained_active.migration_manifest_sha256.as_deref() != active_manifest
            || candidate.migration_manifest_sha256.as_deref() != active_manifest
        {
            return Err(
                "sidecar update must keep the active migration manifest unchanged".to_string(),
            );
        }
        Ok(Self {
            schema_version: SIDECAR_STATE_SCHEMA.to_string(),
            active: candidate,
            previous: Some(retained_active),
            staged: None,
            pending: true,
            pending_operation: Some(SidecarTransitionKind::Update),
        })
    }

    pub fn begin_previous_rollback(&self) -> Result<Self, String> {
        self.validate()?;
        if self.pending {
            return Err("another sidecar transition is already pending".to_string());
        }
        let previous = self
            .previous
            .as_ref()
            .ok_or_else(|| "no previous sidecar is available".to_string())?;
        let active_manifest = self.active.migration_manifest_sha256.as_deref();
        if active_manifest.is_none()
            || previous.migration_manifest_sha256.as_deref() != active_manifest
        {
            return Err("sidecar rollback requires matching migration manifests".to_string());
        }
        Ok(Self {
            schema_version: SIDECAR_STATE_SCHEMA.to_string(),
            active: previous.clone(),
            previous: Some(self.active.clone()),
            staged: None,
            pending: true,
            pending_operation: Some(SidecarTransitionKind::ManualRollback),
        })
    }

    pub fn commit_pending(&self) -> Result<Self, String> {
        self.validate()?;
        if !self.pending {
            return Err("no pending sidecar transition can be committed".to_string());
        }
        let mut next = self.clone();
        next.pending = false;
        next.pending_operation = None;
        next.validate()?;
        Ok(next)
    }

    pub fn rollback_pending(&self) -> Result<Self, String> {
        self.validate()?;
        if !self.pending {
            return Err("no pending sidecar transition can be rolled back".to_string());
        }
        let failed = self.active.clone();
        let rollback_target = self
            .previous
            .clone()
            .ok_or_else(|| "pending sidecar transition has no rollback binary".to_string())?;
        let (previous, staged) = match self.pending_operation {
            Some(SidecarTransitionKind::Update) => (None, Some(failed)),
            Some(SidecarTransitionKind::ManualRollback) => (Some(failed), None),
            None => return Err("pending sidecar transition has no operation".to_string()),
        };
        let next = Self {
            schema_version: SIDECAR_STATE_SCHEMA.to_string(),
            active: rollback_target,
            previous,
            staged,
            pending: false,
            pending_operation: None,
        };
        next.validate()?;
        Ok(next)
    }
}

pub fn validate_migration_manifest_compatibility(
    current: &str,
    candidate: &str,
) -> Result<(), String> {
    if !valid_sha256(current) || !valid_sha256(candidate) {
        return Err("embedded migration manifest identity is invalid".to_string());
    }
    if current != candidate {
        return Err("candidate has a different embedded migration manifest".to_string());
    }
    Ok(())
}

pub fn validate_launch_identity(
    selected: &SidecarRef,
    reported_binary_sha256: &str,
    reported_migration_manifest_sha256: &str,
) -> Result<(), String> {
    selected.validate()?;
    if selected.sha256 != reported_binary_sha256 {
        return Err("running Polis binary SHA-256 does not match the selected sidecar".to_string());
    }
    let manifest = selected
        .migration_manifest_sha256
        .as_deref()
        .ok_or_else(|| "selected sidecar has no pinned migration manifest".to_string())?;
    validate_migration_manifest_compatibility(manifest, reported_migration_manifest_sha256)
}

fn valid_managed_path(path: &str, digest: &str) -> bool {
    path == format!("config/polis-sidecar-candidate-{digest}.exe")
        || path == format!("config/polis-sidecar-retained-{digest}.exe")
}

fn is_candidate_sidecar(sidecar: &SidecarRef) -> bool {
    matches!(
        &sidecar.location,
        SidecarLocation::Managed(path)
            if path == &format!("config/polis-sidecar-candidate-{}.exe", sidecar.sha256)
    )
}

fn is_retained_sidecar(sidecar: &SidecarRef) -> bool {
    matches!(
        &sidecar.location,
        SidecarLocation::Managed(path)
            if path == &format!("config/polis-sidecar-retained-{}.exe", sidecar.sha256)
    )
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
        validate_launch_identity, validate_migration_manifest_compatibility, SidecarLocation,
        SidecarRef, SidecarState, SidecarTransitionKind,
    };

    fn sidecar(hash_digit: char, directory: &str) -> SidecarRef {
        let hash = hash_digit.to_string().repeat(64);
        let path = match directory {
            "candidates" => format!("config/polis-sidecar-candidate-{hash}.exe"),
            "retained" => format!("config/polis-sidecar-retained-{hash}.exe"),
            _ => panic!("unsupported sidecar test directory"),
        };
        let mut reference = SidecarRef::managed(path, hash);
        reference.migration_manifest_sha256 = Some("e".repeat(64));
        reference
    }

    fn installed(hash_digit: char) -> SidecarRef {
        let mut reference = SidecarRef::installed(
            format!("C:/Program Files/Polis/{hash_digit}.exe"),
            hash_digit.to_string().repeat(64),
        );
        reference.migration_manifest_sha256 = Some("e".repeat(64));
        reference
    }

    #[test]
    fn successful_update_keeps_one_previous_binary_and_commits_pending_recovery() {
        let current = installed('a');
        let candidate = sidecar('b', "candidates");
        let retained = sidecar('a', "retained");
        let state = SidecarState::initial(current.clone())
            .select_candidate(candidate.clone())
            .unwrap()
            .begin_update(candidate.clone(), retained.clone())
            .unwrap();

        let recovered = state.commit_pending().unwrap();

        assert_eq!(recovered.active, candidate);
        assert_eq!(recovered.previous, Some(retained));
        assert!(recovered.staged.is_none());
        assert!(!recovered.pending);
        recovered.validate().unwrap();
    }

    #[test]
    fn operator_rollback_selects_the_retained_binary_and_keeps_one_previous() {
        let current = sidecar('b', "candidates");
        let previous = sidecar('a', "retained");
        let state = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: current.clone(),
            previous: Some(previous.clone()),
            staged: None,
            pending: false,
            pending_operation: None,
        };

        let pending = state.begin_previous_rollback().unwrap();
        let recovered = pending.commit_pending().unwrap();

        assert_eq!(recovered.active, previous);
        assert_eq!(recovered.previous, Some(current));
        assert!(!recovered.pending);
    }

    #[test]
    fn successful_manual_rollback_can_be_rolled_forward_again() {
        let installed = installed('a');
        let candidate = sidecar('b', "candidates");
        let retained = sidecar('a', "retained");
        let updated = SidecarState::initial(installed)
            .select_candidate(candidate.clone())
            .unwrap()
            .begin_update(candidate.clone(), retained.clone())
            .unwrap()
            .commit_pending()
            .unwrap();

        let first_rollback_pending = updated.begin_previous_rollback().unwrap();
        assert_eq!(first_rollback_pending.active, retained);
        assert_eq!(first_rollback_pending.previous, Some(candidate.clone()));
        let rolled_back = first_rollback_pending.commit_pending().unwrap();

        let second_rollback_pending = rolled_back.begin_previous_rollback().unwrap();
        assert_eq!(second_rollback_pending.active, candidate);
        assert_eq!(second_rollback_pending.previous, Some(retained));
        second_rollback_pending.validate().unwrap();
        let rolled_forward = second_rollback_pending.commit_pending().unwrap();
        assert_eq!(rolled_forward.active.sha256, "b".repeat(64));
        assert_eq!(
            rolled_forward.previous.as_ref().unwrap().sha256,
            "a".repeat(64)
        );
    }

    #[test]
    fn failed_candidate_start_rolls_pointer_back_and_preserves_failed_binary() {
        let candidate = sidecar('b', "candidates");
        let previous = sidecar('a', "retained");
        let pending = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: candidate.clone(),
            previous: Some(previous.clone()),
            staged: None,
            pending: true,
            pending_operation: Some(SidecarTransitionKind::Update),
        };

        let recovered = pending.rollback_pending().unwrap();

        assert_eq!(recovered.active, previous);
        assert_eq!(recovered.staged, Some(candidate));
        assert!(recovered.previous.is_none());
        assert!(!recovered.pending);
    }

    #[test]
    fn failed_operator_rollback_restores_the_active_binary_and_keeps_previous_pointer() {
        let current = sidecar('b', "candidates");
        let previous = sidecar('a', "retained");
        let state = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: current.clone(),
            previous: Some(previous.clone()),
            staged: None,
            pending: false,
            pending_operation: None,
        };

        let recovered = state
            .begin_previous_rollback()
            .unwrap()
            .rollback_pending()
            .unwrap();

        assert_eq!(recovered.active, current);
        assert_eq!(recovered.previous, Some(previous));
        assert!(recovered.staged.is_none());
        recovered.validate().unwrap();
    }

    #[test]
    fn pending_update_is_an_explicit_startup_recovery_state() {
        let mut state = SidecarState::initial(installed('a'));
        let candidate = sidecar('b', "candidates");
        state = state.select_candidate(candidate.clone()).unwrap();
        state = state
            .begin_update(candidate, sidecar('a', "retained"))
            .unwrap();

        assert!(state.pending);
        assert_eq!(
            state.active.location,
            SidecarLocation::Managed(format!(
                "config/polis-sidecar-candidate-{}.exe",
                "b".repeat(64)
            ))
        );
        state.validate().unwrap();
    }

    #[test]
    fn update_requires_an_exact_embedded_migration_manifest_match() {
        let manifest = "a".repeat(64);
        assert!(validate_migration_manifest_compatibility(&manifest, &manifest).is_ok());
        assert!(validate_migration_manifest_compatibility(&manifest, &"b".repeat(64)).is_err());
    }

    #[test]
    fn launch_identity_must_match_the_persisted_sidecar_hash_and_manifest() {
        let mut selected = sidecar('b', "candidates");
        selected.migration_manifest_sha256 = Some("a".repeat(64));
        assert!(validate_launch_identity(&selected, &selected.sha256, &"a".repeat(64)).is_ok());
        assert!(validate_launch_identity(&selected, &"c".repeat(64), &"a".repeat(64)).is_err());
        assert!(validate_launch_identity(&selected, &selected.sha256, &"d".repeat(64)).is_err());
    }

    #[test]
    fn pending_state_preserves_update_or_manual_rollback_operation() {
        let current = installed('a');
        let candidate = sidecar('b', "candidates");
        let mut retained = sidecar('a', "retained");
        retained.migration_manifest_sha256 = Some("e".repeat(64));
        let mut candidate = candidate;
        candidate.migration_manifest_sha256 = Some("e".repeat(64));
        let state = SidecarState::initial(current.clone())
            .select_candidate(candidate.clone())
            .unwrap()
            .begin_update(candidate, retained)
            .unwrap();
        assert_eq!(state.pending_operation, Some(SidecarTransitionKind::Update));

        let current = sidecar('b', "candidates");
        let mut previous = sidecar('a', "retained");
        previous.migration_manifest_sha256 = Some("e".repeat(64));
        let stable = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: current,
            previous: Some(previous),
            staged: None,
            pending: false,
            pending_operation: None,
        };
        assert_eq!(
            stable.begin_previous_rollback().unwrap().pending_operation,
            Some(SidecarTransitionKind::ManualRollback)
        );
    }

    #[test]
    fn pending_update_rejects_wrong_active_or_previous_location_roles() {
        let current = installed('a');
        let candidate = sidecar('b', "candidates");
        let retained = sidecar('a', "retained");
        let pending = SidecarState::initial(current)
            .select_candidate(candidate.clone())
            .unwrap()
            .begin_update(candidate, retained.clone())
            .unwrap();

        let mut wrong_active = pending.clone();
        wrong_active.active = installed('b');
        assert!(wrong_active.validate().is_err());

        let mut wrong_previous = pending;
        wrong_previous.previous = Some(sidecar('a', "candidates"));
        assert_ne!(wrong_previous.previous, Some(retained));
        assert!(wrong_previous.validate().is_err());
    }

    #[test]
    fn pending_manual_rollback_rejects_wrong_active_or_previous_location_roles() {
        let current = sidecar('b', "candidates");
        let previous = sidecar('a', "retained");
        let stable = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: current.clone(),
            previous: Some(previous.clone()),
            staged: None,
            pending: false,
            pending_operation: None,
        };
        let pending = stable.begin_previous_rollback().unwrap();
        assert!(pending.validate().is_ok());

        let mut wrong_active = pending.clone();
        wrong_active.active = sidecar('a', "candidates");
        assert!(wrong_active.validate().is_err());

        let mut wrong_previous = pending;
        wrong_previous.previous = Some(sidecar('b', "retained"));
        assert_ne!(wrong_previous.previous, Some(current));
        assert!(wrong_previous.validate().is_err());

        let installed_current = installed('c');
        let previous_retained = sidecar('a', "retained");
        let stable = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: installed_current,
            previous: Some(previous_retained),
            staged: None,
            pending: false,
            pending_operation: None,
        };
        assert!(stable.begin_previous_rollback().unwrap().validate().is_ok());
    }
}
