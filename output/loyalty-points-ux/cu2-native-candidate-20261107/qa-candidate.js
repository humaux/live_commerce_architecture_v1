const CANDIDATE_ID="1589:63500";const CANDIDATE_NAME="會員積分 v1 · CU2 CANDIDATE 20261107 · 6863a600";
function assertCandidate(section,id){if(!id||id==='1554:62573'||!section||section.id!==id||section.type!=='SECTION'||section.parent?.id!=='4:8829'||section.name!==CANDIDATE_NAME)throw Error('CU2 candidate-only target rejected');}
const PAGE=await figma.getNodeByIdAsync('4:8829');await PAGE.loadAsync();
const SEC=await figma.getNodeByIdAsync(CANDIDATE_ID);assertCandidate(SEC,CANDIDATE_ID);
const C=await figma.getNodeByIdAsync('1470:167194');
const all=await figma.variables.getLocalVariablesAsync('COLOR');
const coll=all.find(v=>v.id==='VariableID:3:11').variableCollectionId;
const val=n=>{const v=all.find(x=>x.name===n&&x.variableCollectionId===coll)||all.find(x=>x.name===n);if(!v)return {r:1,g:0,b:1,missing:n};return v.resolveForConsumer(C).value;};
const L=c=>{const f=x=>x<=0.03928?x/12.92:Math.pow((x+0.055)/1.055,2.4);return 0.2126*f(c.r)+0.7152*f(c.g)+0.0722*f(c.b);};
const CR=(a,b)=>{const x=L(a),y=L(b);return Math.round(((Math.max(x,y)+0.05)/(Math.min(x,y)+0.05))*100)/100;};
const pairs=[['color/muted','color/surface'],['color/muted','color/subtle'],['color/ink','color/subtle'],['color/danger','color/surface'],['color/danger','color/subtle'],['color/teal','color/surface'],['color/teal','color/subtle'],['color/surface','color/teal'],['color/muted','color/canvas']];
const contrast=pairs.map(([a,b])=>a+' on '+b+' = '+CR(val(a),val(b)));
const issues=[];const frames=[];
for(const f of SEC.children){frames.push([f.id,f.name,f.width+'x'+f.height]);
 const vp=f.findChild(n=>n.name==='Scrollable task viewport');
 if(vp&&vp.children[0]&&vp.children[0].height>vp.height+0.5)issues.push(f.id+' viewport overflow '+Math.round(vp.children[0].height)+'>'+vp.height);
 for(const d of f.children.filter(n=>n.name==='Dialog'||n.name==='Bottom sheet')){if(d.y<0||d.y+d.height>f.height+0.5)issues.push(f.id+' dialog out of frame y'+d.y+' h'+d.height);}
 const texts=f.findAllWithCriteria({types:['TEXT']});
 for(const t of texts){if(typeof t.fontSize==='number'&&t.fontSize<12)issues.push(f.id+' small text '+t.characters.slice(0,10));if(t.characters.length===0)issues.push(f.id+' empty text');}
 const btns=f.findAll(n=>n.name.startsWith('Action ·')||n.name.startsWith('Segment ·'));for(const b of btns)if(b.height<44)issues.push(f.id+' small target '+b.name);
}
return {contrast,issues,frames};
