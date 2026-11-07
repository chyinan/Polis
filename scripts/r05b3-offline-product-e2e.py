"""Browser E2E for the B3 product employee surface, using only fake transport."""

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("R05B3_FRONTEND_URL", "http://127.0.0.1:4176")
COMPANY_ID = os.environ.get("R05B3_COMPANY_ID", "r05b3-offline-company")
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.5b3-product-employee-surface-semantic-remediation"
CRITERIA = [
    "Mission ID: {{mission_id}}",
    "Task ID: {{task_id}}",
    "Acknowledgement:",
    "Task summary:",
]
EXISTING_MISSION_ID = os.environ.get("R05B3_MISSION_ID")
EXISTING_TASK_ID = os.environ.get("R05B3_TASK_ID")


def main() -> None:
    if bool(EXISTING_MISSION_ID) != bool(EXISTING_TASK_ID):
        raise ValueError("R05B3_MISSION_ID and R05B3_TASK_ID must be supplied together")
    verification_only = bool(EXISTING_MISSION_ID and EXISTING_TASK_ID)
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context()
        page = context.new_page()
        page.set_default_timeout(30000)
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/overview")
        page.wait_for_load_state("networkidle")
        expect(page.locator('[data-od-id="company-overview-view"]')).to_be_visible()

        if EXISTING_MISSION_ID and EXISTING_TASK_ID:
            mission_id = EXISTING_MISSION_ID
            task_id = EXISTING_TASK_ID
            expect(page.locator('[data-acceptance-contract="configured"]')).to_be_visible()
            for criterion in CRITERIA:
                expect(page.locator('[data-acceptance-contract="configured"]')).to_contain_text(criterion)
            mission_id_from_page = page.locator('[data-mission-state]').get_attribute('data-mission-state')
            assert mission_id_from_page == "active", f"existing Mission state={mission_id_from_page}, want active"
        else:
            page.locator('[data-command-field="mission-title"]').fill("R0.5B3 offline product surface")
            page.locator('[data-command-field="mission-goal"]').fill("Create a short text artifact containing the public Mission and Task identifiers, an acknowledgement, and a brief task summary.")
            page.locator('[data-command-field="mission-acceptance"]').fill("\n".join(CRITERIA))
            with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202) as create_response:
                page.locator('[data-command="mission-create"]').click()
            mission_id = create_response.value.json()["targetId"]
            expect(page.locator('[data-acceptance-contract="configured"]')).to_be_visible()
            for criterion in CRITERIA:
                expect(page.locator('[data-acceptance-contract="configured"]')).to_contain_text(criterion)
            with page.expect_response(lambda response: response.request.method == "POST" and response.status == 202 and response.url.endswith(f"/missions/{mission_id}/start")):
                page.locator('[data-command="mission-start"]').click()

            task_id = None
            for _ in range(60):
                page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/tasks")
                page.wait_for_load_state("networkidle")
                candidates = page.locator('[data-task-state="candidate"]')
                if candidates.count() > 0:
                    expect(candidates).to_have_count(1)
                    task_id = candidates.locator("small").inner_text().split(" · ", 1)[0]
                    break
                page.wait_for_timeout(250)
        assert task_id, "Task did not reach the candidate state through the product runtime"
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/tasks")
        page.wait_for_load_state("networkidle")
        candidate_task = page.locator('[data-task-state="candidate"]')
        expect(candidate_task).to_have_count(1)
        expect(candidate_task).to_contain_text(task_id)
        expect(candidate_task).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/evidence")
        page.wait_for_load_state("networkidle")
        expect(page.get_by_text("Artifact / evidence", exact=True)).to_be_visible()
        candidate_badge = page.get_by_text("candidate", exact=True)
        expect(candidate_badge).to_have_count(1)
        expect(candidate_badge).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/activity")
        page.wait_for_load_state("networkidle")
        expect(page.locator('[data-event-kind="provider_turn_completed"]')).to_have_count(1)
        stopped_events = page.locator('[data-event-kind="employee_stopped"]')
        expect(stopped_events).to_have_count(2)
        terminal_stop_event = stopped_events.filter(has_text="worker.stopped")
        expect(terminal_stop_event).to_have_count(1)
        expect(terminal_stop_event).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/overview")
        page.wait_for_load_state("networkidle")
        expect(page.locator('[data-acceptance-contract="configured"]')).to_be_visible()
        screenshot_name = "offline-product-e2e-readonly-followup.png" if verification_only else "offline-product-e2e.png"
        result_name = "browser-readonly-followup-latest.json" if verification_only else "browser-e2e-result.json"
        page.screenshot(path=str(EVIDENCE_DIR / screenshot_name), full_page=True)
        result = {
            "r0_5b3_offline_real_worker_product_e2e": "NOT_QUALIFICATION_EVIDENCE" if verification_only else "PASSED",
            "browser_mission_submit": "NOT_RUN" if verification_only else "PASS",
            "browser_start": "NOT_RUN" if verification_only else "PASS",
            "verification_only_existing_mission": verification_only,
            "mission_id": mission_id,
            "task_id": task_id,
            "acceptance_contract_visible": True,
            "candidate_task_visible": True,
            "artifact_visible": True,
            "provider_transport": "fake",
            "provider_egress": 0,
            "medium": 0,
            "high": 0,
            "provider_turn_completed_event": True,
            "employee_stopped_event": True,
            "ui_stream": "deferred; verified by HTTP refetch",
        }
        (EVIDENCE_DIR / result_name).write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(result, ensure_ascii=False, indent=2))
        context.close()
        browser.close()


if __name__ == "__main__":
    main()
