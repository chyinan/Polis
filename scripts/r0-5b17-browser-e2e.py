# pattern: Imperative Shell

from pathlib import Path
import json

from playwright.sync_api import sync_playwright


COMPANY_ID = "r0-5b17-browser"
FRONTEND_URL = "http://127.0.0.1:14182"
BACKEND_URL = "http://127.0.0.1:18092"
EVIDENCE = Path("evidence/development/r0.5b17-checkpoint-projection-hardening").resolve()


def wait_for_runtime(page, url: str) -> None:
    page.goto(url)
    page.wait_for_load_state("networkidle")


def main() -> None:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page()
        overview_response = page.request.get(f"{BACKEND_URL}/api/workbench/companies/{COMPANY_ID}/overview")
        assert overview_response.ok, overview_response.status
        overview = overview_response.json()
        checkpoint = next(item for item in overview["checkpoints"] if item["taskId"] == overview["artifacts"][0]["taskId"])
        artifact = overview["artifacts"][0]

        wait_for_runtime(page, f"{FRONTEND_URL}/companies/{COMPANY_ID}/overview")
        overview_root = page.locator('[data-od-id="company-overview-view"]')
        assert overview_root.is_visible()
        boundary = page.locator('[data-checkpoint-id]')
        assert boundary.count() == 1
        assert boundary.get_attribute("data-checkpoint-id") == checkpoint["checkpointId"]
        assert boundary.get_attribute("data-checkpoint-state") == "qualified"
        assert boundary.get_attribute("data-artifact-id") == artifact["artifactId"]
        browser_checkpoint_id = boundary.get_attribute("data-checkpoint-id")
        browser_artifact_id = boundary.get_attribute("data-artifact-id")
        page.screenshot(path=str(EVIDENCE / "browser-overview.png"), full_page=True)

        wait_for_runtime(page, f"{FRONTEND_URL}/companies/{COMPANY_ID}/tasks")
        assert page.locator('[data-task-state="candidate"]').count() == 1

        wait_for_runtime(page, f"{FRONTEND_URL}/companies/{COMPANY_ID}/activity")
        assert page.locator('[data-event-kind="checkpoint_saved"]').count() >= 1

        result = {
            "status": "PASSED",
            "company_id": COMPANY_ID,
            "authoritative_checkpoint_id": checkpoint["checkpointId"],
            "api_checkpoint_id": checkpoint["checkpointId"],
            "browser_checkpoint_id": browser_checkpoint_id,
            "authoritative_artifact_id": artifact["artifactId"],
            "browser_artifact_id": browser_artifact_id,
            "task_state": next(task["state"] for task in overview["tasks"] if task["taskId"] == checkpoint["taskId"]),
            "activity_checkpoint_event_visible": True,
        }
        (EVIDENCE / "browser-e2e-result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
        browser.close()


if __name__ == "__main__":
    main()
