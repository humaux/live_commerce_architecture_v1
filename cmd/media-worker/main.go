package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	projects := make([]live.MediaProject, 0, len(env.Projects))
	for _, project := range env.Projects {
		projects = append(projects, live.MediaProject{
			ProjectID: project.ProjectID, CredentialVersion: project.CredentialVersion,
			Config: project.Config, Transport: project.Transport,
		})
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
	client, err := live.NewMediaClient(startup, workerPool, executorPool, env.Keys, projects, env.Concurrency)
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
