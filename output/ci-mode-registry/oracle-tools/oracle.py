#!/opt/homebrew/bin/python3
"""Before/after actual Bash argv/environment oracle. All tested process boundaries are controlled stubs."""
import argparse, pathlib, subprocess, os, sys, re, json, hashlib, shutil, signal, time
HERE=pathlib.Path(__file__).resolve().parent
BASE='88d3ba1369de0b13c4a9d4ba8051c1540ca9cfd6'
DEFAULT_REPO=pathlib.Path('/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-pr-modes-coverage')
COMMANDS='record audit env docker go pnpm node bash xvfb-run openssl uname python3 sleep git dirname mkdir rm cp mv tee cat date grep sed paste'.split()
PRELUDE='''function source() {
  case "$1" in "$ORACLE_CASE_ROOT"/*|scripts/dev/*) ;; *) "$ORACLE_BIN/record" unsupported-source "$1"; return 87;; esac
  "$ORACLE_BIN/audit" "$1" || return $?
  builtin source "$@"
}
function . { source "$@"; }
function command() {
  if [[ "${1:-}" == -v ]]; then "$ORACLE_BIN/record" command-v "$@"; builtin command "$@";
  else "$ORACLE_BIN/record" unsupported-command "$@"; return 87; fi
}
'''
LOCK='''lc_lock_dir="$ORACLE_CASE_ROOT/lock"
lc_lock_acquire() { "$ORACLE_BIN/record" lock-acquire "$@"; return "${ORACLE_LOCK_RC:-0}"; }
lc_lock_release() { "$ORACLE_BIN/record" lock-release; }
'''
def sha(data):return hashlib.sha256(data if isinstance(data,bytes) else data.encode()).hexdigest()
def git(repo,*args):return subprocess.check_output(['/usr/bin/git','-C',str(repo),*args],timeout=10)
def freeze(repo,base):
    target=HERE/'inputs/baseline'
    if target.exists():return target
    target.mkdir(parents=True)
    paths=git(repo,'ls-tree','-r','--name-only',base,'scripts/dev').decode().splitlines()
    for path in paths:
        if path.endswith('.sh'):
            p=target/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(git(repo,'show',base+':'+path))
    (target/'BASE_COMMIT').write_text(base)
    return target
def snapshot(repo,label):
    target=HERE/('inputs/candidate-'+label)
    if target.exists():shutil.rmtree(target)
    for p in (repo/'scripts/dev').glob('*.sh'):
        q=target/p.relative_to(repo);q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(p.read_bytes())
    return target
def modes(source):
    line=next(x for x in source.splitlines() if x.lstrip().startswith("printf 'Usage:"))
    values=re.findall(r'--[a-z0-9-]+',line)
    if len(values)!=82 or len(set(values))!=82:raise ValueError('Frozen baseline must contain exactly82 unique flags')
    return values
