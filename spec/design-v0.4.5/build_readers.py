#!/usr/bin/env python3
"""Generate offline readers and canonical extracts from ARCHITECTURE.md.
No network, product runtime, models, account access, or target project execution.
Requires markdown-it-py and beautifulsoup4; does not create images or PDF files.
"""
from pathlib import Path
import re, json, html
from markdown_it import MarkdownIt
from bs4 import BeautifulSoup
ROOT=Path(__file__).resolve().parent
VERSION='0.4.5'

def main():
 md=(ROOT/'ARCHITECTURE.md').read_text()
 def section(n):
  s=md.index(f'<a id="section-{n:02d}">');m=re.search(r'\n<a id="(?:section-|appendix-)',md[s+1:]);e=s+1+m.start() if m else len(md)
  return md[s:e].strip()
 def between(a,b):
  s=md.index(f'<a id="{a}">');e=md.index(b,s)
  return md[s:e].strip()
 def write(p,s):
  x=ROOT/p;x.parent.mkdir(parents=True,exist_ok=True);x.write_text(s,encoding='utf-8')
 def extract(title,body,rel):
  own=set(re.findall(r'<a id="([^"]+)"',body))
  body=re.sub(r'\]\(#([^\)]+)\)',lambda m:'](#'+m[1]+')' if m[1] in own else '](../ARCHITECTURE.md#'+m[1]+')',body)
  s=f'# {title} · Draft {VERSION}\n\n本文件从[主规范](../ARCHITECTURE.md)生成；不作为另一份独立真相。产品与目标应用仍未实现或实测。\n\n'+body+'\n'
  write(rel,s)
 contract_map={29:'C-EXEC',30:'C-AUTHORITY',31:'C-SLEEP',32:'C-REVOKE',33:'C-MEMORY',34:'C-RETRY',35:'C-UPGRADE',36:'C-READY',40:'C-WORKBENCH'}
 extract_records=[]
 for n,name in contract_map.items():
  body=section(n).split('\n',1)[1]
  body=body.replace('](#','](../ARCHITECTURE.md#')
  write(f'contracts/{name}.md',body+'\n\n此文件为主文档对应章节的自动副本，请修改主文档后重新生成。\n')
  extract_records.append(dict(path=f'contracts/{name}.md',section=n))
 subs=[('contract-mission','contract-employee-ops','C-MISSION'),('contract-employee-ops','contract-bootstrap-resource','C-EMPLOYEE-OPS'),('contract-bootstrap-resource','contract-resource','C-BOOTSTRAP'),('contract-resource','contract-continuity','C-RESOURCE'),('contract-continuity','contract-notify','C-CONTINUITY'),('contract-notify','preflight-artifacts','C-NOTIFY'),('contract-workspace','contract-skills','C-WORKSPACE'),('contract-skills','contract-mcp','C-SKILLS'),('contract-mcp','contract-capability-binding','C-MCP')]
 new=[('contract-intake','contract-environment','C-INTAKE'),('contract-environment','contract-jobs','C-ENVIRONMENT'),('contract-jobs','contract-browser','C-JOBS'),('contract-browser','contract-guidance','C-BROWSER'),('contract-guidance','contract-delivery','C-GUIDANCE'),('contract-delivery','contract-feedback','C-DELIVERY'),('contract-feedback','workflow-integration','C-FEEDBACK')]
 for a,b,name in subs+new:
  body=between(a,f'<a id="{b}">').split('\n',1)[1].strip()
  write(f'contracts/{name}.md',body.replace('](#','](../ARCHITECTURE.md#')+'\n\n此文件为主文档对应段落的自动副本，请修改主文档后重新生成。\n')
  extract_records.append(dict(path=f'contracts/{name}.md',start_anchor=a,end_anchor=b))
 body=between('contract-capability-binding','\n### 44.7 ').split('\n',1)[1].strip()
 write('contracts/C-CAPABILITY.md',body.replace('](#','](../ARCHITECTURE.md#')+'\n\n此文件为主文档对应段落的自动副本，请修改主文档后重新生成。\n')
 extract_records.append(dict(path='contracts/C-CAPABILITY.md',start_anchor='contract-capability-binding',end_heading='44.7'))
 extract('企业工作台规范','\n\n'.join(section(n) for n in range(37,42)),'ui/FRONTEND_SPEC.md')
 extract('开工收口规范',section(42),'preflight/PREFLIGHT_SPEC.md')
 extract('QQ官方主动接管通知',between('contract-notify','<a id="preflight-artifacts">'),'preflight/QQ_NOTIFICATION_SPEC.md')
 extract('架构复核与证据边界',section(43),'audit/AUDIT_REPORT.md')
 extract('员工工作环境、Skills与MCP',section(44),'capabilities/CAPABILITY_SPEC.md')
 extract('真实工作闭环与七份工作合同',section(45),'workflows/WORKFLOW_SPEC.md')
 extract('产品定位、分层交付与对抗审查处置',section(46),'product/PRODUCT_CHARTER.md')
 write('audit/EXTRACT_MAP.json',json.dumps(dict(document_version=VERSION,contracts=extract_records),ensure_ascii=False,indent=2)+'\n')
 readers=['ARCHITECTURE.md','ui/FRONTEND_SPEC.md','preflight/PREFLIGHT_SPEC.md','preflight/QQ_NOTIFICATION_SPEC.md','audit/AUDIT_REPORT.md','capabilities/CAPABILITY_SPEC.md','workflows/WORKFLOW_SPEC.md','product/PRODUCT_CHARTER.md']
 for rel in readers:
  p=ROOT/rel;out=p.with_suffix('.html');render(p.read_text(),out)
 print(f'Generated {len(readers)} offline readers and {len(extract_records)} canonical contracts.')

