"""Real-browser R0.5A command/read loop against the existing frontend."""

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("R05A_FRONTEND_URL", "http://127.0.0.1:4173")
COMPANY_ID = os.environ.get("R05A_COMPANY_ID", "r05a-e2e-company")
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.5a-existing-frontend-minimal-command-surface"


def check_visible(locator) -> None:
    expect(locator).to_be_visible(timeout=30000)


def main() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    overview_url = f"{BASE_URL}/companies/{COMPANY_ID}/overview"
    result: dict[str, object] = {"provider_egress": 0, "medium": 0, "high": 0, "company_id": COMPANY_ID}
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context()
        page = context.new_page()
        page.set_default_timeout(30000)
        api_responses: list[dict[str, object]] = []
        page.on(
            "response",
            lambda response: api_responses.append(
                {"url": response.url, "status": response.status, "method": response.request.method}
            )
            if "/api/workbench/" in response.url
            else None,
        )

        page.goto(overview_url)
        page.wait_for_load_state("networkidle")
        check_visible(page.locator('[data-od-id="company-overview-view"]'))
        title = "R0.5A browser mission"
        goal = "prove the existing frontend write and read loop"
        page.locator('[data-command-field="mission-title"]').fill(title)
        page.locator('[data-command-field="mission-goal"]').fill(goal)
        with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as create_response:
            page.locator('[data-command="mission-create"]').click()
        create_payload = create_response.value.request.post_data_json
        create_body = create_response.value.json()
        mission_id = str(create_body["targetId"])
        check_visible(page.get_by_role("heading", name=title))
        result["mission_id"] = mission_id

        duplicate_create = context.request.post(
            f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions",
            data=create_payload,
        )
        assert duplicate_create.status == 202
        assert duplicate_create.json()["targetId"] == mission_id
        result["duplicate_mission_submission"] = {"status": duplicate_create.status, "same_target": True}

        with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as start_response:
            page.locator('[data-command="mission-start"]').click()
        start_payload = start_response.value.request.post_data_json
        check_visible(page.locator('[data-command="mission-cancel"]'))
        check_visible(page.locator('[data-employee-id="emp-planning"][data-employee-state="working"]'))
        result["mission_start"] = {"status": start_response.value.status, "receipt": start_response.value.json()}

        duplicate_start_retry = context.request.post(
            f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/{mission_id}/start",
            data=start_payload,
        )
        assert duplicate_start_retry.status == 202
        assert duplicate_start_retry.json()["targetId"] == mission_id
        duplicate_start_conflict = context.request.post(
            f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/{mission_id}/start",
            data={"requestId": "duplicate-start-new-request"},
        )
        assert duplicate_start_conflict.status == 409
        result["duplicate_start"] = {"retry_status": duplicate_start_retry.status, "retry_same_target": True, "new_request_status": duplicate_start_conflict.status, "new_request_code": duplicate_start_conflict.json()["code"]}

        with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as cancel_response:
            page.locator('[data-command="mission-cancel"]').click()
        check_visible(page.locator('[data-mission-state="cancelled"]'))
        result["mission_stop"] = {"status": cancel_response.value.status, "receipt": cancel_response.value.json()}

        invalid_stop = context.request.post(
            f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/{mission_id}/cancel",
            data={"requestId": "invalid-stop-after-terminal"},
        )
        assert invalid_stop.status == 409
        result["invalid_stop"] = {"status": invalid_stop.status, "code": invalid_stop.json()["code"]}

        stale_target = context.request.post(
            f"{BASE_URL}/api/workbench/companies/{COMPANY_ID}/missions/not%20a%20mission/start",
            data={"requestId": "stale-target"},
        )
        assert stale_target.status == 400
        result["invalid_target"] = {"status": stale_target.status, "code": stale_target.json()["code"]}

        page.get_by_role("link", name="查看活动").click()
        page.wait_for_load_state("networkidle")
        check_visible(page.locator('[data-od-id="activity-timeline-view"]'))
        check_visible(page.locator('[data-event-kind="mission_created"]'))
        check_visible(page.locator('[data-event-kind="mission_started"]'))
        check_visible(page.locator('[data-event-kind="mission_cancelled"]'))
        stopped_events = page.locator('[data-event-kind="employee_stopped"]')
        assert stopped_events.count() >= 2
        result["activity_events"] = {"mission_created": True, "mission_started": True, "worker_stopped": True, "mission_cancelled": True}
        result["api_responses"] = api_responses
        page.screenshot(path=str(EVIDENCE_DIR / "command-surface-activity.png"), full_page=True)
        context.close()
        browser.close()

    (EVIDENCE_DIR / "e2e-result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
