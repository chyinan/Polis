// pattern: Imperative Shell

use serde::Serialize;

const RUN_KEY_PATH: &str = "Software\\Microsoft\\Windows\\CurrentVersion\\Run";
const RUN_VALUE_NAME: &str = "Polis";

#[derive(Clone, Debug, Serialize)]
pub struct WindowsStartupStatus {
    pub supported: bool,
    pub enabled: bool,
}

fn quoted_executable_path(path: &[u16]) -> Result<Vec<u16>, &'static str> {
    if path.is_empty() || path.contains(&0) || path.contains(&(b'"' as u16)) {
        return Err("invalid executable path for Windows startup");
    }
    let mut quoted = Vec::with_capacity(path.len() + 2);
    quoted.push(b'"' as u16);
    quoted.extend_from_slice(path);
    quoted.push(b'"' as u16);
    Ok(quoted)
}

#[cfg(windows)]
pub fn status() -> Result<WindowsStartupStatus, String> {
    use std::os::windows::ffi::OsStrExt;

    let executable = std::env::current_exe()
        .map_err(|_| "failed to resolve the Polis executable for Windows startup".to_string())?;
    let expected =
        quoted_executable_path(&executable.as_os_str().encode_wide().collect::<Vec<_>>())
            .map_err(str::to_string)?;
    let registered = registry::read_value(RUN_KEY_PATH, RUN_VALUE_NAME)
        .map_err(|_| "failed to read Windows startup registration".to_string())?;
    Ok(WindowsStartupStatus {
        supported: true,
        enabled: registered.as_deref() == Some(expected.as_slice()),
    })
}

#[cfg(not(windows))]
pub fn status() -> Result<WindowsStartupStatus, String> {
    Ok(WindowsStartupStatus {
        supported: false,
        enabled: false,
    })
}

#[cfg(windows)]
pub fn set_enabled(enabled: bool) -> Result<WindowsStartupStatus, String> {
    use std::os::windows::ffi::OsStrExt;

    if enabled {
        let executable = std::env::current_exe().map_err(|_| {
            "failed to resolve the Polis executable for Windows startup".to_string()
        })?;
        let command =
            quoted_executable_path(&executable.as_os_str().encode_wide().collect::<Vec<_>>())
                .map_err(str::to_string)?;
        registry::write_value(RUN_KEY_PATH, RUN_VALUE_NAME, &command)
            .map_err(|_| "failed to enable Windows startup for Polis".to_string())?;
    } else {
        registry::remove_value(RUN_KEY_PATH, RUN_VALUE_NAME)
            .map_err(|_| "failed to disable Windows startup for Polis".to_string())?;
    }
    status()
}

#[cfg(not(windows))]
pub fn set_enabled(_enabled: bool) -> Result<WindowsStartupStatus, String> {
    Err("Windows startup is only available on Windows".to_string())
}

#[cfg(windows)]
mod registry {
    use std::ptr::null_mut;

    use winapi::shared::minwindef::{BYTE, DWORD, HKEY, LPBYTE};
    use winapi::shared::winerror::{ERROR_FILE_NOT_FOUND, ERROR_MORE_DATA, ERROR_SUCCESS};
    use winapi::um::winnt::{KEY_QUERY_VALUE, KEY_SET_VALUE, REG_OPTION_NON_VOLATILE, REG_SZ};
    #[cfg(test)]
    use winapi::um::winreg::RegDeleteKeyW;
    use winapi::um::winreg::{
        RegCloseKey, RegCreateKeyExW, RegDeleteValueW, RegOpenKeyExW, RegQueryValueExW,
        RegSetValueExW, HKEY_CURRENT_USER,
    };

    const REG_CREATED_NEW_KEY: DWORD = 1;

    struct RegistryKey(HKEY);

    impl Drop for RegistryKey {
        fn drop(&mut self) {
            unsafe {
                RegCloseKey(self.0);
            }
        }
    }

    fn wide_null(value: &str) -> Vec<u16> {
        value.encode_utf16().chain(std::iter::once(0)).collect()
    }

    fn create_key(subkey: &str) -> Result<(RegistryKey, bool), DWORD> {
        let subkey = wide_null(subkey);
        let mut handle: HKEY = null_mut();
        let mut disposition = 0;
        let status = unsafe {
            RegCreateKeyExW(
                HKEY_CURRENT_USER,
                subkey.as_ptr(),
                0,
                null_mut(),
                REG_OPTION_NON_VOLATILE,
                KEY_QUERY_VALUE | KEY_SET_VALUE,
                null_mut(),
                &mut handle,
                &mut disposition,
            )
        };
        if status != ERROR_SUCCESS as i32 {
            return Err(status as DWORD);
        }
        if handle.is_null() {
            return Err(ERROR_FILE_NOT_FOUND);
        }
        Ok((RegistryKey(handle), disposition == REG_CREATED_NEW_KEY))
    }

    fn open_key(subkey: &str, access: DWORD) -> Result<Option<RegistryKey>, DWORD> {
        let subkey = wide_null(subkey);
        let mut handle: HKEY = null_mut();
        let status =
            unsafe { RegOpenKeyExW(HKEY_CURRENT_USER, subkey.as_ptr(), 0, access, &mut handle) };
        if status == ERROR_FILE_NOT_FOUND as i32 {
            return Ok(None);
        }
        if status != ERROR_SUCCESS as i32 {
            return Err(status as DWORD);
        }
        if handle.is_null() {
            return Err(ERROR_FILE_NOT_FOUND);
        }
        Ok(Some(RegistryKey(handle)))
    }

