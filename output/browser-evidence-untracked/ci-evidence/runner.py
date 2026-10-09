# Purpose: bound this task's gate processes and bind evidence to frozen source and historical output.
# Depends on: Python stdlib, git and the supplied local gate command.
# Used by: Codex-3 PR23 merge validation only; ignored run evidence.
import hashlib, json, os, pathlib, signal, subprocess, sys, time
cfg = json.loads(sys.argv[1])
assert subprocess.check_output(['git','branch','--show-current'], text=True).strip() == 'unit/browser-evidence-untracked'
out = pathlib.Path(__file__).parent
def digest(output=False):
    files = subprocess.check_output(['git','ls-files','-z']).split(b'\0')
    h = hashlib.sha256()
    for raw in files:
        if not raw: continue
        name = raw.decode()
        if output != name.startswith('output/'): continue
        if not output and name.startswith('docs/'): continue
        path = pathlib.Path(name)
        h.update(raw); h.update(path.read_bytes() if path.is_file() else b'<absent>')
    return h.hexdigest()
def status():
    return subprocess.check_output(['git','status','--porcelain','--untracked-files=no'],text=True)
head = subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
before, history, dirty = digest(), digest(True), status()
(out/(cfg['name']+'.before.status')).write_text(dirty)
start = time.monotonic()
with (out/(cfg['name']+'.log')).open('w') as log:
    p = subprocess.Popen(cfg['command'], env={**os.environ,'GOFLAGS':'-p=1',**cfg.get('env',{})}, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    try: code = p.wait(timeout=cfg['timeout'])
    except subprocess.TimeoutExpired:
        os.killpg(p.pid,signal.SIGTERM)
        try: p.wait(timeout=15)
        except subprocess.TimeoutExpired: os.killpg(p.pid,signal.SIGKILL); p.wait()
        code = 124
after, history_after, dirty_after = digest(), digest(True), status()
(out/(cfg['name']+'.after.status')).write_text(dirty_after)
result = {**cfg,'head':head,'exit_code':code,'source_sha256':before,'source_after':after,'source_unchanged':before==after,'historical_output_sha256':history,'historical_output_after':history_after,'historical_output_unchanged':history==history_after,'tracked_status_unchanged':dirty==dirty_after,'tracked_status_clean_before':not dirty,'tracked_status_clean_after':not dirty_after,'elapsed_seconds':round(time.monotonic()-start,3)}
(out/(cfg['name']+'.status.json')).write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result))
sys.exit(code if before==after and history==history_after and dirty==dirty_after else 99)
