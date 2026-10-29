P2 for issue — latest RVFIX305 review at `2768fb917`, output/oa-2/SQL-REQUEST.md:26 and output/oa-2/RVFIX305-DELIVERY.md:44.

The proposed reminder identity CHECK admits SQL NULL if client_reminder_key or reminder_intent_key is omitted. Unique indexes also admit NULL, weakening reminder replay identity. Track in the integrator's schema task: require both keys IS NOT NULL for reminder rows while keeping initial-row NULL rules; add negative REAL_PG cases for each omitted key before adopting the request. No migration assignment is present; this RVFIX follows the card's P2-comment instruction and does not change shared schema.

Correction: the prior delivery file and receipt comment claiming both explicit non-NULL predicates were already added are incorrect. The DDL still lacks them. P2 remains open; this implementation batch corrects that delivery claim, and eventual schema adoption still requires the real-DB negatives. No current-head GREEN or database acceptance is claimed.
