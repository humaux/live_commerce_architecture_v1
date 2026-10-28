#!/usr/bin/env python3
# Purpose: deterministically regenerate the CVS-SNAP 20261028 INSERT block in migration 0206.
# Depends on: Python standard library, the owner-supplied cvsdata snapshot and migration block markers.
# Used by: snapshot maintainers; migrations.Apply embeds the resulting SQL for fresh installs and R2 upgrades.
# Invariants: codes remain strings; input hash, row count, uniqueness and nonempty identity fail closed.
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SNAPSHOT = ROOT / "internal/fulfillment/cvsdata/stores-20261028.json"
MIGRATION = ROOT / "migrations/0206_cvs_store_directory.sql"
SHA256 = "25390346cf849ebd565ecea990efbc663a1a3043c6d73f283c06d8c3debc167c"
CHAINS = {"FAMI": "cvs_familymart", "UNIMART": "cvs_711", "OKMART": "cvs_okmart", "HILIFE": "cvs_hilife"}
BEGIN = "-- BEGIN GENERATED CVS DIRECTORY 20261028\n"
END = "-- END GENERATED CVS DIRECTORY 20261028\n"


def quote(value):
    if value is None:
        return "NULL"
    if not isinstance(value, str) or "\x00" in value:
        raise ValueError("snapshot fields must be SQL-safe strings")
    return "'" + value.replace("'", "''") + "'"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="refuse stale generated SQL without writing")
    args = parser.parse_args()
    raw = SNAPSHOT.read_bytes()
    if hashlib.sha256(raw).hexdigest() != SHA256:
        raise ValueError("snapshot SHA-256 mismatch; refresh requires a new migration/snapshot")
    data = json.loads(raw)
    if sum(map(len, data.values())) != 20439:
        raise ValueError("snapshot row count mismatch")
    rows = []
    seen = set()
    for chain, kind in sorted(CHAINS.items()):
        for store in sorted(data[chain], key=lambda row: row["store_id"]):
            code, name, address = (store[key] for key in ("store_id", "store_name", "address"))
            if not isinstance(code, str) or not code or not name or not address:
                raise ValueError("invalid snapshot store identity")
            if (kind, code) in seen:
                raise ValueError("duplicate chain/code")
            seen.add((kind, code))
            rows.append("(" + ",".join(quote(v) for v in (kind, code, name, address, store["phone"] or None, "20261028")) + ")")
    # Bounded INSERT statements avoid one enormous planner expression; order and line endings are fixed.
    statements = []
    for offset in range(0, len(rows), 500):
        statements.append("INSERT INTO fulfillment.cvs_store_directory(kind,code,name,address,phone,snapshot) VALUES\n" + ",\n".join(rows[offset:offset + 500]) + ";\n")
    current = MIGRATION.read_text(encoding="utf-8")
    prefix, rest = current.split(BEGIN)
    _, suffix = rest.split(END)
    generated = prefix + BEGIN + "".join(statements) + END + suffix
    if args.check:
        if generated != current:
            raise SystemExit("CVS directory SQL is stale; run scripts/dev/cvs-directory-sql.py")
    else:
        MIGRATION.write_text(generated, encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