def profiles(flags):
    out=[{'id':'default-foundation','args':[],'env':{},'control':{},'want':0}]+[{'id':m[2:],'args':[m],'env':{},'control':{},'want':2 if m=='--stripe-browser' else 0} for m in flags]
    def add(id,args,env=None,control=None,want=0):out.append(dict(id=id,args=args,env=env or {},control=control or {},want=want))
    add('explicit-foundation',['foundation'])
    add('invalid-mode',['--not-a-mode'],want=2);add('two-args',['--checkout','extra'],want=2)
    add('guard-synthetic-live-key',[],{'MOCK_GUARD':''.join(('sk_','live_','SYNTHETIC_NOT_REAL'))},want=2)
    add('foundation-run',[],{'LC_FOUNDATION_RUN':'^TestOne$|^TestTwo with spaces$'})
    add('foundation-skip',[],{'LC_FOUNDATION_SKIP':'^TestSkip$'})
    add('foundation-shard',[],{'LC_FOUNDATION_RUN':'^TestOne$','LC_FOUNDATION_SKIP':'^TestSkip$','LC_FOUNDATION_PKGS':'./tests/foundation ./internal/inbox'})
    add('foundation-packages',[],{'LC_FOUNDATION_PKGS':'./internal/inbox ./internal/payments'})
    add('storefront-mock',['--browser-storefront'],{'LC_SHOP_MOCK':'1'})
    add('storefront-mock-custom-evidence',['--browser-storefront'],{'LC_SHOP_MOCK':'1','LC_SHOP_EVIDENCE':'$CASE/output/custom-shop'})
    for mode in ('--browser-inbox','--browser-merchant-orders-ui'):
        add(mode[2:]+'-linux-headless',[mode],control={'os':'Linux'})
        add(mode[2:]+'-linux-display',[mode],{'DISPLAY':':99'},{'os':'Linux'})
        add(mode[2:]+'-linux-no-xvfb',[mode],control={'os':'Linux','no_xvfb':True},want=2)
    add('click-shard1',['--browser-click-sweep'],{'LC_SWEEP_SHARD':'1/3'})
    add('click-shard2',['--browser-click-sweep'],{'LC_SWEEP_SHARD':'2/3'})
    add('visual-existing-journeys',['--browser-visual-lint'],control={'existing_journeys':True})
    add('visual-shard2',['--browser-visual-lint'],{'LC_SWEEP_SHARD':'2/3'})
    add('visual-no-result',['--browser-visual-lint'],control={'visual_missing':True},want=1)
    add('visual-failed-verdict',['--browser-visual-lint'],control={'visual_exit':1},want=1)
    add('webkit-not-installed',['--browser-webkit'],control={'webkit':False},want=2)
    add('webkit-subset',['--browser-webkit'],{'LC_WEBKIT_STEPS':'payment,cvs'})
    add('webkit-go-failure',['--browser-webkit'],{'LC_WEBKIT_STEPS':'payment'},{'go_exit':1},want=1)
    add('webkit-count-failure',['--browser-webkit'],{'LC_WEBKIT_STEPS':'payment'},{'counts_verdict':1},want=1)
    add('stripe-node-only',['--stripe-browser'],{'LC_STRIPE_BROWSER_STEPS':'node-env,payuni-baseline'})
    sandbox={'STRIPE_BROWSER':'1','STRIPE_SANDBOX':'1','LC_STRIPE_BROWSER_STEPS':'node-env,payuni-baseline,sp18,su07,su09,obs'}
    add('stripe-synthetic-sandbox',['--stripe-browser'],sandbox)
    add('stripe-synthetic-split',['--stripe-browser'],dict(sandbox,STRIPE_BROWSER_SPLIT='1'))
    add('stripe-synthetic-go-failure',['--stripe-browser'],dict(sandbox,LC_STRIPE_BROWSER_STEPS='sp18'),{'go_exit':1},want=1)
    add('stripe-invalid-synthetic-key',['--stripe-browser'],sandbox,{'stripe_key':'invalid'},want=2)
    add('stripe-wrong-account',['--stripe-browser'],dict(sandbox,STRIPE_ACCOUNT_ID='MOCK_WRONG_ACCOUNT'),want=2)
    add('pg-retry-once',['--checkout'],control={'ready_failures':1})
    add('ordinary-go-failure',['--checkout'],control={'go_exit':1},want=1)
    for m in ('--checkout','--browser-platform-site','--browser-picklist'):add('lock-busy-'+m[2:],[m],control={'lock_rc':2},want=2)
    return out
def inputs(repo,base,source):
    result={}
    text='\n'.join(p.read_text() for p in source.glob('scripts/dev/*.sh') if p.name.startswith('test-local'))
    for rel in set(re.findall(r'(?<![A-Za-z0-9_])((?:tests|apps|internal|cmd|packages)/[A-Za-z0-9_./-]+\.(?:go|ts|tsx|mjs|json|sh))',text)):
        try:result[rel]=git(repo,'show',base+':'+rel)
        except subprocess.CalledProcessError:pass
    return result
def normalize(value,root):
    if isinstance(value,str):return re.sub(r'lc-foundation-test-[0-9]+','lc-foundation-test-$PID',value.replace(str(root),'$CASE'))
    if isinstance(value,list):return [normalize(x,root) for x in value]
    if isinstance(value,dict):return {k:normalize(v,root) for k,v in value.items()}
    return value
