// Package platform owns Stripe database-role admission, reusing the shared gate.
// It never reads payment credentials, verifies webhooks or moves payment state.
package platform

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenStripeIngressPool admits the insert-only webhook authority. Neither the
// merchant nor payment-worker pool may be substituted when this opener fails.
func OpenStripeIngressPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("stripe ingress database unavailable")
	}
	pool, err := openPool(ctx, dsn, "stripe_ingress")
	if err != nil {
		return nil, errors.New("stripe ingress database unavailable")
	}
	return pool, nil
}

// ValidateStripeIngressPool borrows the caller's pool; failure never closes it.
func ValidateStripeIngressPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("stripe ingress database unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := validatePoolAuthority(bounded, pool, "stripe_ingress"); err != nil {
		return errors.New("stripe ingress database unavailable")
	}
	return nil
}

// OpenStripeRegistrarPool admits only the operator's scoped registry definers.
// Connection/parser errors are masked because they can contain DSN credentials.
func OpenStripeRegistrarPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("stripe registrar database unavailable")
	}
	pool, err := openPool(ctx, dsn, "stripe_registrar")
	if err != nil {
		return nil, errors.New("stripe registrar database unavailable")
	}
	return pool, nil
}

// These are the frozen 0061/post-River0012 ABIs, not broad schema/name grants.
// The shared validator checks them for every pool so a direct or PUBLIC grant
// cannot smuggle registrar/ingress authority into an otherwise ordinary login.
var stripeIngressFunctions = []string{
	"payments.stripe_webhook_material(uuid)",
	"payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint)",
	"payments.stripe_webhook_commit(uuid,uuid,bigint)",
}
var stripeRegistrarFunctions = []string{
	"integration.register_stripe_account(uuid,uuid,uuid,uuid,uuid,text,text,text,bytea,bytea)",
	"integration.rotate_stripe_key(uuid,uuid,uuid,uuid,bigint,text,bytea,bytea)",
	"payments.set_stripe_webhook_endpoint(uuid,uuid,uuid,uuid,uuid,text,bigint,boolean,text,bytea,bytea)",
	"payments.qualify_stripe_method(uuid,uuid,uuid,uuid,uuid,bigint,text,text,timestamp with time zone,timestamp with time zone)",
	"payments.set_stripe_method(uuid,uuid,uuid,uuid,text,uuid,uuid,bigint,boolean,boolean,integer,bigint,bigint,text,text,text)",
}

func validateStripeAuthority(ctx context.Context, pool *pgxpool.Pool, authority string) error {
	var allowed []string
	all := append(append([]string{}, stripeIngressFunctions...), stripeRegistrarFunctions...)
	if authority == "stripe_ingress" {
		allowed = stripeIngressFunctions
	} else if authority == "stripe_registrar" {
		allowed = stripeRegistrarFunctions
	}
	if allowed == nil {
		allowed = []string{}
	}
	var forbidden, missing bool
	// pg_proc OIDs avoid schema USAGE checks on catalog lookup. has_*_privilege
	// includes PUBLIC; reachable covers inherited and SET-only custom roles.
	err := pool.QueryRow(ctx, `WITH reachable AS (
	 SELECT oid FROM pg_roles WHERE rolname=session_user
	  OR pg_has_role(session_user,oid,'USAGE') OR pg_has_role(session_user,oid,'SET')
	), fixed AS (
	 SELECT to_regprocedure(signature)::oid AS oid,signature FROM unnest($1::text[]) signature
	)
	SELECT EXISTS(SELECT 1 FROM reachable r CROSS JOIN fixed f
	 WHERE f.oid IS NOT NULL AND NOT(f.signature=ANY($2::text[]))
	  AND has_function_privilege(r.oid,f.oid,'EXECUTE')),
	 EXISTS(SELECT 1 FROM unnest($2::text[]) signature
	  WHERE to_regprocedure(signature) IS NULL
	   OR NOT coalesce(has_function_privilege(session_user,to_regprocedure(signature),'EXECUTE'),false))`,
		all, allowed).Scan(&forbidden, &missing)
	if err != nil || forbidden || missing {
		return errors.New("unsafe stripe database privileges")
	}
	if len(allowed) == 0 {
		return nil
	}
	// New Stripe roles cannot read any application table. The ingress exception
	// is exactly River InsertTx's job and ID sequence, never lifecycle mutations.
	err = pool.QueryRow(ctx, `WITH reachable AS (
	 SELECT oid FROM pg_roles WHERE rolname=session_user
	  OR pg_has_role(session_user,oid,'USAGE') OR pg_has_role(session_user,oid,'SET')
	), allowed AS (
	 SELECT to_regprocedure(signature)::oid AS oid FROM unnest($2::text[]) signature
	)
	SELECT EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_class c
	 JOIN pg_namespace n ON n.oid=c.relnamespace
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	 AND CASE
	  WHEN c.relkind='S' AND $1='stripe_ingress' AND n.nspname='river_payment' AND c.relname='river_job_id_seq'
	   THEN has_sequence_privilege(r.oid,c.oid,'SELECT,UPDATE')
	  WHEN c.relkind='S' THEN has_sequence_privilege(r.oid,c.oid,'USAGE,SELECT,UPDATE')
	  WHEN c.relkind IN ('r','p','v','m','f') AND $1='stripe_ingress' AND n.nspname='river_payment' AND c.relname='river_job'
	   THEN has_table_privilege(r.oid,c.oid,'UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
	    OR has_any_column_privilege(r.oid,c.oid,'UPDATE,REFERENCES')
	  WHEN c.relkind IN ('r','p','v','m','f') THEN
	   has_table_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
	    OR has_any_column_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')
	  ELSE false END)
	 OR EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
	  WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	   AND NOT EXISTS(SELECT 1 FROM allowed a WHERE a.oid=p.oid)
	   AND has_function_privilege(r.oid,p.oid,'EXECUTE'))
	 OR EXISTS(SELECT 1 FROM reachable r CROSS JOIN pg_namespace n
	  WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	   AND has_schema_privilege(r.oid,n.oid,'CREATE'))
	 OR EXISTS(SELECT 1 FROM reachable r WHERE has_database_privilege(r.oid,current_database(),'CREATE'))`,
		authority, allowed).Scan(&forbidden)
	if err != nil || forbidden {
		return errors.New("unsafe stripe database privileges")
	}
	return nil
}
