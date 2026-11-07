# Scoped adapter runner: existing caller only; persists arguments, hashes and raw receipts before interpreting.
import json,subprocess,hashlib,uuid,datetime,sys,os
from pathlib import Path
OUT=Path(__file__).resolve().parent
CALL='/Volumes/data/live_commerce_architecture_v1/output/integrator/tools/figma-local/call.mjs'
CONN='2baeb2df-5951-49db-81fa-cbbb3deb9b3b'
def call(label,tool,args):
 args={**args,'connectionId':CONN}; req=OUT/(label+'.request.json');req.write_text(json.dumps(args,ensure_ascii=False))
 entry={'pid':os.getpid(),'candidate_target':'1589:63500','label':label,'tool':tool,'started':datetime.datetime.now(datetime.timezone.utc).isoformat(),'request_sha256':hashlib.sha256(req.read_bytes()).hexdigest(),'code_sha256':hashlib.sha256(args.get('code','').encode()).hexdigest(),'deadline_seconds':130,'native_job_id':'NOT_EXPOSED_BY_CALLER','wake':'Orca exec completion / worker_done'}
 with (OUT/'operations.jsonl').open('a') as h:h.write(json.dumps(entry)+'\n')
 receipt=OUT/(label+'.receipt.txt')
 with receipt.open('w') as stream:
  p=subprocess.run(['node',CALL,tool,'@'+str(req),'--out',str(OUT/'figma')],stdout=stream,stderr=stream,text=True,timeout=130)
 raw=receipt.read_text()
 with (OUT/'operations.jsonl').open('a') as h:h.write(json.dumps({'label':label,'exit':p.returncode,'ended':datetime.datetime.now(datetime.timezone.utc).isoformat()})+'\n')
 if p.returncode: raise RuntimeError('STOP '+label+': '+raw[:900])
 return raw

def script(label,code):return call(label,'use_figma',{'code':code,'description':'CU2 '+label,'skillNames':'figma-workbench'})
if __name__=='__main__':
 action=sys.argv[1]
 if action=='snapshot':
  phase=sys.argv[2]; inv=json.loads((OUT/'formal-inventory-before.json').read_text()); ids=[x['id'] for x in inv['children']]
  for i,nid in enumerate(ids):
   label=f'formal-{phase}-{i:02d}'
   code="""const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const root=await figma.getNodeByIdAsync(NID);if(root.parent.id!=='1554:62573')throw Error('formal parent drift');const rows=[];const props=['id','name','type','visible','locked','x','y','width','height','rotation','opacity','blendMode','fills','strokes','effects','boundVariables','explicitVariableModes','layoutMode','layoutSizingHorizontal','layoutSizingVertical','primaryAxisSizingMode','counterAxisSizingMode','itemSpacing','paddingTop','paddingBottom','paddingLeft','paddingRight','clipsContent','overflowDirection','characters','fontName','fontSize','textAutoResize','textTruncation','maxLines','reactions'];function walk(n){const r={};for(const k of props){try{if(n[k]!==undefined)r[k]=n[k];}catch{}}r.children=n.children?n.children.map(c=>c.id):[];rows.push(r);if(n.children)for(const c of n.children)walk(c);}walk(root);return {rootId:root.id,rows};""".replace('NID',json.dumps(nid))
   script(label,code)
   print(label,nid,flush=True)
