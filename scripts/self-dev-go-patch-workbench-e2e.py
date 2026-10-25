#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import re
import time
from pathlib import Path

from playwright.sync_api import expect, sync_playwright


STATE_PATH = Path(os.environ["POLIS_SELF_DEV_E2E_STATE"])
UI_BASE = "http://127.0.0.1:4173"
EVIDENCE = STATE_PATH.parent
COMPLETE_PATH = EVIDENCE / "browser-complete.json"


def run() -> dict[str, object]:
    state = json.loads(STATE_PATH.read_text(encoding="utf-8"))
    STATE_PATH.unlink()
    company_id = state["companyId"]
    task_id = state["taskId"]
    api_base = f"{UI_BASE}/api/workbench"
    browser_errors: list[str] = []
    unexpected_http_errors: list[dict[str, object]] = []
    expected_owner_status_denials = [0]
    job_id = ""
    artifact_id = ""
    disposition = ""
    screenshots: list[str] = []

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 1600, "height": 1200}, locale="zh-CN")
        page = context.new_page()
        page.on("pageerror", lambda error: browser_errors.append(str(error)))
        def record_http_response(response: object) -> None:
            status = getattr(response, "status")
            url = getattr(response, "url")
            if status < 400:
                return
            if status == 403 and url.split("?", 1)[0].endswith("/api/installation/owner/status"):
                expected_owner_status_denials[0] += 1
                return
            if "/api/" in url:
                unexpected_http_errors.append({"url": url, "status": status})

        page.on("response", record_http_response)

        page.goto(f"{UI_BASE}/group/installation", wait_until="networkidle")
        expect(page.get_by_role("heading", name="安装级账户", exact=True)).to_be_visible(timeout=20_000)
        page.get_by_label("所有者密码").fill(state["ownerPassword"])
        page.get_by_role("button", name="登录", exact=True).click()
        expect(page.get_by_text("安装所有者已登录", exact=True)).to_be_visible(timeout=20_000)

        page.goto(f"{UI_BASE}/companies/{company_id}/tasks", wait_until="networkidle")
        expect(page.get_by_role("heading", name="任务", exact=True)).to_be_visible(timeout=20_000)
        task_row = page.locator("button[data-task-state]").filter(has_text=task_id)
        expect(task_row).to_be_visible(timeout=15_000)
        task_row.click()
        page.get_by_role("tab", name="作业", exact=True).click()
        expect(page.get_by_role("heading", name="固定 Go 测试请求", exact=True)).to_be_visible(timeout=15_000)
        page.get_by_label("base snapshot ID").fill(state["baseSnapshotArtifactId"])
        page.get_by_label("candidate snapshot ID").fill(state["candidateSnapshotArtifactId"])
        execute_checkbox = page.get_by_label(re.compile("显式请求执行候选"))
        if not execute_checkbox.is_checked():
            execute_checkbox.check()

        start_url = f"/api/workbench/companies/{company_id}/tasks/{task_id}/jobs"
        with page.expect_response(lambda response: response.url.endswith(start_url) and response.request.method == "POST", timeout=30_000) as start_info:
            page.get_by_role("button", name="提交受限 Go 测试", exact=True).click()
        start_response = start_info.value
        if start_response.status != 202:
            raise RuntimeError(f"real Workbench Go JobRun start returned {start_response.status}: {start_response.text()}")
        start_payload = start_response.request.post_data_json
        if not isinstance(start_payload, dict) or start_payload.get("executeGoTest") is not True:
            raise RuntimeError(f"Workbench did not send the explicit execution opt-in: {start_payload!r}")
        start_receipt = start_response.json()
        job_id = start_receipt.get("jobId", "")
        if not isinstance(job_id, str) or not job_id.startswith("job-"):
            raise RuntimeError(f"Workbench start response omitted JobRun identity: {start_receipt!r}")

        deadline = time.monotonic() + 240
        terminal_job: dict[str, object] | None = None
        while time.monotonic() < deadline:
            jobs_response = context.request.get(f"{api_base}/companies/{company_id}/tasks/{task_id}/jobs", timeout=15_000)
            if jobs_response.status != 200:
                raise RuntimeError(f"Workbench JobRun read returned {jobs_response.status}: {jobs_response.text()}")
            jobs = jobs_response.json()
            if not isinstance(jobs, list):
                raise RuntimeError(f"Workbench JobRun projection is not an array: {jobs!r}")
            terminal_job = next((item for item in jobs if isinstance(item, dict) and item.get("jobId") == job_id), None)
            if terminal_job is not None and terminal_job.get("state") in {"exited", "failed", "cancelled", "outcome_unknown"}:
                break
            time.sleep(1)
        if terminal_job is None or terminal_job.get("state") != "exited":
            raise RuntimeError(f"real Go JobRun did not reach an exited result before deadline: {terminal_job!r}")
        go_test = terminal_job.get("goTest")
        if not isinstance(go_test, dict) or go_test.get("outcome") != "passed":
            raise RuntimeError(f"real Go JobRun has no passing result: {go_test!r}")
        result = go_test.get("result")
        if not isinstance(result, dict) or result.get("executionKind") != "real" or result.get("qualificationEligible") is not True or result.get("cleanupConfirmed") is not True:
            raise RuntimeError(f"stored Go receipt is not a qualified real execution: {result!r}")

        page.reload(wait_until="networkidle")
        expect(page.get_by_role("heading", name="任务", exact=True)).to_be_visible(timeout=20_000)
        page.locator("button[data-task-state]").filter(has_text=task_id).click()
        page.get_by_role("tab", name="作业", exact=True).click()
        publish_button = page.get_by_role("button", name="生成正式 Patch 交付", exact=True)
        expect(publish_button).to_be_visible(timeout=20_000)
        patch_url = f"/api/workbench/companies/{company_id}/tasks/{task_id}/go-patch"
        with page.expect_response(lambda response: response.url.endswith(patch_url) and response.request.method == "POST", timeout=30_000) as publish_info:
            publish_button.click()
        publish_response = publish_info.value
        if publish_response.status != 202:
            raise RuntimeError(f"formal Patch publication returned {publish_response.status}: {publish_response.text()}")
        delivery_receipt = publish_response.json()
        artifact_id = delivery_receipt.get("artifact_id", "")
        if not isinstance(artifact_id, str) or artifact_id == "" or delivery_receipt.get("delivery_state") != "committed":
            raise RuntimeError(f"formal Patch delivery response mismatch: {delivery_receipt!r}")

        page.reload(wait_until="networkidle")
        expect(page.get_by_role("heading", name="任务", exact=True)).to_be_visible(timeout=20_000)
        page.locator("button[data-task-state]").filter(has_text=task_id).click()
        page.get_by_role("tab", name="测试与预览", exact=True).click()
        patch_preview = page.get_by_role("region", name="Go Patch 人工审核")
        expect(patch_preview).to_be_visible(timeout=20_000)
        expect(patch_preview.get_by_text(state["candidateMarker"], exact=False)).to_be_visible(timeout=10_000)
        expect(patch_preview.get_by_text("只读差异", exact=False)).to_be_visible()
        diff_path = EVIDENCE / "go-patch-workbench-diff.png"
        page.screenshot(path=str(diff_path), full_page=True)
        screenshots.append(diff_path.name)

        completion_form = page.get_by_test_id("durable-delivery-completion-form")
        expect(completion_form).to_be_visible(timeout=20_000)
        evidence_fields = (
            "sourceInputs", "environmentBuild", "runInstructions", "limitations", "licenseSource"
        )
        evidence_labels = {
            "sourceInputs": "源输入证据",
            "environmentBuild": "环境 / 构建证据",
            "runInstructions": "运行说明证据",
            "limitations": "限制说明证据",
            "licenseSource": "许可证 / 来源证据",
        }
        for key in evidence_fields:
            evidence = state[key]
            label = evidence_labels[key]
            completion_form.get_by_label(f"{label}引用").fill(evidence["reference"])
            completion_form.get_by_label(f"{label}摘要").fill(evidence["digest"])
            completion_form.get_by_label(f"{label}说明").fill(evidence["detail"])
        completion_url = f"/api/workbench/companies/{company_id}/artifacts/{artifact_id}/delivery/complete"
        with page.expect_response(lambda response: response.url.endswith(completion_url) and response.request.method == "POST", timeout=30_000) as completion_info:
            page.get_by_test_id("durable-delivery-completion-submit").click()
        completion_response = completion_info.value
        if completion_response.status != 202:
            raise RuntimeError(f"durable DeliveryManifest completion returned {completion_response.status}: {completion_response.text()}")

        disposition_form = page.get_by_test_id("durable-user-disposition-form")
        expect(disposition_form).to_be_visible(timeout=20_000)
        disposition_reason = "Disposable synthetic E2E fixture: exercise the owner acceptance UI path only; no source change is applied, merged, or deployed, and this is not production acceptance."
        disposition_form.get_by_label("决定").select_option("accepted")
        disposition_form.get_by_label("理由").fill(disposition_reason)
        disposition_url = f"/api/workbench/companies/{company_id}/artifacts/{artifact_id}/delivery/disposition"
        with page.expect_response(lambda response: response.url.endswith(disposition_url) and response.request.method == "POST", timeout=30_000) as disposition_info:
            page.get_by_test_id("durable-user-disposition-submit").click()
        disposition_response = disposition_info.value
        if disposition_response.status != 202:
            raise RuntimeError(f"owner disposition returned {disposition_response.status}: {disposition_response.text()}")
        disposition_receipt = disposition_response.json()
        disposition = disposition_receipt.get("state", "")
        if disposition != "accepted" or disposition_receipt.get("actor") != "installation-owner":
            raise RuntimeError(f"owner disposition response mismatch: {disposition_receipt!r}")

        delivery_response = context.request.get(f"{api_base}/companies/{company_id}/artifacts/{artifact_id}/delivery", timeout=15_000)
        if delivery_response.status != 200:
            raise RuntimeError(f"durable DeliveryManifest readback returned {delivery_response.status}: {delivery_response.text()}")
        delivery = delivery_response.json()
        if delivery.get("manifest", {}).get("state") != "ready" or delivery.get("userDisposition", {}).get("state") != "accepted" or delivery.get("userDisposition", {}).get("actor") != "installation-owner":
            raise RuntimeError(f"durable owner disposition readback mismatch: {delivery!r}")
        expect(page.get_by_text("已接受", exact=True)).to_be_visible(timeout=15_000)
        final_path = EVIDENCE / "go-patch-workbench-disposition.png"
        page.screenshot(path=str(final_path), full_page=True)
        screenshots.append(final_path.name)
        if browser_errors or unexpected_http_errors:
            raise RuntimeError(f"browser errors={browser_errors!r}; unexpected HTTP failures={unexpected_http_errors!r}")

        context.close()
        browser.close()
        return {
            "result": "passed",
            "companyId": company_id,
            "taskId": task_id,
            "jobRunId": job_id,
            "artifactId": artifact_id,
            "jobState": terminal_job["state"],
            "outcome": go_test["outcome"],
            "executionKind": result["executionKind"],
            "qualificationEligible": result["qualificationEligible"],
            "cleanupConfirmed": result["cleanupConfirmed"],
            "disposition": disposition,
            "diffVisible": True,
            "noAutoApply": True,
            "externalAction": "not_run",
            "providerEgress": 0,
            "realModelWorkerStarted": 0,
            "automaticMerge": "NO",
            "automaticDeploy": "NO",
            "screenshots": screenshots,
            "browserErrors": browser_errors,
            "unexpectedHTTPFailures": unexpected_http_errors,
            "expectedLocalOwnerStatusDenials": expected_owner_status_denials[0],
        }


if __name__ == "__main__":
    result: dict[str, object]
    try:
        result = run()
    except Exception as error:
        result = {
            "result": "failed",
            "error": f"{type(error).__name__}: {error}",
            "providerEgress": 0,
            "realModelWorkerStarted": 0,
            "automaticMerge": "NO",
            "automaticDeploy": "NO",
        }
        (EVIDENCE / "go-patch-workbench-browser-failure.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        COMPLETE_PATH.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
        raise
    COMPLETE_PATH.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=2))
