# Purpose: dev-time audit for LC-R2: list inbox.* functions created by migrations without an explicit
#   REVOKE ALL ... FROM PUBLIC (a PUBLIC EXECUTE would surface in CRP02's privilege matrix once 0169
#   grants commerce_retention_writer USAGE on the inbox schema).
# Depends on: migrations/*.sql only (stdlib re/glob).
# Used by: the LC-R2 implementer (one-off analysis, kept as evidence tooling; not a gate).
import glob
import re

created = {}
revoked = set()
for f in sorted(glob.glob('migrations/*.sql')):
    text = open(f).read()
    for m in re.finditer(r'CREATE (?:OR REPLACE )?FUNCTION\s+inbox\.([a-z_0-9]+)\s*\(', text):
        created.setdefault(m.group(1), f)
    for m in re.finditer(r'REVOKE ALL ON FUNCTION\s+([^;]+?)FROM PUBLIC', text, re.S):
        for part in m.group(1).split(','):
            mm = re.search(r'inbox\.([a-z_0-9]+)\s*\(', part)
            if mm:
                revoked.add(mm.group(1))
missing = {k: v for k, v in created.items() if k not in revoked}
print('created:', len(created), 'revoked:', len(revoked))
print('inbox functions without an explicit REVOKE ALL ... FROM PUBLIC:')
for k, v in sorted(missing.items()):
    print(' ', k, v)
