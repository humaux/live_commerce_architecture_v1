const PAGE=await figma.getNodeByIdAsync('4:8829'); await PAGE.loadAsync();
const FAM='Noto Sans SC';
for (const s of ['Regular','Medium']) await figma.loadFontAsync({family:FAM,style:s});
await figma.loadFontAsync({family:'PingFang SC',style:'Medium'});
const VAR={};
async function collect(n){for(const k of ['fills','strokes']){const a=n[k];if(a&&Array.isArray(a))for(const p of a)if(p.boundVariables&&p.boundVariables.color){const v=await figma.variables.getVariableByIdAsync(p.boundVariables.color.id);if(v&&!VAR[v.name])VAR[v.name]=v;}}if(n.children&&n.type!=='INSTANCE')for(const c of n.children)await collect(c);}
await collect(await figma.getNodeByIdAsync('1470:167194'));
await collect(await figma.getNodeByIdAsync('1429:34299'));
const ALLV=await figma.variables.getLocalVariablesAsync('COLOR');
const COLL=VAR['color/ink'].variableCollectionId;
function V(n){if(VAR[n])return VAR[n];if(n.endsWith('@13')){const b=n.slice(0,-3);const v=ALLV.find(x=>x.name===b&&x.variableCollectionId.endsWith(':13:305'));if(!v)throw new Error('missing var '+n);VAR[n]=v;return v;}const v=ALLV.find(x=>x.name===n&&x.variableCollectionId===COLL)||ALLV.find(x=>x.name===n);if(!v)throw new Error('missing var '+n);VAR[n]=v;return v;}
const CONSUMER=await figma.getNodeByIdAsync('1470:167194');
function P(n,op){const v=V(n);const r=v.resolveForConsumer(CONSUMER).value;const p=figma.variables.setBoundVariableForPaint({type:'SOLID',color:{r:r.r,g:r.g,b:r.b}},'color',v);return Object.assign({},p,{opacity:op==null?1:op});}
function T(s,o={}){const t=figma.createText();t.fontName={family:FAM,style:o.w||'Regular'};t.characters=s;t.fontSize=o.size||14;t.lineHeight={unit:'PERCENT',value:150};t.fills=[P(o.c||'color/ink')];t.name=o.name||s.slice(0,30);if(o.u)t.textDecoration='UNDERLINE';if(o.align)t.textAlignHorizontal=o.align;return t;}
function add(p,c,fill){p.appendChild(c);if(fill&&p.layoutMode&&p.layoutMode!=='NONE'){c.layoutSizingHorizontal='FILL';if(c.type==='TEXT')c.textAutoResize='HEIGHT';}return c;}
function AL(dir,o={}){const f=figma.createFrame();f.name=o.name||'Group';f.layoutMode=dir==='h'?'HORIZONTAL':'VERTICAL';f.primaryAxisSizingMode='AUTO';f.counterAxisSizingMode='AUTO';f.itemSpacing=o.gap==null?8:o.gap;const p=o.pad==null?0:o.pad;if(Array.isArray(p)){f.paddingTop=p[0];f.paddingRight=p[1];f.paddingBottom=p[2];f.paddingLeft=p[3];}else{f.paddingTop=f.paddingRight=f.paddingBottom=f.paddingLeft=p;}f.fills=o.fill?[P(o.fill,o.fillOp)]:[];if(o.stroke){f.strokes=[P(o.stroke)];f.strokeWeight=o.sw||1;f.strokeAlign='INSIDE';}if(o.r)f.cornerRadius=o.r;if(o.align)f.counterAxisAlignItems=o.align;if(o.justify)f.primaryAxisAlignItems=o.justify;f.clipsContent=false;return f;}
let SEC=PAGE.children.find(n=>n.type==='SECTION'&&n.name==='會員積分 v1 · DESIGN');
if(!SEC){SEC=figma.createSection();SEC.name='會員積分 v1 · DESIGN';PAGE.appendChild(SEC);SEC.x=490000;SEC.y=605000;SEC.resizeWithoutConstraints(20000,7000);}
async function BTN(kind,label,o={}){let b;
 if(kind==='danger'){const set=await figma.getNodeByIdAsync('4:12718');const c=set.children.find(c=>c.name.includes('Kind=Danger')&&c.name.includes('State=Default'));b=c.createInstance();SEC.appendChild(b);}
 else{b=(await figma.getNodeByIdAsync(kind==='pri'?'1470:167239':'1470:167241')).clone();SEC.appendChild(b);}
 await b.setReactionsAsync([]);
 b.name='Action · '+label+(kind==='dis'?' · disabled':'');b.opacity=1;
 const tx=b.findOne(n=>n.type==='TEXT');await figma.loadFontAsync(tx.fontName);tx.characters=label;
 if(kind==='dis'){b.fills=[P('color/subtle')];b.strokes=[];tx.fills=[P('color/muted@13')];}
 b.resize(o.w||200,44);return b;}
