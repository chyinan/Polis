// pattern: Imperative Shell

use crate::sidecar::{SidecarLocation, SidecarRef, SidecarState};
use serde::de::DeserializeOwned;
use sha2::{Digest, Sha256};
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use uuid::Uuid;

#[cfg(not(windows))]
use std::fs::File as SyncFile;

const MAX_SIDECAR_BYTES: u64 = 512 * 1024 * 1024;
const MAX_STATE_BYTES: u64 = 64 * 1024;

pub fn stage_binary(app_root: &Path, source: &Path) -> Result<SidecarRef, String> {
    let canonical_root = canonical_private_root(app_root)?;
    let config_directory = resolve_config_directory(&canonical_root)?;
    let source_metadata = fs::symlink_metadata(source)
        .map_err(|_| "selected sidecar binary is unavailable".to_string())?;
    if source_metadata.file_type().is_symlink()
        || !source_metadata.is_file()
        || source_metadata.len() == 0
        || source_metadata.len() > MAX_SIDECAR_BYTES
    {
        return Err("selected sidecar binary is unsafe or exceeds the size limit".to_string());
    }
    let canonical_source = fs::canonicalize(source)
        .map_err(|_| "selected sidecar binary cannot be resolved".to_string())?;
    ensure_regular_non_symlink(&canonical_source)?;
    let digest = file_sha256(&canonical_source)?;
    let destination = config_directory.join(candidate_filename(&digest));
    if destination.exists() {
        verify_digest(&destination, &digest)?;
    } else {
        copy_verified(&canonical_source, &destination, &digest)?;
    }
    let reference = SidecarRef::managed(format!("config/{}", candidate_filename(&digest)), digest);
    reference.validate()?;
    resolve_binary(&canonical_root, &reference)?;
    Ok(reference)
}

