// Purpose: operator-only CLI that suspends/resumes a tenant or store, reports their state, lists the operator audit and
// manages the designated support-principal registry and read-only platform support grants (support-principal-add/-revoke, support-grant / support-revoke / support-list).
// Depends on: pgx (one-connection pool); SQL definers control.set_store_active / set_tenant_active / platform_status /
//
//	read_operator_audit (migration 0143) and identity.grant_support / revoke_support / list_support_grants / add_support_principal / revoke_support_principal (0153), all
//	EXECUTE commerce_platform_operator; env COMMERCE_PLATFORM_OPERATOR_DATABASE_URL.
//
// Used by: deploy/scripts/ops-admin.sh (integrator), tests/foundation/platform_operator_test.go; never the API or a worker.
// Invariants: stderr is one fixed code and stdout one JSON line; no DSN/driver text is ever printed; all inputs are
//
//	validated before a connection opens.
//
// Status: REAL_PG gate.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
)

// Fixed codes: only these strings ever reach stderr (no driver, DSN or flag-value text).
var (
	errUsage    = errors.New("platform_admin_usage")
	errConfig   = errors.New("platform_admin_config")
	errDatabase = errors.New("platform_admin_database")
	errFailed   = errors.New("platform_admin_failed")
	errNotFound = errors.New("platform_admin_not_found")
	errDenied   = errors.New("platform_admin_denied")
	errConflict = errors.New("platform_admin_conflict")
)

var (
	operatorPattern = regexp.MustCompile(`^[a-z0-9._-]{1,40}$`)
	reasons         = map[string]bool{"fraud": true, "non_payment": true, "legal": true, "owner_request": true, "other": true}
	// supportPack is the only permission set a support grant may hold (SQL CHECK in migration 0153 re-validates it).
	supportPack = map[string]bool{"store:read": true, "orders:read": true, "catalog:read": true, "inventory:read": true, "live:read": true, "integration:read": true}
)

// querier is the part of the pool the commands use (QueryRow of one JSON document).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// withDB opens a one-connection pool for the operator login and runs fn; replaceable by tests.
var withDB = func(ctx context.Context, dsn string, fn func(context.Context, querier) ([]byte, error)) ([]byte, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errDatabase // parse errors can echo the DSN
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errDatabase
	}
	defer pool.Close()
	return fn(ctx, pool)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

// call runs one definer that returns a single jsonb document under the caller's context (the 30 s deadline of main
// bounds connect and query alike).
func call(sql string, args ...any) func(context.Context, querier) ([]byte, error) {
	return func(ctx context.Context, q querier) ([]byte, error) {
		var raw []byte
		err := q.QueryRow(ctx, sql, args...).Scan(&raw)
		return raw, err
	}
}

func validTicket(s string) bool {
	if !utf8.ValidString(s) || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > 80 {
		return false
	}
	return strings.IndexFunc(s, unicode.IsControl) < 0
}

