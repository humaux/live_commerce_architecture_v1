# Purpose: backup and apply Buyer-only reactions, with bounded readbacks and no replay.
# Depends on: explicit plan and Fleet Bridge; Used by: Buyer wiring card.
# Invariants: reactions only; <=20 sources/batch; abort on uncertain result.
import runpy,json,sys,subprocess,struct
m=runpy.run_path(__file__.replace('apply-resume.py','resume.py'));call=m['call'];P=m['P'];CLI=m['CLI'];mode=sys.argv[1]
plan=json.loads((P/'plan-resume.json').read_text());ids=list(dict.fromkeys(p['source'] for p in plan))
def norm(x):
 if isinstance(x,float):return struct.unpack("f",struct.pack("f",x))[0]
 if isinstance(x,list):return [norm(v) for v in x]
 if isinstance(x,dict):return {k:norm(v) for k,v in x.items() if not(k=='action' and 'actions' in x) and not(k=='resetVideoPosition' and v is False)}
 return x
if mode=='backup':
 out=[]
 for i,id in enumerate(ids):
  out+=call(f'backup-{i}',"const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const n=await figma.getNodeByIdAsync("+json.dumps(id)+");let t=n;while(t.parent&&t.parent!==p)t=t.parent;if(!t.name.includes('Buyer ')||t.type!=='FRAME')throw Error('wrong owner');return [{id:n.id,root:t.id,reactions:n.reactions||[]}];")
 (P/'rollback-reactions.json').write_text(json.dumps(out,ensure_ascii=False));print('backup',len(out))
elif mode=='apply':
 backup={n['id']:n for n in json.loads((P/'rollback-reactions.json').read_text())}
 for i in range(int(sys.argv[2]) if len(sys.argv)>2 else 0,len(ids),20):
  batch=[]
  for id in ids[i:i+20]:
   rs=backup[id]['reactions']
   for item in [p for p in plan if p['source']==id]:
    if item['replaceClick']:rs=[r for r in rs if r['trigger']['type']!='ON_CLICK']
    elif any(r['trigger']==item['trigger'] for r in rs):raise RuntimeError('shortcut conflict '+id)
    rs=rs+[{'trigger':item['trigger'],'actions':[{'type':'NODE','navigation':'NAVIGATE','destinationId':item['destination'],'transition':None}]}]
   batch.append({'id':id,'root':backup[id]['root'],'reactions':rs})
  code="const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const batch="+json.dumps(batch)+";for(const x of batch){const n=await figma.getNodeByIdAsync(x.id);let t=n;while(t.parent&&t.parent!==p)t=t.parent;if(t.id!==x.root||!t.name.includes('Buyer ')||t.type!=='FRAME')throw Error('owner');for(const r of x.reactions)for(const a of r.actions||[]){if(a.navigation==='NAVIGATE'){const d=await figma.getNodeByIdAsync(a.destinationId);if(!d||d.parent!==p||d.type!=='FRAME'||d===t||(/Desktop/.test(t.name)!==/Desktop/.test(d.name)))throw Error('invalid destination');}}}const mutatedNodeIds=[];for(const x of batch){const n=await figma.getNodeByIdAsync(x.id);await n.setReactionsAsync(x.reactions);mutatedNodeIds.push(n.id);}return {mutatedNodeIds};"
  result=call(f'write-{i}',code)
  if result.get('mutatedNodeIds')!=[x['id'] for x in batch]:raise RuntimeError('uncertain write')
  read=call(f'readback-{i}',"const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const out=[];for(const id of "+json.dumps([x['id'] for x in batch])+"){const n=await figma.getNodeByIdAsync(id);out.push({id,reactions:n.reactions});}return out;")
  for x,y in zip(batch,read):
   if x['id']!=y['id'] or norm(x['reactions'])!=norm(y['reactions']):raise RuntimeError('readback mismatch '+x['id'])
  run=subprocess.run(['node',CLI,'get_metadata',json.dumps({'nodeId':batch[0]['root'],'depth':0})],capture_output=True,text=True,timeout=55)
  (P/'evidence'/f'resume-metadata-{i}.json').write_text(run.stdout)
  if run.returncode:raise RuntimeError('metadata read failed')
  print('written/read back',min(i+20,len(ids)),'/',len(ids),flush=True)
