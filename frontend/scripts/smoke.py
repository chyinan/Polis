# pattern: Imperative Shell

import json
import re
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


BASE_URL = 'http://127.0.0.1:4173'
COMPANY_ID = 'r03a-t2-company-1789221294871371900'
EVIDENCE_DIR = Path(__file__).resolve().parents[2] / 'evidence' / 'development' / 'r1-workbench-skeleton'


def run_smoke_check() -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        page = browser.new_page(viewport={'width': 1440, 'height': 1000}, locale='zh-CN')
        page_errors = []
        request_failures = []
        page.on('pageerror', lambda error: page_errors.append(f'pageerror: {error}'))
        page.on('console', lambda message: page_errors.append(f'console.{message.type}: {message.text}') if message.type == 'error' else None)
        page.on('requestfailed', lambda request: request_failures.append(f'{request.url}: {request.failure}'))
        page.goto(BASE_URL, wait_until='networkidle')
        try:
            expect(page.get_by_role('heading', name='公司总览')).to_be_visible()
        except AssertionError:
            print(json.dumps({'diagnostics': page_errors, 'html': page.content()}, ensure_ascii=False, indent=2))
            raise
        expect(page.get_by_text('snapshot simulated', exact=True)).to_be_visible()
        expect(page.get_by_text('Polis 协作研发公司', exact=True).first).to_be_visible()
        if page.locator('html').get_attribute('data-theme') != 'dark':
            raise AssertionError('dark theme is not the default presentation')
        underlined_links = page.locator('a').evaluate_all("elements => elements.filter(element => getComputedStyle(element).textDecorationLine.includes('underline')).map(element => element.textContent.trim()).filter(Boolean)")
        if underlined_links:
            raise AssertionError(f'links unexpectedly underlined: {underlined_links}')
        page.screenshot(path=str(EVIDENCE_DIR / 'overview-desktop.png'), full_page=True)

        page.get_by_role('link', name='活动', exact=True).click()
        page.wait_for_load_state('networkidle')
        expect(page).to_have_url(re.compile(rf'/companies/{COMPANY_ID}/activity$'))
        expect(page.get_by_role('heading', name='活动时间线')).to_be_visible()
        expect(page.get_by_text('独立集成验收通过', exact=True)).to_be_visible()
        page.get_by_role('button', name='展开证据与调试信息').first.click()
        expect(page.get_by_text('证据引用', exact=True)).to_be_visible()
        expect(page.get_by_text('调试元数据', exact=True)).to_be_visible()
        theme_button = page.get_by_role('complementary').get_by_role('button', name=re.compile('切换为'))
        theme_button.click()
        theme_button.click()
        theme_after_toggle = page.locator('html').get_attribute('data-theme')
        if theme_after_toggle != 'dark':
            raise AssertionError(f'unexpected theme after toggle: {theme_after_toggle}')
        page.screenshot(path=str(EVIDENCE_DIR / 'activity-dark-or-light.png'), full_page=True)

        route_headings = {
            'mission': '使命',
            'tasks': '任务',
            'employees': '员工',
            'org-chart': '组织关系',
            'resources': '资源',
            'evidence': '证据',
            'decisions': '待决事项',
            'feedback': '反馈待办',
            'notifications': '接管通知',
            'settings': '设置',
        }
        for route, heading in route_headings.items():
            page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/{route}', wait_until='networkidle')
            expect(page.get_by_role('heading', name=heading).first).to_be_visible()

        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/employees', wait_until='networkidle')
        page.get_by_role('tab', name='职责').click()
        expect(page.get_by_role('heading', name='责任边界')).to_be_visible()
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/tasks', wait_until='networkidle')
        page.get_by_role('tab', name='作业').click()
        expect(page.get_by_role('heading', name='运行作业')).to_be_visible()
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/mission', wait_until='networkidle')
        page.get_by_role('tab', name='资料').click()
        expect(page.get_by_role('heading', name='资料边界')).to_be_visible()
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/settings', wait_until='networkidle')
        page.get_by_role('tab', name='能力').click()
        expect(page.get_by_role('heading', name='能力目录')).to_be_visible()

        group_route_headings = {
            'overview': '公司总览',
            'new-company': '新建公司',
            'resources': '集团资源',
            'settings': '集团设置',
        }
        for route, heading in group_route_headings.items():
            page.goto(f'{BASE_URL}/group/{route}', wait_until='networkidle')
            expect(page.get_by_role('heading', name=heading).first).to_be_visible()

        page.goto(f'{BASE_URL}/group/new-company', wait_until='networkidle')
        expect(page.get_by_role('button', name='上一步')).to_have_attribute('disabled', '')
        page.get_by_role('button', name='下一步').click()
        expect(page.get_by_role('heading', name='固定团队')).to_be_visible()

        page.set_viewport_size({'width': 1224, 'height': 900})
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/overview', wait_until='networkidle')
        has_medium_desktop_overflow = page.locator('body').evaluate('(element) => element.scrollWidth > element.clientWidth')
        if has_medium_desktop_overflow:
            raise AssertionError('1224px desktop viewport has horizontal overflow')

        page.set_viewport_size({'width': 390, 'height': 844})
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/overview', wait_until='networkidle')
        expect(page.get_by_role('heading', name='公司总览')).to_be_visible()
        page.screenshot(path=str(EVIDENCE_DIR / 'overview-mobile-390.png'), full_page=True)
        has_horizontal_overflow = page.locator('body').evaluate('(element) => element.scrollWidth > element.clientWidth')
        if has_horizontal_overflow:
            raise AssertionError('mobile viewport has horizontal overflow')

        page.goto(f'{BASE_URL}/group/overview', wait_until='networkidle')
        has_group_mobile_overflow = page.locator('body').evaluate('(element) => element.scrollWidth > element.clientWidth')
        if has_group_mobile_overflow:
            raise AssertionError('group overview mobile viewport has horizontal overflow')

        page.set_viewport_size({'width': 320, 'height': 844})
        page.goto(f'{BASE_URL}/companies/{COMPANY_ID}/overview', wait_until='networkidle')
        page.locator('html').evaluate("element => element.style.fontSize = '28px'")
        has_small_viewport_overflow = page.locator('body').evaluate('(element) => element.scrollWidth > element.clientWidth')
        if has_small_viewport_overflow:
            raise AssertionError('320px viewport with 200% text has horizontal overflow')
        if page_errors or request_failures:
            raise AssertionError(f'page/console/request failures observed: {page_errors + request_failures}')

        print(json.dumps({
            'overview': 'visible',
            'activity': 'visible',
            'evidence_disclosure': 'visible',
            'theme_after_toggle': theme_after_toggle,
            'medium_desktop_1224_horizontal_overflow': has_medium_desktop_overflow,
            'mobile_390_horizontal_overflow': has_horizontal_overflow,
            'group_mobile_390_horizontal_overflow': has_group_mobile_overflow,
            'mobile_320_200_percent_text_horizontal_overflow': has_small_viewport_overflow,
            'request_failures': request_failures,
            'screenshots': [str(path) for path in sorted(EVIDENCE_DIR.glob('*.png'))],
        }, ensure_ascii=False, indent=2))
        browser.close()


if __name__ == '__main__':
    run_smoke_check()