pub fn read_state(path: &Path) -> Result<SidecarState, String> {
    let metadata = fs::symlink_metadata(path)
        .map_err(|_| "desktop sidecar state is missing or unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_file() || metadata.len() > MAX_STATE_BYTES
    {
        return Err("desktop sidecar state is unsafe or too large".to_string());
    }
    let state: SidecarState = read_bounded_json(path, "desktop sidecar state")?;
    state.validate()?;
    let parent = path
        .parent()
        .ok_or_else(|| "desktop sidecar state has no parent directory".to_string())?;
    if path.file_name().and_then(|name| name.to_str()) != Some("sidecar-state.json")
        || parent.file_name().and_then(|name| name.to_str()) != Some("config")
    {
        return Err("desktop sidecar state path is outside the app config directory".to_string());
    }
    prune_interrupted_temporary_files(parent)?;
    Ok(state)
}

pub fn write_state(path: &Path, state: &SidecarState) -> Result<(), String> {
    state.validate()?;
    let parent = path
        .parent()
        .ok_or_else(|| "desktop sidecar state has no parent directory".to_string())?;
    fs::create_dir_all(parent)
        .map_err(|_| "failed to create desktop sidecar state directory".to_string())?;
    let parent_metadata = fs::symlink_metadata(parent)
        .map_err(|_| "desktop sidecar state directory is unavailable".to_string())?;
    if parent_metadata.file_type().is_symlink() || !parent_metadata.is_dir() {
        return Err("desktop sidecar state directory is unsafe".to_string());
    }
    if let Ok(metadata) = fs::symlink_metadata(path) {
        if metadata.file_type().is_symlink() || !metadata.is_file() {
            return Err("desktop sidecar state target is unsafe".to_string());
        }
    }
    let bytes = serde_json::to_vec(state)
        .map_err(|_| "failed to encode desktop sidecar state".to_string())?;
    if bytes.len() as u64 > MAX_STATE_BYTES {
        return Err("desktop sidecar state exceeds the size limit".to_string());
    }
    let temporary = parent.join(format!(".sidecar-state-{}.tmp", Uuid::new_v4().simple()));
    write_private_file(&temporary, &bytes)?;
    if let Err(error) = replace_file(&temporary, path) {
        let _ = fs::remove_file(&temporary);
        return Err(error);
    }
    Ok(())
}

pub fn persist_state(
    app_root: &Path,
    path: &Path,
    state: &SidecarState,
) -> Result<Option<String>, String> {
    let canonical_root = canonical_private_root(app_root)?;
    let expected_parent = resolve_config_directory(&canonical_root)?;
    let actual_parent = path
        .parent()
        .and_then(|parent| fs::canonicalize(parent).ok())
        .ok_or_else(|| "desktop sidecar state directory cannot be resolved".to_string())?;
    if path.file_name().and_then(|name| name.to_str()) != Some("sidecar-state.json")
        || actual_parent != expected_parent
    {
        return Err("desktop sidecar state path escapes the app config directory".to_string());
    }
    write_state(path, state)?;
    let config_directory = resolve_config_directory(&canonical_root)?;
    let mut warnings = Vec::new();
    if let Err(error) = prune_interrupted_temporary_files(&config_directory) {
        warnings.push(error);
    }
    if let Err(error) = prune_unreferenced_managed_binaries(&canonical_root, state) {
        warnings.push(error);
    }
    Ok((!warnings.is_empty()).then(|| warnings.join("; ")))
}

fn read_bounded_json<T: DeserializeOwned>(path: &Path, description: &str) -> Result<T, String> {
    let file = File::open(path).map_err(|_| format!("failed to read {description}"))?;
    let mut bytes = Vec::new();
    file.take(MAX_STATE_BYTES + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| format!("failed to read {description}"))?;
    if bytes.len() as u64 > MAX_STATE_BYTES {
        return Err(format!("{description} exceeds the size limit"));
    }
    serde_json::from_slice(&bytes).map_err(|_| format!("{description} is malformed"))
}

pub fn resolve_binary(app_root: &Path, reference: &SidecarRef) -> Result<PathBuf, String> {
    reference.validate()?;
    let path = match &reference.location {
        SidecarLocation::Installed(path) => {
            let installed = PathBuf::from(path);
            ensure_regular_non_symlink(&installed)?;
            installed
        }
        SidecarLocation::Managed(relative) => resolve_managed_file(app_root, relative)?,
    };
    verify_digest(&path, &reference.sha256)?;
    Ok(path)
}

pub fn retain_binary(app_root: &Path, reference: &SidecarRef) -> Result<SidecarRef, String> {
    let canonical_root = canonical_private_root(app_root)?;
    let config_directory = resolve_config_directory(&canonical_root)?;
    let source = resolve_binary(&canonical_root, reference)?;
    let destination = config_directory.join(retained_filename(&reference.sha256));
    if destination.exists() {
        verify_digest(&destination, &reference.sha256)?;
    } else {
        copy_verified(&source, &destination, &reference.sha256)?;
    }
    let mut retained = SidecarRef::managed(
        format!("config/{}", retained_filename(&reference.sha256)),
        reference.sha256.clone(),
    );
    retained.migration_manifest_sha256 = reference.migration_manifest_sha256.clone();
    retained.validate()?;
    resolve_binary(&canonical_root, &retained)?;
    Ok(retained)
}

fn canonical_private_root(root: &Path) -> Result<PathBuf, String> {
    let metadata =
        fs::symlink_metadata(root).map_err(|_| "desktop data root is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("desktop data root is unsafe".to_string());
    }
    fs::canonicalize(root).map_err(|_| "desktop data root cannot be resolved".to_string())
}

fn resolve_config_directory(root: &Path) -> Result<PathBuf, String> {
    let config = root.join("config");
    let metadata = fs::symlink_metadata(&config)
        .map_err(|_| "desktop app config directory is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("desktop app config directory is unsafe".to_string());
    }
    let canonical = fs::canonicalize(&config)
        .map_err(|_| "desktop app config directory cannot be resolved".to_string())?;
    if !canonical.starts_with(root) {
        return Err("desktop app config directory escapes the data root".to_string());
    }
    Ok(canonical)
}

fn candidate_filename(digest: &str) -> String {
    format!("polis-sidecar-candidate-{digest}.exe")
}

fn retained_filename(digest: &str) -> String {
    format!("polis-sidecar-retained-{digest}.exe")
}

fn is_interrupted_temporary_name(name: &str) -> bool {
    let suffix = name
        .strip_prefix(".polis-")
        .or_else(|| name.strip_prefix(".sidecar-state-"))
        .and_then(|value| value.strip_suffix(".tmp"));
    let Some(suffix) = suffix else {
        return false;
    };
    let bytes = suffix.as_bytes();
    suffix.len() == 32
        && bytes
            .iter()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(byte))
        && bytes[12] == b'4'
        && matches!(bytes[16], b'8' | b'9' | b'a' | b'b')
        && Uuid::parse_str(suffix).is_ok()
}

