package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/metareply"
)

var (
	errUsage    = errors.New("meta_admin_usage")
	errConfig   = errors.New("meta_admin_config")
	errDatabase = errors.New("meta_admin_database")
	errRegister = errors.New("meta_admin_register_failed")
	errConflict = errors.New("meta_admin_version_conflict")
)

var (
	assetPattern = regexp.MustCompile(`^[0-9]{1,40}$`)
	scopePattern = regexp.MustCompile(`^[a-z_]{1,64}$`)
)

// register is the database step, replaceable by tests.
var register = func(ctx context.Context, dsn string, keys *metareply.PageTokenKeyring, r metareply.Registration, token string) (int64, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return 0, errDatabase // parse errors can echo the DSN
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return 0, errDatabase
	}
	defer pool.Close()
	// RegisterPageToken: seals the token, then integration.register_meta_page_token as the registrar login.
	v, err := metareply.RegisterPageToken(ctx, pool, keys, r, token)
	switch {
	case err == nil:
		return v, nil
	case errors.Is(err, command.ErrConflict):
		return 0, errConflict
	case errors.Is(err, command.ErrInvalid):
		return 0, errUsage
	default:
		return 0, errRegister
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		// Only fixed sentinel text can reach here: no driver, Graph or flag-value message.
		_, _ = io.WriteString(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if ctx == nil || getenv == nil || stdout == nil || len(args) < 1 || args[0] != "page-token" {
		return errUsage
	}
	fs := flag.NewFlagSet("page-token", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors echo the offending value; usage errors are fixed instead
	var r metareply.Registration
	var scopes string
	fs.StringVar(&r.TenantID, "tenant", "", "")
	fs.StringVar(&r.StoreID, "store", "", "")
	fs.StringVar(&r.PrincipalID, "principal", "", "")
	fs.StringVar(&r.BindingID, "binding", "", "")
	fs.StringVar(&r.Provider, "provider", "", "")
	fs.StringVar(&r.AssetID, "asset", "", "")
	fs.Int64Var(&r.ExpectedVersion, "expected-version", 0, "")
	fs.StringVar(&scopes, "scopes", "", "")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return errUsage
	}
	r.Scopes = strings.Split(scopes, ",")
	valid := command.ValidID(r.TenantID) && command.ValidID(r.StoreID) && command.ValidID(r.PrincipalID) &&
		command.ValidID(r.BindingID) && (r.Provider == "facebook" || r.Provider == "instagram") &&
		assetPattern.MatchString(r.AssetID) && r.ExpectedVersion >= 0 && r.ExpectedVersion < 1<<62 &&
		len(r.Scopes) >= 1 && len(r.Scopes) <= 16
	for _, s := range r.Scopes {
		valid = valid && scopePattern.MatchString(s)
	}
	if !valid {
		return errUsage
	}
	// Everything the operation needs from the environment is validated before any connection opens.
	dsn := getenv("COMMERCE_META_REGISTRAR_DATABASE_URL")
	token := getenv("META_PAGE_ACCESS_TOKEN")
	if strings.TrimSpace(dsn) == "" || len(dsn) > 8192 || token == "" {
		return errConfig
	}
	keys, err := metareply.LoadPageTokenKeyring(getenv)
	if err != nil {
		return errConfig
	}
	version, err := register(ctx, dsn, keys, r, token)
	if err != nil {
		return err
	}
	out, err := json.Marshal(map[string]int64{"version": version})
	if err != nil {
		return errConfig
	}
	_, err = stdout.Write(append(out, '\n'))
	return err
}
