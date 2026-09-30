# pattern: Imperative Shell
from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

base_url = os.environ.get("POLIS_TEST_UI_URL", "http://127.0.0.1:4173")
api_url = os.environ.get("POLIS_TEST_API_URL", "http://127.0.0.1:8081")
company_id = os.environ.get("POLIS_TEST_COMPANY_ID", "r1-input-browser")
intervention_id = os.environ.get("POLIS_TEST_INTERVENTION_ID", "abcdef0123456789abcdef0123456789")
root = Path(__file__).resolve().parents[1]
evidence = root / "evidence" / "development" / "r1-input-qq-foundation"
evidence.mkdir(parents=True, exist_ok=True)
browser_errors: list[str] = []

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page()
    page.on("pageerror", lambda error: browser_errors.append(str(error)))
    page.on("console", lambda message: browser_errors.append(message.text) if message.type == "error" else None)

    overview_url = f"{api_url}/api/workbench/companies/{company_id}/overview"
    initial_overview = page.request.get(overview_url)
    if initial_overview.status != 200:
        raise RuntimeError("initial company overview failed: " + str(initial_overview.status))
    initial_items = initial_overview.json()["attention"]
    initial = next((item for item in initial_items if item["subject"]["id"] == intervention_id), None)
    if initial is None or initial.get("workflowState") != "open" or initial.get("notificationState") != "pending":
        raise RuntimeError("seed intervention is not open with a pending notification intent: " + json.dumps(initial_items))

    page.goto(f"{base_url}/companies/{company_id}/feedback", wait_until="networkidle")
    expect(page.get_by_role("heading", name="反馈待办", exact=True)).to_be_visible()
    expect(page.get_by_text("QQ 提醒 · 待处理", exact=True)).to_be_visible()
    expect(page.get_by_role("button", name="确认接管", exact=True)).to_be_visible()

    ack_path = f"/human-interventions/{intervention_id}/acknowledge"
    with page.expect_response(lambda response: response.url.endswith(ack_path) and response.request.method == "POST") as ack_info:
        page.get_by_role("button", name="确认接管", exact=True).click()
    ack_response = ack_info.value
    if ack_response.status != 202 or ack_response.json().get("resultingState") != "acknowledged":
        raise RuntimeError("acknowledgement command failed: " + ack_response.text())
    expect(page.get_by_role("button", name="标记已解决", exact=True)).to_be_visible()
    acknowledged_overview = page.request.get(overview_url).json()
    acknowledged = next((item for item in acknowledged_overview["attention"] if item["subject"]["id"] == intervention_id), None)
    if acknowledged is None or acknowledged.get("workflowState") != "acknowledged" or acknowledged.get("notificationState") != "superseded":
        raise RuntimeError("acknowledgement did not supersede the pending notice: " + json.dumps(acknowledged))
    page.screenshot(path=str(evidence / "human-intervention-feedback.png"), full_page=True)

    resolve_path = f"/human-interventions/{intervention_id}/resolve"
    with page.expect_response(lambda response: response.url.endswith(resolve_path) and response.request.method == "POST") as resolve_info:
        page.get_by_role("button", name="标记已解决", exact=True).click()
    resolve_response = resolve_info.value
    if resolve_response.status != 202 or resolve_response.json().get("resultingState") != "resolved":
        raise RuntimeError("resolve command failed: " + resolve_response.text())
    final_overview = page.request.get(overview_url).json()
    if any(item["subject"]["id"] == intervention_id for item in final_overview["attention"]):
        raise RuntimeError("resolved intervention remained in the attention list")
    if browser_errors:
        raise RuntimeError("browser errors: " + " | ".join(browser_errors))

    (evidence / "feedback-browser-smoke.json").write_text(
        json.dumps(
            {
                "result": "passed",
                "companyId": company_id,
                "interventionId": intervention_id,
                "initialNotificationState": "pending",
                "afterAcknowledge": {
                    "workflowState": acknowledged["workflowState"],
                    "notificationState": acknowledged["notificationState"],
                },
                "afterResolve": "not_in_attention",
                "externalQQSend": "not_run",
                "generatedAt": datetime.now(timezone.utc).isoformat(),
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    browser.close()
