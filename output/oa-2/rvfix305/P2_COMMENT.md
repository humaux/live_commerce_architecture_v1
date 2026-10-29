P2 for issue — RVFIX305 review at `5edaaa082`, output/oa-2/SQL-REQUEST.md:24.

The proposed reminder identity CHECK admits SQL NULL if client_reminder_key or reminder_intent_key is omitted. Unique indexes also admit NULL, weakening reminder replay identity. Track in the integrator's schema task: require both keys IS NOT NULL for reminder rows while keeping initial-row NULL rules; add negative REAL_PG cases for each omitted key before adopting the request. No migration assignment is present; this RVFIX follows the card's P2-comment instruction and does not change shared schema.
