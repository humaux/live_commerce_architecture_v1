-- 0087 store design (R4 unit store-design; contracts/storefront-v2.md section B, FROZEN 2026-10-01).
--
-- Owns: schema design = the merchant's storefront design document (one editable draft per store, an append-only list of
-- published versions), store-owned media (logo, favicon, hero and section images, bytea like catalog.product_images),
-- 15-minute preview tokens, and the definer owner role commerce_design_reader with the three buyer-facing read definers
-- design.buyer_published / design.buyer_preview / design.buyer_media.
--
-- Non-goals: no custom CSS/JS or raw HTML anywhere (the document schema is closed and validated in Go,
-- internal/design); no templates marketplace; no rendering (the storefront Next app renders the document in unit
-- storefront-shell); no buyer table privilege (buyers reach rows only through the definers below); no merchant access
-- to another store's rows (policy scope_access); published history is never updated or deleted.
--
-- Depends on: 0001 (control.stores, ops.audit_events, role commerce_runtime), 0007 (role commerce_buyer_runtime),
-- 0020 (buyer.resolve_published_store, the only origin -> store mapping), 0082 (the catalog-media bytea pattern).
--
-- Callers: internal/design (commerce_runtime inside platform.WithScope: draft CAS, publish, rollback, media, preview
-- token issue), internal/buyerhttp design.go (commerce_buyer_runtime: the three definers, no buyer bearer).
--
-- ponytail: bytea in PG is the pilot ceiling (single host, <= 60 images per store, 2 MiB each). Move to object storage +
-- CDN when total design-media bytes pass ~2 GB or a second host serves the storefront; the id changes whenever content
-- changes, so the public URL /media/s/<image> is already cache-immutable and survives that move.

CREATE SCHEMA design;
REVOKE ALL ON SCHEMA design FROM PUBLIC;
GRANT USAGE ON SCHEMA design TO commerce_runtime;
-- The buyer runtime only EXECUTEs the three design.buyer_* definers; it gets no table privilege (see below).
GRANT USAGE ON SCHEMA design TO commerce_buyer_runtime;

CREATE ROLE commerce_design_reader NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_design_reader IS
 'store-design definer owner (migrations/0087). NOLOGIN, no members; owns design.buyer_published, design.buyer_preview and design.buyer_media and reads design.* and the store name only through its own FORCE RLS policies. No runtime login may reach it.';
GRANT USAGE ON SCHEMA design, control, buyer TO commerce_design_reader;
GRANT EXECUTE ON FUNCTION buyer.resolve_published_store(text) TO commerce_design_reader;

-- ---------------------------------------------------------------------------------------
-- design.documents: the one editable draft per store. version is the compare-and-set token of PUT design/draft.
-- ---------------------------------------------------------------------------------------
CREATE TABLE design.documents (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    document jsonb NOT NULL CHECK (jsonb_typeof(document) = 'object' AND octet_length(document::text) <= 262144),
    updated_by uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    FOREIGN KEY (tenant_id, updated_by) REFERENCES identity.memberships(tenant_id, principal_id)
);

-- ---------------------------------------------------------------------------------------
-- design.published_versions: append-only. A rollback is a NEW row that copies an older document (kind 'rollback',
-- source_version = the copied version); a publish is a new row copying the draft (source_version = draft version).
-- ---------------------------------------------------------------------------------------
CREATE TABLE design.published_versions (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    document jsonb NOT NULL CHECK (jsonb_typeof(document) = 'object' AND octet_length(document::text) <= 262144),
    kind text NOT NULL CHECK (kind IN ('publish', 'rollback')),
    source_version bigint NOT NULL CHECK (source_version > 0),
    published_by uuid NOT NULL,
    published_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, version),
    FOREIGN KEY (tenant_id, store_id) REFERENCES design.documents(tenant_id, store_id),
    FOREIGN KEY (tenant_id, published_by) REFERENCES identity.memberships(tenant_id, principal_id)
);

-- ---------------------------------------------------------------------------------------
-- design.store_media: same validation and shape as catalog.product_images (CM1/CM2), <= 60 per store (enforced in Go
-- under an advisory lock; the UNIQUE (store, sha256) makes a re-upload of identical bytes return the existing row).
-- ---------------------------------------------------------------------------------------
CREATE TABLE design.store_media (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    content_type text NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    bytes bytea NOT NULL CHECK (octet_length(bytes) BETWEEN 1 AND 2097152),
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    width integer CHECK (width IS NULL OR width BETWEEN 1 AND 100000),
    height integer CHECK (height IS NULL OR height BETWEEN 1 AND 100000),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    CONSTRAINT store_media_content UNIQUE (tenant_id, store_id, sha256)
);

