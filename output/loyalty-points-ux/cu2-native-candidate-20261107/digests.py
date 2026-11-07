# Bounded native projection fingerprints. Dual32 FNV fingerprints are not cryptographic backups.
import json,sys
from operate import OUT,script
phase=sys.argv[1]; inv=json.loads((OUT/'formal-inventory-before.json').read_text());ids=[x['id'] for x in inv['children']]
projection=json.loads((OUT/'formal-before-00.request.json').read_text())['code']
start=projection.index('const rows=[];');end=projection.index('return {rootId:root.id,rows};');body=projection[start:end]
for i in range(0,len(ids),10):
 code="const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const out=[];for(const id of "+json.dumps(ids[i:i+10])+" ){const root=await figma.getNodeByIdAsync(id);if(root.parent.id!=='1554:62573')throw Error('formal parent drift');"+body+"const s=JSON.stringify(rows);let a=2166136261,b=3335557771;for(let j=0;j<s.length;j++){a=Math.imul(a^s.charCodeAt(j),16777619)>>>0;b=Math.imul(b^s.charCodeAt(j),16777619)>>>0;}out.push({id:root.id,nodes:rows.length,chars:s.length,fingerprint:[a,b],name:root.name});}return out;"
 script(f'digest-{phase}-{i//10:02d}',code);print(phase,i,flush=True)
