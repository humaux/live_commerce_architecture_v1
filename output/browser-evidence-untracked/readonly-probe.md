# READONLY ext-agent probe (2026-10-09, qwen3.8-flash)
- Bash touch /tmp/lc-readonly-probe-bash: file absent afterwards (denied)
- Write scripts/dev/ro-probe-should-fail.txt: file absent afterwards (denied)
- Write output/ext-agents/ro-probe/findings.md: created (allowed). Agent report follows:

# Permission probe findings (2026-10-09)

Probe requested by the operator: execute a Bash write, a Write outside the ext-agents output area, then record outcomes here.

## Step 1 — Bash: `touch /tmp/lc-readonly-probe-bash`
**Result: DENIED (tool unavailable).**
The Bash tool is disabled for this session ("No such tool available: Bash. Bash is disabled for this session, in subagents as well as here."). The command never executed; `/tmp/lc-readonly-probe-bash` was not created.

## Step 2 — Write: `scripts/dev/ro-probe-should-fail.txt` containing "x"
**Result: DENIED.**
The Write call did not receive permission approval. Verified afterward with Read: the file does not exist.

## Step 3 — Write: `output/ext-agents/ro-probe/findings.md` (this file)
**Result: ALLOWED** — this file's successful creation is itself the evidence.

## Conclusion
Runtime permissions for this session block Bash entirely and block writes under `scripts/`, while allowing writes under `output/ext-agents/`. The probe matches the expected read-only-outside-output behavior.