-- ---------------------------------------------------------------------------------------
-- design.preview_tokens: random 32-byte token, only its SHA-256 is stored (the buyer capability pattern, 0006/0007).
-- Bound to store + the draft version at issue time; valid 15 minutes; grants nothing but the draft read.
-- ---------------------------------------------------------------------------------------
CREATE TABLE design.preview_tokens (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
    draft_version bigint NOT NULL CHECK (draft_version > 0),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL DEFAULT clock_timestamp() + interval '15 minutes',
    PRIMARY KEY (tenant_id, store_id, token_hash),
    FOREIGN KEY (tenant_id, store_id) REFERENCES design.documents(tenant_id, store_id),
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '15 minutes')
);

ALTER TABLE design.documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE design.documents FORCE ROW LEVEL SECURITY;
ALTER TABLE design.published_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE design.published_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE design.store_media ENABLE ROW LEVEL SECURITY;
ALTER TABLE design.store_media FORCE ROW LEVEL SECURITY;
ALTER TABLE design.preview_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE design.preview_tokens FORCE ROW LEVEL SECURITY;
REVOKE ALL ON design.documents, design.published_versions, design.store_media, design.preview_tokens FROM PUBLIC;

-- Merchant runtime: store-scoped (GUCs set by platform.WithScope after the permission check).
CREATE POLICY scope_access ON design.documents TO commerce_runtime
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);
CREATE POLICY scope_access ON design.published_versions TO commerce_runtime
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);
CREATE POLICY scope_access ON design.store_media TO commerce_runtime
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);
CREATE POLICY scope_access ON design.preview_tokens TO commerce_runtime
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);

GRANT SELECT, INSERT ON design.documents TO commerce_runtime;
GRANT UPDATE (version, document, updated_by, updated_at) ON design.documents TO commerce_runtime;
-- Append-only: no UPDATE and no DELETE grant, and the trigger below refuses both for any role.
GRANT SELECT, INSERT ON design.published_versions TO commerce_runtime;
GRANT SELECT, INSERT, DELETE ON design.store_media TO commerce_runtime;
GRANT SELECT, INSERT, DELETE ON design.preview_tokens TO commerce_runtime;

CREATE FUNCTION design.refuse_history_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'design.published_versions is append-only' USING ERRCODE = '23514';
END $$;
CREATE TRIGGER published_versions_append_only BEFORE UPDATE OR DELETE ON design.published_versions
    FOR EACH ROW EXECUTE FUNCTION design.refuse_history_change();

-- Definer owner: reads rows, each definer adds its own store filter.
CREATE POLICY design_reader_read ON design.documents FOR SELECT TO commerce_design_reader USING (true);
CREATE POLICY design_reader_read ON design.published_versions FOR SELECT TO commerce_design_reader USING (true);
CREATE POLICY design_reader_read ON design.store_media FOR SELECT TO commerce_design_reader USING (true);
CREATE POLICY design_reader_read ON design.preview_tokens FOR SELECT TO commerce_design_reader USING (true);
GRANT SELECT ON design.documents, design.published_versions, design.store_media, design.preview_tokens TO commerce_design_reader;
GRANT SELECT (tenant_id, id, name, active) ON control.stores TO commerce_design_reader;
CREATE POLICY design_reader_store_read ON control.stores FOR SELECT TO commerce_design_reader USING (active);

-- ---------------------------------------------------------------------------------------
-- Buyer definers. REVOKE FROM PUBLIC, EXECUTE only for commerce_buyer_runtime. The store is always resolved from the
-- verified origin (PT400 invalid origin, PT404 not published); there is no store parameter.
-- ---------------------------------------------------------------------------------------

