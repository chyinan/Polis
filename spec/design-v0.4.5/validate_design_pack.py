#!/usr/bin/env python3
"""Validate this design pack without SQL, network, runtime execution, or paid models.

Dependencies: PyYAML, jsonschema, beautifulsoup4.
Run from any directory: python validate_design_pack.py [--json checks.json]
These checks validate document/fixture consistency, NOT architecture correctness.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

try:
    import yaml
    from bs4 import BeautifulSoup
    from jsonschema import Draft202012Validator
except ImportError as exc:
    raise SystemExit('Missing dependency. Install PyYAML, jsonschema, beautifulsoup4. ' + str(exc))

ROOT = Path(__file__).resolve().parent

def validate() -> dict:
    checks: list[dict] = []
    def check(name: str, condition: bool, detail: str = '') -> None:
        checks.append({'name': name, 'status': 'pass' if condition else 'fail', 'detail': detail})
    def read_json(path: str) -> dict:
        return json.loads((ROOT/path).read_text(encoding='utf-8'))
    md = (ROOT/'ARCHITECTURE.md').read_text(encoding='utf-8')
    html = (ROOT/'ARCHITECTURE.html').read_text(encoding='utf-8')
    soup = BeautifulSoup(html, 'html.parser')
    doc_ids = re.findall(r'<a id="([^"]+)"', md)
    html_ids = [t['id'] for t in soup.find_all(id=True)]
    check('main_document_has_46_chapters', re.findall(r'^## (\d\d)\. ', md, re.M) == [f'{i:02d}' for i in range(1,47)])
    check('main_anchor_ids_unique', len(doc_ids) == len(set(doc_ids)))
    check('html_anchor_ids_unique', len(html_ids) == len(set(html_ids)))
    check('html_contains_all_document_anchors', set(doc_ids) <= set(html_ids))
    check('html_contains_full_manuscript', len(soup.find('article').get_text()) > 50000,
          'Guards against accidentally rendering only the navigation/header.')
    broken_html = [a['href'] for a in soup.find_all('a', href=True)
                   if a['href'].startswith('#') and a['href'][1:] not in html_ids]
    check('html_internal_links', not broken_html, str(broken_html))
    external_assets = [t.get('src') or t.get('href') for t in soup.find_all(['script','link','img','iframe'])
                       if (t.get('src') or (t.get('href') if t.name=='link' else None))]
    check('offline_no_external_render_dependencies', not external_assets, str(external_assets))
    # Check local markdown links, including references in standalone chapters.
    bad_links = []
    for f in sorted(ROOT.rglob('*.md')):
        text = f.read_text(encoding='utf-8')
        for href in re.findall(r'\]\(([^)\s]+)\)', text):
            parts = urlsplit(href)
            if parts.scheme or parts.netloc:
                continue
            dest = (f.parent/unquote(parts.path)).resolve() if parts.path else f
            if not dest.exists():
                bad_links.append(f'{f.relative_to(ROOT)}: {href} (missing file)')
            elif parts.fragment and dest.suffix in ('.md','.html'):
                target_text = dest.read_text(encoding='utf-8')
                target_ids = re.findall(r'(?:id|name)="([^"]+)"', target_text)
                if unquote(parts.fragment) not in target_ids:
                    bad_links.append(f'{f.relative_to(ROOT)}: {href} (missing anchor)')
        fences = re.findall(r'^```[^\n]*$', text, re.M)
        check(f'code_fences:{f.relative_to(ROOT)}', len(fences) % 2 == 0)
    check('markdown_local_links', not bad_links, json.dumps(bad_links, ensure_ascii=False))
    # Parse all structured files. DOCUMENT_CHECKS may be a prior result.
    parsed = []
    for f in sorted(ROOT.rglob('*')):
        if f.suffix not in ('.json','.yaml','.yml'):
            continue
        try:
            if f.suffix == '.json':
                json.loads(f.read_text(encoding='utf-8'))
            else:
                yaml.safe_load(f.read_text(encoding='utf-8'))
            parsed.append(str(f.relative_to(ROOT)))
        except (ValueError, yaml.YAMLError) as exc:
            check(f'parse:{f.relative_to(ROOT)}', False, str(exc))
    check('structured_files_parsed', len(parsed) >= 14, f'{len(parsed)} files parsed')
    company = yaml.safe_load((ROOT/'examples/software_company.draft.yaml').read_text(encoding='utf-8'))
    pairs = [('company-design.schema.json',company),
             ('handover-design.schema.json',read_json('examples/handover_bundle.example.json')),
             ('event-design.schema.json',read_json('examples/event_envelope.example.json')),
             ('workbench-employee.schema.json',read_json('examples/workbench_employee.example.json')),
             ('ui-operation.schema.json',read_json('examples/ui_operation.example.json')),
             ('qq-notification-design.schema.json',yaml.safe_load((ROOT/'examples/qq_notification.draft.yaml').read_text())),
             ('intervention-design.schema.json',read_json('examples/human_intervention.example.json')),
             ('employee-ops-design.schema.json',read_json('examples/employee_ops.example.json'))]
    for schema_name, obj in pairs:
        schema = read_json('schemas/'+schema_name)
        try:
            Draft202012Validator.check_schema(schema)
            errors = list(Draft202012Validator(schema).iter_errors(obj))
            check('schema:'+schema_name, not errors, '; '.join(e.message for e in errors))
        except Exception as exc:
            check('schema:'+schema_name, False, str(exc))
    check('company_execution_disabled', company['execution_enabled'] is False and company['budget']['pay_enabled'] is False)
    check('company_budget_zero', company['budget']['limit_microunits']=='0' and company['budget']['closing_reserve_microunits']=='0')
    check('company_fixed_roles', company['dynamic_roles_enabled'] is False and company['capacity']['employee_active_session_limit']==1)
    employees = company['employees']
    check('employee_ids_unique', len({e['employee_id'] for e in employees})==len(employees))
    check('independent_reviewer_exists', any(e['role']=='independent_reviewer' for e in employees))
    check('no_paid_idle_polling', company['sleep']['idle_llm_polling'] is False and company['sleep']['pause_overrides_wake'] is True)
    check('single_postgresql_database', company['state_store']['engine']=='postgresql' and company['state_store']['secondary_database'] is None)
    mission=yaml.safe_load((ROOT/'examples/content_mission.draft.yaml').read_text(encoding='utf-8'))
    check('content_simulated_and_disabled', mission['execution_enabled'] is False and mission['constraints']['publish_environment']=='simulator_only')
    versions=yaml.safe_load((ROOT/'examples/versions.lock.template.yaml').read_text(encoding='utf-8'))
    check('versions_honestly_unqualified', versions['qualification_status']=='unverified' and versions['execution_enabled'] is False and not versions['floating_latest_allowed'])
    check('no_fabricated_exact_version_pins', all(v is None for v in versions['pins'].values()))
    catalog=read_json('tests/scenario_catalog.json')
    test_ids=[t['test_id'] for t in catalog['tests']]
    expected_ids=[f'FT-{n:02d}' for n in range(1,83)]
    check('catalog_82_unique_planned_tests', test_ids==expected_ids)
    check('tests_not_claimed_executed', all(t['execution_status']=='not_run' for t in catalog['tests']))
    doc_test_ids=re.findall(r'^\| (FT-\d\d) \|',md,re.M)
    check('catalog_matches_document_table',doc_test_ids==test_ids)
    mapping=read_json('tests/traceability.json')['requirements']
    check('41_requirements_mapped', sorted(mapping)==[f'REQ-{n:02d}' for n in range(1,42)])
    ui_catalog=read_json('tests/ui_scenario_catalog.json')
    ui_ids=[t['test_id'] for t in ui_catalog['tests']]
    check('48_unique_planned_ui_tests', ui_ids==[f'UI-{n:02d}' for n in range(1,49)])
    check('ui_tests_not_claimed_executed', all(t['execution_status']=='not_run' for t in ui_catalog['tests']))
    check('ui_catalog_matches_document',re.findall(r'^\| (UI-\d\d) \|',md,re.M)==ui_ids)
    notify_catalog=read_json('tests/notification_scenario_catalog.json')
    nt_ids=[t['test_id'] for t in notify_catalog['tests']]
    check('22_unique_planned_notification_tests',nt_ids==[f'NT-{n:02d}' for n in range(1,23)])
    check('notification_tests_not_claimed_executed',all(t['execution_status']=='not_run' for t in notify_catalog['tests']))
    check('notification_catalog_matches_document',re.findall(r'^\| (NT-\d\d) \|',md,re.M)==nt_ids)
    cap_catalog=read_json('tests/capability_scenario_catalog.json')
    cap_ids=[t['test_id'] for t in cap_catalog['tests']]
    check('32_unique_planned_capability_tests',cap_ids==[f'CAP-{n:02d}' for n in range(1,33)])
    check('capability_tests_not_claimed_executed',all(t['execution_status']=='not_run' for t in cap_catalog['tests']))
    check('capability_catalog_matches_document',re.findall(r'^\| (CAP-\d\d) \|',md,re.M)==cap_ids)
    wf_catalog=read_json('tests/workflow_scenario_catalog.json')
    wf_ids=[t['test_id'] for t in wf_catalog['tests']]
    check('36_unique_planned_workflow_tests',wf_ids==[f'WF-{n:02d}' for n in range(1,37)])
    check('workflow_tests_not_claimed_executed',all(t['execution_status']=='not_run' for t in wf_catalog['tests']))
    check('workflow_catalog_matches_document',re.findall(r'^\| (WF-\d\d) \|',md,re.M)==wf_ids)
    pp_catalog=read_json('tests/product_scenario_catalog.json')
    pp_ids=[t['test_id'] for t in pp_catalog['tests']]
    check('12_unique_planned_product_tests',pp_ids==[f'PP-{n:02d}' for n in range(1,13)])
    check('pp_tests_not_claimed_executed',all(t['execution_status']=='not_run' for t in pp_catalog['tests']))
    check('pp_catalog_matches_document',re.findall(r'^\| (PP-\d\d) \|',md,re.M)==pp_ids)
    all_tests=set(test_ids+ui_ids+nt_ids+cap_ids+wf_ids+pp_ids)
    covered={t for refs in mapping.values() for t in refs}
    check('test_refs_resolve',covered<=all_tests)
    check('every_planned_test_has_requirement',covered==all_tests,str(sorted(all_tests-covered)))
    doc_req=re.findall(r'^\| (REQ-\d\d) \|',md,re.M)
    check('document_requirement_table_matches',doc_req==list(mapping))
    contract_map={29:'C-EXEC',30:'C-AUTHORITY',31:'C-SLEEP',32:'C-REVOKE',33:'C-MEMORY',34:'C-RETRY',35:'C-UPGRADE',36:'C-READY',40:'C-WORKBENCH'}
    contract_names=list(contract_map.values())+['C-MISSION','C-EMPLOYEE-OPS','C-BOOTSTRAP','C-RESOURCE','C-CONTINUITY','C-NOTIFY','C-WORKSPACE','C-SKILLS','C-MCP','C-CAPABILITY','C-INTAKE','C-ENVIRONMENT','C-JOBS','C-BROWSER','C-GUIDANCE','C-DELIVERY','C-FEEDBACK']
    check('twenty_six_contracts', sorted(p.stem for p in (ROOT/'contracts').glob('*.md'))==sorted(contract_names))
    for n,name in contract_map.items():
        pattern=rf'<a id="section-{n:02d}"></a>\n(.*?)(?=\n<a id="(?:section-|appendix-)|\Z)'
        found=re.search(pattern,md,re.S)
        part=(ROOT/'contracts'/f'{name}.md').read_text(encoding='utf-8')
        part=part.split('\n\n此文件为主文档对应章节')[0].strip()
        part=re.sub(r'\]\(\.\./ARCHITECTURE\.md#','](#',part)
        expected=found.group(1).strip() if found else ''
        part='## '+part[2:] if part.startswith('# ') else part
        check('contract_matches_main:'+name,re.sub(r'\n{3,}', '\n\n',part)==re.sub(r'\n{3,}', '\n\n',expected))
    # Matrix and traces explicitly remain hypothetical.
    for name in ['wake_race.trace.json','revocation.trace.json']:
        trace=read_json('examples/'+name)
        check('trace_not_runtime_result:'+name, trace.get('results')=='not_run')
    # New frontend specifications and safe, simulated examples.
    adr_ids=re.findall(r'^\| (\d{3}) \|',md,re.M)
    check('64_architecture_decisions',adr_ids==[f'{i:03d}' for i in range(1,65)])
    main_sections=[]
    for i in range(37,42):
        found=re.search(rf'(<a id="section-{i:02d}"></a>\n.*?)(?=\n<a id="(?:section-|appendix-)|\Z)',md,re.S)
        if found:main_sections.append(found.group(1).strip())
    front=(ROOT/'ui/FRONTEND_SPEC.md').read_text(encoding='utf-8')
    front_body=front[front.index('<a id="section-37"'):].strip()
    front_body=front_body.replace('](../ARCHITECTURE.md#','](#')
    check('frontend_chapters_match_main',re.sub(r'\n{3,}','\n\n',front_body)==re.sub(r'\n{3,}','\n\n','\n\n'.join(main_sections)))
    cfg=yaml.safe_load((ROOT/'examples/workbench.draft.yaml').read_text())
    check('ui_configuration_disabled_simulated',cfg['execution_enabled'] is False and cfg['data_mode']=='simulated')
    check('office_explicitly_deferred',cfg['visual']['office_enabled'] is False and cfg['visual']['office_style'] is None and cfg['visual']['scene_engine'] is None)
    check('human_fixed_roster',cfg['organization']['creation']=='human_confirmed_fixed_roster' and cfg['organization']['dynamic_roles_enabled'] is False)
    check('create_does_not_start',cfg['organization']['create_starts_mission'] is False and cfg['organization']['session_discovery_creates_employee'] is False)
    check('view_does_not_wake',cfg['client']['viewing_triggers_model'] is False)
    check('no_offline_command_queue',cfg['client']['offline_mutation_queue'] is False)
    check('no_optimistic_critical_success',cfg['client']['high_risk_optimistic_success'] is False and cfg['client']['command_202_means']=='accepted_not_completed')
    emp=read_json('examples/workbench_employee.example.json')
    op=read_json('examples/ui_operation.example.json')
    check('ui_examples_not_live',emp['execution_enabled'] is False and emp['meta']['data_mode']=='simulated' and op['execution_enabled'] is False and op['data_mode']=='simulated')
    check('sleep_can_have_inflight_tools',emp['status']['primary']=='sleeping' and emp['status']['active_model_requests']==0 and emp['status']['in_flight_tools']>0)
    check('unknown_amount_not_zero',emp['usage']['quality']=='unavailable' and emp['usage']['amount_microunits'] is None)
    check('large_revision_transmitted_as_string',isinstance(emp['meta']['entity_revision'],str) and int(emp['meta']['entity_revision'])>2**53)
    check('rebinding_does_not_mean_running',emp['model_binding']['active_profile_id'] is None and emp['model_binding']['activation']=='on_next_real_work')
    check('accepted_pause_not_completed',op['status']=='accepted' and not op['quiesced'] and not op['effective_for_new_dispatch'])
    pages=read_json('ui/page-map.json')
    paths=[p['path'] for p in pages['pages']]
    check('page_paths_unique',len(paths)==len(set(paths)))
    check('pages_no_office_placeholder',pages['office_enabled'] is False and not any('office' in p.lower() for p in paths))
    check('company_pages_are_scoped',all(':companyId' in p['path'] for p in pages['pages'] if p['scope']=='company'))
    check('pages_honestly_not_implemented',all(p['status']=='not_implemented' for p in pages['pages']))
    check('no_bundled_font_files',not any(p.suffix.lower() in ('.ttf','.otf','.woff','.woff2') for p in ROOT.rglob('*')))
    # Only check explicitly named foreground/background pairs, not entire WCAG conformance.
    tokens=read_json('ui/design-tokens.json')
    css=(ROOT/'ui/tokens.css').read_text()
    def luminance(color:str)->float:
        rgb=[int(color[i:i+2],16)/255 for i in (1,3,5)]
        linear=[v/12.92 if v<=0.04045 else ((v+0.055)/1.055)**2.4 for v in rgb]
        return sum(a*b for a,b in zip(linear,(0.2126,0.7152,0.0722)))
    for theme,colors in tokens['colors'].items():
        for key,color in colors.items():
            check('css_token:'+theme+':'+key,f'--color-{key.replace("_","-")}: {color};' in css)
        for pair in tokens['contrast_pairs']:
            a,b=sorted((luminance(colors[pair['foreground']]),luminance(colors[pair['background']])))
            ratio=(b+0.05)/(a+0.05)
            check('color_pair:'+theme+':'+pair['foreground']+'/'+pair['background'],ratio>=pair['minimum'],f"{ratio:.3f}:1, required {pair['minimum']}:1; declared pair only")
    check('no_remote_fonts_or_idle_animation',tokens['font']['remote_fonts'] is False and tokens['motion']['idle_pulse'] is False)
    # Both reader pages are offline documents, not the planned React workbench.
    for rel in ['ARCHITECTURE.html','ui/FRONTEND_SPEC.html','preflight/PREFLIGHT_SPEC.html','preflight/QQ_NOTIFICATION_SPEC.html','audit/AUDIT_REPORT.html','capabilities/CAPABILITY_SPEC.html','workflows/WORKFLOW_SPEC.html','product/PRODUCT_CHARTER.html']:
        f=ROOT/rel
        view=BeautifulSoup(f.read_text(encoding='utf-8'),'html.parser')
        ids=[e['id'] for e in view.find_all(id=True)]
        check('reader_unique_ids:'+rel,len(ids)==len(set(ids)))
        issues=[]
        for a in view.find_all('a',href=True):
            parts=urlsplit(a['href'])
            if parts.scheme or parts.netloc:continue
            dest=(f.parent/unquote(parts.path)).resolve() if parts.path else f
            if not dest.is_file():issues.append(a['href'])
            elif parts.fragment:
                names=re.findall(r'(?:id|name)="([^"]+)"',dest.read_text(encoding='utf-8'))
                if unquote(parts.fragment) not in names:issues.append(a['href'])
        check('reader_local_links:'+rel,not issues,str(issues))
        ext=[str(t) for t in view.select('script[src],link[href],img[src],iframe[src]')]
        check('reader_no_external_dependencies:'+rel,not ext,str(ext))
        check('reader_has_not_product_notice:'+rel,'本阅读版不是可运行工作台' in view.get_text())
    # 0.4.1: canonical preflight extracts and restrictive design defaults.
    subsection_map={'contract-mission':'C-MISSION','contract-employee-ops':'C-EMPLOYEE-OPS','contract-bootstrap-resource':'C-BOOTSTRAP','contract-resource':'C-RESOURCE','contract-continuity':'C-CONTINUITY','contract-notify':'C-NOTIFY'}
    for anchor,name in subsection_map.items():
        found=re.search(rf'<a id="{anchor}"></a>\n(.*?)(?=\n<a id="(?:contract-|preflight-)|\Z)',md,re.S)
        part=(ROOT/'contracts'/f'{name}.md').read_text().split('\n\n此文件为主文档对应段落')[0].strip().replace('](../ARCHITECTURE.md#','](#')
        check('subcontract_matches_main:'+name, bool(found) and part==found.group(1).strip())
    part=(ROOT/'preflight/PREFLIGHT_SPEC.md').read_text()
    part=part[part.index('<a id="section-42"'):].strip().replace('](../ARCHITECTURE.md#','](#')
    found=re.search(r'(<a id="section-42"></a>\n.*?)(?=\n<a id="section-43")',md,re.S)
    check('preflight_matches_main',bool(found) and part==found.group(1).strip())
    part=(ROOT/'preflight/QQ_NOTIFICATION_SPEC.md').read_text()
    part=part[part.index('<a id="contract-notify"'):].strip().replace('](../ARCHITECTURE.md#','](#')
    found=re.search(r'(<a id="contract-notify"></a>\n.*?)(?=\n<a id="preflight-artifacts")',md,re.S)
    check('qq_spec_matches_main',bool(found) and part==found.group(1).strip())
    qq=yaml.safe_load((ROOT/'examples/qq_notification.draft.yaml').read_text())
    check('qq_disabled_and_no_test_authorization',not qq['send_enabled'] and not qq['test_send_authorized'])
    check('qq_official_not_unapproved_protocol',qq['channel_kind']=='qq_official')
    check('qq_deterministic_not_llm_chat',not qq['llm_generation_enabled'] and not qq['policy']['inbound_management_commands'])
    check('qq_no_secrets_or_recipient_claims',qq['credential_secret_ref'] is None and qq['app_id'] is None and qq['recipient']['user_openid'] is None and qq['recipient']['group_openid'] is None)
    check('qq_account_qualification_not_claimed',qq['qualification']['proactive_without_inbound_msg_id']=='unverified' and not qq['qualification']['unattended_ready'])
    check('qq_no_normal_noise',not qq['policy']['notify_normal_completion'] and not qq['policy']['notify_idle'] and not qq['policy']['notify_recovered_transient_errors'])
    check('qq_bounded_retries',1<=qq['policy']['max_send_attempts']<=10 and 0<=qq['policy']['max_reminders']<=3)
    check('qq_no_magic_remote_link',qq['workbench_link']['verified_base_url'] is None and not qq['workbench_link']['include_bearer_or_approval_token'])
    check('watchdog_not_a_second_scheduler',not qq['watchdog']['can_dispatch_work'] and not qq['watchdog']['reads_company_content'] and not qq['watchdog']['whole_host_failure_covered'])
    hand=read_json('examples/handover_bundle.example.json')
    check('handover_snapshot_design',hand['snapshot_consistency']['isolation']=='repeatable_read' and hand['snapshot_consistency']['facts_and_cursor_same_snapshot'])
    check('handover_read_only_before_activation',not hand['activation_allowed'] and not hand['restore_grant']['business_workspace_writable'] and not hand['restore_grant']['external_dispatch_allowed'])
    for name in ['mission_bootstrap.trace.json','notification_delivery.trace.json']:
        check('new_trace_not_runtime_result:'+name,read_json('examples/'+name)['results']=='not_run')
    # Negative examples test schemas, NOT service authorization / QQ transport.
    import copy
    schema=read_json('schemas/qq-notification-design.schema.json')
    for key in ['send_enabled','llm_generation_enabled','test_send_authorized']:
        bad=copy.deepcopy(qq);bad[key]=True
        check('design_schema_rejects_enabled:'+key,bool(list(Draft202012Validator(schema).iter_errors(bad))))
    bad=copy.deepcopy(qq);bad['app_secret']='NOT_A_REAL_SECRET'
    check('design_schema_rejects_raw_secret_field',bool(list(Draft202012Validator(schema).iter_errors(bad))))
    empop=read_json('examples/employee_ops.example.json');bademp=copy.deepcopy(empop)
    bademp['tool_request']['arguments']['actor']='admin'
    check('employee_design_schema_rejects_extra_actor',bool(list(Draft202012Validator(read_json('schemas/employee-ops-design.schema.json')).iter_errors(bademp))))
    check('all_fixture_versions_current',all(obj.get('document_version','0.4.5')=='0.4.5' for obj in [qq,hand,empop,company]))
    review=read_json('evidence/kokoro-qq-source-review.json')
    check('source_review_not_execution',review['status']=='source_review_only' and 'proactive send' in review['not_tested'])

    # 0.4.2: upstream evidence, planned deployment, and actual account qualification are distinct.
    check('qq_upstream_route_evidence_distinct_from_runtime',
          qq['implementation_evidence']['proactive_route']=='upstream_source_confirmed'
          and qq['implementation_evidence']['our_adapter']=='not_implemented'
          and not qq['implementation_evidence']['is_account_permission_proof'])
    check('sole_sender_disabled_with_revocation_barrier',
          qq['sender']['mode']=='single_orgctl_watch' and not qq['sender']['enabled']
          and qq['sender']['revocation_requires_sender_barrier'])
    check('offline_capsule_no_default_authority',not qq['sender']['offline_capsule_enabled']
          and qq['sender']['offline_capsule_valid_seconds'] is None
          and not qq['sender']['business_payload_during_db_outage']
          and not qq['sender']['restart_reactivates_capsule'])
    for path,value in [(['sender','enabled'],True),(['sender','offline_capsule_enabled'],True),
                       (['qualification','proactive_without_inbound_msg_id'],'supported'),
                       (['qualification','unattended_ready'],True),
                       (['implementation_evidence','is_account_permission_proof'],True),
                       (['recipient','kind'],'unverified_other_platform')]:
        bad=copy.deepcopy(qq);dest=bad
        for key in path[:-1]:dest=dest[key]
        dest[path[-1]]=value
        check('qq_schema_rejects_unsafe_claim:'+'.'.join(path),bool(list(Draft202012Validator(schema).iter_errors(bad))))
    registry=read_json('protocol/employee-ops.registry.json')
    op_names=[op['name'] for op in registry['operations']]
    required_ops={'work.current','context.read','collab.read','collab.ack','collab.send',
                  'obligation.resolve','contract.propose','memory.propose','work.checkpoint',
                  'artifact.submit','work.block','work.yield','plan.propose','review.submit','handover.confirm',
                  'workspace.list','workspace.search','workspace.read','workspace.snapshot',
                  'skills.list','skills.load','scripts.run','tools.list','tools.describe','tools.call','resources.list','resources.read'}
    required_ops |= {'inputs.list','inputs.read','environment.status','environment.ensure','jobs.start','jobs.status','jobs.logs','jobs.stop','research.search','research.fetch','browser.run','browser.results','guidance.read','guidance.respond','questions.ask','delivery.propose','feedback.read','feedback.triage'}
    check('employee_ops_names_unique_complete',set(op_names)==required_ops and len(op_names)==len(required_ops))
    check('employee_ops_not_a_production_schema',registry['status']=='draft_permission_catalog_not_full_runtime_schema')
    check('employee_ops_restore_restricted',set(registry['restore_allowlist'])=={'work.current','context.read','collab.read','handover.confirm','workspace.list','workspace.search','workspace.read','skills.list','skills.load','tools.list','tools.describe','inputs.list','inputs.read','environment.status','jobs.status','jobs.logs','browser.results','guidance.read','feedback.read'})
    check('employee_ops_auth_bound_not_model_supplied',all(o['requires_current_binding'] for o in registry['operations']))
    check('inbox_read_is_not_obligation_resolution',registry['inbox_read_does_not_resolve_obligation'] is True)
    for op_name in required_ops:
        check('employee_operation_in_main:'+op_name, '`'+op_name+'`' in md)
    check('runtime_incarnation_present_in_handover_and_ops',
          all(o.get('runtime_identity',{}).get('runtime_incarnation') for o in [hand,empop]))
    check('activation_pending_environment_not_writing',
          hand['environment_activation']['state']=='activation_pending_environment'
          and hand['environment_activation']['write_capability_receipt'] is None
          and not hand['activation_allowed'])
    for name in ['notification_revoke.trace.json','activation_gap.trace.json']:
        trace=read_json('examples/'+name)
        check('audit_trace_not_executed:'+name,trace.get('results')=='not_run')
    findings=read_json('audit/FINDINGS.json')
    ids=[f['finding_id'] for f in findings['findings']]
    check('review_findings_unique',ids==[f'AR-{n:02d}' for n in range(1,24)])
    check('findings_not_runtime_bug_claims',all(f['kind']=='design_review_not_observed_runtime_bug' for f in findings['findings'] if f['finding_id']!='AR-23') and findings['findings'][-1]['kind']=='document_structure_defect')
    check('findings_patched_not_claimed_verified',all(f['disposition']=='patched_in_spec' and f['verification_status']=='not_run' for f in findings['findings'] if f['finding_id']!='AR-23') and findings['findings'][-1]['verification_status']=='document_verified')
    check('no_fabricated_independent_reviewers',findings['reviewer']=='same_assistant_three_review_views_not_independent_reviewers')
    check('finding_requirement_refs_resolve',all(set(f['requirements'])<=set(mapping) for f in findings['findings']))
    check('finding_test_refs_resolve',all(set(f['planned_tests'])<=(all_tests|{'DOC-NUMBERING'}) for f in findings['findings']))
    check('finding_locations_have_referenced_sections',all(all(re.search(r'^#{2,5} '+re.escape(loc)+r'(?:[ .：]|$)',md,re.M) for loc in re.findall(r'\b\d{2}(?:\.\d+){1,2}[a-z]?',f['spec_locations'])) for f in findings['findings']))
    gates=read_json('audit/OPEN_GATES.json')
    check('twentytwo_evidence_gates_not_hidden',len(gates['gates'])==22 and len({g['gate_id'] for g in gates['gates']})==22)
    check('no_production_or_unknown_risk_zero_claim',gates['production_ready'] is False and gates['unknown_risks']=='not provably empty')
    check('all_gates_open_with_failure_policy',all(g['status'] in ('not_run_or_not_configured','awaiting_evidence_and_owner_threshold_confirmation') and g['evidence_required'] and g['failure_policy'] for g in gates['gates']))
    audit=(ROOT/'audit/AUDIT_REPORT.md').read_text()
    audit_body=audit[audit.index('<a id="section-43"'):].strip().replace('](../ARCHITECTURE.md#','](#')
    canonical=re.search(r'(<a id="section-43"></a>\n.*?)(?=\n<a id="section-44")',md,re.S)
    check('audit_chapter_matches_main',bool(canonical) and audit_body==canonical.group(1).strip())
    qq_evidence=read_json('evidence/qq-proactive-source-review.json')
    check('qq_source_evidence_not_live_send',qq_evidence.get('live_messages_sent')==0 and not qq_evidence.get('credentials_accessed'))
    check('all_test_ids_globally_unique',len(test_ids+ui_ids+nt_ids+cap_ids+wf_ids+pp_ids)==len(all_tests))


    # 0.4.3: capability sample structure and permission defaults, not execution.
    cap=yaml.safe_load((ROOT/'examples/employee_capabilities.draft.yaml').read_text())
    mcp=yaml.safe_load((ROOT/'examples/mcp_service.draft.yaml').read_text())
    manifest=read_json('examples/skill_package.manifest.json')
    call=read_json('examples/capability_call.example.json')
    for schema_name,obj in [
        ('employee-capabilities-design.schema.json',cap),
        ('mcp-service-design.schema.json',mcp),
        ('skill-manifest-design.schema.json',manifest),
        ('capability-call-design.schema.json',call)]:
        sch=read_json('schemas/'+schema_name)
        Draft202012Validator.check_schema(sch)
        errors=list(Draft202012Validator(sch).iter_errors(obj))
        check('schema:'+schema_name,not errors,'; '.join(e.message for e in errors))
        bad=copy.deepcopy(obj);bad['execution_enabled']=True
        check('capability_schema_rejects_execution:'+schema_name,bool(list(Draft202012Validator(sch).iter_errors(bad))))
        bad=copy.deepcopy(obj);bad['app_secret']='NOT_A_REAL_SECRET'
        check('capability_schema_rejects_secret:'+schema_name,bool(list(Draft202012Validator(sch).iter_errors(bad))))
    check('capabilities_no_grants_and_no_scans',not cap['execution_enabled'] and not cap['pay_enabled'] and not cap['effective_grants'] and not cap['public_library']['scan_enabled'])
    check('public_library_fixed_and_no_shadow',cap['public_library']['selection_mode']=='explicit_pinned_list' and cap['public_library']['same_name_policy']=='qualified_identity_no_auto_shadow' and not cap['public_library']['auto_publish'])
    check('workspace_shared_is_not_peer_live_tree',not cap['workspace_policy']['cross_employee_live_worktree_read'] and cap['workspace_policy']['shared_references_read_only'] and not cap['workspace_policy']['control_and_secrets_mounted'])
    check('capability_employees_match_fixed_roster',{e['employee_id'] for e in cap['employees']}=={e['employee_id'] for e in company['employees']})
    by_employee={e['employee_id']:e for e in cap['employees']}
    check('company_binding_refs_resolve',all(by_employee[e['employee_id']]['binding_id']==e['capability_binding_ref'] for e in company['employees']))
    check('all_bindings_remain_unverified',all(e['binding_state']=='draft' and e['qualification']=='unverified' and e['provisioning_state']=='unprovisioned' and not e['script_entrypoint_refs'] for e in cap['employees']))
    check('reviewer_input_read_only',by_employee['emp-review']['requested_workspace_mode']=='read_only_with_scratch')
    check('mcp_definition_not_connection',not mcp['execution_enabled'] and not mcp['connect_enabled'] and not mcp['pay_enabled'] and not mcp['verified_protocols'])
    check('mcp_two_declared_transports_not_live',{e['transport'] for e in mcp['servers']}=={'stdio','streamable_http'} and all(e['endpoint'] is None and e['executable_ref'] is None and not e['qualified'] for e in mcp['servers']))
    check('mcp_sdk_unpinned_not_fabricated',mcp['sdk']['pinned_version'] is None and mcp['sdk']['qualification']=='unverified')
    check('mcp_no_inherited_secrets_or_install',all(e['secret_ref'] is None and not e['inherit_parent_env'] and not e['install_on_connect'] for e in mcp['servers']))
    check('mcp_no_uncontrolled_reverse_calls',not mcp['policy']['sampling_enabled'] and not mcp['policy']['elicitation_enabled'] and not mcp['policy']['auto_oauth_consent'])
    check('mcp_hints_do_not_grant_or_autoadopt',not mcp['policy']['untrusted_annotations_grant_permissions'] and not mcp['policy']['new_tools_auto_approved'] and not mcp['policy']['native_bypass_enabled'])
    draft_bindings={b['binding_ref']:b for b in mcp['binding_drafts']}
    check('mcp_definitions_shared_but_employee_bindings_distinct',len(draft_bindings)==len(cap['employees']) and all(draft_bindings[e['mcp_binding_refs'][0]]['employee_id']==e['employee_id'] for e in cap['employees']))
    check('mcp_draft_bindings_no_active_tools',all(not b['approved'] and not b['granted_tool_names'] for b in draft_bindings.values()))
    check('capability_calls_default_rejected',call['expected_without_qualification']['reason_code']=='CAPABILITY_UNVERIFIED' and not call['expected_without_qualification']['action_dispatched'] and not call['expected_without_qualification']['model_called'])
    check('capability_trace_is_hypothesis',read_json('examples/capability_handover.trace.json')['results']=='not_run')
    hcap=hand['capability_context']
    check('handover_restores_binding_not_authority',hcap['binding_ref']==by_employee['emp-backend']['binding_id'] and not hcap['write_tools_allowed_before_activation'] and not hcap['real_capability_receipts'])
    check('handover_contains_no_native_secrets',not hcap['native_session_or_bearer_handles_included'] and not hcap['secret_material_included'])
    check('handover_mcp_binding_belongs_to_employee',all(draft_bindings[b['binding_ref']]['employee_id']=='emp-backend' for b in hcap['mcp_bindings']))
    check('handover_skill_and_catalogue_agree',all(x['skill_ref']==manifest['skill_ref'] and x['package_digest']==manifest['package_digest'] for x in hcap['loaded_skills']) and all(manifest['skill_ref'] in e['skill_revision_refs'] for e in cap['employees']))
    source_dir=ROOT/'examples/skills/contract-review'
    for entry in manifest['files']:
        f=source_dir/entry['path'];content=f.read_bytes()
        check('skill_fixture_file_hash:'+entry['path'],hashlib.sha256(content).hexdigest()==entry['sha256'] and len(content)==entry['bytes'])
        check('skill_fixture_inside_root:'+entry['path'],f.resolve().is_relative_to(source_dir.resolve()) and not f.is_symlink())
    check('skill_fixture_manifest_digest',hashlib.sha256(json.dumps(manifest['files'],ensure_ascii=False,sort_keys=True,separators=(',',':')).encode()).hexdigest()==manifest['package_digest'])
    skill=(source_dir/'SKILL.md').read_text();meta=yaml.safe_load(skill.split('---',2)[1])
    check('skill_fixture_required_metadata',meta['name']=='contract-review' and 0<len(meta['description'])<=1024 and all(isinstance(v,str) for v in meta.get('metadata',{}).values()))
    check('skill_fixture_no_executable_assets',all(Path(e['path']).suffix=='.md' for e in manifest['files']) and not manifest['scripts_enabled'] and not manifest['grants'])
    descriptor=read_json('examples/mcp_tool_descriptor.example.json')
    check('descriptor_hint_is_not_permission',descriptor['upstream_tool']['annotations']['readOnlyHint'] and not descriptor['annotation_is_authorization'] and not descriptor['allowed_for_real_calls'] and descriptor['trusted_classification']=='unverified')
    descriptor_hash=hashlib.sha256(json.dumps(descriptor['upstream_tool'],ensure_ascii=False,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    check('descriptor_digest_matches_local_fixture',descriptor_hash==descriptor['descriptor_digest']==call['request']['descriptor_digest'])
    check('handover_descriptor_matches_fixture',all(x['descriptor_digest']==descriptor_hash for x in hcap['mcp_bindings']))
    for key in ['connect_enabled','pay_enabled']:
        bad=copy.deepcopy(mcp);bad[key]=True
        check('mcp_draft_rejects:'+key,bool(list(Draft202012Validator(read_json('schemas/mcp-service-design.schema.json')).iter_errors(bad))))
    for key in ['sampling_enabled','new_tools_auto_approved','native_bypass_enabled']:
        bad=copy.deepcopy(mcp);bad['policy'][key]=True
        check('mcp_draft_rejects_policy:'+key,bool(list(Draft202012Validator(read_json('schemas/mcp-service-design.schema.json')).iter_errors(bad))))
    bad=copy.deepcopy(cap);bad['employees'][0]['qualification']='supported'
    check('capability_draft_rejects_fake_qualification',bool(list(Draft202012Validator(read_json('schemas/employee-capabilities-design.schema.json')).iter_errors(bad))))
    for anchor,name,end in [('contract-workspace','C-WORKSPACE','contract-skills'),('contract-skills','C-SKILLS','contract-mcp'),('contract-mcp','C-MCP','contract-capability-binding')]:
        expected=md[md.index('<a id="'+anchor+'">'):md.index('<a id="'+end+'">')].split('\n',1)[1].strip()
        part=(ROOT/'contracts'/f'{name}.md').read_text().split('\n\n此文件为主文档对应段落')[0].strip().replace('](../ARCHITECTURE.md#','](#')
        check('capability_contract_matches:'+name,part==expected)
    start=md.index('<a id="contract-capability-binding">');end=md.index('\n### 44.7 ',start)
    expected=md[start:end].split('\n',1)[1].strip()
    part=(ROOT/'contracts/C-CAPABILITY.md').read_text().split('\n\n此文件为主文档对应段落')[0].strip().replace('](../ARCHITECTURE.md#','](#')
    check('capability_contract_matches:C-CAPABILITY',part==expected)
    chapter44=md[md.index('<a id="section-44">'):md.index('<a id="section-45">')].strip()
    capdoc=(ROOT/'capabilities/CAPABILITY_SPEC.md').read_text();capdoc=capdoc[capdoc.index('<a id="section-44">'):].strip().replace('](../ARCHITECTURE.md#','](#')
    check('capability_standalone_matches_main',capdoc==chapter44)
    check('capability_read_ops_do_not_allow_restoring_execution',not {'tools.call','scripts.run','workspace.snapshot','resources.read'} & set(registry['restore_allowlist']))


    # 0.4.4: real-workflows are plans, fixtures are disabled, documents are unambiguous.
    header_numbers=[]; in_fence=False
    for line in md.splitlines():
        if line.startswith('```'):
            in_fence=not in_fence
            continue
        if not in_fence:
            match=re.match(r'^#{2,5} (\d+(?:\.\d+)*)[. ]',line)
            if match:header_numbers.append(match.group(1))
    check('DOC-NUMBERING',len(header_numbers)==len(set(header_numbers)),str([x for x in set(header_numbers) if header_numbers.count(x)>1]))
    renumber=read_json('audit/SECTION_RENUMBERING.json')
    check('renumber_map_recorded',renumber['source_version']=='0.4.3' and len(renumber['changed'])>0)
    check('all_subsection_anchors_exist',all(x['anchor'] in doc_ids for x in renumber['subsections']))
    check('subsection_numbers_match_registry',len({x['number'] for x in renumber['subsections']})==len(renumber['subsections']))
    check('workflow_chapter_not_lost_in_capabilities','## 45.' not in capdoc)
    workflow=(ROOT/'workflows/WORKFLOW_SPEC.md').read_text()
    workflow=workflow[workflow.index('<a id="section-45"'):].strip().replace('](../ARCHITECTURE.md#','](#')
    canonical=md[md.index('<a id="section-45"'):md.index('<a id="section-46"')].strip()
    check('workflow_standalone_matches_main',workflow==canonical)
    for anchor,next_anchor,name in [
        ('contract-intake','contract-environment','C-INTAKE'),
        ('contract-environment','contract-jobs','C-ENVIRONMENT'),
        ('contract-jobs','contract-browser','C-JOBS'),
        ('contract-browser','contract-guidance','C-BROWSER'),
        ('contract-guidance','contract-delivery','C-GUIDANCE'),
        ('contract-delivery','contract-feedback','C-DELIVERY'),
        ('contract-feedback','workflow-integration','C-FEEDBACK')]:
        expected=md[md.index('<a id="'+anchor+'">'):md.index('<a id="'+next_anchor+'">')].split('\n',1)[1].strip()
        actual=(ROOT/'contracts'/f'{name}.md').read_text().split('\n\n此文件为主文档对应段落')[0].strip().replace('](../ARCHITECTURE.md#','](#')
        check('workflow_contract_matches:'+name,actual==expected)
    workflow_samples={}
    for name in ['intake','project-environment','job-service','browser-run','human-guidance','delivery-manifest','feedback-source']:
        obj=read_json(f'examples/workflow_{name}.example.json')
        sch=read_json(f'schemas/workflow-{name}-design.schema.json')
        Draft202012Validator.check_schema(sch)
        errors=list(Draft202012Validator(sch).iter_errors(obj))
        check('workflow_schema:'+name,not errors,'; '.join(e.message for e in errors))
        check('workflow_disabled_unverified:'+name,not obj['execution_enabled'] and not obj['pay_enabled'] and obj['qualification']=='unverified')
        for field in ['execution_enabled','pay_enabled']:
            bad=copy.deepcopy(obj);bad[field]=True
            check('workflow_schema_rejects_enabled:'+name+':'+field,bool(list(Draft202012Validator(sch).iter_errors(bad))))
        bad=copy.deepcopy(obj);bad['secret_value']='NOT_A_REAL_SECRET'
        check('workflow_schema_rejects_raw_secret:'+name,bool(list(Draft202012Validator(sch).iter_errors(bad))))
        workflow_samples[name]=obj
    intake=workflow_samples['intake'];env=workflow_samples['project-environment'];job=workflow_samples['job-service'];br=workflow_samples['browser-run'];gd=workflow_samples['human-guidance'];de=workflow_samples['delivery-manifest'];fb=workflow_samples['feedback-source']
    check('intake_upload_not_start_or_fake_view',not intake['auto_start_mission'] and all(a['model_delivery_status']=='not_sent' and a['source_hash'] is None for a in intake['assets']))
    check('approved_dependency_route_not_hidden_install',env['preparation_policy']['auto_prepare_within_approved_policy'] and env['preparation_policy']['effective_approval_ref'] is None and not env['preparation_policy']['run_on_skill_load'] and not env['preparation_policy']['allow_host_global_install'])
    check('environment_no_fabricated_ready_version',env['base_image_digest'] is None and env['lockfile_digest'] is None and env['node_version'] is None)
    check('test_db_not_control_db',env['test_database']=='isolated_if_needed_not_control_db')
    check('job_not_started_and_endpoint_not_claimed',job['state']=='not_started' and job['service']['endpoint'] is None and not job['service']['ready'])
    check('no_unknown_service_reuse',not job['service']['reuse_unverified_port'])
    check('preview_no_management_identity',not br['preview_enabled'] and not br['shares_management_origin'] and not br['shares_management_cookies'] and not br['uses_personal_browser'] and not br['control_network_access'])
    check('guidance_not_applied_or_auto_authorized',gd['state']=='draft' and gd['applied_revision'] is None and not gd['grants_changed'])
    check('human_patch_no_unconfirmed_write',not gd['takeover']['old_writer_quiesced'] and not gd['takeover']['human_edit_enabled'] and not gd['takeover']['resume_authorized'])
    check('delivery_no_fake_artifacts_or_acceptance',de['state']=='assembling' and not de['artifacts'] and de['user_disposition']=='not_requested' and not de['download_counts_as_acceptance'])
    check('feedback_source_not_connected',fb['source_kind']=='github_issues_readonly' and not fb['connect_enabled'] and fb['repository_id'] is None and fb['credential_secret_ref'] is None)
    check('feedback_read_only_and_no_terminal_wake',fb['read_only'] and not fb['remote_writes_enabled'] and fb['closed_mission_behavior']=='backlog_without_model' and not fb['collection_outside_mission_authorized'])
    check('feedback_no_false_coverage',fb['coverage']['status']=='not_configured' and fb['coverage']['last_complete_scan'] is None)
    for file in ['normal_workflow.trace.json','human_takeover.trace.json']:
        tr=read_json('examples/'+file)
        check('workflow_trace_is_plan:'+file,tr['results']=='not_run' and not tr['execution_enabled'])
    wc=hand['workflow_context']
    check('workflow_handover_refs_resolve',wc['environment_profile']==env['profile_id'] and job['job_id'] in wc['job_refs'] and gd['guidance_id'] in wc['pending_guidance_refs'])
    check('workflow_handover_no_restore_execution',not wc['restoring_can_prepare_environment'] and not wc['restoring_can_start_browser'] and not wc['restoring_can_control_jobs'] and not wc['secret_material_included'])
    check('restore_no_active_workflow_ops',not {'environment.ensure','jobs.start','jobs.stop','browser.run','research.search','research.fetch','feedback.triage','guidance.respond','delivery.propose'} & set(registry['restore_allowlist']))
    check('sources_include_workflow_primary_documents',{f'W{n:02d}' for n in range(1,10)} <= {x['id'] for x in read_json('sources.json')['sources']})
    check('normal_workflow_uses_existing_company',job['company_id']==company['company_id'] and job['employee_id'] in {x['employee_id'] for x in company['employees']})
    check('workflow_all_reference_profiles_unverified',all(not x['qualified'] for x in read_json('workflows/REFERENCE_PROFILES.json')['profiles']))
    check('all_expected_product_tests_are_plans',len(all_tests)==232)

    # 0.4.5 product decisions: structure and safe drafts only, never empirical proof.
    product=read_json('product/DECISIONS.json')
    check('primary_user_correction_preserved',product['primary_user_confirmation']['do_not_narrow_to_programmers'] is True and '长期' in product['primary_user_confirmation']['text'])
    check('12_product_decisions', [d['decision_id'] for d in product['decisions']]==[f'PD-{n:02d}' for n in range(1,13)])
    check('product_hypotheses_remain_open',all(h['status']=='awaiting_evidence' for h in product['hypotheses']))
    reviews=read_json('product/REVIEW_DISPOSITIONS.json')
    check('17_product_reviews_accounted', [r['finding_id'] for r in reviews['findings']]==[f'PM-{n:02d}' for n in range(1,18)])
    check('product_reviews_not_results',all(r['validation_status']=='not_run' and r['product_hypothesis_status']=='awaiting_evidence' for r in reviews['findings']))
    check('product_review_test_refs',all(set(r['planned_tests'])<=all_tests for r in reviews['findings']))
    check('product_review_sections_exist',all(all(re.search(r'^#{2,5} '+re.escape(loc)+r'(?:[ .：]|$)',md,re.M) for loc in r['spec_sections']) for r in reviews['findings']))
    charter=(ROOT/'product/PRODUCT_CHARTER.md').read_text()
    charter_body=charter[charter.index('<a id="section-46"'):].strip().replace('](../ARCHITECTURE.md#','](#')
    expected=re.search(r'(<a id="section-46"></a>.*?)(?=\n<a id="appendix-a")',md,re.S).group(1).strip()
    check('product_charter_matches_main',charter_body==expected)
    rel=read_json('product/RELEASE_SCOPE.json')
    ex=read_json('product/EXPERIMENTS.json')
    for name,obj in [('release-scope-design.schema.json',rel),('experiments-design.schema.json',ex)]:
        schema=read_json('schemas/'+name);Draft202012Validator.check_schema(schema)
        errs=list(Draft202012Validator(schema).iter_errors(obj))
        check('schema:'+name,not errs,'; '.join(e.message for e in errs))
    check('release_all_requirements_indexed',[r['requirement_id'] for r in rel['requirements']]==list(mapping))
    check('no_scope_reduction_safety_waiver',rel['no_safety_waiver_for_smaller_release'] and all(r['mandatory_safety_for_exposed_capability'] for r in rel['requirements']))
    check('r1_skills_mcp_qq_not_removed',all(next(r for r in rel['requirements'] if r['requirement_id']==x)['earliest_delivery']=='R1' for x in ['REQ-28','REQ-30','REQ-31']))
    check('office_deferral_constraint_applies_from_r1',next(r for r in rel['requirements'] if r['requirement_id']=='REQ-22')['earliest_delivery']=='R1')
    check('r2_feedback_explicit',next(r for r in rel['requirements'] if r['requirement_id']=='REQ-41')['earliest_delivery']=='R2')
    experiments=ex['experiments']
    check('three_experiment_protocols',[e['id'] for e in experiments]==['E-START','E-ORG','E-HANDOVER'])
    check('experiments_not_authorized',all(not e['execution_enabled'] and not e['pay_enabled'] and not e['owner_budget_authorized'] and e['approved_budget'] is None and not e['authorized_accounts'] for e in experiments))
    check('thresholds_not_preapproved',all(e['threshold_status']=='proposed_awaiting_owner_confirmation' and e['results'] is None and e['status']=='not_run' for e in experiments))
    check('experiment_refs_resolve',all(set(e['planned_tests'])<=all_tests for e in experiments))
    schema=read_json('schemas/experiments-design.schema.json')
    for field,value in [('execution_enabled',True),('pay_enabled',True),('owner_budget_authorized',True),('results',{'pretend':'pass'}),('approved_budget',20)]:
        bad=copy.deepcopy(ex);bad['experiments'][0][field]=value
        check('product_negative_experiment:'+field,bool(list(Draft202012Validator(schema).iter_errors(bad))))
    schema=read_json('schemas/release-scope-design.schema.json')
    for field,value in [('execution_enabled',True),('fixed_roles',False),('software_is_validation_domain_not_user_definition',False)]:
        bad=copy.deepcopy(rel);bad[field]=value
        check('product_negative_scope:'+field,bool(list(Draft202012Validator(schema).iter_errors(bad))))
    ref=read_json('product/REFERENCE_COMBINATION.json')
    check('reference_profile_is_not_live',ref['qualification']=='unverified' and not ref['execution_enabled'] and not ref['pay_enabled'] and not ref['notification_send_enabled'])
    check('no_reference_account_or_version_invention',ref['credentials'] is None and ref['exact_versions'] is None and ref['required_money_budget'] is None and ref['primary_model_profile_ref'] is None)
    check('no_strict_money_claim_without_evidence',not ref['strict_dollar_cap_claim'] and ref['budget_guarantee']=='unverified')
    team=read_json('product/TEAM_COVERAGE.json');team_ids=set(team['employee_ids'])
    check('team_ids_match_reference',team_ids==set(ref['employee_ids'])=={e['employee_id'] for e in company['employees']})
    check('team_coverage_is_fixed_draft',not team['execution_enabled'] and not team['role_changes_at_runtime'] and team['template_requires_human_confirmation'])
    check('team_coverage_has_distinct_checker_ids',all(row['owner'] in team_ids and set(row['eligible_independent_checkers'])<=team_ids and row['owner'] not in row['eligible_independent_checkers'] for row in team['coverage']))
    check('team_does_not_claim_cognitive_independence',not team['guarantees_semantic_independence'] and not team['trusted_baseline_mutable_by_workers'])
    appl=read_json('product/TEST_RELEASE_APPLICABILITY.json')
    check('all_tests_have_release_planning',len(appl['tests'])==len(all_tests) and {r['test_id'] for r in appl['tests']}==all_tests)
    check('release_planning_does_not_claim_execution',all(r['execution_status']=='not_run' for r in appl['tests']))
    check('new_sources_resolve',{'D01','D02','D03'}<={x['id'] for x in read_json('sources.json')['sources']})
    check('cold_start_contract_distinguishes_environment','环境准备只要求已批准准备政策' in md and '项目依赖尚未ready不阻止' in md)
    check('feedback_comments_contract_present','正式triage/修复前再读取单issue的有界评论' in md)
    check('product_no_auto_pivot','产品转向须维护者决定' in md)

    checks_ok=all(c['status']=='pass' for c in checks)
    return {'document_version':'0.4.5','check_kind':'static_document_and_fixture_only','status':'pass' if checks_ok else 'fail',
            'counts':{'chapters':46,'requirements':41,'contracts':26,'planned_fault_tests':len(test_ids),'planned_ui_tests':len(ui_ids),'planned_notification_tests':len(nt_ids),'planned_capability_tests':len(cap_ids),'planned_workflow_tests':len(wf_ids),'planned_product_tests':len(pp_ids),'checks':len(checks)},
            'checks':checks,
            'not_executed':['PostgreSQL integration/concurrency tests','Agent runtime implementation',
                            'real harness/model calls','containment/security qualification','paid-model benchmarks',
                            'FT-01 through FT-82','UI-01 through UI-48','WF-01 through WF-36','CAP-01 through CAP-32','live Skills/MCP activation and capability handover','NT-01 through NT-22','QQ live sends and proactive qualification','production React workbench','long-running autonomous operation','PP-01 through PP-12','E-START E-ORG E-HANDOVER and real user research'],
            'caution':'A static validation pass is not evidence that the proposed architecture works.'}

def main() -> int:
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--json',type=Path,help='Optionally save static results as JSON.')
    args=ap.parse_args()
    try:
        report=validate()
    except (OSError,KeyError,TypeError,ValueError) as exc:
        print(f'Validation failed to read this design pack: {exc}',file=sys.stderr)
        return 2
    if args.json:
        args.json.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(f"Static checks: {report['status'].upper()} ({report['counts']['checks']} checks)")
    for c in report['checks']:
        if c['status']!='pass': print('FAIL:',c['name'],c['detail'])
    print('Runtime / PostgreSQL / real models / planned fault tests: NOT RUN')
    return 0 if report['status']=='pass' else 1

if __name__=='__main__':
    raise SystemExit(main())
