package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

type recoveryEpisode struct {
	id            string
	started       time.Time
	coverageKnown bool
	members       []live.RecoveryMember
	admitted      bool
	resolved      bool
	alerted       bool
	observed      map[string]bool
	inflight      map[string]bool
	cancelObserve context.CancelFunc
	observeCtx    context.Context
}

func newRecoveryEpisode() (*recoveryEpisode, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, errWorkerStart
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return &recoveryEpisode{
		id:      encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:],
		started: time.Now(), coverageKnown: true,
		observed: map[string]bool{}, inflight: map[string]bool{},
	}, nil
}

func (e *recoveryEpisode) elapsedMS() int64 { return time.Since(e.started).Milliseconds() }

func runInternalChild(ctx context.Context, getenv func(string) string) error {
	ack := os.NewFile(3, "media-recovery-parent-ack")
	if ack == nil {
		return errWorkerConfig
	}
	defer ack.Close()
	info, err := ack.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return errWorkerConfig
	}
	return runNative(ctx, getenv, func() error {
		_, err := ack.Write([]byte("ready\n"))
		return err
	})
}

func sanitizedNativeEnvironment() []string {
	var result []string
	for _, entry := range os.Environ() {
		name := strings.SplitN(entry, "=", 2)[0]
		if strings.HasPrefix(name, "COMMERCE_MEDIA_RECOVERY_") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "COMMERCE_MEDIA_RECOVERY_SUPERVISED=1", "COMMERCE_MEDIA_RECOVERY_INTERNAL_CHILD=1")
}

