#!/usr/bin/env python3
# Purpose: retain SHA-bound command receipts for Round10 static acceptance.
# Depends on: stdlib subprocess; called manually, never invokes Go tests.
# Invariant: every run has a timeout, terminal exit and retained output.
import datetime, hashlib, json, pathlib, subprocess, sys, time
out = pathlib.Path(__file__).resolve().parent
name, cwd, *cmd = sys.argv[1:]
sha = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=cwd, text=True).strip()
receipt = dict(run_id='r10-'+name, source_sha=sha, command=cmd, cwd=cwd,
               timeout_seconds=180, started_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
               status='running', wake='exec terminal completion; timeout or unexpected exit/ERROR/TOUCHED fails')
path = out/(name+'-receipt.json')
path.write_text(json.dumps(receipt, indent=2)+'\n')
start = time.monotonic()
try:
    result = subprocess.run(cmd, cwd=cwd, capture_output=True, timeout=180)
    stdout, stderr, code = result.stdout, result.stderr, result.returncode
except OSError as exc:
    stdout, stderr, code = b'', str(exc).encode(), 127
except subprocess.TimeoutExpired as exc:
    stdout, stderr, code = exc.stdout or b'', exc.stderr or b'', 124
(out/(name+'-report.txt')).write_bytes(stdout)
(out/(name+'-stderr.log')).write_bytes(stderr)
receipt.update(status='finished', exit_code=code, elapsed_seconds=round(time.monotonic()-start,3),
               ended_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
               summary=[s for s in stdout.decode(errors='replace').splitlines() if s.startswith('SUMMARY')],
               errors=sum('ERROR' in s for s in stdout.decode(errors='replace').splitlines()),
               touched=sum('TOUCHED' in s for s in stdout.decode(errors='replace').splitlines()))
if pathlib.Path(cmd[0]).is_file():
    receipt['executable_sha256']=hashlib.sha256(pathlib.Path(cmd[0]).read_bytes()).hexdigest()
path.write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt))
sys.exit(code)
