# pattern: Imperative Shell
from __future__ import annotations

import atexit
import hashlib
import io
import json
import os
import re
import urllib.request
import zipfile
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


EXPECTED_BASE_URL = "http://127.0.0.1:4173"
EXPECTED_API_URL = "http://127.0.0.1:18084/api/workbench"
EXPECTED_TOKEN = "slice26-browser-session"
BASE_URL = os.environ.get("POLIS_WORKBENCH_FRONTEND_URL", EXPECTED_BASE_URL)
API_URL = os.environ.get("POLIS_WORKBENCH_API_URL", EXPECTED_API_URL)
TOKEN = os.environ.get("POLIS_DESKTOP_SESSION_TOKEN", EXPECTED_TOKEN)
COMPANY_ID = "browser-company-01"
ARTIFACT_ID = "artifact-browser-01"
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r1-r3-implementation-validation-20260925-slice-26"


def validate_smoke_configuration() -> None:
    if BASE_URL != EXPECTED_BASE_URL:
        raise RuntimeError("browser smoke frontend URL must remain pinned to the local loopback fixture")
    if API_URL != EXPECTED_API_URL:
        raise RuntimeError("browser smoke API URL must remain pinned to the local loopback fixture")
    if TOKEN != EXPECTED_TOKEN:
        raise RuntimeError("browser smoke token must use the local fixture-only token")


def stop_fixture_server() -> None:
    fixture_root = EXPECTED_API_URL.split("/api/workbench", 1)[0]
    request = urllib.request.Request(
        f"{fixture_root}/__fixture/shutdown",
        method="POST",
        headers={"X-Polis-Desktop-Token": EXPECTED_TOKEN},
    )
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            if response.status != 202:
                raise RuntimeError(f"fixture shutdown returned {response.status}")
    except Exception:
        # The fixture may already have exited after an earlier startup failure.
        pass


validate_smoke_configuration()
atexit.register(stop_fixture_server)


