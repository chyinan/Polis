// pattern: Imperative Shell

use std::process::Child;

#[cfg(windows)]
use std::ptr::null_mut;
#[cfg(windows)]
use winapi::shared::minwindef::LPVOID;
#[cfg(windows)]
use winapi::um::handleapi::{CloseHandle, INVALID_HANDLE_VALUE};
#[cfg(windows)]
use winapi::um::jobapi2::{AssignProcessToJobObject, CreateJobObjectW, SetInformationJobObject};
#[cfg(windows)]
use winapi::um::processthreadsapi::OpenProcess;
#[cfg(windows)]
use winapi::um::winnt::{
    JobObjectExtendedLimitInformation, JOBOBJECT_EXTENDED_LIMIT_INFORMATION,
    JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, PROCESS_QUERY_INFORMATION, PROCESS_SET_QUOTA,
    PROCESS_TERMINATE,
};

pub struct ChildOwnership {
    #[cfg(windows)]
    handle: winapi::shared::ntdef::HANDLE,
}

unsafe impl Send for ChildOwnership {}
unsafe impl Sync for ChildOwnership {}

impl ChildOwnership {
    pub fn new() -> Result<Self, String> {
        #[cfg(windows)]
        unsafe {
            let handle = CreateJobObjectW(null_mut(), null_mut());
            if handle.is_null() || handle == INVALID_HANDLE_VALUE {
                return Err("failed to create Windows child-process job".to_string());
            }
            let mut limits: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = std::mem::zeroed();
            limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
            if SetInformationJobObject(
                handle,
                JobObjectExtendedLimitInformation,
                &mut limits as *mut _ as LPVOID,
                std::mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
            ) == 0
            {
                CloseHandle(handle);
                return Err("failed to configure Windows child-process job".to_string());
            }
            Ok(Self { handle })
        }
        #[cfg(not(windows))]
        Ok(Self {})
    }

    pub fn assign(&self, child: &Child) -> Result<(), String> {
        #[cfg(windows)]
        unsafe {
            let process = OpenProcess(
                PROCESS_SET_QUOTA | PROCESS_TERMINATE | PROCESS_QUERY_INFORMATION,
                0,
                child.id(),
            );
            if process.is_null() {
                return Err(format!(
                    "failed to open child process {} for ownership",
                    child.id()
                ));
            }
            let result = AssignProcessToJobObject(self.handle, process);
            CloseHandle(process);
            if result == 0 {
                return Err(format!(
                    "failed to assign child process {} to Windows job",
                    child.id()
                ));
            }
        }
        let _ = child;
        Ok(())
    }
}

impl Drop for ChildOwnership {
    fn drop(&mut self) {
        #[cfg(windows)]
        unsafe {
            if !self.handle.is_null() && self.handle != INVALID_HANDLE_VALUE {
                CloseHandle(self.handle);
            }
        }
    }
}
