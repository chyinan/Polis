"""Real-browser negative check for a command-surface backend outage."""

import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = os.environ.get("R05A_FRONTEND_URL", "http://127.0.0.1:4174")
COMPANY_ID = os.environ.get("R05A_COMPANY_ID", "r05a-e2e-company")
EVIDENCE_DIR = Path(__file__).resolve().parents[1] / "evidence" / "development" / "r0.5a-existing-frontend-minimal-command-surface"


def main() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page()
        page.set_default_timeout(30000)
        page.goto(f"{BASE_URL}/companies/{COMPANY_ID}/overview")
        page.wait_for_load_state("networkidle")
        alert = page.locator('[role="alert"]')
        expect(alert).to_be_visible(timeout=30000)
        message = alert.inner_text()
        assert "failed to read workbench API" in message
        result = {"backend_unavailable": True, "ui_error_visible": True, "message": message}
        (EVIDENCE_DIR / "backend-unavailable-result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(result, ensure_ascii=False, indent=2))
        browser.close()


if __name__ == "__main__":
    main()
