# pattern: Imperative Shell
"""Validate produced wire/config artifacts against pinned upstream schemas."""
import hashlib
import json
import re
import tomllib
from pathlib import Path
from jsonschema import Draft7Validator

root=Path(__file__).resolve().parent.parent
evidence=root/'evidence/development/r0.1'
source=(root/'internal/runner/native.go').read_text()
config=tomllib.loads(re.search(r'const NativeConfig\s*=\s*`(.*?)`',source,re.S)[1])
Draft7Validator(json.loads((evidence/'config.schema.json').read_text())).validate(config)
schemas={
    'initialize':'v1/InitializeParams.json',
    'thread/start':'v2/ThreadStartParams.json',
    'turn/start':'v2/TurnStartParams.json',
}
checked=0
for path in (evidence/'real').glob('*/protocol.jsonl'):
    for line in path.read_text().splitlines():
        entry=json.loads(line)
        if entry['direction']!='send' or not isinstance(entry['data'],dict):continue
        msg=entry['data'];method=msg.get('method')
        if method in schemas:
            schema=json.loads((evidence/'codex-schema'/schemas[method]).read_text())
            Draft7Validator(schema).validate(msg['params']);checked+=1
        elif 'result' in msg:
            schema=json.loads((evidence/'codex-schema/DynamicToolCallResponse.json').read_text())
            Draft7Validator(schema).validate(msg['result']);checked+=1
tools=json.loads((evidence/'real/tools.json').read_text())
for tool in tools:Draft7Validator.check_schema(tool['inputSchema'])
report={'status':'passed','native_requests_and_responses_checked':checked,'dynamic_tools':len(tools),'version':'0.151.0','model_turns_started_by_check':0}
(evidence/'schema-validation.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
