-- Preparation commits before upstream River migrations. Until the atomic
-- post-River cutover succeeds, even an empty database is deliberately not ready.
CREATE ROLE commerce_meta_worker NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE SCHEMA river_meta;
REVOKE ALL ON SCHEMA river_meta FROM PUBLIC;

CREATE OR REPLACE FUNCTION meta_inbox.runtime_ready()
RETURNS boolean LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 RETURN false;
END $$;
ALTER FUNCTION meta_inbox.runtime_ready() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.runtime_ready() FROM PUBLIC;
-- No new-lane login grants here. post_river/0004 owns the atomic cutover.