// parsePerms splits a comma list of support permissions; empty means "the default pack" (nil, the SQL default).
// Anything outside supportPack is refused before a connection opens.
func parsePerms(list string) ([]string, bool) {
	if list == "" {
		return nil, true
	}
	var out []string
	for _, p := range strings.Split(list, ",") {
		if !supportPack[p] {
			return nil, false
		}
		out = append(out, p)
	}
	return out, true
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if ctx == nil || getenv == nil || stdout == nil || len(args) < 1 {
		return errUsage
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors echo the offending value; usage errors are fixed instead
	var store, tenant, operator, ticket, reason, since, principal, grant, perm string
	limit, hours := 100, 4
	cmd := args[0]
	switch cmd {
	case "store-suspend", "store-resume", "tenant-suspend", "tenant-resume":
		fs.StringVar(&operator, "operator", "", "")
		fs.StringVar(&ticket, "ticket", "", "")
		fs.StringVar(&reason, "reason", "", "")
		fs.StringVar(&store, "store", "", "")
		fs.StringVar(&tenant, "tenant", "", "")
	case "support-grant":
		fs.StringVar(&store, "store", "", "")
		fs.StringVar(&principal, "principal", "", "")
		fs.IntVar(&hours, "hours", 4, "")
		fs.StringVar(&perm, "perm", "", "")
		fs.StringVar(&operator, "operator", "", "")
		fs.StringVar(&ticket, "ticket", "", "")
	case "support-revoke":
		fs.StringVar(&grant, "grant", "", "")
		fs.StringVar(&store, "store", "", "")
		fs.StringVar(&principal, "principal", "", "")
		fs.StringVar(&operator, "operator", "", "")
		fs.StringVar(&ticket, "ticket", "", "")
	case "support-list":
		fs.StringVar(&store, "store", "", "")
	case "support-principal-add", "support-principal-revoke":
		fs.StringVar(&principal, "principal", "", "")
		fs.StringVar(&operator, "operator", "", "")
		fs.StringVar(&ticket, "ticket", "", "")
	case "status":
		fs.StringVar(&store, "store", "", "")
		fs.StringVar(&tenant, "tenant", "", "")
	case "audit":
		fs.StringVar(&since, "since", "", "")
		fs.IntVar(&limit, "limit", 100, "")
	default:
		return errUsage
	}
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return errUsage
	}
	// Everything the operation needs is validated before any connection opens; the SQL re-validates the same rules.
	var fn func(context.Context, querier) ([]byte, error)
	switch cmd {
	case "store-suspend", "store-resume", "tenant-suspend", "tenant-resume":
		resume := strings.HasSuffix(cmd, "-resume")
		isStore := strings.HasPrefix(cmd, "store-")
		target, other := store, tenant
		if !isStore {
			target, other = tenant, store
		}
		if !command.ValidID(target) || other != "" || !operatorPattern.MatchString(operator) || !validTicket(ticket) ||
			(resume && reason != "") || (!resume && !reasons[reason]) {
			return errUsage
		}
		var reasonArg any // NULL on resume
		if !resume {
			reasonArg = reason
		}
		if isStore {
			fn = call(`SELECT control.set_store_active($1::uuid,$2,$3,$4,$5)::text`, target, resume, operator, ticket, reasonArg)
		} else {
			fn = call(`SELECT control.set_tenant_active($1::uuid,$2,$3,$4,$5)::text`, target, resume, operator, ticket, reasonArg)
		}
	case "support-grant":
		perms, ok := parsePerms(perm)
		if !ok || !command.ValidID(store) || !command.ValidID(principal) || hours < 1 || hours > 72 ||
			!operatorPattern.MatchString(operator) || !validTicket(ticket) {
			return errUsage
		}
		fn = call(`SELECT identity.grant_support($1::uuid,$2::uuid,$3::integer,$4::text[],$5,$6)::text`,
			store, principal, int32(hours), perms, operator, ticket)
	case "support-revoke":
		// exactly one form: --grant, or --store with --principal
		byGrant := grant != "" && store == "" && principal == "" && command.ValidID(grant)
		byPair := grant == "" && command.ValidID(store) && command.ValidID(principal)
		if !(byGrant || byPair) || !operatorPattern.MatchString(operator) || !validTicket(ticket) {
			return errUsage
		}
		var g, s, p any // NULL for the form not used
		if byGrant {
			g = grant
		} else {
			s, p = store, principal
		}
		fn = call(`SELECT identity.revoke_support($1::uuid,$2::uuid,$3::uuid,$4,$5)::text`, g, s, p, operator, ticket)
	case "support-list":
		if !command.ValidID(store) {
			return errUsage
		}
		fn = call(`SELECT identity.list_support_grants($1::uuid)::text`, store)
	case "support-principal-add", "support-principal-revoke":
		if !command.ValidID(principal) || !operatorPattern.MatchString(operator) || !validTicket(ticket) {
			return errUsage
		}
		if cmd == "support-principal-add" {
			fn = call(`SELECT identity.add_support_principal($1::uuid,$2,$3)::text`, principal, operator, ticket)
		} else {
			fn = call(`SELECT identity.revoke_support_principal($1::uuid,$2,$3)::text`, principal, operator, ticket)
		}
	case "status":
		if (store == "") == (tenant == "") || (store != "" && !command.ValidID(store)) || (tenant != "" && !command.ValidID(tenant)) {
			return errUsage
		}
		var t, s any
		if store != "" {
			s = store
		} else {
			t = tenant
		}
		fn = call(`SELECT control.platform_status($1::uuid,$2::uuid)::text`, t, s)
	case "audit":
		var sinceArg any
		if since != "" {
			at, err := time.Parse(time.RFC3339, since)
			if err != nil {
				return errUsage
			}
			sinceArg = at
		}
		if limit < 1 || limit > 500 {
			return errUsage
		}
		fn = call(`SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.occurred_at DESC,a.id),'[]'::jsonb)::text
			FROM control.read_operator_audit($1::timestamptz,$2::integer) a`, sinceArg, int32(limit))
	}
	dsn := getenv("COMMERCE_PLATFORM_OPERATOR_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" || len(dsn) > 8192 {
		return errConfig
	}
	raw, err := withDB(ctx, dsn, fn)
	if err != nil {
		return mapFailure(err)
	}
	var line bytes.Buffer
	if json.Compact(&line, raw) != nil {
		return errFailed
	}
	line.WriteByte('\n')
	_, err = stdout.Write(line.Bytes())
	return err
}

// mapFailure reduces every error to one fixed sentinel; the original (which may name a store or the DSN) is dropped.
func mapFailure(err error) error {
	if errors.Is(err, errDatabase) {
		return errDatabase
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return errUsage
		case "PT404":
			return errNotFound
		case "PT409": // support grant refused: already open, store not serving, principal has regular access
			return errConflict
		case "42501":
			return errDenied
		}
	}
	return errFailed
}
