# pattern: Imperative Shell
from __future__ import annotations

import json
import hashlib
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from uuid import uuid4

from playwright.sync_api import expect, sync_playwright

base_url = os.environ.get("POLIS_TEST_UI_URL", "http://127.0.0.1:4173")
api_url = os.environ.get("POLIS_TEST_API_URL", "http://127.0.0.1:8081")
run_id = uuid4().hex[:12]
company_id = "r1-input-browser"
root = Path(__file__).resolve().parents[1]
evidence = root / "evidence" / "development" / "r1-input-vertical"
evidence.mkdir(parents=True, exist_ok=True)
browser_errors: list[str] = []

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page()
    page.on("pageerror", lambda error: browser_errors.append(str(error)))
    page.on("console", lambda message: browser_errors.append(message.text) if message.type == "error" else None)

    overview_response = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/overview")
    if overview_response.status != 200:
        raise RuntimeError("company overview fixture is unavailable")
    mission_id = overview_response.json()["mission"]["missionId"]
    before_response = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/missions/" + mission_id + "/inputs")
    if before_response.status != 200:
        raise RuntimeError("mission input baseline failed: " + str(before_response.status))
    before = before_response.json()

    page.goto(base_url + "/companies/" + company_id + "/mission", wait_until="networkidle")
    expect(page.get_by_role("heading", name="使命", exact=True)).to_be_visible()
    page.get_by_test_id("tab-inputs").click()
    expect(page.get_by_test_id("tab-inputs")).to_have_attribute("aria-selected", "true")
    page.get_by_test_id("mission-input-file").set_input_files({
        "name": "goal.md",
        "mimeType": "text/markdown",
        "buffer": b"# Goal\nAcceptance run " + run_id.encode("ascii") + b".\n",
    })
    expect(page.get_by_test_id("mission-input-upload-status")).to_contain_text("goal.md", timeout=10_000)
    expect(page.get_by_test_id("mission-input-revisions")).to_contain_text("goal.md · 版本 1")
    input_response = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/missions/" + mission_id + "/inputs")
    if input_response.status != 200:
        raise RuntimeError("mission input list failed: " + str(input_response.status) + " " + input_response.text())
    listed = input_response.json()
    digest = hashlib.sha256(b"# Goal\nAcceptance run " + run_id.encode("ascii") + b".\n").hexdigest()
    uploaded = [item for item in listed if item["contentDigest"] == digest]
    if len(listed) != len(before) + 1 or len(uploaded) != 1 or uploaded[0]["displayName"] != "goal.md" or str(uploaded[0]["revision"]) != "1" or uploaded[0]["state"] != "usable":
        raise RuntimeError("mission input list did not match uploaded revision: " + json.dumps(listed))

    with tempfile.TemporaryDirectory(prefix="polis-r1-directory-smoke-") as temporary_root:
        directory_root = Path(temporary_root) / "project"
        (directory_root / "src").mkdir(parents=True)
        (directory_root / "README.md").write_text("# Directory snapshot " + run_id + "\n", encoding="utf-8")
        (directory_root / "src" / "main.go").write_text("package main // " + run_id + "\n", encoding="utf-8")
        with page.expect_response(lambda response: response.url.endswith("/inputs/directory") and response.request.method == "POST") as directory_response_info:
            page.get_by_test_id("mission-directory-input").set_input_files(str(directory_root))
        directory_response = directory_response_info.value
        if directory_response.status != 202:
            raise RuntimeError("directory upload failed: " + str(directory_response.status) + " " + directory_response.text())
        directory_receipt = directory_response.json()
    expect(page.get_by_test_id("mission-input-upload-status")).to_contain_text("project", timeout=10_000)
    expect(page.get_by_test_id("mission-input-revisions")).to_contain_text("project \u00b7 \u7248\u672c 1")
    directory_list_response = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/missions/" + mission_id + "/inputs")
    directory_listed = directory_list_response.json()
    directory_input = [item for item in directory_listed if item["contentDigest"] == directory_receipt["contentDigest"]]
    if directory_list_response.status != 200 or len(directory_input) != 1 or directory_input[0]["sourceKind"] != "directory_snapshot" or directory_input[0]["displayName"] != "project" or directory_input[0]["state"] != "usable":
        raise RuntimeError("directory snapshot readback did not match upload: " + json.dumps(directory_listed))

    overview_response = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/overview")
    if overview_response.status != 200:
        raise RuntimeError("overview read failed: " + str(overview_response.status))
    if overview_response.json()["mission"]["state"] != "draft":
        raise RuntimeError("input upload changed the Mission state")
    page.screenshot(path=str(evidence / "mission-inputs.png"), full_page=True)

    page.goto(base_url + "/companies/" + company_id + "/notifications", wait_until="networkidle")
    page.get_by_test_id("qq-notification-target").fill("admin-openid-smoke")
    page.get_by_test_id("qq-notification-credential-ref").fill("default")
    page.get_by_test_id("qq-notification-alias").fill("R1 Smoke")
    if not page.get_by_test_id("qq-notification-test").is_disabled():
        raise RuntimeError("QQ test send was enabled without qualification")
    with page.expect_response(lambda response: response.url.endswith("/notifications/route") and response.request.method == "POST") as route_response_info:
        page.get_by_test_id("qq-notification-save").click()
    route_response = route_response_info.value
    if route_response.status != 202:
        raise RuntimeError("QQ route draft save failed: " + str(route_response.status) + " " + route_response.text())
    expect(page.get_by_test_id("qq-route-save-status")).to_be_visible(timeout=10_000)
    route_readback = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/notifications")
    if route_readback.status != 200:
        raise RuntimeError("QQ route readback failed: " + str(route_readback.status))
    qq_routes = [route for route in route_readback.json()["routes"] if route["adapter"] == "qq_official"]
    if len(qq_routes) != 1 or qq_routes[0]["enabled"] or qq_routes[0]["qualificationStatus"] != "unverified" or qq_routes[0]["credentialRef"] != "default" or qq_routes[0]["safetyAlias"] != "R1 Smoke" or "admin-openid-smoke" in qq_routes[0]["destination"]:
        raise RuntimeError("QQ route draft was not safely stored: " + json.dumps(qq_routes))

    lifecycle_create = page.request.post(
        api_url + "/api/workbench/companies/" + company_id + "/missions",
        data={"title": "Lifecycle browser acceptance", "goal": "Pause and resume with the deterministic worker.", "requestId": "r1-lifecycle-mission-" + run_id},
    )
    if lifecycle_create.status != 202:
        raise RuntimeError("lifecycle Mission create failed: " + str(lifecycle_create.status) + " " + lifecycle_create.text())
    lifecycle_mission_id = lifecycle_create.json()["targetId"]
    lifecycle_start = page.request.post(
        api_url + "/api/workbench/companies/" + company_id + "/missions/" + lifecycle_mission_id + "/start",
        data={"requestId": "r1-lifecycle-start-" + run_id},
    )
    if lifecycle_start.status != 202:
        raise RuntimeError("deterministic lifecycle start failed: " + str(lifecycle_start.status) + " " + lifecycle_start.text())
    page.goto(base_url + "/companies/" + company_id + "/mission", wait_until="networkidle")
    expect(page.get_by_test_id("mission-pause")).to_be_visible(timeout=10_000)
    page.get_by_test_id("mission-pause").click()
    expect(page.get_by_test_id("mission-resume")).to_be_visible(timeout=10_000)
    paused_overview = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/overview").json()
    if paused_overview["mission"]["missionId"] != lifecycle_mission_id or paused_overview["mission"]["state"] != "paused":
        raise RuntimeError("pause did not persist the Mission state")
    page.get_by_test_id("mission-resume").click()
    expect(page.get_by_test_id("mission-pause")).to_be_visible(timeout=10_000)
    resumed_overview = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/overview").json()
    if resumed_overview["mission"]["missionId"] != lifecycle_mission_id or resumed_overview["mission"]["state"] != "active":
        raise RuntimeError("resume did not restore the Mission state")
    with page.expect_response(lambda response: response.url.endswith("/cancel") and response.request.method == "POST") as cancel_response_info:
        page.get_by_test_id("mission-cancel").click()
    cancel_response = cancel_response_info.value
    if cancel_response.status != 202:
        raise RuntimeError("deterministic lifecycle cancel failed: " + str(cancel_response_info.value.status) + " " + cancel_response_info.value.text())
    cancel_receipt = cancel_response.json()
    if cancel_receipt.get("resultingState") != "cancelled":
        raise RuntimeError("lifecycle cancel receipt was not terminal: " + json.dumps(cancel_receipt))
    expect(page.get_by_test_id("mission-cancel")).to_have_count(0, timeout=10_000)
    final_overview = page.request.get(api_url + "/api/workbench/companies/" + company_id + "/overview").json()
    if final_overview["mission"]["state"] not in {"draft", "cancelled"}:
        raise RuntimeError("lifecycle smoke left an active or paused Mission")

    page.screenshot(path=str(evidence / "notification-settings.png"), full_page=True)
    if browser_errors:
        raise RuntimeError("browser errors: " + " | ".join(browser_errors))
    (evidence / "browser-smoke.json").write_text(
        json.dumps(
            {
                "result": "passed",
                "runId": run_id,
                "companyId": company_id,
                "missionId": mission_id,
                "inputName": "goal.md",
                "inputRevision": uploaded[0]["revision"],
                "directorySnapshot": {
                    "name": directory_receipt["displayName"],
                    "revision": directory_receipt["revision"],
                    "sourceKind": directory_receipt["sourceKind"],
                    "digest": directory_receipt["contentDigest"],
                    "files": ["project/README.md", "project/src/main.go"],
                },
                "missionStateAfterUpload": "draft",
                "qqRouteDraft": {
                    "adapter": qq_routes[0]["adapter"],
                    "enabled": qq_routes[0]["enabled"],
                    "qualificationStatus": qq_routes[0]["qualificationStatus"],
                    "credentialRef": qq_routes[0]["credentialRef"],
                    "targetMasked": "admin-openid-smoke" not in qq_routes[0]["destination"],
                    "externalTestSendEnabled": False,
                },
                "missionLifecycle": {
                    "missionId": lifecycle_mission_id,
                    "pause": "paused",
                    "resume": "active",
                    "cancelled": cancel_receipt["resultingState"],
                },
                "externalProviderCalls": "not-run",
                "generatedAt": datetime.now(timezone.utc).isoformat(),
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    browser.close()
