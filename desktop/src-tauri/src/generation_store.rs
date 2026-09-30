// pattern: Imperative Shell

use crate::generation::{
    valid_generation_id, GenerationState, PreparedGenerationManifest, RuntimeGenerationRef,
};
use serde::{de::DeserializeOwned, Serialize};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::Path;
use uuid::Uuid;

#[cfg(not(windows))]
use std::fs::File;

const MAX_GENERATION_STATE_BYTES: u64 = 64 * 1024;

pub fn read_generation_state(
    path: &Path,
    legacy_database_name: &str,
) -> Result<GenerationState, String> {
    match fs::symlink_metadata(path) {
        Ok(_) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            let legacy = GenerationState::legacy(legacy_database_name);
            legacy.validate()?;
            return Ok(legacy);
        }
        Err(_) => return Err("failed to inspect desktop generation state".to_string()),
    }
    let state: GenerationState = read_bounded_json(path, "desktop generation state")?;
    state.validate()?;
    Ok(state)
}

pub fn write_generation_state(path: &Path, state: &GenerationState) -> Result<(), String> {
    state.validate()?;
    write_json_atomically(path, state, "desktop generation state")
}

pub fn read_prepared_generation(path: &Path) -> Result<PreparedGenerationManifest, String> {
    let manifest: PreparedGenerationManifest =
        read_bounded_json(path, "prepared generation manifest")?;
    manifest.validate()?;
    Ok(manifest)
}

pub fn write_prepared_generation(
    path: &Path,
    manifest: &PreparedGenerationManifest,
) -> Result<(), String> {
    manifest.validate()?;
    write_json_atomically(path, manifest, "prepared generation manifest")
}

fn read_bounded_json<T: DeserializeOwned>(path: &Path, description: &str) -> Result<T, String> {
    let metadata = fs::symlink_metadata(path)
        .map_err(|_| format!("{description} is missing or unavailable"))?;
    if metadata.file_type().is_symlink()
        || !metadata.is_file()
        || metadata.len() > MAX_GENERATION_STATE_BYTES
    {
        return Err(format!("{description} is unsafe or too large"));
    }
    let bytes = fs::read(path).map_err(|_| format!("failed to read {description}"))?;
    serde_json::from_slice(&bytes).map_err(|_| format!("{description} is malformed"))
}

fn write_json_atomically<T: Serialize>(
    path: &Path,
    value: &T,
    description: &str,
) -> Result<(), String> {
    let parent = path
        .parent()
        .ok_or_else(|| format!("{description} has no parent directory"))?;
    fs::create_dir_all(parent).map_err(|_| format!("failed to create {description} directory"))?;
    if let Ok(metadata) = fs::symlink_metadata(path) {
        if metadata.file_type().is_symlink() || !metadata.is_file() {
            return Err(format!("{description} target is unsafe"));
        }
    }
    let temporary_path = parent.join(format!(".generation-state-{}.tmp", Uuid::new_v4().simple()));
    let bytes = serde_json::to_vec(value).map_err(|_| format!("failed to encode {description}"))?;
    if bytes.len() as u64 > MAX_GENERATION_STATE_BYTES {
        return Err(format!("{description} exceeds its size limit"));
    }
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&temporary_path)
        .map_err(|_| format!("failed to create temporary {description}"))?;
    if file.write_all(&bytes).is_err() || file.sync_all().is_err() {
        drop(file);
        let _ = fs::remove_file(&temporary_path);
        return Err(format!("failed to persist temporary {description}"));
    }
    drop(file);
    if let Err(error) = replace_generation_file(&temporary_path, path) {
        let _ = fs::remove_file(&temporary_path);
        return Err(error);
    }
    Ok(())
}

pub fn resolve_generation_cas_root(
    app_root: &Path,
    generation: &RuntimeGenerationRef,
) -> Result<std::path::PathBuf, String> {
    generation.validate()?;
    let canonical_app_root = fs::canonicalize(app_root)
        .map_err(|_| "desktop data root cannot be resolved".to_string())?;
    let candidate = if generation.generation_id == "legacy" {
        canonical_app_root.join("cas")
    } else {
        resolve_generation_directory(&canonical_app_root, &generation.generation_id)?.join("cas")
    };
    let metadata = fs::symlink_metadata(&candidate)
        .map_err(|_| "generation CAS directory is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("generation CAS directory is unsafe".to_string());
    }
    let canonical_candidate = fs::canonicalize(&candidate)
        .map_err(|_| "generation CAS directory cannot be resolved".to_string())?;
    if !canonical_candidate.starts_with(&canonical_app_root) {
        return Err("generation CAS directory escapes the desktop data root".to_string());
    }
    Ok(canonical_candidate)
}

