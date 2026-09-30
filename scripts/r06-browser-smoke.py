from __future__ import annotations

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("POLIS_WORKBENCH_FRONTEND_URL", "http://127.0.0.1:4173")
COMPANY_ID = "browser-company"
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.6-browser-dogfood"


def run() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    responses: list[dict[str, object]] = []
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page(viewport={"width": 1440, "height": 1000}, locale="zh-CN")
        def record_response(response):
            if "/api/workbench/" not in response.url:
                return
            item = {"url": response.url, "status": response.status}
            if response.status >= 400:
                try:
                    item["body"] = response.text()
                except Exception:
                    item["body"] = "<unavailable>"
            responses.append(item)
        page.on("response", record_response)

        page.goto(BASE_URL, wait_until="domcontentloaded")
        expect(page.get_by_role("heading", name="公司总览")).to_be_visible(timeout=15_000)
        page.wait_for_timeout(2_000)
        expect(page.get_by_text(COMPANY_ID, exact=True).first).to_be_visible()
        page.screenshot(path=str(EVIDENCE_DIR / "overview.png"), full_page=True)

        for path, heading in [
            ("/group/overview", "公司总览"),
            (f"/companies/{COMPANY_ID}/settings", "设置"),
            (f"/companies/{COMPANY_ID}/operations", "运行与资源"),
            (f"/companies/{COMPANY_ID}/collaboration", "协作 Inbox"),
            (f"/companies/{COMPANY_ID}/notifications", "接管通知"),
            (f"/companies/{COMPANY_ID}/mission", "使命"),
        ]:
            page.goto(BASE_URL + path, wait_until="domcontentloaded")
            expect(page.get_by_role("heading", name=heading)).to_be_visible(timeout=15_000)
        page.screenshot(path=str(EVIDENCE_DIR / "mission.png"), full_page=True)
        browser.close()

    failures = [item for item in responses if int(item["status"]) >= 400]
    result = {
        "company_id": COMPANY_ID,
        "frontend": "existing frontend reused",
        "pages": ["overview", "group/overview", "settings", "operations", "collaboration", "notifications", "mission"],
        "api_response_count": len(responses),
        "api_failures": failures,
        "screenshots": [str(EVIDENCE_DIR / "overview.png"), str(EVIDENCE_DIR / "mission.png")],
    }
    (EVIDENCE_DIR / "result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    if failures:
        raise SystemExit(json.dumps(result, indent=2))
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    run()
