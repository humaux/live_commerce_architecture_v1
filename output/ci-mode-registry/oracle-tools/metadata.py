#!/opt/homebrew/bin/python3
"""Retain actual --list and --dry-run outputs; these are not the equivalence oracle."""
import argparse,json,pathlib,sys,re
import oracle
ap=argparse.ArgumentParser();ap.add_argument('--candidate',type=pathlib.Path,required=True);ap.add_argument('--label',default='candidate-listings');a=ap.parse_args()
if not re.fullmatch(r'[A-Za-z0-9-]+',a.label) or (oracle.HERE/a.label).exists():raise ValueError('Use a fresh safe label')
original=oracle.freeze(oracle.DEFAULT_REPO,oracle.BASE);source=oracle.snapshot(a.candidate,a.label)
flags=oracle.modes((original/'scripts/dev/test-local.sh').read_text());fixtures=oracle.inputs(oracle.DEFAULT_REPO,oracle.BASE,original)
cases=[dict(id='list',args=['--list'],env={},control={},want=0)]+[dict(id=m.removeprefix('--'),args=['--dry-run',m],env={},control={},want=0) for m in [*flags,'foundation']]
cases.append(dict(id='storefront-mock',args=['--dry-run','--browser-storefront'],env={'LC_SHOP_MOCK':'1'},control={},want=0))
failures=[]
for c in cases:
 r=oracle.run_case(source,fixtures,c,a.label)
 (oracle.HERE/a.label/(c['id']+'.txt')).write_text(r['stdout'])
 if r['exit'] or not r['supported'] or r['commands']:failures.append(dict(case=c['id'],exit=r['exit'],commands=r['commands'],unsupported=r['unsupported']))
 if c['id']=='list' and sorted(r['stdout'].splitlines())!=sorted([*flags,'foundation']):failures.append(dict(case='list',reason='frozen mode set mismatch'))
 print(c['id'],r['exit'],len(r['commands']),'effect commands',flush=True)
summary=dict(cases=len(cases),source_manifest={str(p.relative_to(source)):oracle.sha(p.read_bytes()) for p in sorted(source.glob('scripts/dev/*.sh'))},failures=failures,status='MOCK metadata only; equivalence uses normal invocation traces')
(oracle.HERE/a.label/'SUMMARY.json').write_text(json.dumps(summary,indent=2,sort_keys=True));sys.exit(bool(failures))