fn prune_interrupted_temporary_files(config: &Path) -> Result<usize, String> {
    let metadata = fs::symlink_metadata(config)
        .map_err(|_| "desktop app config directory is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("desktop app config directory is unsafe".to_string());
    }
    let entries = fs::read_dir(config)
        .map_err(|_| "failed to enumerate interrupted sidecar files".to_string())?;
    let mut stale_files = Vec::new();
    for entry in entries {
        let entry = entry.map_err(|_| "failed to inspect interrupted sidecar entry".to_string())?;
        let name = entry.file_name();
        let Some(name) = name.to_str() else {
            continue;
        };
        if !is_interrupted_temporary_name(name) {
            continue;
        }
        let metadata = fs::symlink_metadata(entry.path())
            .map_err(|_| "failed to inspect interrupted sidecar temporary file".to_string())?;
        if metadata.file_type().is_symlink() || !metadata.is_file() {
            return Err("interrupted sidecar temporary file is unsafe".to_string());
        }
        stale_files.push(entry.path());
    }
    for path in &stale_files {
        fs::remove_file(path)
            .map_err(|_| "failed to remove interrupted sidecar temporary file".to_string())?;
    }
    if !stale_files.is_empty() {
        sync_directory(config)?;
    }
    Ok(stale_files.len())
}

fn managed_filename_digest(name: &str) -> Option<(&'static str, &str)> {
    let (kind, digest) = if let Some(value) = name.strip_prefix("polis-sidecar-candidate-") {
        ("candidate", value)
    } else if let Some(value) = name.strip_prefix("polis-sidecar-retained-") {
        ("retained", value)
    } else {
        return None;
    };
    let digest = digest.strip_suffix(".exe")?;
    if digest.len() != 64
        || !digest
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
    {
        return None;
    }
    Some((kind, digest))
}

pub fn prune_unreferenced_managed_binaries(
    app_root: &Path,
    state: &SidecarState,
) -> Result<usize, String> {
    state.validate()?;
    let canonical_root = canonical_private_root(app_root)?;
    let config = resolve_config_directory(&canonical_root)?;
    let entries = fs::read_dir(&config)
        .map_err(|_| "failed to enumerate managed sidecar files".to_string())?;
    let mut candidates = Vec::new();
    let mut referenced = vec![&state.active];
    referenced.extend(state.previous.iter());
    referenced.extend(state.staged.iter());
    for entry in entries {
        let entry = entry.map_err(|_| "failed to inspect managed sidecar entry".to_string())?;
        let name = entry.file_name();
        let Some(name) = name.to_str() else {
            continue;
        };
        let Some((kind, digest)) = managed_filename_digest(name) else {
            continue;
        };
        let metadata = fs::symlink_metadata(entry.path())
            .map_err(|_| "failed to inspect managed sidecar file".to_string())?;
        if metadata.file_type().is_symlink() || !metadata.is_file() {
            return Err("managed sidecar filename resolves to an unsafe entry".to_string());
        }
        let relative = format!("config/{name}");
        let is_referenced = referenced.iter().any(|reference| {
            reference.sha256 == digest
                && matches!(
                    &reference.location,
                    SidecarLocation::Managed(path) if path == &relative
                )
        });
        if !is_referenced {
            candidates.push((kind, entry.path()));
        }
    }
    for (_, path) in &candidates {
        fs::remove_file(path)
            .map_err(|_| "failed to remove an unreferenced managed sidecar file".to_string())?;
    }
    if !candidates.is_empty() {
        sync_directory(&config)?;
    }
    Ok(candidates.len())
}