// Each child owns its original River queue lifecycle. The supervisor never
// admits a second authority into the child and reaps every abnormal exit.
func superviseNativeChild(ctx context.Context, events chan<- struct{}) {
	executable, err := os.Executable()
	if err != nil {
		slog.Error("media_native_child_executable_unavailable")
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		readEnd, writeEnd, err := os.Pipe()
		if err != nil {
			slog.Warn("media_native_child_pipe_unavailable")
		} else {
			cmd := exec.Command(executable)
			cmd.Env = sanitizedNativeEnvironment()
			cmd.ExtraFiles = []*os.File{writeEnd}
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err = cmd.Start(); err == nil {
				_ = writeEnd.Close()
				ack := make(chan bool, 1)
				go func() {
					var message [6]byte
					_, readErr := io.ReadFull(readEnd, message[:])
					ack <- readErr == nil && string(message[:]) == "ready\n"
					_ = readEnd.Close()
				}()
				done := make(chan struct{})
				go func() { _ = cmd.Wait(); close(done) }()
				select {
				case ready := <-ack:
					if ready {
						slog.Info("media_native_child_ready")
					} else {
						slog.Warn("media_native_child_ack_lost")
					}
				case <-done:
				case <-ctx.Done():
				}
				select {
				case <-done:
				case <-ctx.Done():
					_ = cmd.Process.Signal(syscall.SIGTERM)
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						_ = cmd.Process.Kill()
						<-done
					}
				}
				if ctx.Err() != nil {
					return
				}
				slog.Warn("media_native_child_exited")
				select {
				case events <- struct{}{}:
				default:
				}
				backoff = min(backoff+time.Second, 5*time.Second)
			} else {
				_ = writeEnd.Close()
				_ = readEnd.Close()
				slog.Warn("media_native_child_start_failed")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

type observeResult struct{ episodeID, operationID, disposition string }

func runSupervised(ctx context.Context, getenv func(string) string) error {
	if ctx == nil || getenv == nil {
		return errWorkerConfig
	}
	episode, err := newRecoveryEpisode()
	if err != nil {
		return err
	}
	// Recovery DSN is parsed/opened before material keys or native child config.
	recoveryDSN := getenv("COMMERCE_MEDIA_RECOVERY_DATABASE_URL")
	capacity, capacityErr := livekit.LoadWorkerConcurrency(getenv)
	projects, projectsErr := livekit.LoadWorkerProjects(getenv)
	workerEnabled := getenv("COMMERCE_MEDIA_WORKER_ENABLED") == "1"
	if !workerEnabled {
		slog.Error("media_recovery_native_worker_disabled")
	}
	if capacityErr != nil || projectsErr != nil {
		slog.Error("media_recovery_project_or_capacity_invalid")
	}
	var pool *pgxpool.Pool
	defer func() {
		if pool != nil {
			pool.Close()
		}
	}()
	var ledger, observer *live.MediaRecoveryObserver
	if recoveryDSN != "" {
		openCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		pool, err = platform.OpenMediaRecoveryPool(openCtx, recoveryDSN)
		cancel()
		if err == nil {
			ledger = live.NewMediaRecoveryLedger(pool)
		} else {
			episode.coverageKnown = false
		}
	} else {
		episode.coverageKnown = false
	}
	var nativeEnv livekit.WorkerEnvironment
	var nativeErr error
	if workerEnabled {
		nativeEnv, nativeErr = livekit.LoadWorkerEnvironment(getenv)
		if nativeErr != nil {
			slog.Error("media_recovery_native_config_invalid")
		}
	}
	providerChecked := false
	childCtx, stopChild := context.WithCancel(ctx)
	childEvents := make(chan struct{}, 1)
	childDone := make(chan struct{})
	childStarted := false
	defer func() { stopChild(); <-childDone }()
	results := make(chan observeResult, 32)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if pool == nil && recoveryDSN != "" {
			openCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			pool, err = platform.OpenMediaRecoveryPool(openCtx, recoveryDSN)
			cancel()
			if err == nil {
				ledger = live.NewMediaRecoveryLedger(pool)
			} else {
				episode.coverageKnown = false
			}
		} else if pool == nil {
			episode.coverageKnown = false
		}
		if ledger != nil && !episode.admitted && capacityErr == nil {
			members, beginErr := ledger.Begin(ctx, episode.id, episode.elapsedMS(), capacity, episode.coverageKnown)
			if beginErr == nil {
				episode.members, episode.admitted = members, true
				if len(members) == 1 && members[0].Disposition == "empty" && members[0].CoverageKnown {
					episode.resolved = true
					slog.Info("media_recovery_no_work", "episode_id", episode.id)
				}
			} else {
				episode.coverageKnown = false
			}
		}
		if !childStarted {
			childStarted = true
			if workerEnabled && nativeErr == nil {
				go func() { superviseNativeChild(childCtx, childEvents); close(childDone) }()
			} else {
				close(childDone)
			}
		}
		if episode.admitted && !providerChecked && workerEnabled && nativeErr == nil &&
			capacityErr == nil && projectsErr == nil && episode.elapsedMS() < 90000 {
			providerChecked = true
			observer = readyRecoveryObserver(ctx, pool, nativeEnv, projects)
			if observer == nil {
				slog.Warn("media_recovery_provider_admission_degraded")
			}
		}
		if episode.admitted && !episode.resolved && episode.elapsedMS() < 90000 {
			if observer != nil {
				for _, member := range episode.members {
					if member.Disposition != "pending" || episode.observed[member.OperationID] || episode.inflight[member.OperationID] {
						continue
					}
					episode.inflight[member.OperationID] = true
					if episode.cancelObserve == nil {
						episode.observeCtx, episode.cancelObserve = context.WithDeadline(ctx, episode.started.Add(90*time.Second))
					}
					go func(m live.RecoveryMember, current *recoveryEpisode, active *live.MediaRecoveryObserver) {
						status, _ := active.Observe(current.observeCtx, current.id, m)
						select {
						case results <- observeResult{current.id, m.OperationID, status}:
						case <-ctx.Done():
						}
					}(member, episode, observer)
				}
			}
			readback, readErr := ledger.Read(ctx, episode.id)
			if readErr == nil {
				for _, row := range readback {
					if row.ObservationID == "" || row.Disposition == "witnessed" || row.Disposition == "timeout" {
						continue
					}
					elapsed := episode.elapsedMS() // sampled after committed readback
					if elapsed <= 90000 {
						_, _ = ledger.Witness(ctx, episode.id, row.OperationID, row.ObservationID, elapsed)
					}
				}
				if readback[0].ScopeStatus == "finished" || readback[0].ScopeStatus == "empty" && readback[0].CoverageKnown {
					episode.resolved = true
				}
			}
		}
		if episode.elapsedMS() >= 90000 && !episode.resolved {
			if episode.cancelObserve != nil {
				episode.cancelObserve()
			}
			persisted := false
			count := 0
			status := "unknown"
			if ledger != nil && episode.admitted {
				status, count, err = ledger.Timeout(ctx, episode.id, episode.elapsedMS())
				persisted = err == nil
				if persisted && status == "already_finished" {
					episode.resolved = true
				}
				if persisted && status != "already_finished" {
					episode.resolved = true
				}
			}
			if !episode.alerted && status != "already_finished" {
				slog.Error("media_recovery_deadline_missed", "episode_id", episode.id,
					"scope_status", status, "affected_count", count, "persisted", persisted)
				episode.alerted = true
			}
			if !episode.admitted && ledger == nil {
				// A late DB recovery may still create an overdue, unknown-coverage scope.
				episode.coverageKnown = false
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case result := <-results:
			if result.episodeID == episode.id {
				delete(episode.inflight, result.operationID)
				if result.disposition != "busy" {
					episode.observed[result.operationID] = true
				}
			}
		case <-childEvents:
			if episode.resolved {
				if episode.cancelObserve != nil {
					episode.cancelObserve()
				}
				episode, err = newRecoveryEpisode()
				if err != nil {
					return errWorkerStart
				}
				observer, providerChecked = nil, false
			}
		case <-ticker.C:
		}
	}
}

func readyRecoveryObserver(ctx context.Context, recoveryPool *pgxpool.Pool,
	env livekit.WorkerEnvironment, projects []livekit.WorkerProject) *live.MediaRecoveryObserver {
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	worker, err := platform.OpenMediaWorkerPool(startup, env.WorkerDSN)
	if err != nil {
		return nil
	}
	defer worker.Close()
	executor, err := platform.OpenMediaExecutorPool(startup, env.ExecutorDSN)
	if err != nil {
		return nil
	}
	defer executor.Close()
	if platform.ValidateSameDatabase(startup, recoveryPool, worker) != nil ||
		platform.ValidateSameDatabase(startup, recoveryPool, executor) != nil {
		return nil
	}
	configured := make([]live.MediaProject, 0, len(projects))
	for _, p := range projects {
		configured = append(configured, live.MediaProject{ProjectID: p.ProjectID,
			CredentialVersion: p.CredentialVersion, Config: p.Config, Transport: p.Transport})
	}
	observer, err := live.NewMediaRecoveryObserver(startup, recoveryPool, configured)
	if err != nil {
		return nil
	}
	return observer
}
