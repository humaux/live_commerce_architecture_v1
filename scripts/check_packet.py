#!/usr/bin/env python3
"""Check the architecture packet, not the SaaS implementation.

Python 3.11+ standard library. Checks metadata, references, task DAG, TOML shape,
OpenAPI local references, and recorded model evidence. Does not validate actual
Codex runtime configuration or certify full OpenAPI/JSON Schema semantics.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import re
import sys
import tomllib
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]

def load(path: str) -> Any:
    return json.loads((ROOT/path).read_text(encoding='utf-8'))

def require(value: bool, message: str) -> None:
    if not value:
        raise ValueError(message)

def index(items: list[dict[str, Any]], label: str) -> dict[str, dict[str, Any]]:
    require(isinstance(items,list) and bool(items),f'{label}: empty or invalid registry')
    result={x['id']:x for x in items}
    require(len(result)==len(items),f'{label}: duplicate id')
    return result

def resolve_ref(doc: dict[str, Any], ref: str) -> Any:
    require(ref.startswith('#/'),f'external ref not checked: {ref}')
    node: Any=doc
    for part in ref[2:].split('/'):
        node=node[part.replace('~1','/').replace('~0','~')]
    return node

def walk_refs(node: Any, doc: dict[str, Any]) -> None:
    if isinstance(node,dict):
        for key,value in node.items():
            if key=='$ref':
                resolve_ref(doc,value)
            else:
                walk_refs(value,doc)
    elif isinstance(node,list):
        for value in node: walk_refs(value,doc)

def main(delivery_baseline: bool = False) -> dict[str, Any]:
    regs={k:index(load(f'contracts/{k}.json'),k) for k in ('requirements','invariants','gates','tasks','risks','sources')}
    expected={'requirements':22,'invariants':24,'gates':15,'tasks':23,'risks':32,'sources':50}
    for name,count in expected.items():
        require(len(regs[name])==count,f'{name}: unexpected release baseline count; revise explicitly')
    for gate in regs['gates'].values():
        require(bool(gate['required_cases']),f"{gate['id']}: empty expected cases")
        require(set(gate['invariants'])<=regs['invariants'].keys(),f"{gate['id']}: unknown invariant")
        if delivery_baseline:
            require(gate['status']=='NOT_RUN_PRODUCT',f"{gate['id']}: this design packet must not claim product pass")
    cfg=tomllib.loads((ROOT/'.codex/config.toml').read_text())
    require(cfg['agents']['enabled'] is True,'agents not enabled')
    require(cfg['agents']['max_concurrent_threads_per_session']==4,'unexpected initial concurrency cap')
    require(cfg['sandbox_mode']=='workspace-write','unsafe default sandbox')
    require(cfg['approval_policy']=='on-request','unexpected approval policy')
    require(cfg['sandbox_workspace_write']['network_access'] is False,'network not off by default')
    roles={}
    for path in (ROOT/'.codex/agents').glob('*.toml'):
        role=tomllib.loads(path.read_text())
        require(all(isinstance(role.get(k),str) and role[k] for k in ('name','description','developer_instructions')),f'{path.name}: missing role fields')
        require(role['name']==path.stem,f'{path.name}: name mismatch')
        require(role['name'] not in roles,'duplicate role')
        require(role['sandbox_mode'] in ('read-only','workspace-write'),'unsafe role sandbox')
        roles[role['name']]=role
    require(len(roles)==7,'expected 7 role files')
    covered=set()
    pending=set(regs['tasks'])
    order=[]
    while pending:
        ready=sorted(t for t in pending if set(regs['tasks'][t]['depends_on'])<=set(order))
        require(bool(ready),'task DAG cycle or unresolved dependency')
        for tid in ready:
            t=regs['tasks'][tid]
            require(set(t['requirements'])<=regs['requirements'].keys(),f'{tid}: requirement missing')
            require(set(t['gates'])<=regs['gates'].keys(),f'{tid}: gate missing')
            require(t['role'] in roles,f'{tid}: role missing')
            require(not (roles[t['role']]['sandbox_mode']=='read-only' and t['write_paths']),f'{tid}: readonly role has write paths')
            if delivery_baseline:
                require(t['status']=='NOT_STARTED',f'{tid}: architecture cannot claim product execution')
            covered.update(t['requirements']);order.append(tid);pending.remove(tid)
    require(covered==regs['requirements'].keys(),'unassigned requirement')
    for risk in regs['risks'].values():
        require(set(risk['gates'])<=regs['gates'].keys(),f"{risk['id']}: gate missing")
    architecture=(ROOT/'架构.md').read_text()
    chapters=[int(x) for x in re.findall(r'^## (\d+)\.',architecture,re.M)]
    require(chapters==list(range(30)),'expected sections 0..29 in order')
    refs=set(re.findall(r'\[(S\d{2})\]',architecture))
    defined=set(re.findall(r'^\[(S\d{2})\]:',architecture,re.M))
    require(refs==defined==regs['sources'].keys(),'source cross-references incomplete')
    require(architecture.count('```')%2==0,'unbalanced code fences')
    require((ROOT/'AGENTS.md').stat().st_size<32*1024,'AGENTS.md too large')
    for filename in ('README.md','BOOTSTRAP_PROMPT.md','docs/TASKS.md','experiments/PLAN.md'):
        require((ROOT/filename).is_file() and (ROOT/filename).stat().st_size>0,f'missing {filename}')
    for file in (ROOT/'contracts').glob('*.json'):
        load(str(file.relative_to(ROOT)))
    api=load('contracts/core-openapi.json')
    require(api['openapi']=='3.1.0','unexpected OpenAPI version')
    walk_refs(api,api)
    ops=[]
    for path,item in api['paths'].items():
        for verb,op in item.items():
            ops.append(op['operationId'])
            required=set(re.findall(r'{([^}]+)}',path))
            params=[resolve_ref(api,p['$ref']) if '$ref' in p else p for p in op['parameters']]
            declared={p['name'] for p in params if p['in']=='path' and p['required']}
            require(required==declared,f'{path}: path param mismatch')
            require(bool(op['security']),f'{path}: missing security')
            if verb!='get':
                names={p['name'] for p in params}
                require({'Idempotency-Key','X-CSRF-Token'}<=names,f'{path}: missing write controls')
    require(len(ops)==len(set(ops))==8,'expected 8 unique draft operations')
    for schema_path in ('contracts/event-envelope.schema.json','contracts/task-result.schema.json'):
        schema=load(schema_path)
        require(schema['additionalProperties'] is False,f'{schema_path}: open root')
        require(set(schema['required'])==schema['properties'].keys(),f'{schema_path}: required mismatch')
    result=load('experiments/results/spec-results.json')
    source_hash=hashlib.sha256((ROOT/'experiments/spec_models.py').read_bytes()).hexdigest()
    require(result['source_sha256']==source_hash,'model source differs from recorded run')
    require(result['evidence_level']=='EXECUTABLE_SPEC_MODEL_ONLY','wrong model evidence level')
    cases=result['safety_negative_controls']
    require(len(cases)==12 and len({c['id'] for c in cases})==12,'empty/duplicate/missing cases')
    require(all(c['baseline_pass'] and c['mutant_killed'] for c in cases),'model baseline/negative control failed')
    require(result['summary']=={'baselines_passed':12,'mutants_killed':12,'cases':12},'model summary mismatch')
    ab=result['optimization_ablation']
    require(ab['full']['by_tenant']['B']['p95_completion_ticks']==20,'unexpected model A00 RR')
    require(ab['without_fair_scheduler']['by_tenant']['B']['p95_completion_ticks']==510,'unexpected model A00 FIFO')
    require(ab['full']['total_service_ticks']==ab['without_fair_scheduler']['total_service_ticks']==510,'workload inequivalence')
    return {'checked_at_utc':datetime.now(timezone.utc).isoformat(),'status':'PASS_PACKET_STRUCTURE_ONLY','delivery_baseline_checked':delivery_baseline,
            'counts':{**expected,'roles':len(roles),'openapi_draft_operations':len(ops),'chapters':len(chapters)},
            'task_topological_order':order,'model_source_sha256':source_hash,
            'not_verified':['SaaS production source','real PostgreSQL/River','Codex runtime loading/models/spawns',
                            'complete OpenAPI/JSON Schema conformance','live provider permissions','performance','legal eligibility']}

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--delivery-baseline',action='store_true',help='Also require initial NOT_RUN product and task statuses')
    args=parser.parse_args()
    try:
        result=main(args.delivery_baseline)
        path=ROOT/'experiments/results/packet-check.json'
        path.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
        print(json.dumps(result,ensure_ascii=False,indent=2))
    except (ValueError,KeyError,TypeError,OSError,json.JSONDecodeError,tomllib.TOMLDecodeError) as exc:
        print(f'PACKET_CHECK_FAILED: {exc}',file=sys.stderr)
        sys.exit(1)