fn resolve_managed_file(root: &Path, relative: &str) -> Result<PathBuf, String> {
    let canonical_root = canonical_private_root(root)?;
    let candidate = canonical_root.join(relative);
    let mut current = canonical_root.clone();
    let relative_path = Path::new(relative);
    for component in relative_path.components() {
        match component {
            std::path::Component::Normal(name) => current.push(name),
            _ => return Err("managed sidecar path is unsafe".to_string()),
        }
        let metadata = fs::symlink_metadata(&current)
            .map_err(|_| "managed sidecar binary is unavailable".to_string())?;
        if metadata.file_type().is_symlink() {
            return Err("managed sidecar path contains a symlink".to_string());
        }
    }
    ensure_regular_non_symlink(&candidate)?;
    let canonical_candidate = fs::canonicalize(&candidate)
        .map_err(|_| "managed sidecar binary cannot be resolved".to_string())?;
    if !canonical_candidate.starts_with(&canonical_root) {
        return Err("managed sidecar binary escapes the desktop data root".to_string());
    }
    Ok(canonical_candidate)
}

pub fn publish_file(source: &Path, target: &Path, expected_sha256: &str) -> Result<(), String> {
    ensure_regular_non_symlink(source)?;
    if file_sha256(source)? != expected_sha256 {
        return Err("sidecar publication source does not match its SHA-256".to_string());
    }
    let parent = target
        .parent()
        .ok_or_else(|| "sidecar publication target has no parent directory".to_string())?;
    let metadata = fs::symlink_metadata(parent)
        .map_err(|_| "sidecar publication directory is unavailable".to_string())?;
    if metadata.file_type().is_symlink() || !metadata.is_dir() {
        return Err("sidecar publication directory is unsafe".to_string());
    }
    if let Ok(metadata) = fs::symlink_metadata(target) {
        if metadata.file_type().is_symlink() || !metadata.is_file() {
            return Err("sidecar publication target is unsafe".to_string());
        }
    }
    OpenOptions::new()
        .write(true)
        .open(source)
        .and_then(|file| file.sync_all())
        .map_err(|_| "failed to flush sidecar publication source".to_string())?;
    replace_file(source, target)
}

fn ensure_regular_non_symlink(path: &Path) -> Result<(), String> {
    let metadata =
        fs::symlink_metadata(path).map_err(|_| "sidecar binary is unavailable".to_string())?;
    if metadata.file_type().is_symlink()
        || !metadata.is_file()
        || metadata.len() == 0
        || metadata.len() > MAX_SIDECAR_BYTES
    {
        return Err("sidecar binary is unsafe or exceeds the size limit".to_string());
    }
    Ok(())
}

