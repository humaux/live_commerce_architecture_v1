// payuni_runtime.go owns PAYUNi notify database-role admission, reusing the shared gate.
// It never reads payment credentials, verifies callbacks or moves payment state.

package platform

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPayuniIngressPool admits the notify-only ingress authority. Neither the merchant nor
// payment-worker pool may be substituted when this opener fails.
func OpenPayuniIngressPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("payuni ingress database unavailable")
	}
	pool, err := openPool(ctx, dsn, "payuni_ingress")
	if err != nil {
		return nil, errors.New("payuni ingress database unavailable")
	}
	return pool, nil
}

// ValidatePayuniIngressPool borrows the caller's pool; failure never closes it.
func ValidatePayuniIngressPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("payuni ingress database unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := validatePoolAuthority(bounded, pool, "payuni_ingress"); err != nil {
		return errors.New("payuni ingress database unavailable")
	}
	return nil
}

// These are the frozen 0136 ABIs, not broad schema/name grants. The notify ingress resolves one
// enabled endpoint and records one idempotent receipt (waking the attempt's existing query job as
// the definer owner); it holds no other payment, integration or river authority.
var payuniIngressFunctions = []string{
	"payments.payuni_resolve_endpoint(bytea)",
	"payments.payuni_record_notify(bytea,bytea,text,text,bigint,text,text)",
}

func validatePayuniAuthority(ctx context.Context, pool *pgxpool.Pool, authority string) error {
	var allowed []string
	all := append([]string{}, payuniIngressFunctions...)
	if authority == "payuni_ingress" {
		allowed = payuniIngressFunctions
	}
	if allowed == nil {
		allowed = []string{}
	}
	var forbidden, missing bool
	var allowedOIDs []uint32
	// pg_proc OIDs avoid schema USAGE checks on catalog lookup. has_*_privilege
	// includes PUBLIC; reachable covers inherited and SET-only custom roles.
	err := pool.QueryRow(ctx, `WITH reachable AS (
	 SELECT oid FROM pg_roles WHERE rolname=session_user
	  OR pg_has_role(session_user,oid,'USAGE') OR pg_has_role(session_user,oid,'SET')
	), fixed AS (
	 SELECT p.oid,w.signature FROM unnest($1::text[]) w(signature)
	 LEFT JOIN pg_catalog.pg_namespace n ON n.nspname=split_part(w.signature,'.',1)
	 LEFT JOIN pg_catalog.pg_proc p ON p.pronamespace=n.oid
	  AND p.proname=split_part(split_part(w.signature,'(',1),'.',2)
	  AND p.proargtypes::pg_catalog.text=pg_catalog.array_to_string(ARRAY(
	   SELECT t.oid FROM unnest(string_to_array(substring(w.signature from '\((.*)\)$'),',')) WITH ORDINALITY a(name,ord)
	   JOIN pg_catalog.pg_type t ON t.typnamespace='pg_catalog'::regnamespace
	    AND t.typname=CASE a.name WHEN 'bigint' THEN 'int8' WHEN 'integer' THEN 'int4'
	     WHEN 'boolean' THEN 'bool' WHEN 'timestamp with time zone' THEN 'timestamptz'
	     WHEN 'text[]' THEN '_text' ELSE a.name END
	   ORDER BY a.ord),' ')
	)
	SELECT EXISTS(SELECT 1 FROM reachable r CROSS JOIN fixed f
	 WHERE f.oid IS NOT NULL AND NOT(f.signature=ANY($2::text[]))
	  AND has_function_privilege(r.oid,f.oid,'EXECUTE')),
	 EXISTS(SELECT 1 FROM fixed f WHERE f.signature=ANY($2::text[])
	  AND (f.oid IS NULL OR NOT coalesce(has_function_privilege(session_user,f.oid,'EXECUTE'),false))),
	 (SELECT coalesce(array_agg(f.oid),'{}'::oid[]) FROM fixed f
	  WHERE f.signature=ANY($2::text[]) AND f.oid IS NOT NULL)`,
		all, allowed).Scan(&forbidden, &missing, &allowedOIDs)
	if err != nil || forbidden || missing {
		return errors.New("unsafe payuni database privileges")
	}
	if len(allowed) == 0 {
		return nil
	}
	// The notify ingress may hold no application table, sequence, schema or database
	// privilege, and may EXECUTE only the two fixed definers (plus the inert PUBLIC
	// river_job_state_in_bitmask helper every role inherits from PUBLIC).
	err = pool.QueryRow(ctx, `WITH reachable AS (
	 SELECT oid FROM pg_roles WHERE rolname=session_user
	  OR pg_has_role(session_user,oid,'USAGE') OR pg_has_role(session_user,oid,'SET')
	), allowed AS (
	 SELECT unnest($1::oid[]) AS oid
	)
	SELECT EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_class c
	 JOIN pg_namespace n ON n.oid=c.relnamespace
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	 AND CASE
	  WHEN c.relkind='S' THEN has_sequence_privilege(r.oid,c.oid,'USAGE,SELECT,UPDATE')
	  WHEN c.relkind IN ('r','p','v','m','f') THEN
	   has_table_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
	    OR has_any_column_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')
	  ELSE false END)
	 OR EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
	  WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	   AND NOT EXISTS(SELECT 1 FROM allowed a WHERE a.oid=p.oid)
	   AND NOT (p.proname='river_job_state_in_bitmask'
	    AND n.nspname IN ('river','river_meta','river_payment','river_expiry','river_media')
	    AND p.provolatile='i' AND NOT p.prosecdef AND p.pronargs=2
	    AND p.proargtypes[0]='pg_catalog.bit'::regtype
	    AND p.proargtypes[1]=(SELECT t.oid FROM pg_catalog.pg_type t
	     WHERE t.typnamespace=n.oid AND t.typname='river_job_state')
	    AND EXISTS(SELECT 1 FROM pg_catalog.pg_class j WHERE j.relnamespace=n.oid
	     AND j.relname='river_job' AND j.relowner=p.proowner))
	   AND has_function_privilege(r.oid,p.oid,'EXECUTE'))
	 OR EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_namespace n
	  WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	   AND has_schema_privilege(r.oid,n.oid,'CREATE'))
	 OR EXISTS(SELECT 1 FROM reachable r WHERE has_database_privilege(r.oid,current_database(),'CREATE'))`,
		allowedOIDs).Scan(&forbidden)
	if err != nil || forbidden {
		return errors.New("unsafe payuni database privileges")
	}
	return nil
}
