# Purpose: independently compare actual old/new planner APIs and Git CLIs across representative path sets.
# Depends on: isolated candidate/old snapshots, Node, local synthetic Git commits; no source worktree writes.
# Used by: Codex-2 read-only cross-review of e3857691.
import pathlib,subprocess,json,os
root=pathlib.Path(__file__).parent
candidate=root/'candidate'; old=root/'old'; fixture=root/'cli-fixture'; fixture.mkdir(exist_ok=True)
cases=[
 ('empty',[]),('docs',['docs/delivery/GATES.md']),('output',['output/check/DELIVERY.md']),
 ('backend-go',['internal/customers/read.go']),('cmd-go',['cmd/api/main.go']),('go-module',['go.mod']),
 ('migration',['migrations/0152_customer_import.sql']),('contract',['contracts/invariants.json']),
 ('untagged-foundation',['tests/foundation/orders_test.go']),('tagged-account',['tests/foundation/account_process_test.go']),
 ('tagged-studio',['tests/foundation/studio_process_test.go']),('browser-foundation',['tests/foundation/browser_customers_billing_test.go']),
 ('admin-ui',['apps/admin/components/Customers.tsx']),('storefront-ui',['apps/storefront/components/CheckoutFlow.tsx']),
 ('ui-package',['packages/ui/src/index.ts']),('github-gates',['.github/workflows/gates.yml']),
 ('deploy-workflow',['.github/workflows/deploy-smoke.yml']),('deploy-verdict',['.github/scripts/smoke-verdict.py']),
 ('deploy-script',['deploy/scripts/smoke.sh']),('deploy-r3-script',['deploy/scripts/smoke-r3.sh']),
 ('deploy-config',['deploy/compose.yml']),('deploy-node-tests',['tests/deploy/deploy-prep-r3.test.mjs']),
 ('general-script',['scripts/dev/check-gates.sh']),('legacy-deploy-script',['scripts/deploy-prep.sh']),
 ('ci-test',['tests/ci/pr-modes.test.mjs']),('app-readme',['apps/admin/NOTES.md']),('dockerignore',['.dockerignore']),
 ('mixed-backend-ui',['internal/customers/read.go','apps/admin/components/Customers.tsx']),
 ('mixed-tagged-deploy',['tests/foundation/account_process_test.go','.github/scripts/smoke-verdict.py']),
 ('unicode-doc',['架构.md']),('unicode-backend',['internal/示例.go']),
 ('tab-untagged-foundation',['tests/foundation/plain\tname_test.go']),
 ('space-deploy-prefix',[' deploy/scripts/synthetic-smoke.sh'])]
(root/'pathsets.json').write_text(json.dumps(cases,ensure_ascii=False,indent=2)+'\n')
js="""import {readFileSync} from 'node:fs'; import {pathToFileURL} from 'node:url';
const root=process.argv[1];const old=await import(pathToFileURL(root+'/old/scripts/dev/pr-modes.mjs'));const next=await import(pathToFileURL(root+'/candidate/scripts/dev/pr-modes.mjs'));
const usage=readFileSync(root+'/candidate/scripts/dev/test-local.sh','utf8');const cases=JSON.parse(readFileSync(root+'/pathsets.json','utf8'));
const legacyUsage=readFileSync(root+'/legacy-test-local.sh','utf8');
const rows=cases.map(([name,paths])=>{const before=old.planPr(paths,legacyUsage),after=next.planPr(paths,usage);return {name,paths,before,after,dropped:before.modes.filter(x=>!after.modes.includes(x)),deployDropped:before.deploy&&!after.deploy};}); console.log(JSON.stringify(rows));"""
api=json.loads(subprocess.check_output(['node','--input-type=module','-e',js,str(root)],text=True,cwd=root))
(root/'api-comparison.json').write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
def git(*args):return subprocess.check_output(['git','-c','core.hooksPath=/dev/null',*args],cwd=fixture,text=True,stderr=subprocess.PIPE).strip()
git('init','--quiet');git('config','user.name','Synthetic independent reviewer');git('config','user.email','reviewer@example.invalid');git('config','commit.gpgsign','false');git('config','core.quotePath','true')
for name,src in [('pr-modes.mjs',candidate/'scripts/dev/pr-modes.mjs'),('pr-modes-old.mjs',old/'scripts/dev/pr-modes.mjs'),('test-local.sh',candidate/'scripts/dev/test-local.sh')]:
 p=fixture/'scripts/dev'/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(src.read_bytes())
git('add','-A');git('commit','--quiet','-m','synthetic baseline');base=git('rev-parse','HEAD')
cli=[]
for i,(name,paths) in enumerate(cases):
 git('checkout','--quiet','--detach',base)
 for file in paths:
  p=fixture/file;p.parent.mkdir(parents=True,exist_ok=True)
  existing=p.read_text() if p.exists() else (candidate/file).read_text() if (candidate/file).is_file() else '// synthetic untagged source\n' if file.endswith('.go') else '# synthetic fixture\n'
  p.write_text(existing+'\n'+ ('#' if file.endswith(('.sh','.yml','.yaml','.py')) else '//')+f' independent case {i}\n')
 if paths:git('add','-A');git('commit','--quiet','-m',f'synthetic case {i}')
 head=git('rev-parse','HEAD')
 outputs=[]
 for script in ['pr-modes-old.mjs','pr-modes.mjs']:
  registry=fixture/'scripts/dev/test-local.sh'; saved_registry=registry.read_bytes()
  if script == 'pr-modes-old.mjs':registry.write_bytes((root/'legacy-test-local.sh').read_bytes())
  proc=subprocess.run(['node',str(fixture/'scripts/dev'/script),base,head],cwd=fixture,text=True,capture_output=True,env={**os.environ,'GITHUB_OUTPUT':''})
  registry.write_bytes(saved_registry)
  if proc.returncode:raise RuntimeError((name,script,proc.returncode,proc.stderr))
  outputs.append(json.loads(proc.stdout))
 before,after=outputs
 display=subprocess.check_output(['git','diff','--name-only',base+'...'+head],cwd=fixture,text=True)
 cli.append({'name':name,'paths':paths,'base':base,'head':head,'legacy_git_display':display,'before':before,'after':after,'dropped':list(set(before['modes'])-set(after['modes'])),'deployDropped':before['deploy'] and not after['deploy']})
(root/'cli-comparison.json').write_text(json.dumps(cli,ensure_ascii=False,indent=2)+'\n')
summary={'path_sets':len(cases),'api_drops':[x['name'] for x in api if x['dropped'] or x['deployDropped']], 'cli_drops':[{'name':x['name'],'old_modes':len(x['before']['modes']),'new_modes':len(x['after']['modes']),'deploy_dropped':x['deployDropped'],'display':x['legacy_git_display']} for x in cli if x['dropped'] or x['deployDropped']]}
(root/'comparison-summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n');print(json.dumps(summary,ensure_ascii=False))

