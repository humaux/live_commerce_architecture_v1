// Purpose: claims-worker process — poll the claims intake, dispatch external operations (Meta reply/audience/live-videos, inbox resubscribe, ECPay CVS) through the River default queue, and run the retention purge and store-domain verify sweeps. The only process holding the Meta Page private keyring and ECPay custody.
// Depends on: platform worker pools, metareply routes, claimsintake, retention, storefrontdomains, ecpayroute, core.NewDispatcher, river (schema "river"), COMMERCE_CLAIMS_WORKER_ENABLED.
// Used by: the deployed claims-worker binary; cmd/claims-worker/main_test.go, cmd/claims-worker/console_listen_test.go, console_testhook.go (test tag only); tests/foundation process tests (claims_retention_worker_test.go, claims_worker_console_process_test.go, order_attempt_*).
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/claims"
	"livecommerce/internal/claimsintake"
	"livecommerce/internal/domains"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/integrations/shipping/ecpay"
	"livecommerce/internal/integrations/shipping/ecpay/ecpayroute"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
	"livecommerce/internal/retention"
	"livecommerce/internal/storefrontdomains"
)

var (
	errWorkerConfig   = errors.New("claims_worker_invalid_config")
	errWorkerDatabase = errors.New("claims_worker_database_unavailable")
	errWorkerRoutes   = errors.New("claims_worker_routes_unavailable")
	errWorkerStart    = errors.New("claims_worker_start_failed")
	errWorkerStop     = errors.New("claims_worker_stop_failed")
	// The bound comment bridge stopped serving (any time after the bind, before or after ready). A bind failure stays
	// errWorkerStart: it always happens before ready (#83, #107).
	errWorkerConsole = errors.New("claims_worker_console_failed")
)

type workerConfig struct {
	enabled               bool
	intakeDSN             string
	workerDSN             string
	retentionDSN          string
	storeVerifyDSN        string
	storeVerifyBaseDomain string
	linkKey               claims.ReplyLinkKey
	pageKeys              *metareply.PageTokenKeyring
	pageOpen              *pageopen.Keyring // HPKE private ring of meta-page-token-v2 (merchant connect); nil = v2 credentials are denied
	graph                 metareply.Config
	// Live-console comment bridge (live-console-v1 §2, unit LC-B2): served only when consoleAddr is set.
	// bridgeToken is shared with cmd/api's BridgeClient; cursorKey is the worker's older_cursor HMAC key.
	consoleAddr   string
	consoleHolder string
	bridgeToken   []byte
	cursorKey     []byte
	// meta connection-health probe (§4): the Page app id P3 matches against, and the §3.1 reader config the probe
	// shares with the API's snapshot reader (same Advanced Access / DM flags).
	metaHealthPageAppID string
	metaHealthRCfg      metaconnect.ReaderConfig
	// ECPay CVS route (taiwan-cvs-logistics-v1 §7.4): registered only when ecpayCfg.Enabled.
	ecpayCfg    ecpay.Config
	ecpayKeys   *ecpay.Keyring
	ecpayClient *ecpay.Client
	// OA1 custody is loaded only when LC_ORDER_ATTEMPT_ENABLED=1.
	orderAttempt orderAttemptConfig
}

