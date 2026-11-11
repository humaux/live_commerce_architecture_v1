# Purpose: bounded Buyer prototype inspection and receipt capture.
# Depends on: Fleet Bridge CLI; Used by: Buyer wiring card.
# Invariants: no tests/servers; <=10 roots per read; no automatic retry.
import json,pathlib,subprocess,sys
P=pathlib.Path(__file__).resolve().parent
CLI='/Volumes/data/live_commerce_architecture_v1/output/integrator/tools/figma-local/call.mjs'
def call(label,code):
 a=P/'args'/f'resume-{label}.json';a.write_text(json.dumps({'code':code,'skillNames':'figma-use'}))
 r=subprocess.run(['node',CLI,'use_figma','@'+str(a)],capture_output=True,text=True,timeout=55)
 (P/'evidence'/f'resume-{label}.json').write_text(r.stdout)
 if r.returncode:raise RuntimeError(r.stderr or r.stdout[:300])
 return json.loads(r.stdout)
if __name__=='__main__':
 roots=[{'id':x[0],'name':x[1],'type':'FRAME'} for x in json.load(open(P/'live-buyers.json'))['buyers']]
 buyers=[x for x in roots if not any(s in x['name'] for s in ['SPEC','已被取代','非主路徑'])]
 out=[]
 for i in range(0,len(buyers),1):
  code="const p=await figma.getNodeByIdAsync('4:8829'); await p.loadAsync(); const out=[]; for(const id of "+json.dumps([x['id'] for x in buyers[i:i+1]])+""" ){const t=await figma.getNodeByIdAsync(id); const ns=t.findAll(n=>n.type!=='TEXT'&&((n.reactions||[]).length||/Action|Button|CTA|link|返回|確認|繼續|查詢|搜尋|門市|選擇|取消|重試|上一|下一|前往|付款|訂單|商品|資料|隱私|同意|收件|empty|error|expired/i.test(n.name))); out.push({id:t.id,name:t.name,nodes:ns.map(n=>({id:n.id,n:n.name,p:n.parent.id,t:n.type,txt:n.findAll?n.findAll(c=>c.type==='TEXT').map(c=>c.characters).join(' ').slice(0,70):'',r:(n.reactions||[]).map(r=>({trigger:r.trigger,actions:(r.actions||[r.action]).map(a=>({type:a.type,navigation:a.navigation,destinationId:a.destinationId}))}))}))});}return out;"""
  out.extend(call(f'controls-{i:03}',code))
  print(f'controls {min(i+1,len(buyers))}/{len(buyers)}',flush=True)
 (P/'controls-resume.json').write_text(json.dumps(out,ensure_ascii=False))
