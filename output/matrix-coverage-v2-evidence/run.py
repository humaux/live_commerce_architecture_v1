# Purpose: Bound owned GROUP3 commands and preserve uncompressed hash-bound evidence.
# Depends on: Python stdlib and the supplied DB-free gate command.
# Used by: matrix-coverage-v2-evidence delivery; no browser or PG commands.
import hashlib, json, os, pathlib, signal, subprocess, sys, time
name, seconds, *command = sys.argv[1:]
out = pathlib.Path(__file__).parent
paths = ["tests/foundation/catalog_matrix_ledger_test.go", "tests/foundation/catalog_matrix_evidence_test.go", "docs/delivery/GATES.md"]
def hashes():
    return {p: hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest() for p in paths}
before = hashes()
head = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
start = time.monotonic()
with (out / (name + ".log")).open("w") as log:
    process = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, start_new_session=True, env={**os.environ, "GOFLAGS": "-p=1"})
    (out / (name + ".running.json")).write_text(json.dumps({"pid":process.pid,"timeout_seconds":int(seconds),"command":command,"log":str(out / (name + ".log"))}))
    try: code = process.wait(timeout=int(seconds))
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGTERM)
        try: process.wait(timeout=10)
        except subprocess.TimeoutExpired: os.killpg(process.pid, signal.SIGKILL); process.wait()
        code = 124
after = hashes()
receipt = {"name":name,"command":command,"exit_code":code,"head":head,"source_before":before,"source_after":after,"unchanged":before==after,"elapsed_seconds":round(time.monotonic()-start,3)}
(out / (name + ".json")).write_text(json.dumps(receipt,indent=2)+"\n")
(out / (name + ".running.json")).unlink()
print(json.dumps(receipt))
sys.exit(code if before==after else 99)
