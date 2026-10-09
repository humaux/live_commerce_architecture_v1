<!-- Purpose: park a product UX issue outside the frozen evidence-only brief.
Depends on: actual PasswordAuth timestamps and CI countdown discrepancy.
Used by: integrator FOLLOWUPS triage; not a release/identity-leak claim. -->
# Deferred P2: PasswordAuth initial resend counter can show 61 seconds

At apps/admin/components/PasswordAuth.tsx:163-164, setNow(Date.now()) and setResendAt(Date.now()+60000) sample separately. A 1ms gap plus Math.ceil at100 yields an initial61; this happens independently of account existence and was not introduced by PR23 evidence paths. Small product follow-up: reuse one captured timestamp for both states, with an actual component clock advancing1ms RED test. This batch follows the evidence-only brief's no-product-edit rule; it fixes the test's sequential wall-time comparison using one common browser Date instant, keeps complete-copy equality and the independent real60s cooldown test unchanged. No numeric normalization or assertion relaxation.