pub fn resolve_generation_directory(
    app_root: &Path,
    generation_id: &str,
) -> Result<std::path::PathBuf, String> {
    if !valid_generation_id(generation_id) {
        return Err("recovery generation ID is invalid".to_string());
    }
    let canonical_app_root = fs::canonicalize(app_root)
        .map_err(|_| "desktop data root cannot be resolved".to_string())?;
    let generations_root = canonical_app_root.join("generations");
    let parent_metadata = fs::symlink_metadata(&generations_root)
        .map_err(|_| "desktop generations directory is unavailable".to_string())?;
    if parent_metadata.file_type().is_symlink() || !parent_metadata.is_dir() {
        return Err("desktop generations directory is unsafe".to_string());
    }
    let canonical_generations_root = fs::canonicalize(&generations_root)
        .map_err(|_| "desktop generations directory cannot be resolved".to_string())?;
    if !canonical_generations_root.starts_with(&canonical_app_root) {
        return Err("desktop generations directory escapes the data root".to_string());
    }
    let generation_directory = canonical_generations_root.join(generation_id);
    let metadata = fs::symlink_metadata(&generation_directory)
        .map_err(|_| "recovery generation directory is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("recovery generation directory is unsafe".to_string());
    }
    let canonical_generation_directory = fs::canonicalize(&generation_directory)
        .map_err(|_| "recovery generation directory cannot be resolved".to_string())?;
    if !canonical_generation_directory.starts_with(&canonical_generations_root) {
        return Err("recovery generation directory escapes its parent".to_string());
    }
    Ok(canonical_generation_directory)
}

pub fn create_generation_directory(
    app_root: &Path,
    generation: &RuntimeGenerationRef,
) -> Result<std::path::PathBuf, String> {
    generation.validate()?;
    if generation.generation_id == "legacy" {
        return Err("legacy generation cannot be created as a recovery generation".to_string());
    }
    let canonical_app_root = fs::canonicalize(app_root)
        .map_err(|_| "desktop data root cannot be resolved".to_string())?;
    let generations_root = canonical_app_root.join("generations");
    match fs::symlink_metadata(&generations_root) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_dir() => {
            return Err("desktop generations directory is unsafe".to_string());
        }
        Ok(_) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            fs::create_dir(&generations_root)
                .map_err(|_| "failed to create desktop generations directory".to_string())?;
        }
        Err(_) => return Err("failed to inspect desktop generations directory".to_string()),
    }
    let canonical_generations_root = fs::canonicalize(&generations_root)
        .map_err(|_| "desktop generations directory cannot be resolved".to_string())?;
    if !canonical_generations_root.starts_with(&canonical_app_root) {
        return Err("desktop generations directory escapes the data root".to_string());
    }
    let target = canonical_generations_root.join(&generation.generation_id);
    fs::create_dir(&target).map_err(|_| {
        "recovery generation directory already exists or cannot be created".to_string()
    })?;
    let canonical_target = fs::canonicalize(&target)
        .map_err(|_| "recovery generation directory cannot be resolved".to_string())?;
    if !canonical_target.starts_with(&canonical_generations_root) {
        return Err("recovery generation directory escapes its parent".to_string());
    }
    Ok(canonical_target)
}

#[cfg(not(windows))]
fn replace_generation_file(temporary_path: &Path, path: &Path) -> Result<(), String> {
    fs::rename(temporary_path, path)
        .map_err(|_| "failed to replace desktop generation state".to_string())?;
    let parent = path
        .parent()
        .ok_or_else(|| "desktop generation file has no parent directory".to_string())?;
    File::open(parent)
        .and_then(|directory| directory.sync_all())
        .map_err(|_| "failed to persist desktop generation state directory".to_string())
}

