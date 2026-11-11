# Purpose: requested reach/wire audit with bounded Fleet Bridge reads.
# Depends on: resume.py CLI receipt helper; Used by: Buyer wiring acceptance.
# Invariants: <=10 roots per call; no writes, retries, tests, or servers.
import runpy,json,re,sys
m=runpy.run_path(__file__.replace('audit-resume.py','resume.py'));call=m['call'];P=m['P'];phase=sys.argv[1]
roots=[]
for i in range(0,2200,200):
 part=call(f'{phase}-roots-{i}',f"const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();return p.children.slice({i},{i+200}).map(n=>({{id:n.id,name:n.name,type:n.type}}));")
 roots+=part
 if len(part)<200:break
active=[n for n in roots if n['type']=='FRAME' and re.search('Desktop|Mobile',n['name']) and not re.search('已被取代|非主路徑|superseded',n['name'],re.I)]
rows=[]
for i in range(0,len(active),10):
 code="const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const out=[];for(const id of "+json.dumps([n['id'] for n in active[i:i+10]])+""" ){const t=await figma.getNodeByIdAsync(id);const es=[];const ns=t.findAll(()=>true);for(const n of ns)for(const r of n.reactions||[])for(const a of (r.actions||[r.action]).filter(Boolean))if(a.destinationId||a.type==='BACK'||a.navigation==='CHANGE_TO')es.push([a.type,a.navigation||null,a.destinationId||null]);let h=2166136261; if(t.name.includes('Buyer ')){for(const n of [t,...ns]){const s=JSON.stringify([n.id,n.name,n.type,n.visible,n.x,n.y,n.width,n.height,n.characters,n.fills,n.strokes,n.layoutMode,n.paddingLeft,n.paddingRight,n.paddingTop,n.paddingBottom,n.itemSpacing]);for(let k=0;k<s.length;k++){h^=s.charCodeAt(k);h=Math.imul(h,16777619);}}}out.push({id:t.id,name:t.name,product:!/SPEC/i.test(t.name)&&ns.some(n=>n.type!=='TEXT'),edges:es,visualHash:t.name.includes('Buyer ')?(h>>>0).toString(16):null});}return out;"""
 rows+=call(f'{phase}-audit-{i}',code)
ids={n['id'] for n in rows if n['product']};g={n['id']:{e[2] for e in n['edges'] if e[2] in ids} for n in rows};entries=[n['id'] for n in rows if n['product'] and re.search(r'工作台|歡迎來到大夢甄選|登入商家工作區|^.*·\s*登入\b',n['name'])]
def reach(start):
 seen=set(start)
 while True:
  s=seen|{y for x in seen for y in g.get(x,[])}
  if s==seen:return seen
  seen=s
seen=reach(entries);un={}
for n in rows:
 if n['id'] in ids and n['id'] not in seen:
  match=re.search(r'(?:Desktop|Mobile)\s*·\s*([^·]+)',n['name']);k=('buyer' if 'Buyer' in n['name'] else 'merchant')+':'+(match[1].strip()[:10] if match else '?');un[k]=un.get(k,0)+1
buyer={}
for side,home in [('Desktop','4:9192'),('Mobile','4:9296')]:
 b=[n for n in rows if n['id'] in ids and 'Buyer' in n['name'] and side in n['name']];ss=reach([home]);buyer[side]={'total':len(b),'reached':sum(n['id'] in ss for n in b),'unreached':[n['id'] for n in b if n['id'] not in ss]}
summary={'entries':len(entries),'total':len(ids),'reached':len(seen),'unreached':un,'buyer':buyer,'changeTo':sum(e[1]=='CHANGE_TO' for n in rows for e in n['edges']),'bad':sum(e[1] in ['NAVIGATE','OVERLAY','SWAP'] and (not e[2] or e[2]==n['id']) for n in rows for e in n['edges']),'crossPlatformBuyer':[{'root':n['id'],'destination':e[2]} for n in rows if 'Buyer' in n['name'] for e in n['edges'] if e[2] in g and ('Desktop' in n['name'])!=('Desktop' in next(t['name'] for t in rows if t['id']==e[2]))]}
(P/f'audit-{phase}.json').write_text(json.dumps({'summary':summary,'rows':rows},ensure_ascii=False));print(json.dumps(summary,ensure_ascii=False))
