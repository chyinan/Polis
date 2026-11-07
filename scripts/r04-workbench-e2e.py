from __future__ import annotations

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("POLIS_WORKBENCH_FRONTEND_URL", "http://127.0.0.1:4173")
COMPANY_ID = "r04-workbench-company"
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.4-existing-frontend-real-backend-integration"


def run() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    api_responses: list[dict[str, object]] = []
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page(viewport={"width": 1440, "height": 1000}, locale="zh-CN")
        page.on(
            "response",
            lambda response: api_responses.append(
                {"url": response.url, "status": response.status}
            )
            if "/api/workbench/" in response.url
            else None,
        )
        page.goto(BASE_URL, wait_until="networkidle")
        expect(page.get_by_role("heading", name="公司总览")).to_be_visible()
        expect(page.get_by_text("snapshot real", exact=True)).to_be_visible()
        expect(page.get_by_text(COMPANY_ID, exact=True).first).to_be_visible()
        expect(page.get_by_text("emp-backend", exact=True).first).to_be_visible()
        expect(page.get_by_text("r04-backend-task", exact=True).first).to_be_visible()
        expect(page.get_by_text("r04-checkpoint-1", exact=False).first).to_be_visible()
        expect(page.get_by_text("r04-artifact-1", exact=False).first).to_be_visible()
        page.screenshot(path=str(EVIDENCE_DIR / "company-overview-real.png"), full_page=True)

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/mission", wait_until="networkidle")
        page.get_by_role("tab", name="指导").click()
        expect(page.get_by_text("Consume the accepted cursor and limit contract.", exact=True)).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/employees", wait_until="networkidle")
        expect(page.get_by_role("heading", name="员工")).to_be_visible()
        expect(page.get_by_text("Backend Employee", exact=True)).to_be_visible()
        expect(page.get_by_text("WorkerSession", exact=True).first).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/tasks", wait_until="networkidle")
        expect(page.get_by_role("heading", name="任务")).to_be_visible()
        expect(page.get_by_text("r04-frontend-task", exact=False).first).to_be_visible()

        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/activity", wait_until="networkidle")
        expect(page.get_by_role("heading", name="活动时间线")).to_be_visible()
        expect(page.get_by_text("seq 8", exact=True).first).to_be_visible()
        expect(page.get_by_text("artifact.submit", exact=True).first).to_be_visible()
        expect(page.get_by_text("已到当前快照末尾", exact=True)).to_be_visible()
        page.screenshot(path=str(EVIDENCE_DIR / "activity-real-terminal-page.png"), full_page=True)
        browser.close()

    result = {
        "frontend": "existing frontend reused",
        "company_id": COMPANY_ID,
        "api_responses": api_responses,
        "screenshots": [
            str(EVIDENCE_DIR / "company-overview-real.png"),
            str(EVIDENCE_DIR / "activity-real-terminal-page.png"),
        ],
    }
    (EVIDENCE_DIR / "e2e-result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    run()
