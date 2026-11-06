# Purpose: add bounded main-journey state transitions to explicit wiring plan.
# Depends on: Buyer controls and Fleet Bridge; Used by: Buyer wiring card.
# Invariants: source/destination same platform; reactions only.
import runpy,json
m=runpy.run_path(__file__.replace('finish-plan.py','resume.py'));call=m['call'];P=m['P']
plan=json.loads((P/'plan-resume.json').read_text());rs=json.loads((P/'controls-resume.json').read_text());D={r['id']:r for r in rs}
for root,source,dest in [('1429:14166','1429:14204','1429:14624'),('1429:14237','1429:14262','1429:14688')]:
 plan.append({'root':root,'source':source,'sourceName':'Store option / 121275 / whole-row target','destination':dest,'trigger':{'type':'ON_CLICK'},'replaceClick':True,'reason':'Select actual region result row'})
trans=[]
for r in rs:
 name=r['name'];desktop='Desktop' in name
 if 'v2' in name and '· submitting' in name:
  dest=('1198:14815' if desktop else '1198:15153') if '· pickup ·' in name else ('1222:21795' if desktop else '1222:22313') if '· cod ·' in name else ('25:943' if desktop else '25:995') if '· cvs-online ·' in name else ('4:9963' if desktop else '4:10049')
  trans.append([r['id'],dest])
 elif '選店器' in name and '/confirming' in name:
  dest=('1429:15686' if desktop else '1429:15752') if '搜尋/' in name else ('1429:15568' if desktop else '1429:15634');trans.append([r['id'],dest])
for i in range(0,len(trans),10):
 b=trans[i:i+10];out=call(f'timer-sources-{i}',"const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const out=[];for(const [root,destination] of "+json.dumps(b)+"){const t=await figma.getNodeByIdAsync(root);const n=t.children.find(n=>n.type==='FRAME');out.push({root,source:n.id,sourceName:n.name,destination});}return out;")
 for x in out:plan.append({**x,'trigger':{'type':'AFTER_TIMEOUT','timeout':1.2},'replaceClick':False,'reason':'Prototype submission/confirmation resolves to server-observed order/summary'})
# Real store-change entry and signed summary mode are ordinary clicks.
for root,pat,dest in [('782:3904','Action / 更換或修改門市','1429:13961'),('782:4087','Action / 更換或修改門市','1429:14020')]:
 n=next(n for n in D[root]['nodes'] if n['n']==pat)
 plan.append({'root':root,'source':n['id'],'sourceName':n['n'],'destination':dest,'trigger':{'type':'ON_CLICK'},'replaceClick':True,'reason':'Open signed CVS picker'})
# Remove redundant older click overrides for the same source; preserve all keyboard outcomes.
last={x['source']:i for i,x in enumerate(plan) if x['replaceClick']};plan=[x for i,x in enumerate(plan) if not x['replaceClick'] or last[x['source']]==i]
(P/'plan-resume.json').write_text(json.dumps(plan,ensure_ascii=False,indent=2));print('plan',len(plan),'sources',len({x['source'] for x in plan}))
