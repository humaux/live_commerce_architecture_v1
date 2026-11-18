// Purpose: run the disposable Go W1 HTTP-to-PostgreSQL adapter on IPv4 loopback.
// Depends on: the frozen neutral fixture, pgx and the l01_w1_ingress login on PostgreSQL 18.
// Used by: the neutral HTTP conformance client and integrator-owned fixture runs.
// Invariants: the command connects as fixture ingress only and exposes a single fixture route.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	loopback    = "127.0.0.1"
	database    = "lc_neutral_w1_test"
	ingressRole = "l01_w1_ingress"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./bench/l01/neutral/go <loopback-http-port>")
	}
	port, err := fixturePort(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := run(port); err != nil {
		log.Fatal(err)
	}
}

func run(port uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := openIngress(ctx, os.Getenv("PGPASSWORD"))
	if err != nil {
		return err
	}
	defer pool.Close()

	addr := fmt.Sprintf("%s:%d", loopback, port)
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}
	log.Printf("W1_FIXTURE_LISTENING %s", addr)
	server := &http.Server{
		Handler:           newWebhook(pool.BeginTx),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      8 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
	return server.Serve(listener)
}

func fixturePort(port string) (uint64, error) {
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 || strconv.FormatUint(number, 10) != port {
		return 0, errors.New("an explicit TCP port from 1 to 65535 is required")
	}
	return number, nil
}

func fixtureConnString(role string) (string, error) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("PGHOST") != loopback ||
		os.Getenv("PGHOSTADDR") != "" || os.Getenv("PGSERVICE") != "" || os.Getenv("PGOPTIONS") != "" {
		return "", errors.New("W1 requires disposable database opt-in and literal loopback PGHOST with no routing overrides")
	}
	port, err := fixturePort(os.Getenv("PGPORT"))
	if err != nil {
		return "", err
	}
	// Role is selected by the fixture code, never by a request or arbitrary DSN.
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s sslmode=disable connect_timeout=2", loopback, port, database, role), nil
}

func openIngress(ctx context.Context, password string) (*pgxpool.Pool, error) {
	connString, err := fixtureConnString(ingressRole)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, errors.New("invalid W1 ingress connection configuration")
	}
	if password != "" {
		cfg.ConnConfig.Password = password
	}
	cfg.ConnConfig.RuntimeParams = map[string]string{"application_name": "l01-w1-go"}
	cfg.MaxConns = 8
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		var valid bool
		err := conn.QueryRow(ctx, `
			SELECT session_user = 'l01_w1_ingress' AND current_user = session_user
				AND current_database() = 'lc_neutral_w1_test'
				AND current_setting('server_version_num')::int BETWEEN 180000 AND 189999
				AND NOT (rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole)
			FROM pg_roles WHERE rolname = session_user`).Scan(&valid)
		if err != nil || !valid {
			return errors.New("W1 requires the restricted ingress login on PostgreSQL 18")
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not create W1 fixture pool")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("could not connect to W1 as the restricted ingress login")
	}
	return pool, nil
}
