-- Purpose: store-scoped hand-kept members and immutable points, LP1-LP19 (LP9 withdrawn).
-- Depends on: 0139 customer authority/fence; command receipts; checkout orders.
-- Used by: internal/loyalty and manual-order final merchant transaction.
-- Invariants: no buyer identity edge, no auto points, one row lock per balance, no PII receipts.
DO $$
DECLARE d text; ps text[];
BEGIN
 SELECT pg_get_constraintdef(oid) INTO STRICT d FROM pg_constraint WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check';
 SELECT array_agg(m[1]) INTO ps FROM regexp_matches(d,'''([a-z_]+:[a-z_]+)''::text','g') m;
 IF ps IS NULL OR NOT 'customers:write'=ANY(ps) THEN RAISE EXCEPTION '0223 permission baseline drift'; END IF;
 IF NOT 'customers:points'=ANY(ps) THEN ps:=array_append(ps,'customers:points'); END IF;
 ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
 EXECUTE format('ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK (permission IN (%s))',(SELECT string_agg(quote_literal(p),',') FROM unnest(ps) p));
END $$;
INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 SELECT tenant_id,store_id,principal_id,p FROM identity.store_staff,unnest(identity.staff_role_permissions(role)) p WHERE role IN ('owner','admin') ON CONFLICT DO NOTHING;

CREATE SCHEMA loyalty;
GRANT USAGE ON SCHEMA loyalty TO commerce_privacy_writer,commerce_runtime;
CREATE TABLE loyalty.members(
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),
 display_name text NOT NULL CHECK(char_length(display_name) BETWEEN 1 AND 60),contact_note text CHECK(char_length(contact_note)<=120),notes text CHECK(char_length(notes)<=1000),
 balance bigint NOT NULL DEFAULT 0 CHECK(balance BETWEEN 0 AND 1000000000000),version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_by uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),erased_at timestamptz,
 PRIMARY KEY(tenant_id,store_id,id),FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id));
CREATE INDEX members_list ON loyalty.members(tenant_id,store_id,created_at DESC,id DESC);
CREATE TABLE loyalty.point_entries(
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),member_id uuid NOT NULL,
 delta integer NOT NULL CHECK(delta<>0 AND delta BETWEEN -1000000 AND 1000000),reason text NOT NULL CHECK(reason IN ('manual_adjust','manual_order_award')),
 order_id uuid,note text,actor_principal_id uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,id),FOREIGN KEY(tenant_id,store_id,member_id) REFERENCES loyalty.members,
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,id),
 CHECK((reason='manual_order_award' AND delta>0 AND order_id IS NOT NULL AND note IS NULL) OR (reason='manual_adjust' AND note IS NOT NULL AND char_length(note) BETWEEN 1 AND 200)));
CREATE UNIQUE INDEX point_entries_one_award ON loyalty.point_entries(tenant_id,store_id,order_id) WHERE reason='manual_order_award';
CREATE INDEX point_entries_member ON loyalty.point_entries(tenant_id,store_id,member_id,created_at DESC,id DESC);
CREATE INDEX point_entries_order ON loyalty.point_entries(tenant_id,store_id,order_id) WHERE order_id IS NOT NULL;
GRANT SELECT,INSERT ON loyalty.members TO commerce_privacy_writer;
GRANT UPDATE(display_name,contact_note,notes,balance,version,updated_at,erased_at) ON loyalty.members TO commerce_privacy_writer;
GRANT SELECT,INSERT ON loyalty.point_entries TO commerce_privacy_writer;
GRANT UPDATE(note) ON loyalty.point_entries TO commerce_privacy_writer;
GRANT SELECT(source) ON checkout.orders TO commerce_privacy_writer;
DO $$
DECLARE t text;
BEGIN
 FOREACH t IN ARRAY ARRAY['members','point_entries'] LOOP
  EXECUTE format('ALTER TABLE loyalty.%I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE loyalty.%I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY scoped_read ON loyalty.%I FOR SELECT TO commerce_privacy_writer USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',t);
  EXECUTE format('CREATE POLICY scoped_insert ON loyalty.%I FOR INSERT TO commerce_privacy_writer WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',t);
  EXECUTE format('CREATE POLICY scoped_update ON loyalty.%I FOR UPDATE TO commerce_privacy_writer USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid) WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',t);
 END LOOP;
END $$;
CREATE FUNCTION loyalty.immutable_entry() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' AND OLD.note IS NOT NULL AND NEW.note='[erased]' AND (to_jsonb(OLD)-'note')=(to_jsonb(NEW)-'note') AND EXISTS(SELECT 1 FROM loyalty.members m WHERE m.tenant_id=OLD.tenant_id AND m.store_id=OLD.store_id AND m.id=OLD.member_id AND m.erased_at IS NOT NULL) THEN RETURN NEW;END IF;
 RAISE EXCEPTION 'immutable ledger' USING ERRCODE='42501';
END $$;
CREATE TRIGGER point_entries_immutable BEFORE UPDATE OR DELETE ON loyalty.point_entries FOR EACH ROW EXECUTE FUNCTION loyalty.immutable_entry();
CREATE TRIGGER point_entries_no_truncate BEFORE TRUNCATE ON loyalty.point_entries FOR EACH STATEMENT EXECUTE FUNCTION loyalty.immutable_entry();
CREATE FUNCTION loyalty.member_json(m loyalty.members) RETURNS jsonb LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT to_jsonb(m)-ARRAY['tenant_id','store_id','created_by']
$$;
CREATE FUNCTION loyalty.entry_json(e loyalty.point_entries) RETURNS jsonb LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT to_jsonb(e)-ARRAY['tenant_id','store_id']
$$;
CREATE FUNCTION loyalty.check_text(n text,c text,b text) RETURNS void LANGUAGE plpgsql IMMUTABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF n IS NULL OR char_length(n) NOT BETWEEN 1 AND 60 OR n<>btrim(n) OR n<>normalize(n,NFC) OR n ~ '[[:cntrl:]]' OR char_length(c)>120 OR char_length(b)>1000 THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
END $$;
CREATE FUNCTION loyalty.create_member(h bytea,s uuid,n text,c text,b text) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');PERFORM loyalty.check_text(n,c,b);
 PERFORM pg_advisory_xact_lock(hashtextextended('loyalty:create:'||s::text,0));
 IF (SELECT count(*) FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s)>=20000 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT422';END IF;
 INSERT INTO loyalty.members(tenant_id,store_id,display_name,contact_note,notes,created_by) VALUES(a.tenant_id,s,n,c,b,a.principal_id) RETURNING * INTO m;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details) VALUES(a.tenant_id,s,a.principal_id,'member.created',jsonb_build_object('member_id',m.id));
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('member_id',m.id,'version',m.version);
END $$;
CREATE FUNCTION loyalty.update_member(h bytea,s uuid,i uuid,v integer,fs text[],n text,c text,b text) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 IF fs IS NULL OR cardinality(fs)=0 OR NOT fs <@ ARRAY['display_name','contact_note','notes'] THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.erased_at IS NOT NULL THEN RAISE EXCEPTION 'member_erased' USING ERRCODE='PT409';END IF;
 IF m.version<>v THEN RAISE EXCEPTION 'conflict' USING ERRCODE='PT409';END IF;
 IF 'display_name'=ANY(fs) THEN m.display_name:=n;END IF;IF 'contact_note'=ANY(fs) THEN m.contact_note:=c;END IF;IF 'notes'=ANY(fs) THEN m.notes:=b;END IF;
 PERFORM loyalty.check_text(m.display_name,m.contact_note,m.notes);
 UPDATE loyalty.members SET display_name=m.display_name,contact_note=m.contact_note,notes=m.notes,version=version+1,updated_at=clock_timestamp() WHERE tenant_id=a.tenant_id AND store_id=s AND id=i RETURNING * INTO m;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details) VALUES(a.tenant_id,s,a.principal_id,'member.updated',jsonb_build_object('member_id',i,'fields',fs));
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('member_id',i,'version',m.version);
END $$;
CREATE FUNCTION loyalty.adjust_points(h bytea,s uuid,i uuid,d integer,n text,o uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;e uuid;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 IF d IS NULL OR d=0 OR d NOT BETWEEN -1000000 AND 1000000 OR n IS NULL OR char_length(n) NOT BETWEEN 1 AND 200 OR btrim(n)='' THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.erased_at IS NOT NULL THEN RAISE EXCEPTION 'member_erased' USING ERRCODE='PT409';END IF;
 IF o IS NOT NULL AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=s AND id=o) THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.balance+d<0 THEN RAISE EXCEPTION 'points_insufficient' USING ERRCODE='PT409';END IF;
 IF m.balance+d>1000000000000 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT422';END IF;
 INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,note,order_id,actor_principal_id) VALUES(a.tenant_id,s,i,d,'manual_adjust',n,o,a.principal_id) RETURNING id INTO e;
 UPDATE loyalty.members SET balance=balance+d,version=version+1,updated_at=clock_timestamp() WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('entry_id',e,'balance',m.balance+d);
END $$;
CREATE FUNCTION loyalty.precheck_award(h bytea,s uuid,i uuid,p integer) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 IF p IS NULL OR p NOT BETWEEN 1 AND 1000000 THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.erased_at IS NOT NULL THEN RAISE EXCEPTION 'member_erased' USING ERRCODE='PT409';END IF;
 IF m.balance+p>1000000000000 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT422';END IF;
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);RETURN '{}'::jsonb;
END $$;
CREATE FUNCTION loyalty.try_award_manual_order(h bytea,s uuid,o uuid,i uuid,p integer) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;e loyalty.point_entries;r text;outcome jsonb;
BEGIN
 outcome:=jsonb_build_object('member_id',i,'points',p,'state','not_awarded');
 BEGIN
  SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 EXCEPTION WHEN SQLSTATE 'PT403' OR SQLSTATE 'PT401' THEN RETURN outcome||jsonb_build_object('reason','forbidden');WHEN SQLSTATE 'PT404' THEN RETURN outcome||jsonb_build_object('reason','member_unavailable');END;
 IF NOT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=s AND id=o AND source='merchant_manual') THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
 IF NOT FOUND THEN r:='member_unavailable';ELSIF m.erased_at IS NOT NULL THEN r:='member_erased';ELSIF p IS NULL OR p NOT BETWEEN 1 AND 1000000 OR m.balance+p>1000000000000 THEN r:='limit_reached';END IF;
 BEGIN
  PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);
 EXCEPTION WHEN SQLSTATE 'PT403' THEN r:='forbidden';END;
 IF r IS NOT NULL THEN RETURN outcome||jsonb_build_object('reason',r);END IF;
 SELECT * INTO e FROM loyalty.point_entries WHERE tenant_id=a.tenant_id AND store_id=s AND order_id=o AND reason='manual_order_award';
 IF NOT FOUND THEN
  INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,order_id,actor_principal_id) VALUES(a.tenant_id,s,i,p,'manual_order_award',o,a.principal_id) RETURNING * INTO e;
 UPDATE loyalty.members SET balance=balance+p,version=version+1,updated_at=clock_timestamp() WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 END IF;
 RETURN jsonb_build_object('member_id',e.member_id,'points',e.delta,'state','awarded','entry_id',e.id);
END $$;
-- Cursor contains only row keys and a digest binding scope/filter; no searched PII.
CREATE FUNCTION loyalty.cursor_encode(t timestamptz,i uuid,b text) RETURNS text LANGUAGE sql IMMUTABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT replace(encode(convert_to(jsonb_build_object('t',t,'i',i,'b',b)::text,'UTF8'),'base64'),E'\n','')
$$;
CREATE FUNCTION loyalty.cursor_decode(c text,b text) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v jsonb;
BEGIN
 IF c IS NULL OR c='' THEN RETURN NULL;END IF;
 BEGIN
 v:=convert_from(decode(c,'base64'),'UTF8')::jsonb;
 IF v->>'b' IS DISTINCT FROM b OR (v->>'t')::timestamptz IS NULL OR (v->>'i')::uuid IS NULL THEN RAISE EXCEPTION 'invalid';END IF;
 EXCEPTION WHEN OTHERS THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END;
 RETURN v;
END $$;
CREATE FUNCTION loyalty.read_members(h bytea,s uuid,q text,inc boolean,c text,l integer) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;b text;v jsonb;outcome jsonb;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:read');
 IF l IS NULL OR l NOT BETWEEN 1 AND 100 OR (q IS NOT NULL AND char_length(q) NOT BETWEEN 1 AND 60) THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422';END IF;
 b:=encode(sha256(convert_to(jsonb_build_array(a.tenant_id,s,q,inc)::text,'UTF8')),'hex');v:=loyalty.cursor_decode(c,b);
 WITH rows AS MATERIALIZED(
 SELECT m.* FROM loyalty.members m WHERE m.tenant_id=a.tenant_id AND m.store_id=s AND (inc OR m.erased_at IS NULL)
 AND (q IS NULL OR strpos(lower(m.display_name),lower(q))>0 OR strpos(lower(coalesce(m.contact_note,'')),lower(q))>0)
 AND (v IS NULL OR (m.created_at,m.id)<((v->>'t')::timestamptz,(v->>'i')::uuid)) ORDER BY m.created_at DESC,m.id DESC LIMIT l+1),
 kept AS (SELECT * FROM rows ORDER BY created_at DESC,id DESC LIMIT l)
 SELECT jsonb_build_object('members',coalesce((SELECT jsonb_agg(jsonb_build_object('id',id,'display_name',display_name,'contact_note',contact_note,'balance',balance,'updated_at',updated_at,'erased',erased_at IS NOT NULL) ORDER BY created_at DESC,id DESC) FROM kept),'[]'::jsonb),
 'next_cursor',CASE WHEN (SELECT count(*) FROM rows)>l THEN (SELECT loyalty.cursor_encode(created_at,id,b) FROM kept ORDER BY created_at,id LIMIT 1) ELSE NULL END) INTO outcome;
 PERFORM customers.tn_fence(h,s,'customers:read',a.tenant_id,a.principal_id,a.authz_revision);RETURN outcome;
END $$;
-- Internal current-row reconstruction uses the write's original permission. Public Get always passes customers:read.
CREATE FUNCTION loyalty.read_member(h bytea,s uuid,i uuid,c text,p text DEFAULT 'customers:read') RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;b text;v jsonb;outcome jsonb;
BEGIN
 IF p NOT IN ('customers:read','customers:points','customers:privacy') OR p IS NULL THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';END IF;
 SELECT * INTO a FROM customers.tn_authority(h,s,p);
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 b:=encode(sha256(convert_to(jsonb_build_array(a.tenant_id,s,i)::text,'UTF8')),'hex');v:=loyalty.cursor_decode(c,b);
 WITH rows AS MATERIALIZED(SELECT * FROM loyalty.point_entries e WHERE e.tenant_id=a.tenant_id AND e.store_id=s AND e.member_id=i
 AND(v IS NULL OR (e.created_at,e.id)<((v->>'t')::timestamptz,(v->>'i')::uuid)) ORDER BY e.created_at DESC,e.id DESC LIMIT 51),
 kept AS (SELECT * FROM rows ORDER BY created_at DESC,id DESC LIMIT 50)
 SELECT jsonb_build_object('member',loyalty.member_json(m),'entries',coalesce((SELECT jsonb_agg((to_jsonb(k)-ARRAY['tenant_id','store_id','member_id','actor_principal_id'])||jsonb_build_object('actor',jsonb_build_object('principal_id',k.actor_principal_id,'display_name',NULL)) ORDER BY k.created_at DESC,k.id DESC) FROM kept k),'[]'::jsonb),
 'next_ledger_cursor',CASE WHEN(SELECT count(*) FROM rows)>50 THEN(SELECT loyalty.cursor_encode(created_at,id,b) FROM kept ORDER BY created_at,id LIMIT 1) ELSE NULL END) INTO outcome;
 PERFORM customers.tn_fence(h,s,p,a.tenant_id,a.principal_id,a.authz_revision);RETURN outcome;
END $$;
CREATE FUNCTION loyalty.read_entry(h bytea,s uuid,i uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;e loyalty.point_entries;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:points');
 SELECT * INTO e FROM loyalty.point_entries WHERE tenant_id=a.tenant_id AND store_id=s AND id=i;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 PERFORM customers.tn_fence(h,s,'customers:points',a.tenant_id,a.principal_id,a.authz_revision);RETURN loyalty.entry_json(e);
END $$;
CREATE FUNCTION loyalty.read_order_entries(h bytea,s uuid,o uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;outcome jsonb;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:read');
 IF NOT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=s AND id=o) THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 WITH rows AS MATERIALIZED(SELECT e.*,m.display_name FROM loyalty.point_entries e JOIN loyalty.members m ON(m.tenant_id,m.store_id,m.id)=(e.tenant_id,e.store_id,e.member_id) WHERE e.tenant_id=a.tenant_id AND e.store_id=s AND e.order_id=o ORDER BY e.created_at,e.id LIMIT 101),
 kept AS(SELECT * FROM rows ORDER BY created_at,id LIMIT 100)
 SELECT jsonb_build_object('entries',coalesce((SELECT jsonb_agg(jsonb_build_object('id',id,'member_id',member_id,'member_display_name',display_name,'delta',delta,'reason',reason,'note',note,'created_at',created_at) ORDER BY created_at,id) FROM kept),'[]'::jsonb),'truncated',(SELECT count(*) FROM rows)>100) INTO outcome;
 PERFORM customers.tn_fence(h,s,'customers:read',a.tenant_id,a.principal_id,a.authz_revision);RETURN outcome;
END $$;
CREATE FUNCTION loyalty.export_member(h bytea,s uuid,i uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;outcome jsonb;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:privacy');
 -- Share lock prevents erasure from racing the export body within its transaction.
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 SELECT jsonb_build_object('member',loyalty.member_json(m),'entries',coalesce(jsonb_agg(loyalty.entry_json(e)-'member_id' ORDER BY e.created_at,e.id),'[]'::jsonb)) INTO outcome FROM loyalty.point_entries e WHERE e.tenant_id=a.tenant_id AND e.store_id=s AND e.member_id=i;
 IF octet_length(outcome::text)>1048576 THEN RAISE EXCEPTION 'export_too_large' USING ERRCODE='PT409';END IF;
 PERFORM customers.tn_fence(h,s,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);RETURN outcome;
END $$;
CREATE FUNCTION loyalty.erase_member(h bytea,s uuid,i uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;m loyalty.members;n bigint;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(h,s,'customers:privacy');
 SELECT * INTO m FROM loyalty.members WHERE tenant_id=a.tenant_id AND store_id=s AND id=i FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';END IF;
 IF m.erased_at IS NOT NULL THEN RAISE EXCEPTION 'member_erased' USING ERRCODE='PT409';END IF;
 UPDATE loyalty.members SET display_name='[erased]',contact_note=NULL,notes=NULL,erased_at=coalesce(erased_at,clock_timestamp()),updated_at=clock_timestamp(),version=version+1 WHERE tenant_id=a.tenant_id AND store_id=s AND id=i RETURNING * INTO m;
 UPDATE loyalty.point_entries SET note='[erased]' WHERE tenant_id=a.tenant_id AND store_id=s AND member_id=i AND note IS NOT NULL AND note<>'[erased]';GET DIAGNOSTICS n=ROW_COUNT;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details) VALUES(a.tenant_id,s,a.principal_id,'member.erased',jsonb_build_object('member_id',i,'entries_redacted',n));
 PERFORM customers.tn_fence(h,s,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);RETURN jsonb_build_object('member_id',i,'erased_at',m.erased_at,'entries_redacted',n);
END $$;
-- Like customers.replay_erasures, only the migration/ops owner may call this restore helper.
-- Its fixed explicit id list is authoritative; runtime and workers have no EXECUTE.
CREATE FUNCTION loyalty.replay_member_erasures(ids uuid[]) RETURNS integer LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE m loyalty.members;n integer:=0;t text;s text;
BEGIN
 t:=current_setting('app.tenant_id',true);s:=current_setting('app.store_id',true);
 FOR m IN SELECT * FROM loyalty.members WHERE id=ANY(ids) LOOP
  PERFORM set_config('app.tenant_id',m.tenant_id::text,true);PERFORM set_config('app.store_id',m.store_id::text,true);
  PERFORM 1 FROM loyalty.members WHERE tenant_id=m.tenant_id AND store_id=m.store_id AND id=m.id FOR UPDATE;
  UPDATE loyalty.members SET display_name='[erased]',contact_note=NULL,notes=NULL,erased_at=coalesce(erased_at,clock_timestamp()),version=version+1,updated_at=clock_timestamp() WHERE tenant_id=m.tenant_id AND store_id=m.store_id AND id=m.id;
  UPDATE loyalty.point_entries SET note='[erased]' WHERE tenant_id=m.tenant_id AND store_id=m.store_id AND member_id=m.id AND note IS NOT NULL AND note<>'[erased]';n:=n+1;
 END LOOP;
 PERFORM set_config('app.tenant_id',coalesce(t,''),true);PERFORM set_config('app.store_id',coalesce(s,''),true);RETURN n;
END $$;
-- Restore enumeration matches CD8's privileged reader, all merchant functions above additionally filter exact authenticated scope.
CREATE POLICY restore_read ON loyalty.members FOR SELECT TO commerce_privacy_writer USING(true);
DO $$
DECLARE f record;
BEGIN
 FOR f IN SELECT p.oid::regprocedure sig,p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='loyalty' LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_privacy_writer',f.sig);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f.sig);
  EXECUTE format('COMMENT ON FUNCTION %s IS %L',f.sig,'internal/loyalty LP-1: '||f.proname||'; scoped merchant ledger/privacy contract, no automatic identity links.');
  IF f.proname=ANY(ARRAY['create_member','update_member','adjust_points','precheck_award','try_award_manual_order','read_members','read_member','read_entry','read_order_entries','export_member','erase_member']) THEN EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime',f.sig);END IF;
 END LOOP;
END $$;
-- Preserve the current audited action set and add only member actions with the same authenticated scope fence.
DO $$
DECLARE v text;
BEGIN
 SELECT pg_get_expr(polwithcheck,polrelid) INTO STRICT v FROM pg_policy WHERE polrelid='ops.audit_events'::regclass AND polname='privacy_audit_insert';
 IF position('customers.note_deleted' IN v)=0 THEN RAISE EXCEPTION '0223 audit policy baseline drift';END IF;
 EXECUTE format('ALTER POLICY privacy_audit_insert ON ops.audit_events WITH CHECK ((%s) OR (action IN (''member.created'',''member.updated'',''member.exported'',''member.erased'') AND tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid AND principal_id=nullif(current_setting(''app.principal_id'',true),'''')::uuid))',v);
END $$;