def run_case(source,fixture_inputs,case,label):
    root=HERE/'scratch'/label/case['id']
    if root.exists():shutil.rmtree(root)
    root.mkdir(parents=True)
    for p in source.glob('scripts/dev/*.sh'):
        q=root/p.relative_to(source);q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(p.read_bytes())
    for rel,data in fixture_inputs.items():p=root/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
    (root/'.git').mkdir();(root/'home').mkdir();(root/'tmp').mkdir();(root/'bin').mkdir();(root/'output').mkdir()
    (root/'scripts/dev/test-lock.sh').write_text(LOCK)
    for command in COMMANDS:
        if command=='xvfb-run' and case['control'].get('no_xvfb'):continue
        (root/'bin'/command).symlink_to(HERE/'stub.py')
    (root/'prelude.sh').write_text(PRELUDE)
    secrets=root/'home/secrets.env';secrets.write_text('STRIPE_SECRET_KEY='+case['control'].get('stripe_key',''.join(('sk_','test_','SYNTHETIC_ORACLE_NOT_REAL')))+'\n')
    trace=root/'trace.jsonl';trace.write_text('')
    env=dict(PATH=str(root/'bin'),HOME=str(root/'home'),TMPDIR=str(root/'tmp'),LANG='C',LC_ALL='C',TZ='UTC',BASH_ENV=str(root/'prelude.sh'),LC_SECRETS_FILE=str(secrets),LC_TEST_LOCK_DIR=str(root/'lock'),ORACLE_CASE_ROOT=str(root),ORACLE_BIN=str(root/'bin'),ORACLE_TRACE=str(trace),ORACLE_PROFILE=json.dumps(case['control']),ORACLE_LOCK_RC=str(case['control'].get('lock_rc',0)))
    env.update({k:v.replace('$CASE',str(root)) for k,v in case['env'].items()})
    if case['control'].get('existing_journeys'):
        p=root/'output/ui-click-sweep/journeys.json';p.parent.mkdir(parents=True,exist_ok=True);p.write_text('{"MOCK_ORIGINAL":true}\n')
    audit=subprocess.run([str(root/'bin/audit'),str(root/'scripts/dev/test-local.sh')],env=env,cwd=root,text=True,capture_output=True,timeout=5)
    start=time.monotonic()
    if audit.returncode:rc=audit.returncode;stdout=audit.stdout;stderr=audit.stderr
    else:
        proc=subprocess.Popen(['/bin/bash',str(root/'scripts/dev/test-local.sh'),*case['args']],cwd=root,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
        try:stdout,stderr=proc.communicate(timeout=15);rc=proc.returncode
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid,signal.SIGKILL);stdout,stderr=proc.communicate();rc=124
    rows=[json.loads(x) for x in trace.read_text().splitlines()]
    unsupported=[r for r in rows if r['category']=='unsupported' or r['command'].startswith('unsupported-')]
    if 'command not found' in stderr:unsupported.append({'command':'shell','reason':'unregistered command: '+stderr})
    commands=[r for r in rows if r['category']=='command']
    utilities=sorted((r for r in rows if r['category']=='utility'),key=lambda x:json.dumps(x,sort_keys=True))
    artifacts={}
    for p in sorted((root/'output').rglob('*')):
        if p.is_file():
            data=p.read_bytes()
            try:data=normalize(data.decode(),root).encode()
            except UnicodeDecodeError:pass
            artifacts[str(p.relative_to(root))]=sha(data)
    result=dict(case=case,exit=rc,want=case['want'],supported=not unsupported and rc!=124,commands=commands,utilities=utilities,events=rows,artifacts=artifacts,stdout=normalize(stdout,root),stderr=normalize(stderr,root),duration=round(time.monotonic()-start,3),unsupported=unsupported)
    out=HERE/label/(case['id']+'.json');out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2,sort_keys=True))
    if rc==case['want'] and not unsupported: shutil.rmtree(root)
    return result

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--repo',type=pathlib.Path,default=DEFAULT_REPO);ap.add_argument('--candidate',type=pathlib.Path);ap.add_argument('--only');ap.add_argument('--label',default='baseline');ap.add_argument('--compare',action='store_true');args=ap.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9-]+',args.label):raise ValueError('Unsafe result label')
    if (HERE/args.label).exists():raise ValueError('Result label exists; use a fresh label to preserve evidence')
    original=freeze(args.repo,BASE);source=snapshot(args.candidate,args.label) if args.candidate else original
    flags=modes((original/'scripts/dev/test-local.sh').read_text());cases=profiles(flags)
    if args.only:cases=[c for c in cases if re.search(args.only,c['id'])]
    fixtures=inputs(args.repo,BASE,original); fixtures.update(inputs(args.repo,BASE,source))
    manifest={str(p.relative_to(source)):sha(p.read_bytes()) for p in sorted(source.glob('scripts/dev/*.sh'))}
    results=[];diffs=[]
    for case in cases:
        r=run_case(source,fixtures,case,args.label);results.append(r)
        print(case['id'],r['exit'],'SUPPORTED' if r['supported'] else 'UNSUPPORTED',flush=True)
        if args.compare:
            previous=json.loads((HERE/'baseline'/(case['id']+'.json')).read_text())
            for key in ('exit','supported','commands','utilities','artifacts'):
                if previous[key]!=r[key]:diffs.append(dict(case=case['id'],field=key,before=previous[key],after=r[key]))
    summary=dict(base_commit=BASE,tool_manifest={p.name:sha(p.read_bytes()) for p in (HERE/'oracle.py',HERE/'stub.py')},source_manifest=manifest,bash='/bin/bash 3.2.57',cases=len(results),supported=sum(r['supported'] for r in results),expected_exit=sum(r['exit']==r['want'] for r in results),unexpected=[dict(case=r['case']['id'],exit=r['exit'],want=r['want'],stderr=r['stderr']) for r in results if not r['supported'] or r['exit']!=r['want']],comparison_differences=diffs)
    p=HERE/args.label/'SUMMARY.json';p.write_text(json.dumps(summary,indent=2,sort_keys=True));print('SUMMARY',len(results),'supported',summary['supported'],'expected',summary['expected_exit'],'diffs',len(diffs),flush=True)
    return 1 if summary['unexpected'] or diffs else 0
if __name__=='__main__':sys.exit(main())
