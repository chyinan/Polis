# REQ-38 BrowserRun control-plane foundation

Slice285 adds Schema117 BrowserRun identities/events and fake-only product surface @18. A browser request is bound to the current WorkerSession and a current same-Mission service generation, then recorded as `blocked/browser_runtime_unqualified`. Results are read-only control-plane metadata; no URL navigation, credential use, browser process, download or network egress exists in this slice.

The next REQ-38 work remains isolated Playwright/Chromium execution and evidence binding after explicit origin, test identity, profile and management-network qualification.
