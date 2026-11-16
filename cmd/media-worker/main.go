// Responsibility: default-OFF native media entrypoint and shared loader mapping.
// Dependencies: LiveKit configuration, platform pools, River lifecycle.
// Callers: main and supervised exec child; readiness precedes release.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

var (
	errWorkerConfig   = errors.New("media_worker_invalid_config")
	errWorkerDatabase = errors.New("media_worker_database_unavailable")
	errWorkerStart    = errors.New("media_worker_start_failed")
	errWorkerStop     = errors.New("media_worker_stop_failed")
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runEntrypoint(ctx, os.Getenv); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	return runNative(ctx, getenv, nil)
}

func runEntrypoint(ctx context.Context, getenv func(string) string) error {
	if getenv == nil {
		return errWorkerConfig
	}
	switch getenv("COMMERCE_MEDIA_RECOVERY_SUPERVISED") {
	case "", "0":
		if getenv("COMMERCE_MEDIA_RECOVERY_INTERNAL_CHILD") != "" {
			return errWorkerConfig
		}
		return runNative(ctx, getenv, nil)
	case "1":
		if getenv("COMMERCE_MEDIA_RECOVERY_INTERNAL_CHILD") == "1" {
			return runInternalChild(ctx, getenv)
		}
		if getenv("COMMERCE_MEDIA_RECOVERY_INTERNAL_CHILD") != "" {
			return errWorkerConfig
		}
		return runSupervised(ctx, getenv)
	default:
		return errWorkerConfig
	}
}

func runNative(ctx context.Context, getenv func(string) string, ready func() error) error {
	env, err := livekit.LoadWorkerEnvironment(getenv)
	if err != nil {
		return errWorkerConfig
	}
	if !env.Enabled {
		return nil
	}
	if ctx == nil {
		return errWorkerConfig
	}
	projects, input, err := workerRuntimes(env.Projects, env.BrowserInputEnabled)
	if err != nil {
		return errWorkerConfig
	}
	startup, done := context.WithTimeout(ctx, 10*time.Second)
	workerPool, err := platform.OpenMediaWorkerPool(startup, env.WorkerDSN)
	if err != nil {
		done()
		return errWorkerDatabase
	}
	defer workerPool.Close()
	executorPool, err := platform.OpenMediaExecutorPool(startup, env.ExecutorDSN)
	if err != nil {
		done()
		return errWorkerDatabase
	}
	defer executorPool.Close()
	var client *river.Client[pgx.Tx]
	if env.BrowserInputEnabled {
		client, err = live.NewMediaClientWithBrowserInput(startup, workerPool, executorPool, env.Keys, projects, input, env.Concurrency)
	} else {
		client, err = live.NewMediaClient(startup, workerPool, executorPool, env.Keys, projects, env.Concurrency)
	}
	done()
	if err != nil {
		return errWorkerDatabase
	}
	if ready != nil && ready() != nil {
		return errWorkerStart
	}
	// Job shutdown only releases this process; durable media cleanup remains lease-fenced in the queue.
	switch err := jobqueue.Run(ctx, client, "media_worker_ready"); {
	case errors.Is(err, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(err, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return err
	}
}

// workerRuntimes maps the shared strict loader output in both parent and exec
// child. Neither process receives an in-memory runtime from the other.
func workerRuntimes(loaded []livekit.WorkerProject, mixed bool) ([]live.MediaProject, *live.BrowserInputRuntime, error) {
	projects := make([]live.MediaProject, 0, len(loaded))
	inputs := make([]live.BrowserInputProject, 0, len(loaded))
	for _, p := range loaded {
		projects = append(projects, live.MediaProject{ProjectID: p.ProjectID, CredentialVersion: p.CredentialVersion, Config: p.Config, Transport: p.Transport})
		if p.BrowserInput != nil {
			if !mixed {
				return nil, nil, errWorkerConfig
			}
			inputs = append(inputs, live.BrowserInputProject{ProjectID: p.ProjectID, CredentialVersion: p.CredentialVersion, Config: p.BrowserInput.Config, Transport: p.BrowserInput.Transport, BrowserURL: p.BrowserInput.BrowserURL})
		}
	}
	if !mixed {
		return projects, nil, nil
	}
	input, err := live.NewBrowserInputRuntime(inputs)
	if err != nil {
		return nil, nil, errWorkerConfig
	}
	return projects, input, nil
}
