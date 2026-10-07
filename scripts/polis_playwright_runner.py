#!/usr/bin/env python3
# pattern: Imperative Shell
"""Run one bounded, same-origin Playwright browser plan.

The Go runner supplies a freshly empty profile directory and a JSON plan on
stdin. This script is deliberately a protocol adapter: it never reads a
cookie jar or storage state, accepts no downloads, blocks cross-origin
requests, closes WebSockets, and emits bounded JSON evidence only. It is not
enabled by the product runtime until the host/browser qualification gate is
opened.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import Error as PlaywrightError
from playwright.sync_api import sync_playwright


PROTOCOL = "polis-browser-runner@1"
MAX_TEXT_BYTES = 64 * 1024
MAX_TITLE_BYTES = 1024


def canonical_origin(value: str) -> str:
    parsed = urlsplit(value)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ("", "/"):
        raise ValueError("target origin must be an HTTPS origin")
    host = parsed.hostname.lower()
    if ":" in host:
        host = f"[{host}]"
    port = parsed.port
    if port not in (None, 443):
        host = f"{host}:{port}"
    return f"https://{host}"


def origin_of(value: str) -> str:
    parsed = urlsplit(value)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.fragment:
        raise ValueError("URL is not an HTTPS browser target")
    host = parsed.hostname.lower()
    if ":" in host:
        host = f"[{host}]"
    port = parsed.port
    if port not in (None, 443):
        host = f"{host}:{port}"
    return f"https://{host}"


def emit_failure(reason: str) -> None:
    print(json.dumps({
        "protocol": PROTOCOL,
        "state": "failed",
        "reason_code": reason,
        "final_url": "",
        "title": "",
        "text": "",
        "request_count": 0,
        "blocked_request_count": 0,
        "response_bytes": 0,
        "download_count": 0,
        "websocket_count": 0,
    }, separators=(",", ":")))


def main() -> int:
    try:
        plan = json.load(sys.stdin)
        target_origin = canonical_origin(str(plan["target_origin"]))
        timeout_ms = int(plan["timeout_ms"])
        max_requests = int(plan["max_requests"])
        max_response_bytes = int(plan["max_response_bytes"])
        test_only_allow_insecure_tls = bool(plan.get("test_only_allow_insecure_tls", False))
        profile_dir = Path(str(plan["profile_dir"])).resolve()
        browser_executable = str(plan.get("browser_executable", ""))
        if timeout_ms < 1000 or timeout_ms > 120000 or max_requests < 1 or max_requests > 512 or max_response_bytes < 1 or max_response_bytes > 8 * 1024 * 1024:
            raise ValueError("browser plan bounds are invalid")
        if not profile_dir.is_dir() or any(profile_dir.iterdir()):
            raise ValueError("browser profile directory must start empty")
        if browser_executable and not Path(browser_executable).is_file():
            raise ValueError("configured browser executable is unavailable")
        if test_only_allow_insecure_tls and urlsplit(target_origin).hostname not in ("localhost", "127.0.0.1", "::1"):
            raise ValueError("insecure TLS is only permitted for a loopback fixture")
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        emit_failure("browser_plan_invalid")
        print(str(error), file=sys.stderr)
        return 0

    request_count = 0
    blocked_request_count = 0
    response_bytes = 0
    download_count = 0
    websocket_count = 0
    final_url = ""
    title = ""
    text = ""
    browser_context = None
    try:
        with sync_playwright() as playwright:
            launch_options = {
                "headless": True,
                "accept_downloads": False,
                "service_workers": "block",
                "ignore_https_errors": test_only_allow_insecure_tls,
                "args": [
                    "--disable-background-networking",
                    "--disable-component-update",
                    "--disable-default-apps",
                    "--disable-sync",
                    "--no-first-run",
                ],
            }
            if browser_executable:
                launch_options["executable_path"] = browser_executable
            browser_context = playwright.chromium.launch_persistent_context(str(profile_dir), **launch_options)
            browser_context.set_default_timeout(timeout_ms)

            def handle_route(route) -> None:
                nonlocal blocked_request_count, request_count
                request = route.request
                request_count += 1
                try:
                    allowed = origin_of(request.url) == target_origin
                except ValueError:
                    allowed = False
                if not allowed or request_count > max_requests:
                    blocked_request_count += 1
                    route.abort()
                    return
                route.continue_()

            browser_context.route("**/*", handle_route)
            page = browser_context.pages[0] if browser_context.pages else browser_context.new_page()

            def handle_response(response) -> None:
                nonlocal response_bytes
                content_length = response.headers.get("content-length", "")
                if content_length.isdigit():
                    response_bytes += int(content_length)

            def handle_download(download) -> None:
                nonlocal download_count
                download_count += 1
                download.cancel()

            def handle_websocket(websocket) -> None:
                nonlocal websocket_count
                websocket_count += 1
                websocket.close()

            page.on("response", handle_response)
            page.on("download", handle_download)
            page.on("websocket", handle_websocket)
            page.goto(target_origin + "/", wait_until="networkidle", timeout=timeout_ms)
            final_url = page.url
            title = page.title()[:MAX_TITLE_BYTES]
            text = page.locator("body").inner_text(timeout=timeout_ms)[:MAX_TEXT_BYTES]
            browser_context.close()
    except (PlaywrightError, ValueError, OSError) as error:
        emit_failure("browser_execution_failed")
        print(str(error), file=sys.stderr)
        return 0

    print(json.dumps({
        "protocol": PROTOCOL,
        "state": "succeeded",
        "reason_code": "browser_run_succeeded",
        "final_url": final_url,
        "title": title,
        "text": text,
        "request_count": request_count,
        "blocked_request_count": blocked_request_count,
        "response_bytes": response_bytes,
        "download_count": download_count,
        "websocket_count": websocket_count,
    }, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
