#!/opt/homebrew/bin/python3
"""Controlled external-command boundary. No Docker/Go/build/JS/shell payload is executed."""
import sys, os, json, re, pathlib, shutil, hashlib
name=pathlib.Path(sys.argv[0]).name; args=sys.argv[1:]
root=pathlib.Path(os.environ['ORACLE_CASE_ROOT']).resolve()
profile=json.loads(os.environ.get('ORACLE_PROFILE','{}'))
def norm(value):
    if isinstance(value,str): return re.sub(r'lc-foundation-test-[0-9]+','lc-foundation-test-$PID',value.replace(str(root),'$CASE'))
    if isinstance(value,list): return [norm(x) for x in value]
    if isinstance(value,dict): return {k:norm(v) for k,v in value.items()}
    return value
def record(command, argv, category='command', **extra):
    env={k:v for k,v in os.environ.items() if not k.startswith('ORACLE_') and k not in ('_','SHLVL','BASH_ENV')}
    row=norm(dict(command=command,argv=argv,cwd=os.getcwd(),env=env,category=category,**extra))
    with open(os.environ['ORACLE_TRACE'],'a') as f: f.write(json.dumps(row,sort_keys=True)+'\n')
def owned(value):
    p=pathlib.Path(value); p=(p if p.is_absolute() else pathlib.Path.cwd()/p).resolve()
    if p!=root and root not in p.parents: raise ValueError('Refused path outside case: '+str(p))
    return p
def out(value): print(value,end='' if value.endswith('\n') else '\n')
def fail(reason):
    record(name,args,'unsupported',reason=reason); print('ORACLE_UNSUPPORTED: '+reason,file=sys.stderr);sys.exit(87)