def render(md,out):
 engine=MarkdownIt('commonmark',{'html':True,'linkify':False}).enable('table')
 content=engine.render(md);soup=BeautifulSoup(content,'html.parser')
 # Local references point to generated HTML; external links remain opt-in.
 for a in soup.find_all('a',href=True):
  h=a['href']
  if not re.match(r'^[a-zA-Z][\w+.-]*:',h):a['href']=h.replace('ARCHITECTURE.md','ARCHITECTURE.html')
  elif h.startswith(('http://','https://')):a['rel']='noopener noreferrer';a['target']='_blank'
 for table in soup.find_all('table'):
  div=soup.new_tag('div',attrs={'class':'table-wrap','tabindex':'0','aria-label':'可横向滚动的规范表格'});table.wrap(div)
 # Outline uses existing named anchors for chapter and generated subsection ids.
 entries=[];count=0
 for h in soup.find_all(['h2','h3']):
  a=h.find('a',id=True)
  if a:key=a['id']
  else:
   prev=h.find_previous_sibling()
   if prev and prev.name=='p' and prev.find('a',id=True):key=prev.find('a',id=True)['id']
   elif prev and prev.name=='a' and prev.get('id'):key=prev['id']
   else:
    count+=1;key='reader-heading-'+str(count);h['id']=key
  title=h.get_text(' ',strip=True)
  if title:entries.append((h.name,key,title))
 title=soup.h1.get_text(' ',strip=True) if soup.h1 else 'Agent Organization Architecture'
 outline='';opened=False
 for level,key,t in entries:
  if level=='h2':
   if opened:outline+='</div></details>'
   outline+=f'<details class="navgroup"><summary><a href="#{html.escape(key)}">{html.escape(t)}</a></summary><div class="subnav">';opened=True
  else:
   if not opened:outline+='<details class="navgroup" open><summary>目录</summary><div class="subnav">';opened=True
   outline+=f'<a href="#{html.escape(key)}">{html.escape(t)}</a>'
 if opened:outline+='</div></details>'
 style='''
:root{color-scheme:light;--bg:#f6f7f9;--paper:#fff;--text:#182432;--muted:#536273;--line:#dbe2e9;--accent:#264e73;--soft:#edf3f7}
*{box-sizing:border-box}html{scroll-behavior:smooth;scroll-padding-top:76px}body{margin:0;font:16px/1.8 -apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans CJK SC","Microsoft YaHei",sans-serif;color:var(--text);background:var(--bg)}a{color:var(--accent);text-decoration:none;overflow-wrap:anywhere}a:hover{text-decoration:underline}a:focus-visible,button:focus-visible,input:focus-visible,summary:focus-visible{outline:3px solid #397fae;outline-offset:3px}header{position:sticky;top:0;z-index:20;display:flex;align-items:center;justify-content:space-between;gap:12px;height:62px;padding:0 24px;background:var(--paper);border-bottom:1px solid var(--line)}.brand{font-size:14px;font-weight:650;line-height:1.4}.edition{color:var(--muted);font-size:12px;letter-spacing:.08em}.toolbar{display:flex;gap:8px;flex-shrink:0}button{font:inherit;font-size:13px;border:1px solid var(--line);border-radius:6px;padding:6px 12px;background:var(--paper);color:var(--text);cursor:pointer;min-height:36px}.layout{display:grid;grid-template-columns:278px minmax(0,1fr);max-width:1600px;margin:auto}.sidebar{position:sticky;top:62px;align-self:start;height:calc(100vh - 62px);overflow:auto;padding:20px 14px;border-right:1px solid var(--line);background:var(--paper)}.sidebar input{width:100%;font:inherit;font-size:13px;padding:9px 10px;border:1px solid #8794a1;border-radius:6px;margin-bottom:14px;color:var(--text);background:var(--paper)}.navgroup{border-bottom:1px solid var(--line);padding:7px 0}.navgroup summary{font-size:13px;line-height:1.6;cursor:pointer}.navgroup summary a{color:var(--text)}.subnav{padding:6px 0 0 14px}.subnav a{display:block;font-size:12px;line-height:1.5;margin:0 0 8px;color:var(--muted)}main{min-width:0;padding:32px 40px 70px}article{background:var(--paper);border:1px solid var(--line);border-radius:10px;padding:36px 44px;max-width:1100px;margin:auto;overflow-wrap:anywhere}h1{font-size:32px;line-height:1.3;margin:0 0 18px;letter-spacing:-.02em}h2{font-size:25px;line-height:1.5;margin:52px 0 18px;border-top:1px solid var(--line);padding-top:24px}h3{font-size:20px;line-height:1.5;margin:32px 0 14px}h4{font-size:17px;margin:24px 0 12px}p{margin:13px 0}li{margin:5px 0}blockquote{margin:24px 0;padding:12px 20px;border-left:4px solid var(--accent);background:var(--soft);border-radius:0 6px 6px 0}blockquote p{margin:6px 0}.notice{max-width:1100px;margin:0 auto 18px;padding:13px 17px;font-size:13px;border:1px solid var(--line);background:var(--soft);border-radius:8px;color:var(--muted)}code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:.9em;background:var(--soft);padding:2px 4px;border-radius:4px;overflow-wrap:anywhere}pre{max-width:100%;overflow:auto;background:#15222f;color:#eef3f8;border-radius:8px;padding:18px;font-size:13px;line-height:1.6;tab-size:2}pre code{background:none;color:inherit;padding:0;white-space:pre;overflow-wrap:normal}.table-wrap{width:100%;overflow-x:auto;margin:20px 0;border:1px solid var(--line);border-radius:7px}table{border-collapse:collapse;width:100%;font-size:14px;line-height:1.65;min-width:540px}th{text-align:left;background:var(--soft);font-weight:650}td,th{padding:11px 13px;border-bottom:1px solid var(--line);vertical-align:top}tr:last-child td{border-bottom:0}td{min-width:80px}hr{border:0;border-top:1px solid var(--line);margin:28px 0}.mobile-menu{display:none}.back{display:block;margin:24px 0;font-size:13px}.print-note{font-size:12px;color:var(--muted);text-align:center;margin-top:24px}[hidden]{display:none!important}
body.dark{color-scheme:dark;--bg:#101822;--paper:#16222f;--text:#e6edf5;--muted:#adbccd;--line:#334557;--accent:#96bfdf;--soft:#213547}
@media(max-width:1000px){.layout{grid-template-columns:236px minmax(0,1fr)}main{padding:24px}article{padding:28px}}
@media(max-width:760px){header{padding:0 12px;height:64px}.brand{font-size:12px;max-width:180px}.edition{font-size:10px}.toolbar{gap:5px}button{padding:6px 9px;font-size:12px}.mobile-menu{display:inline-block}.layout{display:block}.sidebar{display:none;position:fixed;z-index:15;top:64px;height:calc(100dvh - 64px);width:min(88vw,330px);box-shadow:12px 0 28px #0002}.sidebar.open{display:block}main{padding:16px 10px 40px}article{padding:22px 16px;border-radius:8px}.notice{padding:12px;font-size:12px}body{font-size:15px}h1{font-size:27px}h2{font-size:22px;margin-top:36px;padding-top:20px}h3{font-size:19px}h4{font-size:16px}pre{padding:12px;font-size:12px}table{font-size:13px;min-width:520px}blockquote{padding:10px 13px;margin:20px 0}.table-wrap{margin:16px 0}}
@media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}}
@media print{header,.sidebar,.notice,.print-note{display:none}.layout{display:block}main{padding:0}article{border:0;max-width:none;padding:0}pre{white-space:pre-wrap;overflow-wrap:anywhere}pre code{white-space:pre-wrap}.table-wrap{overflow:visible}table{min-width:0;font-size:10px}body{font-size:11px}h2{break-before:auto}tr,blockquote,pre{break-inside:avoid}}
'''
 script='''
const sidebar=document.getElementById('sidebar');const menu=document.getElementById('menu');
menu.addEventListener('click',()=>{const v=sidebar.classList.toggle('open');menu.setAttribute('aria-expanded',String(v));});
document.getElementById('theme').addEventListener('click',()=>{document.body.classList.toggle('dark');});
document.getElementById('filter').addEventListener('input',e=>{let q=e.target.value.trim().toLowerCase();document.querySelectorAll('.navgroup').forEach(g=>{let match=g.textContent.toLowerCase().includes(q);g.hidden=!match;if(q&&match)g.open=true;g.querySelectorAll('.subnav a').forEach(a=>{a.hidden=q&&!a.textContent.toLowerCase().includes(q);});});});
sidebar.querySelectorAll('a').forEach(a=>a.addEventListener('click',()=>{sidebar.classList.remove('open');menu.setAttribute('aria-expanded','false');}));
function highlight(){let key=decodeURIComponent(location.hash.slice(1));sidebar.querySelectorAll('a').forEach(a=>{let active=a.getAttribute('href')==='#'+key;a.toggleAttribute('aria-current',active);if(active){let g=a.closest('details');if(g)g.open=true;}});}window.addEventListener('hashchange',highlight);highlight();
document.addEventListener('keydown',e=>{if(e.key==='Escape'){sidebar.classList.remove('open');menu.setAttribute('aria-expanded','false');}});
'''
 page=f'''<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><title>{html.escape(title)} · Draft {VERSION}</title><style>{style}</style></head><body><header><div><div class="brand">长期自治 Agent 组织运行时</div><div class="edition">DRAFT {VERSION} · DESIGN DOCUMENT</div></div><div class="toolbar"><button id="menu" class="mobile-menu" aria-controls="sidebar" aria-expanded="false">目录</button><button id="theme" aria-label="切换深浅主题">深浅主题</button></div></header><div class="layout"><nav class="sidebar" id="sidebar" aria-label="文档目录"><label for="filter" class="edition">章节筛选</label><input id="filter" type="search" placeholder="查找章节…">{outline}</nav><main><div class="notice"><strong>设计文档 · 非运行结果</strong>　2026-09-08<br>本阅读版不是可运行工作台；真实产品、目标项目、模型与账号测试尚未完成。外部参考链接只在你点击时打开。</div><article>{str(soup)}</article><div class="print-note">离线阅读 · 无外部字体与渲染依赖 · 所有运行资格以真实证据为准</div></main></div><script>{script}</script></body></html>'''
 out.write_text(page,encoding='utf-8')
if __name__=='__main__':main()