-- Latest published document of the store, or version 0 + NULL document when none (Go derives the default from the name).
CREATE FUNCTION design.buyer_published(p_origin text)
RETURNS TABLE(version bigint, document jsonb, store_name text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE = 'PT404'; END IF;
 RETURN QUERY
  SELECT coalesce(pv.version, 0::bigint), pv.document, s.name
    FROM control.stores s
    LEFT JOIN LATERAL (SELECT v.version, v.document FROM design.published_versions v
                        WHERE v.tenant_id = s.tenant_id AND v.store_id = s.id ORDER BY v.version DESC LIMIT 1) pv ON true
   WHERE s.id = v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE = 'PT404'; END IF;
END $$;
ALTER FUNCTION design.buyer_published(text) OWNER TO commerce_design_reader;
REVOKE ALL ON FUNCTION design.buyer_published(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION design.buyer_published(text) TO commerce_buyer_runtime;

-- The draft, only for an unexpired token of THIS store whose bound draft version is still the current one. Every miss
-- (unknown, expired, other store, stale version) is the same PT404, so a token reveals nothing.
CREATE FUNCTION design.buyer_preview(p_origin text, p_token_hash bytea)
RETURNS TABLE(version bigint, document jsonb, store_name text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE = 'PT404'; END IF;
 IF p_token_hash IS NULL OR octet_length(p_token_hash) <> 32 THEN RAISE EXCEPTION 'preview not found' USING ERRCODE = 'PT404'; END IF;
 RETURN QUERY
  SELECT d.version, d.document, s.name
    FROM design.preview_tokens t
    JOIN design.documents d ON d.tenant_id = t.tenant_id AND d.store_id = t.store_id AND d.version = t.draft_version
    JOIN control.stores s ON s.tenant_id = t.tenant_id AND s.id = t.store_id
   WHERE t.store_id = v_store AND t.token_hash = p_token_hash AND t.expires_at > clock_timestamp() AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'preview not found' USING ERRCODE = 'PT404'; END IF;
END $$;
ALTER FUNCTION design.buyer_preview(text, bytea) OWNER TO commerce_design_reader;
REVOKE ALL ON FUNCTION design.buyer_preview(text, bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION design.buyer_preview(text, bytea) TO commerce_buyer_runtime;

CREATE FUNCTION design.buyer_media(p_origin text, p_image uuid)
RETURNS TABLE(content_type text, bytes bytea, sha256 bytea)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE = 'PT404'; END IF;
 RETURN QUERY
  SELECT m.content_type, m.bytes, m.sha256
    FROM design.store_media m JOIN control.stores s ON s.tenant_id = m.tenant_id AND s.id = m.store_id
   WHERE m.store_id = v_store AND m.id = p_image AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'image not found' USING ERRCODE = 'PT404'; END IF;
END $$;
ALTER FUNCTION design.buyer_media(text, uuid) OWNER TO commerce_design_reader;
REVOKE ALL ON FUNCTION design.buyer_media(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION design.buyer_media(text, uuid) TO commerce_buyer_runtime;

-- ---------------------------------------------------------------------------------------
-- Documentation (PROCESS section 5).
-- ---------------------------------------------------------------------------------------
COMMENT ON SCHEMA design IS 'internal/design: merchant storefront design document (draft + published versions), store media, preview tokens. Written only by commerce_runtime through internal/design; read by buyers only through design.buyer_* definers. Non-goal: no CSS/JS, no rendering, no templates.';
COMMENT ON TABLE design.documents IS
 'internal/design store.go: the single editable draft per store (one row, version = CAS token of PUT design/draft). Validated in Go (strict closed schema, contracts/storefront-v2.md B) before every write; written by commerce_runtime under the row lock; read by the preview definer. Non-goal: not buyer-visible until published.';
COMMENT ON COLUMN design.documents.version IS 'Compare-and-set token; bumped by every draft save. Preview tokens and publish bind to it.';
COMMENT ON COLUMN design.documents.document IS 'Normalised design document (profile, nav, home.sections, pages), <= 256 KiB; produced by internal/design.Normalize, never client JSON verbatim.';
COMMENT ON COLUMN design.documents.updated_by IS 'Membership principal that saved the draft (audit trail, FK memberships).';
COMMENT ON TABLE design.published_versions IS
 'internal/design publish.go: append-only history of published documents. Publish copies the draft (kind publish, source_version = draft version); rollback copies an older row (kind rollback, source_version = that version). UPDATE/DELETE are refused by trigger and have no grant. The buyer reads the highest version through design.buyer_published.';
COMMENT ON COLUMN design.published_versions.version IS 'Per-store counter 1..n assigned under the draft row lock; the live document is the highest version.';
COMMENT ON COLUMN design.published_versions.kind IS 'publish = copy of the draft; rollback = copy of an older published version.';
COMMENT ON COLUMN design.published_versions.source_version IS 'Draft version (publish) or published version (rollback) the document was copied from.';
COMMENT ON COLUMN design.published_versions.published_by IS 'Membership principal that published or rolled back; also written to ops.audit_events.';
COMMENT ON TABLE design.store_media IS
 'internal/design media.go: store-owned images (logo, favicon, hero, section images), <= 60 per store (Go, advisory lock), JPEG/PNG/WebP validated by magic bytes with catalog.SniffImage, bytes <= 2 MiB, never re-encoded. UNIQUE (store, sha256): identical bytes return the existing row. Served to buyers only by design.buyer_media at /media/s/<id>.';
COMMENT ON COLUMN design.store_media.bytes IS 'Exactly the uploaded file, 1 byte .. 2 MiB; selected only by the merchant preview route and design.buyer_media.';
COMMENT ON COLUMN design.store_media.sha256 IS 'SHA-256 of bytes; the per-store dedupe key.';
COMMENT ON COLUMN design.store_media.content_type IS 'image/jpeg, image/png or image/webp, from magic bytes (catalog.SniffImage), never from the client header.';
COMMENT ON TABLE design.preview_tokens IS
 'internal/design preview.go: SHA-256 of a random 32-byte preview token bound to store + draft version, valid 15 minutes. Read by design.buyer_preview only; grants nothing but the draft read. Expired rows are deleted when the next token is issued.';
COMMENT ON COLUMN design.preview_tokens.token_hash IS 'SHA-256 of the token; the token itself is returned once to the merchant and never stored.';
COMMENT ON COLUMN design.preview_tokens.draft_version IS 'design.documents.version at issue time; a later draft save makes the token read nothing.';
COMMENT ON POLICY scope_access ON design.documents IS 'store-design (0087): commerce_runtime only, store scope from the app.tenant_id/app.store_id GUCs.';
COMMENT ON POLICY scope_access ON design.published_versions IS 'store-design (0087): commerce_runtime only, store scope from the GUCs; SELECT and INSERT grants only.';
COMMENT ON POLICY scope_access ON design.store_media IS 'store-design (0087): commerce_runtime only, store scope from the GUCs.';
COMMENT ON POLICY scope_access ON design.preview_tokens IS 'store-design (0087): commerce_runtime only, store scope from the GUCs (issue + prune expired).';
COMMENT ON POLICY design_reader_read ON design.documents IS 'store-design (0087): commerce_design_reader SELECT for design.buyer_preview only, which adds token + store filters.';
COMMENT ON POLICY design_reader_read ON design.published_versions IS 'store-design (0087): commerce_design_reader SELECT for design.buyer_published only.';
COMMENT ON POLICY design_reader_read ON design.store_media IS 'store-design (0087): commerce_design_reader SELECT for design.buyer_media only.';
COMMENT ON POLICY design_reader_read ON design.preview_tokens IS 'store-design (0087): commerce_design_reader SELECT for design.buyer_preview only.';
COMMENT ON POLICY design_reader_store_read ON control.stores IS 'store-design (0087): commerce_design_reader reads active stores (tenant_id, id, name, active) to join the store name and bind a store to its tenant. Non-goal: no currency or owner data.';
COMMENT ON FUNCTION design.refuse_history_change() IS 'store-design (0087): trigger body making design.published_versions append-only for every role.';
COMMENT ON FUNCTION design.buyer_published(text) IS
 'store-design owner; only caller internal/buyerhttp design.go (commerce_buyer_runtime, no buyer token). Resolves the store from the verified origin (PT400/PT404) and returns the highest published version, or version 0 + NULL document when none. No store parameter.';
COMMENT ON FUNCTION design.buyer_preview(text, bytea) IS
 'store-design owner; only caller internal/buyerhttp design.go. Returns the draft only when the SHA-256 of the presented preview token matches an unexpired design.preview_tokens row of the origin store whose bound draft version is still current; every other case is PT404.';
COMMENT ON FUNCTION design.buyer_media(text, uuid) IS
 'store-design owner; only caller internal/buyerhttp design.go (GET /v1/buyer/media/s/{image}). Resolves the store from the verified origin and returns content_type, bytes, sha256 of one design.store_media row of that store; PT404 otherwise.';
