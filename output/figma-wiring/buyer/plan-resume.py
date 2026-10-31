# Purpose: explicit Buyer-only reaction plan, preserving click behavior and disabled controls.
# Depends on: live controls and bounded audit; Used by: Buyer wiring card.
# Invariants: no visual writes; outcome shortcuts documented; no self/cross-platform links.
import json,pathlib,re,collections
P=pathlib.Path(__file__).resolve().parent
rs=json.loads((P/'controls-resume.json').read_text());D={r['id']:r for r in rs};plan=[];keys=collections.Counter();clicks=set()
def side(r):return 'Desktop' if 'Desktop' in r['name'] else 'Mobile'
def find(root,pat):
 ns=[n for n in D[root]['nodes'] if (re.search(pat,n['n']) or re.search(pat,n['txt'])) and not re.search('disabled|while confirming',n['n'])]
 return next((n for n in ns if n['n'].startswith('Action /') or n['n'] in ['SubmitAction','Back']),ns[0] if ns else None)
def add(root,pat,dest,reason,click=False):
 n=find(root,pat)
 if not n:raise RuntimeError((root,pat))
 if root==dest or side(D[root])!=side(D[dest]):raise RuntimeError('invalid target')
 trigger={'type':'ON_CLICK'} if click else {'type':'ON_KEY_DOWN','device':'KEYBOARD','keyCodes':[list(range(49,58))+list(range(65,91))][0][keys[n['id']]:keys[n['id']]+1]}
 if not click:keys[n['id']]+=1
 if not trigger.get('keyCodes',[1]):raise RuntimeError('too many keys')
 plan.append({'root':root,'source':n['id'],'sourceName':n['n'],'destination':dest,'trigger':trigger,'replaceClick':click,'reason':reason})
 if click:clicks.add(n['id'])
# Preserve existing graphs, including clicks on header/footer and simulation controls.
g={r['id']:{a.get('destinationId') for n in r['nodes'] for z in n['r'] for a in z['actions'] if a.get('destinationId') in D} for r in rs}
for r in rs:
 s=side(r);online='1429:23435' if s=='Desktop' else '1429:23484'
 for n in r['nodes']:
  if any(a.get('navigation')=='NAVIGATE' and a.get('destinationId') not in D for z in n['r'] for a in z['actions']) and re.search('結帳|下單',n['txt']):
   add(r['id'],re.escape(n['n']),online,'Replace superseded checkout destination',True)
# Signed simplified checkout entry from each main cart.
for root,dest in [('4:9668','1429:23435'),('4:9746','1429:23484')]:
 if not any(p['root']==root and p['replaceClick'] for p in plan):add(root,'前往結帳',dest,'Signed simplified checkout entry',True)
# Both storefront live product cards lead to claim confirmation (then view product/cart).
for root,dest in [('6:21771','4:12347'),('6:21825','4:12417')]:add(root,'Product /',''+dest,'Live product claim entry',True)
# Checkout -> region picker; latest search picker tabs/rows and confirmations.
for root in [r['id'] for r in rs if r['id'].startswith('1429:') and '選店器' in r['name'] or r['id'] in ['1429:14065','1429:14122','1429:14294','1429:14359','1429:14519','1429:14578','1429:14738','1429:14800']]:
 r=D[root];s=side(r);off=0 if s=='Desktop' else 1
 targets={'region':['1429:13961','1429:14020'][off],'search':['1429:14065','1429:14122'][off],'loaded':['1429:14166','1429:14237'][off],'results':['1429:14294','1429:14359'][off],'selected':['1429:14624','1429:14688'][off],'searchSelected':['1429:14738','1429:14800'][off],'confirm':['1429:14849','1429:14914'][off],'searchConfirm':['1429:14965','1429:15028'][off]}
 for pat,k in [('Action / 依地區$|依縣市／區找門市','region'),('Action / 搜尋$|改用搜尋','search'),('Input / 選擇行政區','loaded'),('Store option /','searchSelected'),('Action / 更換門市|Action / 更換$','results')]:
  n=find(root,pat);dest=targets[k]
  if n and root!=dest and not n['r']:add(root,pat,dest,'Picker tab/selection',True)
 n=find(root,'Action / 確認取貨門市$');dest=targets['searchConfirm' if '搜尋' in r['name'] else 'confirm']
 if n and root!=dest and not n['r']:add(root,'Action / 確認取貨門市$',dest,'Confirm selected store',True)
 if 'checkout summary' in r['name'] and find(root,'確認下單'):add(root,'確認下單','1429:24505' if off==0 else '1429:24554','Return selected CVS to online checkout',True)