async function TAG(tone,label){const set=await figma.getNodeByIdAsync('1401:38432');const c=set.children.find(c=>c.name.includes('Tone='+tone));const i=c.createInstance();SEC.appendChild(i);const tx=i.findOne(n=>n.type==='TEXT');await figma.loadFontAsync(tx.fontName);tx.characters=label;i.name='Tag / '+label;return i;}
function FIELD(label,value,o={}){const f=AL('v',{name:'Field / '+label,gap:6});
 add(f,T(label,{size:13,c:'color/muted',name:'Label'}),true);
 const box=AL('h',{name:'Value / '+label,pad:[10,12,10,12],fill:'color/subtle',r:5,align:'CENTER',gap:8});
 if(o.err){box.strokes=[P('color/danger')];box.strokeWeight=1.5;box.strokeAlign='INSIDE';}else if(o.focus){box.strokes=[P('color/teal')];box.strokeWeight=1.5;box.strokeAlign='INSIDE';}
 add(f,box,true);
 const tv=T(value,{size:15,c:o.ph?'color/muted@13':'color/ink',name:o.ph?'Placeholder':'Value'});add(box,tv,true);
 if(o.suffix)add(box,T(o.suffix,{size:14,c:'color/muted@13',name:'Suffix'}),false);
 if(o.tall){box.counterAxisAlignItems='MIN';box.counterAxisSizingMode='FIXED';box.resize(box.width,88);}
 if(o.err)add(f,T(o.err,{size:13,c:'color/danger',name:'Inline error'}),true);
 if(o.help)add(f,T(o.help,{size:13,c:'color/muted',name:'Helper'}),true);
 return f;}
function CELL(n,w){if(n.type==='TEXT'){n.textAutoResize='HEIGHT';n.resize(w,n.height);return n;}const c=AL('h',{name:'Cell',align:'CENTER'});c.appendChild(n);c.primaryAxisSizingMode='FIXED';c.resize(w,c.height);return c;}
function ROW(cells,widths,o={}){const r=AL('h',{name:o.name||'Table row',pad:[12,0,12,0],gap:12,align:'CENTER'});r.strokes=[P('color/line')];r.strokeTopWeight=0;r.strokeLeftWeight=0;r.strokeRightWeight=0;r.strokeBottomWeight=1;r.strokeAlign='INSIDE';
 cells.forEach((c,i)=>{let n=typeof c==='string'?T(c,{size:o.head?13:14,c:o.head?'color/muted':'color/ink'}):c;const cell=CELL(n,widths[i]||100);r.appendChild(cell);if(!widths[i])cell.layoutGrow=1;if(o.trunc&&o.trunc.includes(i)&&n.type==='TEXT'){n.textTruncation='ENDING';n.maxLines=1;}});return r;}
