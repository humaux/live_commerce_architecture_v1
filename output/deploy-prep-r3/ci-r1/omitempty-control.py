#!/usr/bin/python3
# Purpose: reproduce compose-go omitempty serialization; all Docker behavior stays native.
# Depends on: Python stdlib and the original absolute Docker CLI path captured before PATH override.
# Used by: CI-r1 local red/green compatibility replay; never production or project runtime.
import json, subprocess, sys
real='/usr/local/bin/docker'
args=sys.argv[1:]
r=subprocess.run([real,*args],capture_output=True)
sys.stderr.buffer.write(r.stderr)
if r.returncode==0 and args and args[0]=='compose' and 'config' in args and '--format' in args and 'json' in args:
    model=json.loads(r.stdout)
    for svc in model.get('services',{}).values():
        for v in svc.get('volumes',[]):
            if isinstance(v,dict) and v.get('bind',{}).get('create_host_path') is False:
                v['bind'].pop('create_host_path')
    print(json.dumps(model))
else: sys.stdout.buffer.write(r.stdout)
sys.exit(r.returncode)
