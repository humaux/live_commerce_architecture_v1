# Evidence indexing only; does not execute Figma or alter generated assets.
import json,hashlib,datetime
from pathlib import Path
from operate import OUT
root=OUT.parent
exports=json.loads((OUT/'candidate-exports.json').read_text());viewed={(k,d) for k in ['B1','A12','E1','C8','D1'] for d in ['D','M']}|{('C1','M'),('C3','M')}
for r in exports:r['viewed']=(r['key'],r['device']) in viewed
(OUT/'candidate-exports.json').write_text(json.dumps(exports,ensure_ascii=False,indent=2))
geo=[r for p in sorted(OUT.glob('geometry-*.receipt.txt')) for r in json.loads(p.read_text())];lookup={(r['key'],r['dev']):r for r in geo}
checks=[]
def fact(name,value):checks.append({'fact':name,'observed':value})
for d in ['D','M']:
 a=lookup['A12',d];fact('A12 '+d+' phone and full helper','0900000000' in a['text'] and '由你自行填寫，系統不自動計算；不填就不記積分。' in a['text'])
 e=lookup['E1',d];fact('E1 '+d+' complete canonical amounts',all(any(s in t for t in e['text']) for s in ['單價 NT$3,000','商品小計 NT$6,000','−NT$2,000','運費 NT$100','合計 NT$4,100']))
 fact('B1 '+d+' members',len(lookup['B1',d]['memberRows']))
 for k in ['C1','C3','C7','C8','C9']:fact(k+' '+d+' ledger rows',len(lookup[k,d]['ledgerRows']))
 for k in ['D1','D2','D3','E4']:
  r=lookup[k,d];fact(k+' '+d+' required explanation height',[b['height'] for b in r['boxes'] if '說明（必填）' in b['name']]);fact(k+' '+d+' sheet in frame',all(s['y']>=0 and s['y']+s['height']<=r['frame']['height'] for s in r['sheets']))
assert all(r['observed'] is not False for r in checks)
(OUT/'measured-facts.json').write_text(json.dumps(checks,ensure_ascii=False,indent=2))
pins=json.loads((OUT/'source-pins.json').read_text());assert all(hashlib.sha256((root/'figma-src'/f).read_bytes()).hexdigest()==h for f,h in pins.items())
formal={f:hashlib.sha256((root/f).read_bytes()).hexdigest() for f in ['DESIGN-MANIFEST.md','PNG-SHA256SUMS.txt']};assert formal=={'DESIGN-MANIFEST.md':'2e63d5b5b1d40d0a73238d905f5df39194650bae4f3fcfebfce5666bfd262575','PNG-SHA256SUMS.txt':'bdf8b75a96da40040cfe658e901a5bbf92f3b287ba01b01daab6ad898fe8306b'}
(OUT/'unchanged-source-formal-files.json').write_text(json.dumps({**pins,**formal},indent=2))
qa=json.loads((OUT/'qa-candidate.receipt.txt').read_text())
lines=['# CU2 isolated native candidate · 2026-11-07','','E2 native candidate only; independent L1 NOT_RUN, cloud save UNKNOWN, formal adoption NOT_RUN, LP2 HOLD.','', 'File iJ9lDdFIXgTlKkt5Tvrc0X; page4:8829; candidate1589:63500; formal1554:62573 untouched.','Source candidate08a670b8a804534a4f3e62ed635d6eebab3939af; pre-native report HEADa5b4ecb24d319b23b51d93b2faac00b57efe9c11.','Exact source pins: source-pins.json; adapted scripts/requests and operations.jsonl record every dispatch, script hash and explicit connection. Native bridge job IDs are not exposed by this existing caller; create and generation scripts return their own operation labels, not fabricated native job IDs.','',f'Exported99/99 at1x,49desktop1440x900+50mobile390x844, total{sum(r["bytes"] for r in exports)}bytes; viewed12/99 by implementer,87 NOT_VIEWED. Extra3 scroll-content screenshots are separate and not interactive proof.','Original qa.js retained3 overflow findings: '+json.dumps(qa['issues'])+'. No guard removed. Generic4.30 muted/subtle uses different collection; candidate actual6disabled paints measure5.381026271914327, all opacity1.','Scroll-layout receipt records native VERTICAL clipping with content699/591 vs viewport582; B1 fifth-row bottom510 and pagination bottom566 after available117px scroll; B7/B8 last-row bottom566 after9px scroll; dock starts750 after viewport ends726. This proves bounded native layout reachability, not performed prototype scroll. Interactive user controls/refresh persistence NOT_RUN.','', 'Formal preservation: original99-root metadata exactly byte equal,100004bytes SHA256023ae156eb05edf26e4d7b32de1567691025220d888a8eefdaac2c3c0837dd24; full scoped projection of5862nodes has matching dual32 FNV fingerprints/counts/lengths for99roots. Fingerprints are noncryptographic and do not prove complete .fig/file/token/component/global byte equivalence.75 detailed pre-reads complete,24 raw pipe receipts truncated and retained.','Collection correction: existing call.mjs ends process.exit immediately after console output; pipe capture lost bytes above64KiB, direct-file stdout capture returned full100004byte identical metadata. No shared caller edit, no replayed mutations; original truncated receipts/parse failures retained. create_session was catalog-listed but native service returned unsupported before canvas execution; loaded guide explicitly permits legacy calls, used with exact connectionId thereafter.','', 'Rollback: preserve deliverable1589:63500 pending root disposition; remove exactly that recorded candidate section only after disposition, never the formal99. Existing native.fig/profile backups remain untouched. No formal promotion, token/component definition changes, app restart/reload/logout/install/network troubleshooting, runtime product edits, push/merge/deploy.','', '| Key | Device | Actual node | Size | SHA256 | Viewed |','|---|---|---|---|---|---|']
for r in exports:lines.append(f'| {r["key"]} | {r["device"]} | {r["nodeId"]} | {r["width"]}×{r["height"]} | {r["sha256"]} | {r["viewed"]} |')
(OUT/'CANDIDATE-MANIFEST.md').write_text('\n'.join(lines)+'\n')
print('facts indexed0; source/formal5hash unchanged;99exports12viewed')