function MICON(){const f=figma.createFrame();f.name='Member icon / 18px';f.resize(18,18);f.fills=[];const e=figma.createEllipse();e.resize(6,6);f.appendChild(e);e.x=6;e.y=2;e.fills=[];e.strokes=[P('color/teal')];e.strokeWeight=1.5;const v=figma.createVector();v.vectorPaths=[{windingRule:'NONE',data:'M 3 16 C 3 12.5 5.7 10.5 9 10.5 C 12.3 10.5 15 12.5 15 16'}];f.appendChild(v);v.fills=[];v.strokes=[P('color/teal')];v.strokeWeight=1.5;v.strokeCap='ROUND';v.x=3;v.y=10.5;return f;}
async function SHELL(dev,name,crumb,title,nav,x,y){
 for(const old of SEC.children.filter(n=>n.name===name))old.remove();
 const f=(await figma.getNodeByIdAsync(dev==='D'?'1470:166550':'1470:166602')).clone();SEC.appendChild(f);f.name=name;f.x=x;f.y=y;
 const tn=f.children.find(n=>n.type==='TEXT');tn.characters=title;tn.name=title;
 const vp=f.findChild(n=>n.name==='Scrollable task viewport');const comp=vp.children[0];for(const c of [...comp.children])c.remove();
 const dock=f.findChild(n=>n.name==='Fixed action dock');for(const c of [...dock.children])c.remove();
 if(dev==='D'){const hd=f.findChild(n=>n.name==='Shell A / header');const cr=hd.findOne(n=>n.type==='TEXT');cr.characters=crumb;const lg=hd.findOne(n=>n.type==='TEXT'&&n.characters==='繁中');if(lg)lg.y=Math.round((hd.height-lg.height)/2);
  const nv=f.findChild(n=>n.name==='Merchant navigation');const man=nv.findChild(n=>n.name==='Navigation / 手工單');const cust=nv.findChild(n=>n.name==='Navigation / 顧客');const ord=nv.findChild(n=>n.name==='Navigation / 訂單');
  const mem=man.clone();nv.insertChild(nv.children.indexOf(cust)+1,mem);await mem.setReactionsAsync([]);mem.name='Navigation / 會員';
  mem.findOne(n=>n.type==='TEXT').characters='會員';const vec=mem.findOne(n=>n.type==='VECTOR');const ic=MICON();mem.appendChild(ic);ic.x=vec.x;ic.y=vec.y;vec.remove();
  if(nav==='members'){const oa=JSON.parse(JSON.stringify(ord.fills));const cd=JSON.parse(JSON.stringify(cust.fills));ord.setProperties({State:'Default'});cust.setProperties({State:'Active'});cust.fills=oa;ord.fills=cd;man.fills=[];mem.fills=[P('color/subtle')];}
  else mem.fills=[];}
 return {f,comp,dock,D:dev==='D',W:dev==='D'?1136:358};}
function CARD(sh,name,gap){const c=AL('v',{name:name||'Task surface',gap:gap==null?16:gap,pad:sh.D?28:16,fill:'color/surface',r:7});add(sh.comp,c,true);return c;}
async function DOCK(sh,btns){sh.dock.visible=btns.length>0;for(const [k,l] of btns){const b=await BTN(k,l,{w:sh.D?200:163});sh.dock.appendChild(b);if(!sh.D)b.layoutGrow=1;}}
function SPLIT(parent,left,right,o={}){const r=AL('h',{name:o.name||'Row',justify:'SPACE_BETWEEN',align:o.align||'CENTER',gap:12});add(parent,r,true);r.appendChild(left);if(left.type==='TEXT'){left.layoutGrow=1;left.textAutoResize='HEIGHT';}if(right)r.appendChild(right);return r;}
function OVERLAY(sh,title){const s=figma.createRectangle();s.name='Scrim';s.resize(sh.f.width,sh.f.height);s.fills=[P('color/ink')];s.opacity=0.45;sh.f.appendChild(s);
 const d=AL('v',{name:sh.D?'Dialog':'Bottom sheet',gap:16,pad:sh.D?28:[20,16,28,16],fill:'color/surface',r:sh.D?12:0});sh.f.appendChild(d);d.counterAxisSizingMode='FIXED';d.resize(sh.D?520:390,100);if(!sh.D){d.topLeftRadius=16;d.topRightRadius=16;}
 add(d,T(title,{size:20,w:'Medium',name:'Dialog title'}),true);return d;}
function PLACE(sh,d){if(sh.D){d.x=Math.round((1440-d.width)/2);d.y=Math.max(40,Math.round((900-d.height)/2));}else{d.x=0;d.y=844-d.height;}}
async function ACTIONS(sh,parent,btns){const r=AL('h',{name:'Dialog actions',gap:8,justify:'MAX'});add(parent,r,true);for(const [k,l] of btns){const b=await BTN(k,l,{w:sh.D?160:175});r.appendChild(b);if(!sh.D)b.layoutGrow=1;}return r;}
function SKEL(parent,w,h){const r=figma.createRectangle();r.name='Skeleton';r.resize(w,h||14);r.cornerRadius=4;r.fills=[P('color/subtle')];parent.appendChild(r);return r;}
function STATEBLOCK(card,eyebrow,title,body,note,eyeColor){if(eyebrow)add(card,T(eyebrow,{size:13,c:eyeColor||'color/muted',name:'Eyebrow'}),true);add(card,T(title,{size:20,w:'Medium',name:'State title'}),true);if(body)add(card,T(body,{size:14,name:'State body'}),true);if(note)add(card,T(note,{size:13,c:'color/muted',name:'State note'}),true);}
function DN(dev,s){return 'DESIGN · Admin '+(dev==='D'?'Desktop':'Mobile')+' · '+s;}
