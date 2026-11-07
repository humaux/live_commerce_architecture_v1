"""Offline entry regression; dependencies are stdlib, operate/fingerprint and isolated fixtures."""
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent

class Failstop(unittest.TestCase):
 def test_entry(self):
  for scenario,count in [('firstfail',1),('timeout',1),('secondfail',2),('good',3)]:
   with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as tmp:
    root=Path(tmp); out=root/'cu2'; out.mkdir(); old=root/'cu2-native-candidate-20261107'; old.mkdir()
    for name in ['operate.py','fingerprint.py','probe-baseline.js']:
     (out/name).write_bytes((HERE/name).read_bytes())
    (old/'formal-inventory-before.json').write_text('{"children":[{"id":"f"}]}')
    (old/'digest-before-00.request.json').write_text(json.dumps({'code':'const rows=[];const s=JSON.stringify(rows);}return out;'}))
    (out/'candidate-reconcile.receipt.txt').write_text('{"children":[{"id":"c"}]}')
    calls=[]
    def stub(argv,**kw):
     calls.append(argv); kw['stdout'].write('{"verdict":"RED"}'); kw['stdout'].flush()
     n=len(calls)
     if scenario=='timeout' and n==1: raise subprocess.TimeoutExpired(argv,130)
     return subprocess.CompletedProcess(argv,1 if (scenario=='firstfail' and n==1) or (scenario=='secondfail' and n==2) else 0)
    saved={k:sys.modules.pop(k,None) for k in ['operate','fingerprint']}
    sys.path.insert(0,str(out))
    try:
     # Import must neither read fixture input nor submit work.
     with patch('subprocess.run',side_effect=AssertionError('import submitted')):
      __import__('fingerprint')
     with patch.dict('os.environ',{'CU2_FIGMA_CONNECTION':'OFFLINE_STUB'}), patch('subprocess.run',side_effect=stub), patch.object(sys,'argv',['operate.py','preflight']), contextlib.redirect_stdout(io.StringIO()):
      sys.modules.pop('operate',None); sys.modules.pop('fingerprint',None)
      if scenario=='good': runpy.run_path(str(out/'operate.py'),run_name='__main__')
      else:
       with self.assertRaises(Exception): runpy.run_path(str(out/'operate.py'),run_name='__main__')
     self.assertEqual(len(calls),count)
     rows=[json.loads(x) for x in (out/'operations.jsonl').read_text().splitlines()]
     self.assertEqual(len([x for x in rows if 'started' in x]),count)
     self.assertEqual(len([x for x in rows if 'ended' in x]),count)
     self.assertEqual(len(list(out.glob('*.receipt.txt'))),count+1)
     self.assertIn('PREFLIGHT_EXPECTED_RED' if scenario=='good' else 'STOP_UNKNOWN',(out/'preflight-terminal.json').read_text())
     print(json.dumps({'scenario':scenario,'calls':count,'operations':rows,'raw_receipts':{p.name:p.read_text() for p in out.glob('*.receipt.txt')}}))
     if scenario=='timeout': self.assertEqual(rows[-1]['state'],'TIMEOUT_UNKNOWN')
    finally:
     sys.path.pop(0)
     for k,v in saved.items():
      sys.modules.pop(k,None)
      if v is not None: sys.modules[k]=v

if __name__=='__main__': unittest.main(verbosity=2)
