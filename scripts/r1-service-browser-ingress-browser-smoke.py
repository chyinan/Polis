# pattern: Imperative Shell
from playwright.sync_api import sync_playwright


def redact_url(value: str) -> str:
    if "/_polis/open/" in value:
        return value.split("/_polis/open/", 1)[0] + "/_polis/open/<redacted>"
    return value.split("?", 1)[0]


def main() -> None:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page()
        errors: list[str] = []
        failed_requests: list[str] = []
        responses: list[str] = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.on("requestfailed", lambda request: failed_requests.append(f"{request.method} {request.url}: {request.failure}"))
        page.on("response", lambda response: responses.append(f"{response.status} {redact_url(response.url)} headers={response.all_headers() if response.status >= 400 else {}}"))

        page.goto("http://127.0.0.1:45169/", wait_until="networkidle")
        page.get_by_role("link", name="Open service").click()
        page.wait_for_load_state("networkidle")
        page.wait_for_load_state("networkidle")
        try:
            page.get_by_text("rendered; fetch-site=same-origin").wait_for(timeout=10_000)
        except Exception:
            print(f"diagnostic_url={redact_url(page.url)}")
            print(f"diagnostic_title={page.title()!r}")
            print(f"diagnostic_body={page.locator('body').inner_text()[:2000]!r}")
            print(f"diagnostic_page_errors={errors!r}")
            print(f"diagnostic_failed_requests={failed_requests!r}")
            print(f"diagnostic_responses={responses!r}")
            raise

        if page.title() != "Polis browser ingress fixture":
            raise AssertionError(f"unexpected browser page title: {page.title()!r}")
        if "_polis/open/" in page.url:
            raise AssertionError("the one-time browser ticket remained in the final URL")
        if errors:
            raise AssertionError(f"browser page errors: {errors}")

        print("browser_ingress=PASS rendered_page=PASS fetch_site=same-origin ticket_removed=PASS")
        browser.close()


if __name__ == "__main__":
    main()
