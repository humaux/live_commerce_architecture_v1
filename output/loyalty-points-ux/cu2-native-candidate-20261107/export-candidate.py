# Supported native get_screenshot exporter only; no rendering/emulation.
import json,hashlib,struct
from pathlib import Path
from operate import OUT,call
frames=[]
for f in sorted(OUT.glob('generate-*.receipt.txt')):frames.extend(json.loads(f.read_text())['frames'])
assert len(frames)==99 and len({x[2] for x in frames})==99
rows=[]
for i,(key,device,nid,name) in enumerate(frames):
 raw=call('export-'+key+'-'+device,'get_screenshot',{'nodeId':nid,'scale':1})
 lines=[l[6:] for l in raw.splitlines() if l.startswith('IMAGE ')]
 if len(lines)!=1:raise RuntimeError('export missing/ambiguous: '+key+device)
 image=Path(lines[0]); data=image.read_bytes();assert data[:8]==b'\x89PNG\r\n\x1a\n';width,height=struct.unpack('>II',data[16:24])
 rows.append({'key':key,'device':device,'nodeId':nid,'name':name,'path':str(image.relative_to(OUT)),'width':width,'height':height,'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'viewed':False})
 (OUT/'candidate-exports.json').write_text(json.dumps(rows,ensure_ascii=False,indent=2))
 if i%10==0:print('exported',i+1,flush=True)
print('exported',len(rows),flush=True)
