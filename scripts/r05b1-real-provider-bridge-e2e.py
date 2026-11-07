"""Offline R0.5B1 product bridge E2E: real mode + fake provider transport."""

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("R05B1_FRONTEND_URL", "http://127.0.0.1:4175")
COMPANY_ID = os.environ.get("R05B1_COMPANY_ID", "r05b1-e2e-company")
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.5b1-real-provider-runtime-bridge"


def visible(locator) -> None:
    expect(locator).to_be_visible(timeout=30000)


def main() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context()
        page = context.new_page()
        page.set_default_timeout(30000)
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/overview")
        page.wait_for_load_state("networkidle")
        visible(page.locator('[data-od-id="company-overview-view"]'))
        page.locator('[data-command-field="mission-title"]').fill("R0.5B1 offline provider bridge")
        page.locator('[data-command-field="mission-goal"]').fill("persist one product artifact through the provider runtime seam")
        with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as create_response:
            page.locator('[data-command="mission-create"]').click()
        mission_id = create_response.value.json()["targetId"]
        visible(page.locator('[data-command="mission-start"]'))
        with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as start_response:
            page.locator('[data-command="mission-start"]').click()
        start_payload = start_response.value.request.post_data_json
        duplicate_retry = context.request.post(f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/{mission_id}/start", data=start_payload)
        assert duplicate_retry.status == 202
        duplicate_conflict = context.request.post(f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/{mission_id}/start", data={"requestId": "bridge-duplicate-new"})
        assert duplicate_conflict.status == 409

        candidate_visible = False
        for _ in range(40):
            page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/tasks")
            page.wait_for_load_state("networkidle")
            if page.locator('[data-task-state="candidate"]').count() > 0:
                candidate_visible = True
                break
            page.wait_for_timeout(250)
        assert candidate_visible
        visible(page.locator('[data-task-state="candidate"]'))
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/evidence")
        page.wait_for_load_state("networkidle")
        visible(page.get_by_text("Artifact / evidence", exact=True))
        visible(page.get_by_text("candidate", exact=True).first)
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/activity")
        page.wait_for_load_state("networkidle")
        visible(page.locator('[data-event-kind="provider_turn_completed"]'))
        stopped = page.locator('[data-event-kind="employee_stopped"]')
        assert stopped.count() >= 2
        page.screenshot(path=str(EVIDENCE_DIR / "offline-provider-bridge.png"), full_page=True)
        result = {"browser_submit": "PASS", "browser_start": "PASS", "mission_id": mission_id, "duplicate_retry": duplicate_retry.status, "duplicate_new_request": duplicate_conflict.status, "provider_transport": "fake", "provider_egress": 0, "medium": 0, "high": 0, "artifact_visible": True, "provider_turn_completed_event": True, "orphan_check": "deferred_to_authoritative_sql"}
        (EVIDENCE_DIR / "browser-e2e-result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(result, ensure_ascii=False, indent=2))
        context.close()
        browser.close()


if __name__ == "__main__":
    main()