fn copy_verified(source: &Path, destination: &Path, expected: &str) -> Result<(), String> {
    ensure_regular_non_symlink(source)?;
    let parent = destination
        .parent()
        .ok_or_else(|| "sidecar destination has no parent directory".to_string())?;
    let temporary = parent.join(format!(".polis-{}.tmp", Uuid::new_v4().simple()));
    let result = (|| {
        let mut input =
            File::open(source).map_err(|_| "failed to read sidecar binary".to_string())?;
        let mut options = OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        let mut output = options
            .open(&temporary)
            .map_err(|_| "failed to create private sidecar copy".to_string())?;
        let mut hasher = Sha256::new();
        let mut buffer = [0u8; 64 * 1024];
        let mut bytes = 0u64;
        loop {
            let count = input
                .read(&mut buffer)
                .map_err(|_| "failed to read sidecar binary".to_string())?;
            if count == 0 {
                break;
            }
            bytes = bytes.saturating_add(count as u64);
            if bytes > MAX_SIDECAR_BYTES {
                return Err("sidecar binary exceeds the size limit".to_string());
            }
            output
                .write_all(&buffer[..count])
                .map_err(|_| "failed to write private sidecar copy".to_string())?;
            hasher.update(&buffer[..count]);
        }
        if bytes == 0 || format!("{:x}", hasher.finalize()) != expected {
            return Err("sidecar binary changed while it was being copied".to_string());
        }
        output
            .sync_all()
            .map_err(|_| "failed to persist private sidecar copy".to_string())?;
        drop(output);
        publish_file(&temporary, destination, expected)
    })();
    if result.is_err() {
        let _ = fs::remove_file(&temporary);
    }
    result
}

fn file_sha256(path: &Path) -> Result<String, String> {
    let mut file = File::open(path).map_err(|_| "failed to read sidecar binary".to_string())?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 64 * 1024];
    let mut bytes = 0u64;
    loop {
        let count = file
            .read(&mut buffer)
            .map_err(|_| "failed to read sidecar binary".to_string())?;
        if count == 0 {
            break;
        }
        bytes = bytes.saturating_add(count as u64);
        if bytes > MAX_SIDECAR_BYTES {
            return Err("sidecar binary exceeds the size limit".to_string());
        }
        hasher.update(&buffer[..count]);
    }
    if bytes == 0 {
        return Err("sidecar binary is empty".to_string());
    }
    Ok(format!("{:x}", hasher.finalize()))
}

fn verify_digest(path: &Path, expected: &str) -> Result<(), String> {
    ensure_regular_non_symlink(path)?;
    if file_sha256(path)? != expected {
        return Err("sidecar binary SHA-256 does not match its recorded identity".to_string());
    }
    Ok(())
}

#[cfg(unix)]
fn sync_directory(path: &Path) -> Result<(), String> {
    SyncFile::open(path)
        .and_then(|directory| directory.sync_all())
        .map_err(|_| "failed to sync managed sidecar directory".to_string())
}

#[cfg(windows)]
fn sync_directory(_path: &Path) -> Result<(), String> {
    Ok(())
}

fn write_private_file(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(path)
        .map_err(|_| "failed to create temporary desktop sidecar state".to_string())?;
    if file.write_all(bytes).is_err() || file.sync_all().is_err() {
        drop(file);
        let _ = fs::remove_file(path);
        return Err("failed to persist temporary desktop sidecar state".to_string());
    }
    Ok(())
}

#[cfg(not(windows))]
fn replace_file(temporary: &Path, target: &Path) -> Result<(), String> {
    fs::rename(temporary, target)
        .map_err(|_| "failed to replace desktop sidecar state".to_string())?;
    let parent = target
        .parent()
        .ok_or_else(|| "desktop sidecar state has no parent directory".to_string())?;
    SyncFile::open(parent)
        .and_then(|directory| directory.sync_all())
        .map_err(|_| "failed to persist desktop sidecar state directory".to_string())
}

