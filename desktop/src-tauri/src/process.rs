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

pub fn assign_or_terminate(
    child: &mut Option<Child>,
    assign: impl FnOnce(&Child) -> Result<(), String>,
) -> Result<(), String> {
    let assignment_error = {
        let process = child
            .as_ref()
            .ok_or_else(|| "child process is not tracked".to_string())?;
        assign(process).err()
    };
    let Some(assignment_error) = assignment_error else {
        return Ok(());
    };

    let stop_result = terminate_and_reap(child);
    match stop_result {
        Ok(()) => Err(assignment_error),
        Err(stop_error) => Err(format!(
            "{assignment_error}; child process stop is unconfirmed: {stop_error}"
        )),
    }
}

fn terminate_and_reap(child: &mut Option<Child>) -> Result<(), String> {
    let process = child
        .as_mut()
        .ok_or_else(|| "child process is not tracked".to_string())?;
    if process
        .try_wait()
        .map_err(|error| format!("failed to inspect child process: {error}"))?
        .is_none()
    {
        if let Err(kill_error) = process.kill() {
            if process
                .try_wait()
                .map_err(|error| format!("failed to confirm child process exit: {error}"))?
                .is_none()
            {
                return Err(format!("failed to terminate child process: {kill_error}"));
            }
        }
        process
            .wait()
            .map_err(|error| format!("failed to reap child process: {error}"))?;
    }
    *child = None;
    Ok(())
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

#[cfg(test)]
mod tests {
    use super::assign_or_terminate;
    use std::process::{Child, Command};

    #[cfg(windows)]
    fn long_lived_child() -> Command {
        let mut command = Command::new("powershell.exe");
        command.args(["-NoProfile", "-Command", "Start-Sleep -Seconds 30"]);
        command
    }

    #[cfg(not(windows))]
    fn long_lived_child() -> Command {
        Command::new("sleep").arg("30")
    }

    #[test]
    fn assignment_failure_terminates_and_reaps_the_child_before_returning() {
        let mut child: Option<Child> = Some(long_lived_child().spawn().unwrap());
        let child_id = child.as_ref().unwrap().id();
        let error = assign_or_terminate(&mut child, |_| {
            Err("injected ownership assignment failure".to_string())
        })
        .unwrap_err();

        assert!(error.contains("injected ownership assignment failure"));
        assert!(child.is_none(), "terminated child handle should be reaped");
        assert!(
            !child_is_running(child_id),
            "child process {child_id} is still live"
        );
    }

    #[cfg(windows)]
    fn child_is_running(child_id: u32) -> bool {
        Command::new("tasklist")
            .args(["/FI", &format!("PID eq {child_id}"), "/FO", "CSV", "/NH"])
            .output()
            .map(|output| {
                String::from_utf8_lossy(&output.stdout).contains(&format!("\"{child_id}\""))
            })
            .unwrap_or(true)
    }

    #[cfg(not(windows))]
    fn child_is_running(child_id: u32) -> bool {
        Command::new("kill")
            .args(["-0", &child_id.to_string()])
            .status()
            .map(|status| status.success())
            .unwrap_or(true)
    }
}
