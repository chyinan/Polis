# Slice 321 — Windows BrowserRun runner smoke

Date: 2026-10-07

## Scope

The Playwright runner now accepts an explicit Python package root and passes it
as controlled `PYTHONPATH`; no ambient user-site or environment is inherited.
The test-only insecure-TLS switch is accepted only for loopback origins. This
enabled a disposable Windows-native smoke against a Go TLS loopback fixture.

## Command and result

The Windows amd64 `internal/browser` test binary was cross-compiled in WSL2,
then run on Windows with:

```text
POLIS_TEST_PYTHON_PATH=C:\Python314\python.exe
POLIS_TEST_PYTHON_PACKAGE_ROOT=C:\Users\chyinan\AppData\Roaming\Python\Python314\site-packages
POLIS_TEST_BROWSER_PATH=C:\Program Files\Google\Chrome\Application\chrome.exe
POLIS_TEST_BROWSER_SCRIPT=D:\Programs\Polis-cloud-main\scripts\polis_playwright_runner.py
browser-windows.test.exe -test.v -test.run=TestPlaywrightRunnerWindowsChromeLoopbackTLSFixture
```

Result:

```text
=== RUN   TestPlaywrightRunnerWindowsChromeLoopbackTLSFixture
--- PASS: TestPlaywrightRunnerWindowsChromeLoopbackTLSFixture (1.97s)
PASS
```

The fixture rendered through Windows Chrome, the same-origin page was read,
and the runner validated zero downloads/WebSockets and cleaned its profile.
The temporary Windows test binary was removed after the run.

## Boundary

The TLS handshake warnings are expected for the test-only loopback
self-signed certificate. This does not qualify production TLS, WFP/profile
isolation, management-network denial, clean VM, Provider credentials or a
business BrowserRun. No external network or account was used.

