# lc-b4-sends plan (migration 0128)
1. SQL 0128: tables inbox.outbound_messages / send_secrets / bundle_peers (FORCE RLS), claims.bundles.link_pending_manual (+clear trigger on claims.links).
2. Producers (owner commerce_integration_writer, EXECUTE commerce_runtime): inbox.plan_dm, inbox.plan_manual_private_reply, inbox.plan_public_reply, live.plan_offer_recommend (P2-4 signatures; lcn-dup/lcn-rec advisory locks FIRST; PT409/PT422 deny codes).
3. Worker side (EXECUTE commerce_claims_worker): inbox.check_send(op) -> deny code or OK; inbox.load_send_secret(op,gen,lease); inbox.finish_send(op,gen,lease,state,recipient) = Finish hook (bundle_peers ON CONFLICT DO NOTHING, last_outbound_at, secret wipe); inbox.dm_window_for_bundle (P2-5 signature).
4. Auto path (§14.1 cl.2-3): claim_reply_plannable -> reply_used skip + link_pending_manual; plan_claim_reply request gains origin/conversation_known/takeover_generation; claims.check_meta_reply -> human_takeover/takeover_changed.
5. Read side: social.conversation_heads (P2-1), inbox.read_outbound (A9 merge), A8 bundle-only items (P2-2), A13 link_pending_manual + (c) empty items.
6. Go: pagetoken SealSend/pageopen OpenSend (HPKE info meta-send/v1), internal/inbox send*.go (seal display copy + HMAC + dispatch copy, commands, River job), metareply send_dm/public_reply/manual_reply routes (private_reply route branches on message_type), httpapi A4/A5/A6/A12.
7. Wire cmd/api (Inbox sender, seal keys) + cmd/claims-worker (routes). DB-free full-router test.
8. Real-PG foundation tests: LCN06/07/08/10/11/13 slices + exact ACL pins (external_operation_authority_test etc.).
9. Regression: test-focused.sh 'Claim|Meta|Inbox|Template|LiveConsole|LCN|T06'. Commit per green slice; DELIVERY.md.
Deviations noted: send_secrets gets key_id column; mpr index left as is (already admits :m1).
