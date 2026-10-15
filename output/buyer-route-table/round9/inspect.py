#!/usr/bin/env python3
# Purpose: retain built-CLI scratch observations, never execute Go tests.
# Depends on: stdlib Python, existing Git clone and diagnostic/production binaries.
# Invariant: mutations affect only task-owned scratch; every source is restored.
import hashlib, json, pathlib, subprocess, sys
out = pathlib.Path(__file__).resolve().parent
root = out.parents[2]
scratch = out/'scratch'

def run(label, repo, cli):
    result = subprocess.run([sys.executable, str(out/'run.py'), label, str(repo), str(cli), '-root', str(repo), '-base', 'trunk' if repo.name=='fixture' else 'e4dad98cc'], capture_output=True, text=True, timeout=190)
    receipt = json.loads((out/(label+'-receipt.json')).read_text())
    return result.returncode, (out/(label+'-report.txt')).read_text(), receipt

def candidates(repo, diag):
    return json.loads(subprocess.check_output([str(diag),str(repo)], timeout=30))

if sys.argv[1]=='A':
    fixture=scratch/'fixture'
    source=(root/'scripts/dev/contractdrift/go_route_table_cli_test.go').read_text()
    # Raw fixture constant contains no backticks in its Go source.
    base=source.split('const routeTableCLISource = `',1)[1].split('`',1)[0]
    rows=base.replace('Method, Path, kind string','Method, Path, kind string; Handler http.HandlerFunc')
    rows=rows.replace('kind: "one"},','kind: "one", Handler: rowTarget},').replace('kind: "item"},','kind: "item", Handler: rowTarget},')
    tails={
      'direct':'\nfunc rowTarget(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}\n',
      'alias':'\nfunc rowLeaf(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}\nvar rowTarget = rowLeaf\n',
      'literal':'\nvar rowTarget = func(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}\n',
      'method-value':'\ntype leaf struct{}\nfunc (leaf) serve(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}\nvar rowTarget = leaf{}.serve\n'}
    result={}
    for name,tail in tails.items():
        path=fixture/'internal/router/routes.go';before=rows+tail;after=rows+tail.replace('w.WriteHeader(204)','w.WriteHeader(205)',1)
        path.write_text(before);pins=candidates(fixture,scratch/'diag-cli');pin=next(p for p in pins if p['table']=='router.Routes');pin['reviewed']='R9 scratch before mutation'
        (fixture/'contracts/route-table-dispatchers.json').write_text(json.dumps(pins))
        path.write_text(after);post=candidates(fixture,scratch/'diag-cli')[0]
        code,report,receipt=run('A-correct-'+name,fixture,scratch/'contractdrift-final')
        result[name]=dict(before=pin,after=post,hash_equal=pin['dispatcher_sha256']==post['dispatcher_sha256'],dispatcher_body_unchanged='w.WriteHeader(204)\n}' in after,cli_exit=code,summary=receipt['summary'])
        assert result[name]['hash_equal'] and code==0 and receipt['errors']==0 and receipt['touched']==0,result[name]
    (out/'A-corrected-closures.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({k:{'hash_equal':v['hash_equal'],'cli_exit':v['cli_exit']} for k,v in result.items()}))
else:
    repo=scratch/'repo'; cli=scratch/'contractdrift-final'; cat=repo/'internal/buyerhttp/catalogv2.go'; handler=repo/'internal/buyerhttp/handler.go'
    original=cat.read_text();hs=handler.read_text();decl='var v2Slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)'
    mutations={
      'N1':{'zz_init.go':'package buyerhttp\nimport "regexp"\nfunc init(){ v2Slug = regexp.MustCompile(".*") }\n'},
      'N2':{'a_slug_linux.go':'//go:build linux\n\npackage buyerhttp\nimport "regexp"\nvar v2Slug = regexp.MustCompile(".*")\n','b_slug_other.go':'//go:build !linux\n\npackage buyerhttp\nimport "regexp"\n'+decl+'\n'},
      'N3':{},
      'address':{'zz_pointer.go':'package buyerhttp\nfunc untouched(){ p := &v2Slug; _ = p }\n'},
      'duplicate':{'zz_duplicate.go':'package buyerhttp\nimport "regexp"\n'+decl+'\n'},
      'build-tag':{'zz_tag.go':'//go:build custom\n\npackage buyerhttp\n'},
      'init-hash':{'zz_init.go':'package buyerhttp\nfunc init(){ println(123) }\n'},
      'exported':{}}
    results={}
    for name,files in mutations.items():
        saved={};pinfile=repo/'contracts/route-table-dispatchers.json';oldpin=pinfile.read_bytes()
        try:
            if name=='N2':cat.write_text(original.replace(decl,''))
            if name=='N3':
                pos=hs.index('{',hs.index('func New('));handler.write_text(hs[:pos+1]+'\n v2Slug = regexp.MustCompile(".*")\n'+hs[pos+1:])
            if name=='exported':
                for p in cat.parent.glob('*.go'):
                    if p.name.endswith('_test.go'):continue
                    content=p.read_text()
                    if 'v2Slug' in content:saved[p]=content;p.write_text(content.replace('v2Slug','V2Slug'))
                pins=candidates(repo,scratch/'diag-cli')
                for p in pins:p['reviewed']='R9 pre-fix scratch exported baseline'
                pinfile.write_text(json.dumps(pins))
            for file,content in files.items():(cat.parent/file).write_text(content)
            # Old CLI is a control for additional bypasses and the init root.
            if name in {'duplicate','build-tag','init-hash','exported'}:
                code,report,rec=run('red-'+name,repo,scratch/'contractdrift');assert code==0 and rec['errors']==0,(name,report)
            code,report,rec=run('final-'+name,repo,cli)
            diagnostic=[line for line in report.splitlines() if 'ERROR (dispatcher freeze)' in line]
            assert code==1 and len(diagnostic)==46,(name,report)
            results[name]=dict(exit=code,freeze_errors=len(diagnostic),first=diagnostic[0],summary=rec['summary'])
        finally:
            cat.write_text(original);handler.write_text(hs);pinfile.write_bytes(oldpin)
            for p,content in saved.items():p.write_text(content)
            for file in files:(cat.parent/file).unlink()
    (out/'B-mutation-results.json').write_text(json.dumps(results,indent=2)+'\n')
    print(json.dumps(results,indent=2))
