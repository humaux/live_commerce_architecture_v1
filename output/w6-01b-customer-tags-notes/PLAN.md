# w6-01b plan (Sonnet, fallback for DeepSeek)
1. Migration 0139: tables customers.tags/owner_tags/notes (FORCE RLS, privacy_writer + commerce_auth ACL), customers:write permission (CHECK re-derive + owner/admin backfill), audit policy widened, helper + 9 definers (owner commerce_privacy_writer, EXECUTE commerce_runtime).
2. read_merchant_customers: new 8-arg signature (p_tag DEFAULT NULL), old DROPped; list rows get tags, detail gets tags_revision + notes(50). Body copied in full so W5-02B can extend it.
3. Erasure: apply_erasure patched (0113 drift-guarded pattern) to call customers.erase_tags_notes; buyer_read_privacy(detail) patched to append tags+notes; Go BuyerExport key set updated.
4. Go: internal/customers/{tags,notes}.go (command.Run idempotency), types.go (Tags/Notes fields), read.go (tag filter+cursor binding); httpapi/customer_tags.go routes registered in registerCustomerRoutes; httperror codes tag_exists/limit_reached.
5. Tests tests/foundation/customer_tags_test.go (CT01-CT09, red first), update ACL pins (schema/consent tests), DB-free router test; contracts amendments.
Role ruling: customers:write -> owner/admin (live_operator not added, flagged). Edit/delete others' notes requires customers:privacy (owner/admin only).
