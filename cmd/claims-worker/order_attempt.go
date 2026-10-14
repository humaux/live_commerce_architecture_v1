// Purpose: opt-in OA1 key custody and native worker composition.
// Depends on: orderattempt, inbox notification planning and the existing claims-worker pool.
// Used by: loadConfig and run; disabled mode reads no OA1 or notification keyrings.
// Invariants: distinct plan/pay keys, fixed startup errors, no new pool or provider calls.
package main

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/orderattempt"
)

type orderAttemptConfig struct {
	enabled bool
	plans   *orderattempt.PlanKeys
	pay     *orderattempt.PayEntryKeys
	payload *inbox.Keyring
	seal    *pagetoken.SealKeys
}

func loadOrderAttemptConfig(getenv func(string) string) (orderAttemptConfig, error) {
	var c orderAttemptConfig
	switch getenv("LC_ORDER_ATTEMPT_ENABLED") {
	case "", "0":
		return c, nil
	case "1":
		c.enabled = true
	default:
		return orderAttemptConfig{}, errWorkerConfig
	}
	var err error
	if c.plans, c.pay, err = orderattempt.LoadKeys(getenv); err != nil {
		return orderAttemptConfig{}, errWorkerConfig
	}
	if c.payload, err = inbox.LoadKeyring(getenv); err != nil {
		return orderAttemptConfig{}, errWorkerConfig
	}
	if c.seal, err = pagetoken.LoadSealKeys(getenv); err != nil {
		return orderAttemptConfig{}, errWorkerConfig
	}
	return c, nil
}

// addOrderAttemptWorkers uses the already-probed worker pool for both C2 and
// notification InsertTx. The returned periodic job belongs to the claims lane.
func addOrderAttemptWorkers(c orderAttemptConfig, pool *pgxpool.Pool, workers *river.Workers) (*river.PeriodicJob, error) {
	if !c.enabled {
		return nil, nil
	}
	notifications, err := inbox.NewService(c.payload)
	if err != nil {
		return nil, errWorkerStart
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, errWorkerStart
	}
	notifications.EnableSend(c.seal, jobs, msgtemplates.NewService())
	executor, err := orderattempt.NewExecutor(pool, c.plans, c.pay, notifications)
	if err != nil {
		return nil, errWorkerStart
	}
	if err = orderattempt.AddWorkers(workers, executor); err != nil {
		return nil, errWorkerStart
	}
	return orderattempt.PeriodicJob(), nil
}