def run() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    browser_errors: list[str] = []
    download_headers: list[dict[str, str]] = []
    api_responses: list[dict[str, object]] = []
    package_path = EVIDENCE_DIR / "polis-delivery.zip"

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)

        anonymous = browser.new_context()
        anonymous_response = anonymous.request.get(
            f"{API_URL}/companies/{COMPANY_ID}/artifacts/{ARTIFACT_ID}/manifest"
        )
        if anonymous_response.status != 401:
            raise RuntimeError(f"unauthenticated manifest status was {anonymous_response.status}, expected 401")
        anonymous.close()

        context = browser.new_context(
            viewport={"width": 1440, "height": 1000},
            locale="zh-CN",
            accept_downloads=True,
            extra_http_headers={"X-Polis-Desktop-Token": TOKEN},
        )
        raw_package_response = context.request.get(
            f"{API_URL}/companies/{COMPANY_ID}/artifacts/{ARTIFACT_ID}/download"
        )
        if raw_package_response.status != 200:
            raise RuntimeError(f"fixture package request returned {raw_package_response.status}")
        with zipfile.ZipFile(io.BytesIO(raw_package_response.body()), "r") as inspect_archive:
            zip_metadata = [
                {"name": info.filename, "flags": info.flag_bits, "method": info.compress_type, "compressedSize": info.compress_size, "size": info.file_size}
                for info in inspect_archive.infolist()
            ]
        (EVIDENCE_DIR / "artifact-delivery-zip-debug.json").write_text(
            json.dumps(zip_metadata, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        page = context.new_page()
        page.on("pageerror", lambda error: browser_errors.append(str(error)))
        page.on("console", lambda message: browser_errors.append(message.text) if message.type == "error" else None)
        page.on(
            "response",
            lambda response: (
                api_responses.append({"url": response.url, "status": response.status}),
                download_headers.append(response.headers) if response.url.endswith(f"/artifacts/{ARTIFACT_ID}/download") else None,
            ) if "/api/workbench/" in response.url else None,
        )

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/tasks", wait_until="networkidle")
        expect(page.get_by_role("heading", name="任务", exact=True)).to_be_visible(timeout=15_000)
        page.get_by_test_id("tab-browser").click()
        download_button = page.get_by_role("button", name=re.compile("下载含完整 Manifest 的 ZIP"))
        expect(download_button).to_be_visible(timeout=15_000)
        expect(page.get_by_text(re.compile("Manifest SHA-256"))).to_be_visible()
        page.screenshot(path=str(EVIDENCE_DIR / "artifact-delivery-browser-before-download.png"), full_page=True)

        try:
            with page.expect_download(timeout=10_000) as download_info:
                download_button.click()
        except Exception:
            failure = {
                "apiResponses": api_responses,
                "browserErrors": browser_errors,
                "visibleAlerts": page.get_by_role("alert").all_text_contents(),
            }
            (EVIDENCE_DIR / "artifact-delivery-browser-failure.json").write_text(
                json.dumps(failure, ensure_ascii=False, indent=2), encoding="utf-8"
            )
            page.screenshot(path=str(EVIDENCE_DIR / "artifact-delivery-browser-failure.png"), full_page=True)
            raise
        download = download_info.value
        download.save_as(str(package_path))
        page.screenshot(path=str(EVIDENCE_DIR / "artifact-delivery-browser.png"), full_page=True)

        if not download_headers or download_headers[-1].get("x-content-sha256") is None:
            raise RuntimeError("browser download response omitted its package integrity headers")
        package_bytes = package_path.read_bytes()
        if hashlib.sha256(package_bytes).hexdigest() != download_headers[-1]["x-content-sha256"]:
            raise RuntimeError("saved browser download differs from the package hash response header")
        if download_headers[-1].get("x-polis-manifest-sha256") is None:
            raise RuntimeError("browser download response omitted the manifest integrity header")

        with zipfile.ZipFile(package_path, "r") as archive:
            entries = {name: archive.read(name) for name in archive.namelist()}
        expected_entries = {"artifact.bin", "manifest.json", "SHA256SUMS"}
        if set(entries) != expected_entries:
            raise RuntimeError(f"browser downloaded unexpected ZIP entries: {sorted(entries)}")
        manifest_bytes = entries["manifest.json"]
        manifest_digest = hashlib.sha256(manifest_bytes).hexdigest()
        if manifest_digest != download_headers[-1]["x-polis-manifest-sha256"]:
            raise RuntimeError("embedded manifest hash differs from the browser response header")
        manifest = json.loads(manifest_bytes.decode("utf-8"))
        artifact_digest = hashlib.sha256(entries["artifact.bin"]).hexdigest()
        expected_checksums = f"{manifest_digest}  manifest.json\n{artifact_digest}  artifact.bin\n"
        if manifest["artifactId"] != ARTIFACT_ID or manifest["companyId"] != COMPANY_ID:
            raise RuntimeError("embedded manifest does not match the requested company/artifact")
        if artifact_digest != manifest["content"]["sha256"] or entries["SHA256SUMS"].decode("utf-8") != expected_checksums:
            raise RuntimeError("artifact bytes or SHA256SUMS differ from the embedded manifest")
        if browser_errors:
            raise RuntimeError("browser errors: " + " | ".join(browser_errors))

        result = {
            "result": "passed",
            "companyId": COMPANY_ID,
            "artifactId": ARTIFACT_ID,
            "unauthenticatedManifestStatus": anonymous_response.status,
            "authenticatedManifestStatus": 200,
            "downloadFilename": download.suggested_filename,
            "downloadPackageSha256": hashlib.sha256(package_bytes).hexdigest(),
            "manifestSha256": manifest_digest,
            "artifactSha256": artifact_digest,
            "zipEntries": sorted(entries),
            "browserErrors": browser_errors,
            "realModelOrExternalProvider": "not-run",
        }
        (EVIDENCE_DIR / "artifact-delivery-browser.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        context.close()
        browser.close()
        print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    run()
