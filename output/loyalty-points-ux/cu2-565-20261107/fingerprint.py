# Reuses preserved native projection collector; read only, bounded <=10 roots.
import json,sys
from pathlib import Path
from operate import OUT,script
OLD=OUT.parent/'cu2-native-candidate-20261107'
kind,phase=sys.argv[1:3]
if kind=='formal':ids=[r['id'] for r in json.loads((OLD/'formal-inventory-before.json').read_text())['children']];parent='1554:62573'
else:ids=[r['nodeId'] for r in json.loads((OUT/'baseline-six.json').read_text())] if kind=='six' else [r['id'] for r in json.loads((OUT/'candidate-reconcile.receipt.txt').read_text())['children']];parent='1589:63500'
source=json.loads((OLD/'digest-before-00.request.json').read_text())['code'];start=source.index('const rows=[];');end=source.index('const s=JSON.stringify(rows);');body=source[start:end];tail=source[end:source.rindex('}return out;')]
for i in range(0,len(ids),10):
 code="const p=await figma.getNodeByIdAsync('4:8829');await p.loadAsync();const out=[];for(const id of "+json.dumps(ids[i:i+10])+" ){const root=await figma.getNodeByIdAsync(id);if(root.parent.id!=="+json.dumps(parent)+")throw Error('root ownership drift');"+body+tail+"}return out;"
 script(f'{kind}-fingerprint-{phase}-{i//10:02d}',code);print(kind,phase,i,flush=True)
