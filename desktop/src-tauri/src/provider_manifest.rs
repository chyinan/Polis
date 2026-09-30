// pattern: Functional Core

use serde_json::Value;

#[derive(Clone, Debug, PartialEq, Eq)]
pub(crate) struct ProviderManifest {
    pub(crate) version: String,
    pub(crate) binary_sha256: String,
    pub(crate) helper_sha256: String,
}

pub(crate) fn parse_provider_manifest(raw: &str) -> Result<ProviderManifest, String> {
    let value: Value =
        serde_json::from_str(raw).map_err(|_| "provider manifest is invalid JSON".to_string())?;
    let version = required_string(&value, "codex_version")?;
    let binary_sha256 = required_digest(&value, "codex_binary_sha256")?;
    let helper_sha256 = required_digest(&value, "code_mode_host_sha256")?;
    Ok(ProviderManifest {
        version,
        binary_sha256,
        helper_sha256,
    })
}

pub(crate) fn validate_provider_auth(raw: &str) -> Result<(), String> {
    let value: Value =
        serde_json::from_str(raw).map_err(|_| "provider auth is invalid JSON".to_string())?;
    if value.get("auth_mode").and_then(Value::as_str) != Some("chatgpt") {
        return Err("provider auth mode is not chatgpt".to_string());
    }
    let Some(tokens) = value.get("tokens").and_then(Value::as_object) else {
        return Err("provider auth tokens are missing".to_string());
    };
    for name in ["id_token", "access_token", "refresh_token"] {
        if tokens
            .get(name)
            .and_then(Value::as_str)
            .map(str::is_empty)
            .unwrap_or(true)
        {
            return Err(format!("provider auth token is missing: {name}"));
        }
    }
    Ok(())
}

fn required_string(value: &Value, field: &str) -> Result<String, String> {
    value
        .get(field)
        .and_then(Value::as_str)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .ok_or_else(|| format!("provider manifest field is missing: {field}"))
}

fn required_digest(value: &Value, field: &str) -> Result<String, String> {
    let digest = required_string(value, field)?;
    if digest.len() != 64 || !digest.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(format!("provider manifest digest is invalid: {field}"));
    }
    Ok(digest)
}

#[cfg(test)]
mod tests {
    use super::{parse_provider_manifest, validate_provider_auth};

    #[test]
    fn accepts_complete_runtime_manifest() {
        let raw = r#"{"schema_version":"windows-runtime-artifact-v1","codex_version":"0.154.0","codex_binary_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","code_mode_host_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}"#;
        let manifest = parse_provider_manifest(raw).expect("manifest should parse");
        assert_eq!(manifest.version, "0.154.0");
    }

    #[test]
    fn rejects_manifest_without_artifact_pins() {
        let error = parse_provider_manifest(r#"{"codex_version":"0.154.0"}"#)
            .expect_err("missing pins should fail");
        assert!(error.contains("codex_binary_sha256"));
    }

    #[test]
    fn rejects_incomplete_provider_auth() {
        let error =
            validate_provider_auth(r#"{"auth_mode":"chatgpt","tokens":{"id_token":"only"}}"#)
                .expect_err("incomplete auth should fail");
        assert!(error.contains("access_token"));
    }
}