    pub(super) fn read_value(subkey: &str, name: &str) -> Result<Option<Vec<u16>>, DWORD> {
        let Some(key) = open_key(subkey, KEY_QUERY_VALUE)? else {
            return Ok(None);
        };
        let name = wide_null(name);
        let mut value_type = 0;
        let mut byte_count = 0;
        let status = unsafe {
            RegQueryValueExW(
                key.0,
                name.as_ptr(),
                null_mut(),
                &mut value_type,
                null_mut(),
                &mut byte_count,
            )
        };
        if status == ERROR_FILE_NOT_FOUND as i32 {
            return Ok(None);
        }
        if status != ERROR_SUCCESS as i32 {
            return Err(status as DWORD);
        }
        if value_type != REG_SZ {
            return Ok(None);
        }
        if byte_count > 65536 || byte_count % std::mem::size_of::<u16>() as u32 != 0 {
            return Err(ERROR_MORE_DATA);
        }
        let mut value = vec![0u16; (byte_count / 2) as usize];
        let status = unsafe {
            RegQueryValueExW(
                key.0,
                name.as_ptr(),
                null_mut(),
                &mut value_type,
                value.as_mut_ptr() as LPBYTE,
                &mut byte_count,
            )
        };
        if status != ERROR_SUCCESS as i32 {
            return Err(status as DWORD);
        }
        value.truncate((byte_count / 2) as usize);
        if let Some(nul) = value.iter().position(|unit| *unit == 0) {
            value.truncate(nul);
        }
        Ok(Some(value))
    }

    pub(super) fn write_value(subkey: &str, name: &str, value: &[u16]) -> Result<(), DWORD> {
        if value.is_empty() || value.contains(&0) {
            return Err(ERROR_MORE_DATA);
        }
        let (key, _) = create_key(subkey)?;
        let name = wide_null(name);
        let mut terminated = value.to_vec();
        terminated.push(0);
        let byte_count = u32::try_from(terminated.len() * std::mem::size_of::<u16>())
            .map_err(|_| ERROR_MORE_DATA)?;
        let status = unsafe {
            RegSetValueExW(
                key.0,
                name.as_ptr(),
                0,
                REG_SZ,
                terminated.as_ptr() as *const BYTE,
                byte_count,
            )
        };
        if status != ERROR_SUCCESS as i32 {
            return Err(status as DWORD);
        }
        Ok(())
    }

    pub(super) fn remove_value(subkey: &str, name: &str) -> Result<(), DWORD> {
        let Some(key) = open_key(subkey, KEY_SET_VALUE)? else {
            return Ok(());
        };
        let name = wide_null(name);
        let status = unsafe { RegDeleteValueW(key.0, name.as_ptr()) };
        if status != ERROR_SUCCESS as i32 && status != ERROR_FILE_NOT_FOUND as i32 {
            return Err(status as DWORD);
        }
        Ok(())
    }

    #[cfg(test)]
    pub(super) fn create_test_key(subkey: &str) -> Result<bool, DWORD> {
        let (key, created) = create_key(subkey)?;
        drop(key);
        Ok(created)
    }

    #[cfg(test)]
    pub(super) fn delete_test_key(subkey: &str) -> Result<(), DWORD> {
        let subkey = wide_null(subkey);
        let status = unsafe { RegDeleteKeyW(HKEY_CURRENT_USER, subkey.as_ptr()) };
        if status != ERROR_SUCCESS as i32 && status != ERROR_FILE_NOT_FOUND as i32 {
            return Err(status as DWORD);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::{quoted_executable_path, RUN_VALUE_NAME};

    #[test]
    fn startup_command_quotes_executable_paths_with_spaces() {
        let executable_path: Vec<u16> =
            r"C:\Program Files\Polis\Polis.exe".encode_utf16().collect();
        let expected: Vec<u16> = r#""C:\Program Files\Polis\Polis.exe""#.encode_utf16().collect();

        assert_eq!(quoted_executable_path(&executable_path).unwrap(), expected);
    }

    #[test]
    fn startup_registry_value_matches_nsis_product_name_cleanup() {
        let configuration: serde_json::Value =
            serde_json::from_str(include_str!("../tauri.conf.json")).unwrap();
        let product_name = configuration["productName"].as_str().unwrap();

        assert_eq!(RUN_VALUE_NAME, product_name);
    }
}

#[cfg(all(test, windows))]
mod windows_registry_tests {
    use super::registry;

    struct IsolatedRegistryKey(String);

    impl Drop for IsolatedRegistryKey {
        fn drop(&mut self) {
            let _ = registry::remove_value(&self.0, "Polis");
            let _ = registry::delete_test_key(&self.0);
        }
    }

    #[test]
    fn current_user_registry_value_round_trips_in_an_isolated_key() {
        let key_path = format!("Software\\PolisStartupTest-{}", uuid::Uuid::new_v4());
        assert!(registry::create_test_key(&key_path).expect("test key should be created"));
        let _cleanup = IsolatedRegistryKey(key_path.clone());

        assert_eq!(registry::read_value(&key_path, "Polis").unwrap(), None);
        registry::write_value(&key_path, "Polis", &[34, 67, 58, 92, 80, 34]).unwrap();
        assert_eq!(
            registry::read_value(&key_path, "Polis").unwrap(),
            Some(vec![34, 67, 58, 92, 80, 34])
        );
        registry::remove_value(&key_path, "Polis").unwrap();
        assert_eq!(registry::read_value(&key_path, "Polis").unwrap(), None);
    }
}