#[cfg(windows)]
fn replace_file(temporary: &Path, target: &Path) -> Result<(), String> {
    use std::os::windows::ffi::OsStrExt;
    use winapi::um::winbase::{MoveFileExW, MOVEFILE_REPLACE_EXISTING, MOVEFILE_WRITE_THROUGH};

    let source: Vec<u16> = temporary.as_os_str().encode_wide().chain(Some(0)).collect();
    let destination: Vec<u16> = target.as_os_str().encode_wide().chain(Some(0)).collect();
    let moved = unsafe {
        MoveFileExW(
            source.as_ptr(),
            destination.as_ptr(),
            MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH,
        )
    };
    if moved == 0 {
        return Err("failed to persist desktop sidecar state replacement".to_string());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::{
        file_sha256, persist_state, publish_file, read_state, resolve_binary, retain_binary,
        stage_binary, write_state,
    };
    use crate::sidecar::{SidecarRef, SidecarState};
    use std::fs;
    use std::path::PathBuf;
    use uuid::Uuid;

    fn root() -> PathBuf {
        std::env::temp_dir().join(format!("polis-sidecar-store-{}", Uuid::new_v4()))
    }

    #[test]
    fn staging_same_binary_is_idempotent_and_returns_exact_hash() {
        let root = root();
        fs::create_dir_all(root.join("config")).unwrap();
        let source = root.join("candidate.exe");
        fs::write(&source, b"candidate binary bytes").unwrap();

        let first = stage_binary(&root, &source).unwrap();
        let second = stage_binary(&root, &source).unwrap();

        assert_eq!(first, second);
        assert_eq!(first.sha256.len(), 64);
        assert!(resolve_binary(&root, &first).is_ok());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn candidate_is_published_as_a_flat_content_addressed_file_in_config() {
        let root = root();
        fs::create_dir_all(root.join("config")).unwrap();
        let source = root.join("candidate.exe");
        fs::write(&source, b"candidate binary bytes").unwrap();

        let staged = stage_binary(&root, &source).unwrap();

        assert_eq!(
            staged,
            SidecarRef::managed(
                format!("config/polis-sidecar-candidate-{}.exe", staged.sha256),
                staged.sha256.clone(),
            )
        );
        assert!(root
            .join(format!(
                "config/polis-sidecar-candidate-{}.exe",
                staged.sha256
            ))
            .is_file());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn published_candidate_and_retained_files_are_durable_replacements() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let source = config.join("publish-source.tmp");
        let target = config.join("polis-sidecar-retained-test.exe");
        fs::write(&source, b"first durable bytes").unwrap();
        fs::write(&target, b"old bytes").unwrap();
        let first_digest = file_sha256(&source).unwrap();

        publish_file(&source, &target, &first_digest).unwrap();

        assert_eq!(fs::read(&target).unwrap(), b"first durable bytes");
        assert!(!source.exists());
        fs::write(&source, b"replacement durable bytes").unwrap();
        let second_digest = file_sha256(&source).unwrap();
        publish_file(&source, &target, &second_digest).unwrap();
        assert_eq!(fs::read(&target).unwrap(), b"replacement durable bytes");
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn durable_state_update_prunes_only_unreferenced_managed_binaries() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let source_a = root.join("source-a.exe");
        let source_b = root.join("source-b.exe");
        let source_c = root.join("source-c.exe");
        let source_orphan = root.join("source-orphan.exe");
        fs::write(&source_a, b"previous retained binary").unwrap();
        fs::write(&source_b, b"current active binary").unwrap();
        fs::write(&source_c, b"selected staged candidate").unwrap();
        fs::write(&source_orphan, b"unreferenced candidate").unwrap();
        let candidate_a = stage_binary(&root, &source_a).unwrap();
        let retained_a = retain_binary(&root, &candidate_a).unwrap();
        let active_b = stage_binary(&root, &source_b).unwrap();
        let staged_c = stage_binary(&root, &source_c).unwrap();
        let orphan = stage_binary(&root, &source_orphan).unwrap();
        let state = SidecarState {
            schema_version: "polis-desktop-sidecar-state@2".to_string(),
            active: active_b.clone(),
            previous: Some(retained_a.clone()),
            staged: Some(staged_c.clone()),
            pending: false,
            pending_operation: None,
        };

        assert!(
            persist_state(&root, &config.join("sidecar-state.json"), &state)
                .unwrap()
                .is_none()
        );

        assert!(resolve_binary(&root, &active_b).is_ok());
        assert!(resolve_binary(&root, &retained_a).is_ok());
        assert!(resolve_binary(&root, &staged_c).is_ok());
        assert!(!config
            .join(format!(
                "polis-sidecar-candidate-{}.exe",
                candidate_a.sha256
            ))
            .exists());
        assert!(!config
            .join(format!("polis-sidecar-candidate-{}.exe", orphan.sha256))
            .exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn state_pruning_refuses_non_regular_exact_managed_entries() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let digest = "c".repeat(64);
        let unsafe_directory = config.join(format!("polis-sidecar-candidate-{digest}.exe"));
        fs::create_dir(&unsafe_directory).unwrap();
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));

        let warning = persist_state(&root, &config.join("sidecar-state.json"), &state)
            .unwrap()
            .expect("unsafe matching entry must stop pruning");

        assert!(warning.contains("unsafe entry"));
        assert!(unsafe_directory.is_dir());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn state_update_removes_only_exact_interrupted_temp_files() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let id = Uuid::new_v4().simple().to_string();
        let binary_temp = config.join(format!(".polis-{id}.tmp"));
        let state_temp = config.join(format!(".sidecar-state-{id}.tmp"));
        let unrelated = config.join(format!(".polis-{}.tmp", "0".repeat(32)));
        fs::write(&binary_temp, b"interrupted binary copy").unwrap();
        fs::write(&state_temp, b"interrupted state write").unwrap();
        fs::write(&unrelated, b"unrelated file").unwrap();
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));

        assert!(
            persist_state(&root, &config.join("sidecar-state.json"), &state)
                .unwrap()
                .is_none()
        );

        assert!(!binary_temp.exists());
        assert!(!state_temp.exists());
        assert_eq!(fs::read(&unrelated).unwrap(), b"unrelated file");
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn loading_valid_state_removes_exact_interrupted_temp_files_only() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let state_path = config.join("sidecar-state.json");
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));
        write_state(&state_path, &state).unwrap();
        let id = Uuid::new_v4().simple().to_string();
        let stale = config.join(format!(".sidecar-state-{id}.tmp"));
        let unrelated = config.join(".sidecar-state-not-a-uuid.tmp");
        fs::write(&stale, b"interrupted state write").unwrap();
        fs::write(&unrelated, b"preserve").unwrap();

        assert_eq!(read_state(&state_path).unwrap(), state);

        assert!(!stale.exists());
        assert_eq!(fs::read(&unrelated).unwrap(), b"preserve");
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn state_update_refuses_exact_temp_pattern_directories_without_recursing() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let id = Uuid::new_v4().simple().to_string();
        let unsafe_temp = config.join(format!(".polis-{id}.tmp"));
        fs::create_dir(&unsafe_temp).unwrap();
        fs::write(unsafe_temp.join("keep.txt"), b"keep").unwrap();
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));

        let warning = persist_state(&root, &config.join("sidecar-state.json"), &state)
            .unwrap()
            .expect("unsafe temp entry must stop cleanup");

        assert!(warning.contains("interrupted sidecar temporary file is unsafe"));
        assert_eq!(fs::read(unsafe_temp.join("keep.txt")).unwrap(), b"keep");
        fs::remove_dir_all(root).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn state_load_refuses_symlink_with_exact_temp_pattern() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let outside = root.with_extension("outside");
        fs::create_dir_all(&outside).unwrap();
        let target = outside.join("outside.txt");
        fs::write(&target, b"outside protected").unwrap();
        let id = Uuid::new_v4().simple().to_string();
        std::os::unix::fs::symlink(&target, config.join(format!(".sidecar-state-{id}.tmp")))
            .unwrap();
        let state_path = config.join("sidecar-state.json");
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));
        write_state(&state_path, &state).unwrap();

        assert!(read_state(&state_path).is_err());
        assert_eq!(fs::read(target).unwrap(), b"outside protected");
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn managed_binary_storage_refuses_a_config_symlink_escape() {
        let root = root();
        let outside = root.with_extension("outside");
        fs::create_dir_all(&root).unwrap();
        fs::create_dir_all(&outside).unwrap();
        std::os::unix::fs::symlink(&outside, root.join("config")).unwrap();
        let source = root.join("candidate.exe");
        fs::write(&source, b"candidate binary bytes").unwrap();

        assert!(stage_binary(&root, &source).is_err());
        assert_eq!(fs::read_dir(&outside).unwrap().count(), 0);
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn state_pruning_refuses_unsafe_managed_symlinks_without_following_them() {
        let root = root();
        let config = root.join("config");
        fs::create_dir_all(&config).unwrap();
        let outside = root.with_extension("outside");
        fs::create_dir_all(&outside).unwrap();
        let digest = "b".repeat(64);
        let outside_file = outside.join("payload.exe");
        fs::write(&outside_file, b"outside protected bytes").unwrap();
        std::os::unix::fs::symlink(
            &outside_file,
            config.join(format!("polis-sidecar-candidate-{digest}.exe")),
        )
        .unwrap();
        let state = SidecarState::initial(SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        ));

        let result = persist_state(&root, &config.join("sidecar-state.json"), &state);

        assert!(result.unwrap().is_some());
        assert_eq!(fs::read(&outside_file).unwrap(), b"outside protected bytes");
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[test]
    fn staged_candidate_hash_tampering_is_rejected() {
        let root = root();
        fs::create_dir_all(root.join("config")).unwrap();
        let source = root.join("candidate.exe");
        fs::write(&source, b"candidate binary bytes").unwrap();
        let staged = stage_binary(&root, &source).unwrap();
        let path = resolve_binary(&root, &staged).unwrap();
        fs::write(path, b"tampered candidate").unwrap();

        assert!(resolve_binary(&root, &staged).is_err());
        fs::remove_dir_all(root).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn staged_candidate_symlink_escape_is_rejected() {
        let root = root();
        let outside = root.with_extension("outside");
        fs::create_dir_all(root.join("config")).unwrap();
        fs::create_dir_all(&outside).unwrap();
        let source = root.join("candidate.exe");
        fs::write(&source, b"candidate binary bytes").unwrap();
        let staged = stage_binary(&root, &source).unwrap();
        let inside = resolve_binary(&root, &staged).unwrap();
        let outside_file = outside.join("outside.exe");
        fs::write(&outside_file, b"candidate binary bytes").unwrap();
        fs::remove_file(&inside).unwrap();
        std::os::unix::fs::symlink(&outside_file, &inside).unwrap();

        assert!(resolve_binary(&root, &staged).is_err());
        fs::remove_dir_all(root).unwrap();
        fs::remove_dir_all(outside).unwrap();
    }

    #[test]
    fn state_round_trip_and_path_escape_are_checked() {
        let root = root();
        fs::create_dir_all(root.join("config")).unwrap();
        let installed = SidecarRef::installed(
            std::env::current_exe().unwrap().display().to_string(),
            "a".repeat(64),
        );
        let mut state = SidecarState::initial(installed);
        state.staged = Some(SidecarRef::managed(
            "../../outside.exe".to_string(),
            "b".repeat(64),
        ));
        assert!(write_state(&root.join("config/sidecar-state.json"), &state).is_err());
        state.staged = None;
        write_state(&root.join("config/sidecar-state.json"), &state).unwrap();
        assert_eq!(
            read_state(&root.join("config/sidecar-state.json")).unwrap(),
            state
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn retaining_an_installed_binary_creates_one_verified_private_copy() {
        let root = root();
        fs::create_dir_all(root.join("config")).unwrap();
        let source = root.join("installed.exe");
        fs::write(&source, b"installed binary bytes").unwrap();
        let staged = stage_binary(&root, &source).unwrap();
        let retained = retain_binary(&root, &staged).unwrap();

        assert!(resolve_binary(&root, &retained).is_ok());
        assert!(matches!(
            retained.location,
            crate::sidecar::SidecarLocation::Managed(_)
        ));
        fs::remove_dir_all(root).unwrap();
    }
}