try:
    if name=='record': record(args[0],args[1:]);sys.exit(0)
    if name=='audit':
        p=owned(args[0]);s=p.read_text()
        # This is a bounded oracle for trusted repository scripts, not a sandbox for arbitrary shell programs.
        if re.search(r'(^|[;|(&\n])\s*(?:exec\s+|command\s+)?/(?:bin|usr|opt|sbin|Users|Volumes)/',s): fail('absolute external command in '+str(p))
        if re.search(r'(^|[;\n])\s*(?:eval|exec)\s',s): fail('unsupported eval/exec in '+str(p))
        sys.exit(0)
    if name in ('go','docker','pnpm','node','bash','xvfb-run','openssl','env','uname','python3','sleep','git','mkdir','rm','cp','mv'): record(name,args)
    else: record(name,args,'utility')
    if name=='env':
        if not args:
            for k,v in sorted(os.environ.items()): print(k+'='+v)
        else:
            env=dict(os.environ);i=0
            while i<len(args) and re.match(r'^[A-Za-z_][A-Za-z0-9_]*=',args[i]):
                k,v=args[i].split('=',1);env[k]=v;i+=1
            if i==len(args):
                for k,v in sorted(env.items()): print(k+'='+v)
            else:
                exe=root/'bin'/args[i]
                if '/' in args[i] or not exe.is_symlink() or exe.resolve()!=pathlib.Path(__file__).resolve(): fail('env attempted unknown command')
                os.execve(str(exe),[str(exe)]+args[i+1:],env)
    elif name=='docker':
        if args[:1]==['port']: out('127.0.0.1:35432')
        elif args[:1]==['inspect']: out(args[-1])
        elif args[:1]==['run']: out('MOCK_CONTAINER_ID')
        elif args[:1]==['exec'] and 'pg_isready' in args:
            f=root/'ready-count';n=int(f.read_text()) if f.exists() else 0;f.write_text(str(n+1))
            if n<int(profile.get('ready_failures',0)):sys.exit(1)
        elif args and args[0] in ('exec','rm','info','image'): pass
        else: fail('unsupported Docker arguments')
    elif name=='go':
        if not args or args[0] not in ('test','vet','version'): fail('unsupported Go arguments')
        if profile.get('visual_missing')!=True and any('TestBrowserClickSweep' in x for x in args) and os.environ.get('LC_SWEEP_ONLY')=='visual-audit':
            d=root/'output/ui-visual-audit/mock';d.mkdir(parents=True,exist_ok=True)
            (d/'lint.json').write_text('{"verdict":{"exit":0}}');(d.parent/'LATEST').write_text(str(d))
        if '-json' in args:
            for i in range(50):print(json.dumps({'Action':'pass','Package':'mock','Test':'MOCK/leaf'+str(i)}))
            print(json.dumps({'Action':'pass','Package':'mock','Test':'MOCK'}))
        sys.exit(int(profile.get('go_exit',0)))
    elif name=='node':
        if 'webkit.executablePath()' in ' '.join(args): sys.exit(0 if profile.get('webkit',True) else 1)
        if 'lint.json' in ' '.join(args): out('MOCK visual verdict');sys.exit(int(profile.get('visual_exit',0)))
        if any(x.endswith('/shop-gate.mjs') for x in args):out('cases=12 MOCK_ONLY')
        sys.exit(int(profile.get('node_exit',0)))
    elif name in ('pnpm','bash','xvfb-run'): pass
    elif name=='openssl':
        if args==['rand','-hex','24']:out('a'*48)
        else: fail('unsupported OpenSSL arguments')
    elif name=='uname':out(profile.get('os','Darwin'))
    elif name=='sleep':pass
    elif name=='git':
        if args in (['rev-parse','--git-common-dir'],['rev-parse','--path-format=absolute','--git-common-dir']):out(str(root/'.git'))
        elif args[:1]==['rev-parse'] and any(x.startswith('--short') for x in args):out('0123456789ab')
        elif args==['rev-parse','HEAD']:out('0123456789abcdef0123456789abcdef01234567')
        else:fail('unsupported Git arguments')
    elif name=='python3':
        # Only test-local's counts subprocess exists here. The supplied program is recorded, never executed.
        program=sys.stdin.read();record('python-stdin',[hashlib.sha256(program.encode()).hexdigest()],'diagnostic')
        if len(args)<3 or args[0]!='-' or 'VERDICT' not in program:fail('unsupported Python payload')
        verdict=int(profile.get('counts_verdict',0));out('pass=51 fail=0 skip=0 leaf_pass=50 leaf_fail=0 leaf_skip=0 VERDICT='+str(verdict))
    elif name=='dirname':out(str(pathlib.Path(args[0]).parent))
    elif name=='mkdir':
        for value in args:
            if not value.startswith('-'):owned(value).mkdir(parents=True,exist_ok=True)
    elif name in ('rm','cp','mv'):
        values=[x for x in args if not x.startswith('-')]
        if name=='rm':
            for value in values:
                p=owned(value)
                if p.is_dir():shutil.rmtree(p)
                elif p.exists():p.unlink()
        else:
            if len(values)!=2:fail('unsupported copy/move arity')
            a,b=map(owned,values)
            if name=='cp':shutil.copyfile(a,b)
            else:shutil.move(str(a),str(b))
    elif name=='tee':
        data=sys.stdin.read()
        for value in args:
            if not value.startswith('-'):
                p=owned(value);p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)
        print(data,end='')
    elif name=='sed':
        if len(args)!=3 or args[0]!='-nE':fail('unsupported controlled sed arguments')
        expression=args[1]
        if not (expression.startswith('s/') and expression.endswith('/p')):fail('unsupported controlled sed expression')
        pattern,replacement=expression[2:-2].rsplit('/',1)
        for line in owned(args[2]).read_text().splitlines():
            match=re.search(pattern,line)
            if match:out(match.expand(replacement))
    elif name=='paste':
        if args[:1]!=['-sd'] or len(args)!=3 or args[2]!='-':fail('unsupported controlled paste arguments')
        out(args[1].join(sys.stdin.read().splitlines()))
    elif name=='cat':
        if not args:print(sys.stdin.read(),end='')
        for value in args:
            p=owned(value)
            if not p.exists():sys.exit(1)
            print(p.read_text(),end='')
    elif name=='date':out('1700000000')
    elif name=='grep':
        flags=[x for x in args if x.startswith('-')];values=[x for x in args if not x.startswith('-')]
        if not values:fail('grep lacks pattern')
        pattern=values[0];data=''.join(owned(x).read_text() for x in values[1:]) if len(values)>1 else sys.stdin.read()
        if pattern.startswith('^func '):matched=re.search(pattern.split('(')[0],data,re.M) is not None
        else:matched=re.search(pattern,data,re.M) is not None
        if not any('q' in x for x in flags) and matched:out(data)
        sys.exit(0 if matched else 1)
    else:fail('unimplemented controlled utility')
except Exception as e:
    fail(type(e).__name__+': '+str(e))
