package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
)

// Fixed codes: only these strings ever reach stderr (no driver, DSN, flag-value or evidence text).
var (
	errUsage          = errors.New("store_admin_usage")
	errConfig         = errors.New("store_admin_config")
	errDatabase       = errors.New("store_admin_database")
	errFailed         = errors.New("store_admin_failed")
	errNotFound       = errors.New("store_admin_not_found")
	errConflict       = errors.New("store_admin_conflict")
	errDetached       = errors.New("store_admin_domain_detached")
	errOwnedElsewhere = errors.New("store_admin_domain_owned_elsewhere")
	errNoOwner        = errors.New("store_admin_no_owner_principal")
)

// withDB opens a one-connection registrar pool (login lc_store_registrar) and runs fn; replaceable by tests.
var withDB = func(ctx context.Context, dsn string, fn func(storefrontadmin.Querier) (any, error)) (any, error) {
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
	return fn(pool)
}

var now = time.Now // replaceable by tests

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if ctx == nil || getenv == nil || stdout == nil || len(args) < 1 {
		return errUsage
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors echo the offending value; usage errors are fixed instead
	var store, origin, evidence, validUntil string
	var call func(storefrontadmin.Querier) (any, error)
	switch args[0] {
	case "domain-bind":
		fs.StringVar(&store, "store", "", "")
		fs.StringVar(&origin, "origin", "", "")
		fs.StringVar(&evidence, "evidence", "", "")
		fs.StringVar(&validUntil, "valid-until", "", "")
	case "domain-suspend", "domain-detach":
		fs.StringVar(&origin, "origin", "", "")
	case "status":
		fs.StringVar(&store, "store", "", "")
	default:
		return errUsage
	}
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return errUsage
	}
	// Everything the operation needs is validated before any connection opens.
	switch args[0] {
	case "domain-bind":
		until, err := time.Parse(time.RFC3339, validUntil)
		at := now()
		if err != nil || !command.ValidID(store) || !domains.ValidOrigin(origin) || !storefrontadmin.ValidEvidence(evidence) ||
			!until.After(at) || until.After(at.Add(storefrontadmin.MaxProofLifetime)) {
			return errUsage
		}
		// storefrontadmin re-validates the same rules; checking here keeps a bad flag from opening a connection.
		call = func(q storefrontadmin.Querier) (any, error) {
			return storefrontadmin.BindDomain(ctx, q, store, origin, evidence, until, at)
		}
	case "domain-suspend":
		if !domains.ValidOrigin(origin) {
			return errUsage
		}
		call = func(q storefrontadmin.Querier) (any, error) { return storefrontadmin.SuspendDomain(ctx, q, origin) }
	case "domain-detach":
		if !domains.ValidOrigin(origin) {
			return errUsage
		}
		call = func(q storefrontadmin.Querier) (any, error) { return storefrontadmin.DetachDomain(ctx, q, origin) }
	case "status":
		if !command.ValidID(store) {
			return errUsage
		}
		call = func(q storefrontadmin.Querier) (any, error) { return storefrontadmin.Status(ctx, q, store) }
	}
	dsn := getenv("COMMERCE_STORE_REGISTRAR_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" || len(dsn) > 8192 {
		return errConfig
	}
	out, err := withDB(ctx, dsn, call)
	if err != nil {
		return mapFailure(err)
	}
	line, err := json.Marshal(out)
	if err != nil {
		return errConfig
	}
	_, err = stdout.Write(append(line, '\n'))
	return err
}

// mapFailure reduces every error to one fixed sentinel; the original (which may name a store or origin) is dropped.
func mapFailure(err error) error {
	switch {
	case errors.Is(err, errDatabase):
		return errDatabase
	case errors.Is(err, command.ErrInvalid):
		return errUsage
	case errors.Is(err, platform.ErrScopeNotFound):
		return errNotFound
	case errors.Is(err, storefrontadmin.ErrDomainDetached):
		return errDetached
	case errors.Is(err, storefrontadmin.ErrDomainOwnedElsewhere):
		return errOwnedElsewhere
	case errors.Is(err, storefrontadmin.ErrNoOwner):
		return errNoOwner
	case errors.Is(err, command.ErrConflict):
		return errConflict
	default:
		return errFailed
	}
}