#[cfg(windows)]
fn replace_generation_file(temporary_path: &Path, path: &Path) -> Result<(), String> {
    use std::os::windows::ffi::OsStrExt;
    use winapi::um::winbase::{MoveFileExW, MOVEFILE_REPLACE_EXISTING, MOVEFILE_WRITE_THROUGH};

    let source: Vec<u16> = temporary_path
        .as_os_str()
        .encode_wide()
        .chain(Some(0))
        .collect();
    let destination: Vec<u16> = path.as_os_str().encode_wide().chain(Some(0)).collect();
    let moved = unsafe {
        MoveFileExW(
            source.as_ptr(),
            destination.as_ptr(),
            MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH,
        )
    };
    if moved == 0 {
        return Err("failed to persist desktop generation state replacement".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::{
        create_generation_directory, read_generation_state, read_prepared_generation,
        resolve_generation_cas_root, write_generation_state, write_prepared_generation,
    };
    use crate::generation::{GenerationState, PreparedGenerationManifest, RuntimeGenerationRef};
    use std::fs;
    use std::path::{Path, PathBuf};
    use uuid::Uuid;

    fn test_root() -> PathBuf {
        std::env::temp_dir().join(format!("polis-generation-store-{}", Uuid::new_v4()))
    }

    fn restored_generation(id: &str) -> RuntimeGenerationRef {
        RuntimeGenerationRef {
            generation_id: id.to_string(),
            database_name: format!("polis_recovery_{id}"),
            cas_relative_path: format!("generations/{id}/cas"),
            recovery_manifest_sha256: Some("b".repeat(64)),
        }
    }

    fn prepared_generation(id: &str, package_root: &Path) -> PreparedGenerationManifest {
        PreparedGenerationManifest {
            schema_version: "polis-desktop-prepared-generation@1".to_string(),
            generation: restored_generation(id),
            recovery_package_path: package_root.display().to_string(),
        }
    }

    #[test]
    fn active_generation_state_is_atomically_replaced_and_read_back() {
        let root = test_root();
        fs::create_dir_all(&root).unwrap();
        let path = root.join("config").join("generation-state.json");
        let legacy = GenerationState::legacy("polis_r0_desktop");
        write_generation_state(&path, &legacy).expect("legacy state should persist");

        let pending = legacy
            .begin_switch(restored_generation("0123456789abcdef0123456789abcdef"))
            .expect("candidate should stage");
        write_generation_state(&path, &pending).expect("pending switch should replace state");
        let restored =
            read_generation_state(&path, "polis_r0_desktop").expect("persisted state should parse");
        assert_eq!(restored, pending);
        assert!(restored.pending);

        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn absent_state_uses_legacy_database_generation() {
        let root = test_root();
        let path = root.join("missing-generation-state.json");
        let state = read_generation_state(&path, "polis_r0_desktop")
            .expect("absent state should select legacy layout");
        assert_eq!(state, GenerationState::legacy("polis_r0_desktop"));
    }

    #[test]
    fn invalid_state_does_not_overwrite_the_previous_file() {
        let root = test_root();
        fs::create_dir_all(&root).unwrap();
        let path = root.join("generation-state.json");
        let valid = GenerationState::legacy("polis_r0_desktop");
        write_generation_state(&path, &valid).unwrap();
        let original = fs::read(&path).unwrap();
        let mut invalid = valid.clone();
        invalid.schema_version = "unknown".to_string();

        assert!(write_generation_state(&path, &invalid).is_err());
        assert_eq!(fs::read(&path).unwrap(), original);
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn generation_cas_root_must_remain_inside_the_desktop_data_root() {
        let root = test_root();
        let id = "0123456789abcdef0123456789abcdef";
        let cas = root.join("generations").join(id).join("cas");
        fs::create_dir_all(&cas).unwrap();
        let generation = restored_generation(id);
        assert_eq!(
            resolve_generation_cas_root(&root, &generation).unwrap(),
            cas.canonicalize().unwrap()
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn generation_directory_is_created_as_a_direct_child_of_the_private_root() {
        let root = test_root();
        fs::create_dir_all(&root).unwrap();
        let id = "0123456789abcdef0123456789abcdef";
        let generation = restored_generation(id);
        let directory = create_generation_directory(&root, &generation).unwrap();
        assert_eq!(
            directory,
            root.join("generations").join(id).canonicalize().unwrap()
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn generation_cas_symlink_cannot_escape_the_desktop_data_root() {
        let root = test_root();
        let outside = test_root();
        let id = "0123456789abcdef0123456789abcdef";
        let generation_parent = root.join("generations").join(id);
        fs::create_dir_all(&generation_parent).unwrap();
        fs::create_dir_all(outside.join("cas")).unwrap();
        std::os::unix::fs::symlink(outside.join("cas"), generation_parent.join("cas")).unwrap();
        let result = resolve_generation_cas_root(&root, &restored_generation(id));
        assert!(result.is_err());
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn generation_cas_rejects_a_symlinked_generation_ancestor_inside_the_root() {
        let root = test_root();
        let id = "0123456789abcdef0123456789abcdef";
        let generations = root.join("generations");
        let actual = generations.join("actual-generation");
        fs::create_dir_all(actual.join("cas")).unwrap();
        std::os::unix::fs::symlink(&actual, generations.join(id)).unwrap();

        assert!(resolve_generation_cas_root(&root, &restored_generation(id)).is_err());
        fs::remove_dir_all(root).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn generation_directory_creation_rejects_a_symlinked_parent() {
        let root = test_root();
        let outside = test_root();
        fs::create_dir_all(&root).unwrap();
        fs::create_dir_all(&outside).unwrap();
        std::os::unix::fs::symlink(&outside, root.join("generations")).unwrap();
        let generation = restored_generation("0123456789abcdef0123456789abcdef");
        assert!(create_generation_directory(&root, &generation).is_err());
        assert_eq!(fs::read_dir(&outside).unwrap().count(), 0);
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[test]
    fn prepared_generation_manifest_round_trips_atomically() {
        let root = test_root();
        fs::create_dir_all(&root).unwrap();
        let package = root.join("backup-package");
        fs::create_dir_all(&package).unwrap();
        let manifest_path = root.join("generation.json");
        let manifest = prepared_generation("0123456789abcdef0123456789abcdef", &package);
        write_prepared_generation(&manifest_path, &manifest).unwrap();
        assert_eq!(read_prepared_generation(&manifest_path).unwrap(), manifest);
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn prepared_generation_reader_rejects_relative_package_paths() {
        let root = test_root();
        fs::create_dir_all(&root).unwrap();
        let manifest_path = root.join("generation.json");
        let mut manifest =
            prepared_generation("0123456789abcdef0123456789abcdef", &root.join("unused"));
        manifest.recovery_package_path = "relative/package".to_string();
        assert!(write_prepared_generation(&manifest_path, &manifest).is_err());
        fs::remove_dir_all(root).unwrap();
    }
}
