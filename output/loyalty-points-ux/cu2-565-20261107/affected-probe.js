// Affected native facts only; extends existing candidate geometry probe, no renderer or mutation.
const page=await figma.getNodeByIdAsync('4:8829');await page.loadAsync();
const sec=await figma.getNodeByIdAsync('1589:63500');
if(sec.type!=='SECTION'||sec.parent.id!==page.id||sec.id==='1554:62573')throw Error('candidate ownership');
const observations=[];
for(const [key,dev,id] of TARGETS){
 const f=await figma.getNodeByIdAsync(id);if(f.parent.id!==sec.id)throw Error('affected frame ownership');
 const texts=f.findAllWithCriteria({types:['TEXT']}).map(t=>t.characters);
 const checks=[];const check=(name,pass)=>checks.push({name,pass});
 if(key==='A8'||key==='A10')for(const fact of ['單價 NT$3,000','商品小計 NT$6,000','−NT$2,000','運費 NT$100','合計 NT$4,100'])check(fact,texts.some(t=>t.includes(fact)));
 const ledger=f.findOne(n=>n.name==='Points ledger');
 const rows=ledger?ledger.children.filter(n=>n.name.startsWith('Ledger item ·')||n.name.startsWith('Ledger row ·')).map(n=>({id:n.id,text:n.findAllWithCriteria({types:['TEXT']}).map(t=>t.characters),bounds:n.absoluteBoundingBox})):[];
 if(key==='C12'){
  const order=rows.map(r=>r.text.find(t=>/^[+−]\d+( 點)?$/.test(t))?.replace(' 點',''));
  check('four numeric rows in order',JSON.stringify(order)===JSON.stringify(['−50','+50','−30','+100']));
  check('balance70',texts.includes('70 點'));
  check('no restored member PII',!texts.some(t=>t.includes('王小美')||t.includes('LINE：mei0412')));
  check('no restored erased notes',!texts.some(t=>['買家退貨','兌換小禮物（現場）','舊會員卡點數轉入'].includes(t)));
 }
 const vp=f.findChild(n=>n.name==='Scrollable task viewport');const comp=vp?.children[0];
 observations.push({key,dev,id,texts,rows,checks,pass:checks.every(c=>c.pass),frame:{width:f.width,height:f.height},viewport:vp?{id:vp.id,y:vp.y,height:vp.height,overflowDirection:vp.overflowDirection,clipsContent:vp.clipsContent,absoluteY:vp.absoluteBoundingBox.y}:null,content:comp?{id:comp.id,height:comp.height,clipsContent:comp.clipsContent}:null});
}
return {observations,verdict:observations.every(o=>o.pass)?'GREEN':'RED',interactiveScroll:'NOT_RUN'};