func (workerConfig) String() string     { return "claimsWorkerConfig{redacted}" }
func (c workerConfig) GoString() string { return c.String() }
func (workerConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal("claimsWorkerConfig{redacted}")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func validDSN(s string) bool { return len(s) >= 1 && len(s) <= 8192 && strings.TrimSpace(s) != "" }

// validConsoleAddr accepts a TCP listen address for the internal comment bridge (":8081" or
// "0.0.0.0:8081"); it never accepts a URL (the bridge base URL lives on the API side).
func validConsoleAddr(s string) bool {
	_, port, err := net.SplitHostPort(s)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

// metaPageAppIDPattern is the Meta app id shape (COMMERCE_META_PAGE_APP_ID, §4.2 P3).
var metaPageAppIDPattern = regexp.MustCompile(`^[0-9]{1,40}$`)

// loadConfig reads only the variables listed in doc.go: never K_actor or a Stripe
// variable. OA1 notification custody is opt-in and checked before pool startup.
func loadConfig(getenv func(string) string) (workerConfig, error) {
	var c workerConfig
	if getenv == nil {
		return c, errWorkerConfig
	}
	switch getenv("COMMERCE_CLAIMS_WORKER_ENABLED") {
	case "", "0":
		return c, nil
	case "1":
		c.enabled = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	c.intakeDSN = getenv("COMMERCE_CLAIMS_INTAKE_DATABASE_URL")
	c.workerDSN = getenv("COMMERCE_WORKER_DATABASE_URL")
	// U08 (claims-retention-purge-v1 §5): the hourly purge job runs on its own login, required whenever the worker is on.
	c.retentionDSN = getenv("COMMERCE_RETENTION_JOB_DATABASE_URL")
	// R5 store-domains (P0-2): the DNS/TLS verify sweep runs here on its own login; base domain = the platform zone.
	c.storeVerifyDSN = getenv("COMMERCE_STORE_VERIFY_DATABASE_URL")
	c.storeVerifyBaseDomain = strings.ToLower(strings.TrimSpace(getenv("LC_STORE_BASE_DOMAIN")))
	if !validDSN(c.intakeDSN) || !validDSN(c.workerDSN) || !validDSN(c.retentionDSN) || !validDSN(c.storeVerifyDSN) {
		return workerConfig{}, errWorkerConfig
	}
	if !domains.ValidOrigin("https://" + c.storeVerifyBaseDomain) {
		return workerConfig{}, errWorkerConfig
	}
	raw, err := base64.StdEncoding.DecodeString(getenv("COMMERCE_CLAIMS_REPLY_LINK_KEY"))
	if err != nil {
		return workerConfig{}, errWorkerConfig
	}
	if c.linkKey, err = claims.NewReplyLinkKey(raw); err != nil {
		return workerConfig{}, errWorkerConfig
	}
	if c.pageKeys, err = metareply.LoadPageTokenKeyring(getenv); err != nil {
		return workerConfig{}, errWorkerConfig
	}
	// The v2 private ring is the OPEN half of the merchant connect's seal (cmd/api holds only the public ring). Present = it must load.
	if getenv("COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS") != "" || getenv("COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS_FILE") != "" {
		if c.pageOpen, err = pageopen.LoadKeyring(getenv); err != nil {
			return workerConfig{}, errWorkerConfig
		}
	}
	c.graph = metareply.Config{GraphBaseURL: metareply.GraphHost, GraphVersion: getenv("COMMERCE_META_GRAPH_VERSION")}
	if base := getenv("COMMERCE_META_GRAPH_BASE_URL"); base != "" {
		c.graph.GraphBaseURL = base
	}
	switch getenv("COMMERCE_META_GRAPH_AUTH_HEADER") {
	case "", "0":
	case "1":
		c.graph.AuthorizationHeader = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	if c.graph.Validate() != nil {
		return workerConfig{}, errWorkerConfig
	}
	// Live-console comment bridge (LC-B2): off unless the listen address is set; when set, the shared
	// 32-byte bridge token and the 32-byte cursor key are required (base64, like the reply link key).
	c.consoleAddr = strings.TrimSpace(getenv("COMMERCE_CLAIMS_CONSOLE_ADDR"))
	if c.consoleAddr != "" {
		if !validConsoleAddr(c.consoleAddr) {
			return workerConfig{}, errWorkerConfig
		}
		c.consoleHolder = getenv("HOSTNAME")
		if c.consoleHolder == "" {
			c.consoleHolder = "claims-worker"
		}
		if c.bridgeToken, err = base64.StdEncoding.DecodeString(getenv("COMMERCE_CLAIMS_BRIDGE_TOKEN")); err != nil || len(c.bridgeToken) != 32 {
			return workerConfig{}, errWorkerConfig
		}
		if c.cursorKey, err = base64.StdEncoding.DecodeString(getenv("COMMERCE_CLAIMS_CURSOR_KEY")); err != nil || len(c.cursorKey) != 32 {
			return workerConfig{}, errWorkerConfig
		}
	}
	// meta connection-health probe (§4.2 P3): our app id is required (a missing value would make every Page read as
	// "not subscribed"); the Advanced Access / DM flags default to the safe pre-review values.
	// Optional (2026-10-06 integrator fix): unset = the health probe is OFF (one startup log line) instead of the whole
	// worker refusing to start — the pilot and existing deployments predate this variable. Set but malformed is refused.
	c.metaHealthPageAppID = strings.TrimSpace(getenv("COMMERCE_META_PAGE_APP_ID"))
	if c.metaHealthPageAppID != "" && !metaPageAppIDPattern.MatchString(c.metaHealthPageAppID) {
		return workerConfig{}, errWorkerConfig
	}
	c.metaHealthRCfg = metaconnect.NewReaderConfig(getenv("COMMERCE_META_ADVANCED_ACCESS"), getenv("COMMERCE_META_DM_RECEIVER_CONFIRMED") == "1")
	if c.ecpayCfg, err = ecpay.LoadConfig(getenv); err != nil {
		return workerConfig{}, errWorkerConfig
	}
	if c.ecpayCfg.Enabled {
		// One ECPay environment per deployment = its payment profile (§12, LQ5): PROVIDER_MOCK runs SANDBOX.
		env := ecpay.EnvSandbox
		switch getenv("COMMERCE_PAYMENT_PROFILE") {
		case "PROVIDER_MOCK", "SANDBOX":
		case "LIVE":
			env = ecpay.EnvLive
		default:
			return workerConfig{}, errWorkerConfig
		}
		if c.ecpayKeys, err = ecpay.LoadKeyring(getenv); err != nil {
			return workerConfig{}, errWorkerConfig
		}
		if c.ecpayClient, err = ecpay.NewClient(env, nil); err != nil {
			return workerConfig{}, errWorkerConfig
		}
	}
	if c.orderAttempt, err = loadOrderAttemptConfig(getenv); err != nil {
		return workerConfig{}, errWorkerConfig
	}
	return c, nil
}

func run(ctx context.Context, getenv func(string) string) error {
	c, err := loadConfig(getenv)
	if err != nil || !c.enabled {
		return err
	}
	if ctx == nil {
		return errWorkerConfig
	}
	startup, done := context.WithTimeout(ctx, 20*time.Second)
	defer done()
	intakePool, err := platform.OpenClaimsIntakePool(startup, c.intakeDSN)
	if err != nil {
		return errWorkerDatabase
	}
	defer intakePool.Close()
	workerPool, err := platform.OpenWorkerPool(startup, c.workerDSN, platform.WorkerClaims)
	if err != nil {
		return errWorkerDatabase
	}
	defer workerPool.Close()
	if !sameDatabase(startup, intakePool, workerPool) {
		return errWorkerDatabase // the operation the poller plans must be the one the dispatcher reads
	}
	// platform.OpenRetentionJobPool: admission of lc_retention_job (exactly one authority); the purge job's only SQL.
	retentionPool, err := platform.OpenRetentionJobPool(startup, c.retentionDSN)
	if err != nil {
		return errWorkerDatabase
	}
	defer retentionPool.Close()
	if !sameDatabase(startup, retentionPool, workerPool) {
		return errWorkerDatabase // the purge must run against the database whose river schema holds its job
	}
	retentionWorker, err := retention.NewWorker(retentionPool)
	if err != nil {
		return errWorkerDatabase
	}
	// platform.OpenStoreDomainVerifyPool: admission of lc_store_domain_verify (exactly one authority); the sweep's only SQL.
	storeVerifyPool, err := platform.OpenStoreDomainVerifyPool(startup, c.storeVerifyDSN)
	if err != nil {
		return errWorkerDatabase
	}
	defer storeVerifyPool.Close()
	if !sameDatabase(startup, storeVerifyPool, workerPool) {
		return errWorkerDatabase // the sweep must run against the database whose rows it advances
	}
	storeVerifyWorker, err := storefrontdomains.NewWorker(storeVerifyPool, c.storeVerifyBaseDomain)
	if err != nil {
		return errWorkerDatabase
	}
	routes, err := metareply.RoutesV2(workerPool, c.linkKey, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	// D9: read-only Page audience route reuses this process's private token custody.
	audienceRoutes, err := metareply.AudienceRoutes(workerPool, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	routes = append(routes, audienceRoutes...)
	// A5-3 (MOCK): read-only Page "live videos" route reuses this process's private token custody
	// (pages_read_engagement only); the merchant API never loads a Page token.
	liveVideoRoutes, err := metareply.LiveVideoRoutes(workerPool, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	routes = append(routes, liveVideoRoutes...)
	// live-console-v1 §4.3 (LC-B4): DM / public reply / offer recommend routes; the manual private reply shares the meta.private_reply route above.
	sendRoutes, err := metareply.SendRoutes(workerPool, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	routes = append(routes, sendRoutes...)
	if c.ecpayCfg.Enabled {
		ecpayRoutes, err := ecpayroute.Routes(workerPool, c.ecpayKeys, c.ecpayClient, c.ecpayCfg)
		if err != nil {
			return errWorkerRoutes
		}
		routes = append(routes, ecpayRoutes...)
	}
	dispatcher, err := core.NewDispatcher(startup, workerPool, routes, core.DefaultDispatcherOptions())
	if err != nil {
		return errWorkerRoutes
	}
	// Disconnect's durable "DELETE /{page}/subscribed_apps" jobs (migration 0100): needs the private ring that only this process holds.
	unsubscriber, err := metareply.NewUnsubscriber(workerPool, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	// Inbox's durable "POST /{page}/subscribed_apps subscribed_fields=feed,messages" jobs (migration 0119): the same private ring.
	resubscriber, err := metareply.NewResubscriber(workerPool, c.pageKeys, c.pageOpen, c.graph)
	if err != nil {
		return errWorkerRoutes
	}
	// meta connection-health probe sweep (§4): read-only Graph probe of due Pages, every 5 min + on start.
	var prober *metareply.Prober
	if c.metaHealthPageAppID != "" {
		if prober, err = metareply.NewProber(workerPool, c.pageKeys, c.pageOpen, c.graph.GraphBaseURL, c.graph.GraphVersion, c.metaHealthPageAppID, c.metaHealthRCfg); err != nil {
			return errWorkerRoutes
		}
	} else {
		slog.Info("meta_health_probe_disabled", "reason", "COMMERCE_META_PAGE_APP_ID unset")
	}
	poller, err := claimsintake.New(startup, intakePool, c.linkKey, claimsintake.Config{})
	if err != nil {
		return errWorkerDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, dispatcher)
	river.AddWorker(workers, retentionWorker)
	river.AddWorker(workers, storeVerifyWorker)
	if prober != nil {
		river.AddWorker(workers, prober)
	}
	periodic := []*river.PeriodicJob{retention.PeriodicJob(), storefrontdomains.PeriodicJob(), metareply.ProbePeriodicJob()}
	queues := map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}
	if job, err := addOrderAttemptWorkers(c.orderAttempt, workerPool, workers); err != nil {
		return err
	} else if job != nil {
		periodic = append(periodic, job)
		queues["claims"] = river.QueueConfig{MaxWorkers: 4}
	}
	// Default queue of the main river schema: the only queue external_operation_v1 jobs use. A stuck job
	// (crash mid-dispatch) is rescued after one minute so a reply is not stranded for River's default hour.
	client, err := river.NewClient(riverpgxv5.New(workerPool), &river.Config{
		Schema: "river", Workers: workers, RescueStuckJobsAfter: retention.RescueWindow,
		// U08: hourly + on start, unique per hour (retention.JobArgs.InsertOpts); inserted by commerce_claims_worker (IR-4).
		// R5 store-domains (P0-2): once a minute + on start (storefrontdomains.PeriodicJob).
		// W1-01B: the meta connection-health sweep every 5 min + on start (metareply.ProbePeriodicJob).
		PeriodicJobs: periodic,
		Queues:       queues,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return errWorkerStart
	}
	done()
	// IR-13: operations without a registered route stay READY by design, so the log names every route this
	// process serves. Fixed provider/action/purpose strings only.
	names := make([]string, 0, len(routes))
	for _, r := range routes {
		names = append(names, r.Provider+"/"+r.Action+"/"+r.Purpose)
	}
	slog.Info("claims_worker_routes", "routes", strings.Join(names, ","))

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	var consoleDown <-chan struct{} // nil without a bridge: never ready
	var consoleRun sync.WaitGroup   // joins console.Run (zero value: no bridge, Wait returns at once)
	pollCtx, stopPoll := context.WithCancel(runCtx)
	polled := make(chan struct{})
	go func() {
		defer close(polled)
		_ = poller.Run(pollCtx)
	}()
	unsubscribed := make(chan struct{})
	go func() {
		defer close(unsubscribed)
		unsubscriber.Run(pollCtx)
	}()
	// Live-console comment bridge (LC-B2): the internal bridge listener (backend Docker network, shared
	// 32-byte bearer) and the comment poller. Both stop with the process; the server shuts down cleanly.
	if c.consoleAddr != "" {
		console, err := metareply.NewConsole(workerPool, c.pageKeys, c.pageOpen, metareply.ConsoleConfig{
			Graph: c.graph, HolderID: c.consoleHolder, BridgeToken: c.bridgeToken, CursorKey: c.cursorKey,
		})
		if err != nil {
			stopPoll()
			<-polled
			<-unsubscribed
			return errWorkerRoutes
		}
		consoleServer := newConsoleServer(c.consoleAddr, console.Handler(), pollCtx)
		failed, err := startConsoleServer(consoleServer)
		if err != nil {
			// Required listener unavailable: never log claims_worker_ready (#83).
			stopPoll()
			<-polled
			<-unsubscribed
			return errWorkerStart
		}
		defer func() {
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = consoleServer.Shutdown(shutdown)
		}()
		consoleRun.Go(func() { _ = console.Run(pollCtx) })
		consoleDown = superviseConsole(failed, pollCtx.Done(), cancelRun)
	}
	resubscribed := make(chan struct{})
	go func() {
		defer close(resubscribed)
		resubscriber.Run(pollCtx)
	}()
	// Fixed local startup witness; never implies Meta access.
	runErr := jobqueue.Run(runCtx, client, "claims_worker_ready")
	stopPoll()
	<-polled
	<-unsubscribed
	<-resubscribed
	// CW-1 (#147, review-148 P1-1): the sweep itself honours ctx (DB calls, rate wait and Graph calls all take it, Graph under
	// CallTimeout), but console.Run's stopAll takes Console.mu, and a bridge handler may hold that mutex across a DB call made
	// with its request ctx. newConsoleServer derives that ctx from pollCtx so stopPoll cancels it; the join is still bounded as a
	// backstop, and a hit is logged, never silent. The exit token is unchanged either way.
	joinConsole(&consoleRun, consoleStopWait)
	return exitError(consoleDown, runErr)
}

// consoleStopWait bounds the console join at shutdown, matching the 5s Server.Shutdown bound in run().
const consoleStopWait = 5 * time.Second

// newConsoleServer builds the comment-bridge server. base is the ctx every request inherits (BaseContext), so cancelling it
// at shutdown cancels in-flight handlers that hold Console.mu across DB calls (review-148 P1-1).
func newConsoleServer(addr string, h http.Handler, base context.Context) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		BaseContext:       func(net.Listener) context.Context { return base },
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

// joinConsole waits for console.Run at most d and logs (fixed text, no values) when the bound hits; it reports whether the join completed.
func joinConsole(wg *sync.WaitGroup, d time.Duration) bool {
	joined := make(chan struct{})
	go func() { wg.Wait(); close(joined) }()
	select {
	case <-joined:
		return true
	case <-time.After(d):
		slog.Warn("claims_worker_console_join_timeout")
		return false
	}
}

// superviseConsole watches the bound bridge's Serve result: a failure (not a clean Shutdown, which closes failed) is
// recorded by closing the returned channel (a lasting signal: every later read sees it) and then cancels the run, so a
// ready process never keeps running without its bridge. stop ends the watch on a normal shutdown.
func superviseConsole(failed <-chan error, stop <-chan struct{}, cancelRun context.CancelFunc) <-chan struct{} {
	down := make(chan struct{})
	go func() {
		select {
		case err, ok := <-failed:
			if ok && err != nil {
				close(down)
				cancelRun()
			}
		case <-stop:
		}
	}()
	return down
}

// exitError maps the River run result to the process token; a recorded bridge Serve failure wins over it and keeps its
// own token, so a runtime bridge loss is never reported as a startup failure (#107).
func exitError(consoleDown <-chan struct{}, runErr error) error {
	select {
	case <-consoleDown:
		return errWorkerConsole
	default:
	}
	switch {
	case errors.Is(runErr, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(runErr, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return runErr
	}
}

// startConsoleServer binds the comment-bridge address synchronously, so a bind failure is returned before
// claims_worker_ready (#83), then serves in the background. The returned channel receives a non-ErrServerClosed
// Serve failure, or is closed after a clean Shutdown/Close. Errors are never logged here (they may carry the address).
func startConsoleServer(srv *http.Server) (<-chan error, error) {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, err
	}
	if consoleBound != nil {
		consoleBound(ln)
	}
	return serveConsole(srv, ln), nil
}

// consoleBound is nil in production; only the claimsconsoletest build (console_testhook.go) sets it, so a process test
// can fail the bound listener and prove run()'s post-bind wiring (#116).
var consoleBound func(net.Listener)

// serveConsole serves srv on an already bound listener; the channel contract is startConsoleServer's.
func serveConsole(srv *http.Server, ln net.Listener) <-chan error {
	failed := make(chan error, 1)
	go func() {
		defer close(failed)
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()
	return failed
}

// sameDatabase proves both pools reach the same physical database with a bounded lock probe.
func sameDatabase(ctx context.Context, a, b *pgxpool.Pool) bool {
	return platform.ValidateSameDatabase(ctx, a, b) == nil
}