# Signed default checkout exposes picker via store details without changing copy.
for root,dest in [('1429:23781','1429:13961'),('1429:23832','1429:14020'),('782:3904','1429:13961'),('782:4087','1429:14020')]:
 pat='更換|門市' if root.startswith('782:') else 'ChooseOnline|ChooseCOD|SubmitAction'
 add(root,pat,dest,'Open CVS picker outcome branch')
# Apply proposed graph, then connect orphan states from their producing step with unique keyboard outcomes.
for p in plan:g[p['root']].add(p['destination'])
def reach():
 seen={'4:9192','4:9296'}
 while True:
  new=seen|{y for x in seen for y in g.get(x,[])}
  if new==seen:return seen
  seen=new
def producer(r):
 s=side(r);i=0 if s=='Desktop' else 1;n=r['name'];rid=r['id']
 if '確認直播認領' in n:return ['6:21771','6:21825'][i],'Product /'
 if any(x in n for x in ['認領連結失效','已在購物車中','目前無法購買','部分商品可購買','認領結果待確認']):return ['4:12347','4:12417'][i],'前往結帳'
 if '資料副本暫時' in n:return ['5:15460','5:15540'][i],'下載我的資料'
 if any(x in n for x in ['訂單連結已失效','已完成訂單','已取消訂單','訂單查詢查不到']):return ['26:708','26:995'][i],'Action / 查詢訂單'
 if '沒有符合的商品' in n:return ['4:9192','4:9296'][i],'Field / 搜尋'
 if '商品已下架' in n:return ['6:19772','6:19859'][i],'商品|所有商品'
 if '店鋪暫停' in n or '網店 · 線上版本' in n:return ['4:9192','4:9296'][i],'探索日常選物'
 if 'Shared cart' in n:return ['4:9668','4:9746'][i],'前往結帳'
 if '結帳' in n:
  if 'v2' in n:
   mode=next((x for x in ['cvs-online','pickup','cod','online'] if '· '+x+' ·' in n),'online')
   root={'online':['1429:23435','1429:23484'],'pickup':['1429:23781','1429:23832'],'cod':['1429:24143','1429:24194'],'cvs-online':['1429:24505','1429:24554']}[mode][i]
   if root==rid: return ['4:9668','4:9746'][i],'前往結帳'
   return root,'SubmitAction'
  return ['782:3904','782:4087'][i],'確認下單'
 if '門市' in n or '取貨付款' in n:
  if '選店器' in n or '門市 · 搜尋' in n or '核對門市' in n:
   if '依地區/initial' in n:return ['1429:23781','1429:23832'][i],'ChooseOnline|ChooseCOD|SubmitAction'
   if '搜尋／空白' in n:return ['1429:13961','1429:14020'][i],'Action / 搜尋'
   if 'list loaded' in n:return ['1429:13961','1429:14020'][i],'Input / 選擇行政區'
   if '依地區/no results' in n:return ['1429:13961','1429:14020'][i],'Input / 選擇行政區'
   if '找不到門市' in n:return ['1429:14065','1429:14122'][i],'搜尋|門市'
   if '依地區/selected' in n:return ['1429:14166','1429:14237'][i],'Action / 7-ELEVEN'
   if '依地區/confirming' in n:return ['1429:14624','1429:14688'][i],'確認取貨門市'
   if '搜尋/confirming' in n or '核對門市' in n:return ['1429:14738','1429:14800'][i],'確認取貨門市'
   if any(x in n for x in ['server refused','busy/429','checkout summary']):return ['1429:14738','1429:14800'][i] if '搜尋' in n else ['1429:14624','1429:14688'][i],'確認取貨門市'
   return ['1429:13961','1429:14020'][i],'Action / 搜尋'
  return ['782:3904','782:4087'][i],'更換或修改門市|更換|門市'
 if '訂單 LC-' in n or '付款' in n:return ['4:9963','4:10049'][i],'重新查詢付款狀態|付款'
 if '收件資訊' in n:return ['782:3904','782:4087'][i],'Input / 收件人'
 if '購物車' in n:return ['4:9668','4:9746'][i],'前往結帳'
 return None
# Only connect missing roots via explicit semantic producer; never a generic home fallback.
remaining=[]
for r in rs:
 if r['id'] in reach():continue
 pair=producer(r)
 if not pair:remaining.append(r['id']);continue
 root,pat=pair
 if root==r['id']:remaining.append(r['id']);continue
 add(root,pat,r['id'],'Alternate outcome / variant from producing step; keyboard branch')
 g[root].add(r['id'])
(P/'plan-resume.json').write_text(json.dumps(plan,ensure_ascii=False,indent=2));print('planned',len(plan),'reachable',len(reach()),'remaining',[r['id'] for r in rs if r['id'] not in reach()]);print('unclassified',remaining)
