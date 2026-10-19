#!/usr/bin/env python3
# Purpose: reproduce seven review bypasses and closure sinks, without Go tests.
# Depends on: task-owned shared clone and bounded Round10 receipt runner.
# Invariant: each mutation is restored in finally; source and binaries are hashed.
import hashlib,json,pathlib,subprocess,sys
out=pathlib.Path(__file__).resolve().parent
repo=out/'scratch/repo'
cli=out/'scratch'/('old-cli' if sys.argv[1]=='red' else 'new-cli')
phase=sys.argv[1]
handler=repo/'internal/buyerhttp/handler.go'
routes=repo/'internal/buyerhttp/routes.go'
feed=repo/'internal/attribution/feed.go'
cases={
 'pointer-alias':(handler,'New', 'p := v2Slug; *p = *regexp.MustCompile(".*");', ''),
 'pointer-helper':(handler,'New','widen(v2Slug);','func widen(p *regexp.Regexp){*p = *regexp.MustCompile(".*")}'),
 'pointer-method':(handler,'New','v2Slug.Longest();',''),
 'slice-alias':(feed,None,'','func unusedAlias(){c := feedColumns; c[0]="sku"}'),
 'slice-helper':(feed,None,'','func unusedClobber(){clobber(feedColumns)};func clobber(p []string){p[0]="sku"}'),
 # Existing closure function's result is not needed to demonstrate the escape.
 'return-escape':(handler,None,'','func leak() *regexp.Regexp{return v2Slug}'),
 'goroutine-capture':(handler,'New','go func(){v2Slug.Longest()}();',''),
}
if phase=='inside':
 cases={
  'return':(handler,'keyFor','_ = leak();','func leak() *regexp.Regexp{return v2Slug}'),
  'goroutine':(handler,'keyFor','go func(){v2Slug.Longest()}();',''),
  'method':(handler,'keyFor','v2Slug.Longest();',''),
  'local-alias':(handler,'keyFor','p:=v2Slug; p.Longest();',''),
  'field':(handler,'keyFor','sink:=struct{P *regexp.Regexp}{};sink.P=v2Slug;',''),
  'map':(handler,'keyFor','sink:=map[int]*regexp.Regexp{};sink[0]=v2Slug;',''),
  'slice':(handler,'keyFor','_=[]*regexp.Regexp{v2Slug};',''),
  'channel':(handler,'keyFor','ch:=make(chan *regexp.Regexp,1);ch<-v2Slug;',''),
  'helper':(handler,'keyFor','widen(v2Slug);','func widen(p *regexp.Regexp){*p = *regexp.MustCompile(".*")}'),
  'initializer':(handler,None,'','var alias=v2Slug'),
  'init-method':(handler,None,'','func init(){v2Slug.Longest()}'),
  'method-outside':(handler,None,'','func (*handler) mutate(){v2Slug.Longest()}'),
 }
results={}
for name,(path,fn,body,extra) in cases.items():
 original=path.read_text()
 try:
  source=original
  if fn:
   start=source.index('func '+fn+'('); pos=source.index('{',start)
   source=source[:pos+1]+body+source[pos+1:]
  source+='\n'+extra+'\n'
  path.write_text(source)
  label=phase+'-'+name
  if phase!='red':
   subprocess.run([sys.executable,str(out/'run.py'),label+'-build',str(repo),'go','build','./internal/buyerhttp','./internal/attribution'],capture_output=True,timeout=190)
   build=json.loads((out/(label+'-build-receipt.json')).read_text())
   assert build['exit_code']==0,(name,build)
  subprocess.run([sys.executable,str(out/'run.py'),label,str(repo),str(cli),'-root',str(repo),'-base','e4dad98cc'],capture_output=True,timeout=190)
  rec=json.loads((out/(label+'-receipt.json')).read_text());report=(out/(label+'-report.txt')).read_text()
  frozen=[l for l in report.splitlines() if 'ERROR (dispatcher freeze)' in l]
  results[name]={'exit':rec['exit_code'],'summary':rec['summary'],'freeze_errors':len(frozen),'first':frozen[:1],'mutation_sha256':hashlib.sha256(source.encode()).hexdigest()}
  if phase=='red':
   assert rec['exit_code']==0 and rec['errors']==0,results[name]
  else: assert rec['exit_code']==1 and frozen and 'frozen var' in frozen[0],results[name]
 finally:path.write_text(original)
(out/(phase+'-map.json')).write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results,indent=2))
