CREATE TABLE fulfillment.pickup_versions (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),
 kind text NOT NULL CHECK(kind IN ('cvs_711','cvs_familymart')),
 namespace text NOT NULL CHECK(namespace ~ '^[a-z][a-z0-9_.:-]{0,63}$'),
 code text NOT NULL CHECK(code ~ '^[A-Za-z0-9_-]{1,32}$'),version bigint NOT NULL CHECK(version>0),
 country text NOT NULL DEFAULT 'TW' CHECK(country='TW'),
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 120 AND btrim(name)<>'' AND name !~ '[[:cntrl:]]'),
 address text NOT NULL CHECK(char_length(address) BETWEEN 1 AND 400 AND btrim(address)<>'' AND address !~ '[[:cntrl:]]'),
 verification_kind text NOT NULL DEFAULT 'MANUAL_ATTESTED' CHECK(verification_kind='MANUAL_ATTESTED'),
 evidence_ref text NOT NULL CHECK(char_length(evidence_ref) BETWEEN 1 AND 240 AND btrim(evidence_ref)<>'' AND evidence_ref !~ '[[:cntrl:]]'),
 principal_id uuid NOT NULL,attested_at timestamptz NOT NULL,valid_until timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),UNIQUE(tenant_id,store_id,kind,namespace,code,version),
 UNIQUE(tenant_id,store_id,kind,namespace,code,version,id),UNIQUE(tenant_id,store_id,id,kind,country),
 CHECK(valid_until>attested_at AND valid_until<=attested_at+interval '7 days'),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE fulfillment.pickup_heads (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,kind text NOT NULL,namespace text NOT NULL,code text NOT NULL,
 current_version bigint NOT NULL,pickup_id uuid NOT NULL,enabled boolean NOT NULL,
 PRIMARY KEY(tenant_id,store_id,kind,namespace,code),
 FOREIGN KEY(tenant_id,store_id,kind,namespace,code,current_version,pickup_id)
  REFERENCES fulfillment.pickup_versions(tenant_id,store_id,kind,namespace,code,version,id)
);
CREATE TABLE storefront.destination_snapshots (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),
 cart_id uuid NOT NULL,cart_version bigint NOT NULL CHECK(cart_version>0),
 creator_session_id uuid NOT NULL,version bigint NOT NULL CHECK(version>0),
 kind text NOT NULL CHECK(kind IN ('home','cvs_711','cvs_familymart')),
 country text NOT NULL CHECK(country ~ '^[A-Z]{2}$'),
 recipient_name text NOT NULL CHECK(char_length(recipient_name) BETWEEN 1 AND 120 AND btrim(recipient_name)<>'' AND recipient_name !~ '[[:cntrl:]]'),
 phone text NOT NULL CHECK(phone ~ '^[+0-9() -]{6,32}$' AND char_length(regexp_replace(phone,'[^0-9]','','g')) BETWEEN 6 AND 20),
 region text NOT NULL DEFAULT '' CHECK(char_length(region)<=100 AND region !~ '[[:cntrl:]]'),
 city text NOT NULL DEFAULT '' CHECK(char_length(city)<=100 AND city !~ '[[:cntrl:]]'),
 postal_code text NOT NULL DEFAULT '' CHECK(char_length(postal_code)<=20 AND postal_code !~ '[[:cntrl:]]'),
 line1 text NOT NULL DEFAULT '' CHECK(char_length(line1)<=200 AND line1 !~ '[[:cntrl:]]'),
 line2 text NOT NULL DEFAULT '' CHECK(char_length(line2)<=200 AND line2 !~ '[[:cntrl:]]'),
 pickup_id uuid,selected_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,owner_id,id),UNIQUE(tenant_id,store_id,owner_id,cart_id,version),
 UNIQUE(tenant_id,store_id,owner_id,cart_id,version,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id) REFERENCES storefront.carts(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,creator_session_id) REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,pickup_id,kind,country) REFERENCES fulfillment.pickup_versions(tenant_id,store_id,id,kind,country),
 CHECK((kind='home' AND pickup_id IS NULL AND btrim(city)<>'' AND btrim(line1)<>'') OR
  (kind IN ('cvs_711','cvs_familymart') AND country='TW' AND pickup_id IS NOT NULL AND region='' AND city='' AND postal_code='' AND line1='' AND line2='')),
 CHECK(expires_at>selected_at AND expires_at<=selected_at+interval '30 minutes')
);
CREATE TABLE storefront.destination_heads (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,cart_id uuid NOT NULL,
 current_version bigint NOT NULL,destination_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,store_id,owner_id,cart_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,cart_id,current_version,destination_id)
  REFERENCES storefront.destination_snapshots(tenant_id,store_id,owner_id,cart_id,version,id)
);
CREATE TABLE storefront.destination_events (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),
 destination_id uuid NOT NULL,session_id uuid NOT NULL,action text NOT NULL CHECK(action='destination.selected'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,destination_id) REFERENCES storefront.destination_snapshots(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,session_id) REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id)
);

DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['pickup_versions','pickup_heads'] LOOP
  EXECUTE format('ALTER TABLE fulfillment.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE fulfillment.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY scoped_read ON fulfillment.%I FOR SELECT TO commerce_runtime,commerce_buyer_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
  EXECUTE format('CREATE POLICY merchant_insert ON fulfillment.%I FOR INSERT TO commerce_runtime
   WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
 END LOOP;
 FOREACH r IN ARRAY ARRAY['destination_snapshots','destination_heads','destination_events'] LOOP
  EXECUTE format('ALTER TABLE storefront.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE storefront.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY buyer_read ON storefront.%I FOR SELECT TO commerce_buyer_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)',r);
  EXECUTE format('CREATE POLICY buyer_insert ON storefront.%I FOR INSERT TO commerce_buyer_runtime
   WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid AND owner_id=nullif(current_setting(''app.buyer_id'',true),'''')::uuid)',r);
 END LOOP;
END $$;
CREATE POLICY pickup_actor ON fulfillment.pickup_versions AS RESTRICTIVE FOR INSERT TO commerce_runtime
 WITH CHECK(principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY pickup_update ON fulfillment.pickup_heads FOR UPDATE TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY pickup_buyer_lock ON fulfillment.pickup_heads FOR UPDATE TO commerce_buyer_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid) WITH CHECK(false);
CREATE POLICY destination_session ON storefront.destination_snapshots AS RESTRICTIVE FOR INSERT TO commerce_buyer_runtime
 WITH CHECK(creator_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE POLICY destination_event_session ON storefront.destination_events AS RESTRICTIVE FOR INSERT TO commerce_buyer_runtime
 WITH CHECK(session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE POLICY destination_update ON storefront.destination_heads FOR UPDATE TO commerce_buyer_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid);

GRANT SELECT,INSERT ON fulfillment.pickup_versions,fulfillment.pickup_heads TO commerce_runtime;
GRANT UPDATE(current_version,pickup_id,enabled) ON fulfillment.pickup_heads TO commerce_runtime;
GRANT USAGE ON SCHEMA fulfillment TO commerce_buyer_runtime;
GRANT SELECT(tenant_id,store_id,id,kind,namespace,code,version,country,name,address,verification_kind,attested_at,valid_until) ON fulfillment.pickup_versions TO commerce_buyer_runtime;
GRANT SELECT ON fulfillment.pickup_heads TO commerce_buyer_runtime;
GRANT UPDATE(current_version) ON fulfillment.pickup_heads TO commerce_buyer_runtime;
GRANT SELECT,INSERT ON storefront.destination_snapshots,storefront.destination_heads,storefront.destination_events TO commerce_buyer_runtime;
GRANT UPDATE(current_version,destination_id) ON storefront.destination_heads TO commerce_buyer_runtime;
