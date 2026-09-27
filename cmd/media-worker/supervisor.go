package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

type recoveryEpisode struct {
	id            string
	withInput     bool
	started       time.Time
	coverageKnown bool
	members       []live.RecoveryMember
	admitted      bool
	resolved      bool
	resolvedAt    time.Time
	resolvedProof atomic.Bool
	pendingProof  atomic.Bool
	alerted       atomic.Bool
	stopDeadline  context.CancelFunc
	observed      map[string]bool
	inflight      map[string]bool
	pending       map[string]recoveryAttestation
	witnessed     map[string]bool
	rejected      map[string]bool
	cancelObserve context.CancelFunc
	observeCtx    context.Context
}

type recoveryAttestation struct {
	observationID string
	inputID       string
	egressID      string
	elapsedMS     int64
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
		pending: map[string]recoveryAttestation{}, witnessed: map[string]bool{}, rejected: map[string]bool{},
	}, nil
}

func newRecoveryEpisodeAt(started time.Time) (*recoveryEpisode, error) {
	episode, err := newRecoveryEpisode()
	if err == nil {
		episode.started = started
	}
	return episode, err
}

func (e *recoveryEpisode) elapsedMS() int64 { return time.Since(e.started).Milliseconds() }

func (e *recoveryEpisode) markResolved() {
	e.resolved = true
	if e.resolvedAt.IsZero() {
		e.resolvedAt = time.Now()
	}
	e.resolvedProof.Store(true)
	if e.stopDeadline != nil {
		e.stopDeadline()
	}
}

func (e *recoveryEpisode) updateProofState() {
	if !e.admitted || len(e.members) == 0 || !e.members[0].CoverageKnown ||
		e.members[0].CandidateCount != len(e.members) || e.members[0].OperationID == "" {
		e.pendingProof.Store(false)
		return
	}
	allWitnessed, allAccounted := true, true
	for _, member := range e.members {
		if e.witnessed[member.OperationID] {
			continue
		}
		allWitnessed = false
		if _, pending := e.pending[member.OperationID]; !pending {
			allAccounted = false
		}
	}
	if allWitnessed {
		e.markResolved()
	}
	e.pendingProof.Store(!allWitnessed && allAccounted)
}

func (e *recoveryEpisode) attemptPending(ctx context.Context, ledger *live.MediaRecoveryObserver) {
	ids := make([]string, 0, len(e.pending))
	for id := range e.pending {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		proof := e.pending[id]
		var status string
		var err error
		if e.withInput {
			status, err = ledger.WitnessWithInput(ctx, e.id, id, proof.inputID, proof.egressID, proof.elapsedMS)
		} else {
			status, err = ledger.Witness(ctx, e.id, id, proof.observationID, proof.elapsedMS)
		}
		if err != nil {
			continue
		} // Retain exact evidence for a bounded retry.
		switch status {
		case "witnessed", "already_witnessed":
			e.witnessed[id] = true
			delete(e.pending, id)
		case "timeout_wins", "unqualified":
			e.rejected[id] = true
			delete(e.pending, id)
		}
	}
	e.updateProofState()
}

func (e *recoveryEpisode) acceptReadback(rows []live.RecoveryReadback, elapsedMS int64) {
	if e.withInput {
		e.acceptCompositeReadback(rows, elapsedMS)
		return
	}
	if len(rows) == 0 {
		return
	}
	for _, row := range rows {
		if row.Disposition == "witnessed" {
			e.witnessed[row.OperationID] = true
			delete(e.pending, row.OperationID)
			continue
		}
		if row.Disposition == "timeout" {
			delete(e.pending, row.OperationID)
			continue
		}
		if row.OperationID == "" || row.ObservationID == "" ||
			(row.ObservationSource != "ROOM" && row.ObservationSource != "QUERY") ||
			row.ObservationGeneration <= row.BaselineGeneration || elapsedMS > 90000 || e.rejected[row.OperationID] {
			continue
		}
		if _, exists := e.pending[row.OperationID]; !exists {
			e.pending[row.OperationID] = recoveryAttestation{observationID: row.ObservationID, elapsedMS: elapsedMS}
		}
	}
	if rows[0].ScopeStatus == "finished" || rows[0].ScopeStatus == "empty" && rows[0].CoverageKnown {
		e.markResolved()
	} else {
		e.updateProofState()
	}
}

// A mixed readback is one atomic classification unit. No member or scope
// disposition is honored until the entire batch matches immutable admission.
func (e *recoveryEpisode) acceptCompositeReadback(rows []live.RecoveryReadback, elapsedMS int64) {
	if !e.admitted || elapsedMS < 0 || len(rows) == 0 || len(e.members) == 0 {
		return
	}
	if len(e.members) == 1 && e.members[0].OperationID == "" {
		row := rows[0]
		if len(rows) == 1 && e.members[0].Disposition == "empty" &&
			e.members[0].EpisodeID == e.id && e.members[0].CoverageKnown &&
			e.members[0].CandidateCount == 0 &&
			row.EpisodeID == e.id && row.ScopeStatus == "empty" && row.CoverageKnown &&
			row.CandidateCount == 0 && row.OperationID == "" && row.JobID == 0 &&
			row.ExecutionProfile == "" && !row.InputRequired && !row.EgressRequired &&
			row.InputObservationID == "" && row.ObservationID == "" {
			e.markResolved()
		}
		return
	}
	if len(rows) != len(e.members) || !e.members[0].CoverageKnown ||
		e.members[0].CandidateCount != len(e.members) {
		return
	}
	members := make(map[string]live.RecoveryMember, len(e.members))
	for _, member := range e.members {
		if member.EpisodeID != e.id || member.OperationID == "" || member.JobID < 1 ||
			!member.CoverageKnown || member.CandidateCount != len(e.members) || member.BlockedByEpisodeID != "" ||
			(!member.InputRequired && !member.EgressRequired) ||
			(member.ExecutionProfile == "PROVIDER_MOCK" && (member.InputRequired || !member.EgressRequired)) ||
			(member.ExecutionProfile != "PROVIDER_MOCK" && member.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS") ||
			members[member.OperationID].OperationID != "" {
			return
		}
		members[member.OperationID] = member
	}
	seen := make(map[string]bool, len(rows))
	status := rows[0].ScopeStatus
	if status != "pending" && status != "finished" {
		return
	}
	allWitnessed := true
	for _, row := range rows {
		member, ok := members[row.OperationID]
		if !ok || seen[row.OperationID] || row.EpisodeID != e.id || row.ScopeStatus != status ||
			!row.CoverageKnown || row.CandidateCount != len(e.members) || row.BlockedByEpisodeID != "" ||
			row.JobID != member.JobID || row.BaselineGeneration != member.BaselineGeneration ||
			row.ExecutionProfile != member.ExecutionProfile || row.InputRequired != member.InputRequired ||
			row.EgressRequired != member.EgressRequired ||
			(row.InputRequired && row.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS") ||
			(row.ExecutionProfile != "PROVIDER_MOCK" && row.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS") ||
			!validCompositeProofShape(row) {
			return
		}
		seen[row.OperationID] = true
		allWitnessed = allWitnessed && row.Disposition == "witnessed"
	}
	if (status == "finished") != allWitnessed {
		return
	}
	if status == "finished" {
		for _, row := range rows {
			if row.Disposition != "witnessed" || !qualifiedComposite(row) {
				return
			}
		}
	}
	for _, row := range rows {
		if row.Disposition == "witnessed" && qualifiedComposite(row) {
			e.witnessed[row.OperationID] = true
			delete(e.pending, row.OperationID)
			continue
		}
		if row.Disposition == "timeout" {
			delete(e.pending, row.OperationID)
			continue
		}
		if elapsedMS > 90000 || e.rejected[row.OperationID] || !qualifiedComposite(row) {
			continue
		}
		if _, exists := e.pending[row.OperationID]; !exists {
			e.pending[row.OperationID] = recoveryAttestation{
				inputID: row.InputObservationID, egressID: row.ObservationID, elapsedMS: elapsedMS,
			}
		}
	}
	if status == "finished" {
		e.markResolved()
	} else {
		e.updateProofState()
	}
}

func qualifiedComposite(row live.RecoveryReadback) bool {
	return validCompositeProofShape(row) && (!row.InputRequired || row.InputObservationID != "") &&
		(!row.EgressRequired || row.ObservationID != "") && (row.InputRequired || row.EgressRequired)
}

func validCompositeProofShape(row live.RecoveryReadback) bool {
	if row.InputRequired {
		if row.InputObservationID == "" {
			if row.InputObservationSource != "" || row.InputObservationResult != "" || row.InputObservationGeneration != 0 {
				return false
			}
		} else if row.InputObservationGeneration <= row.BaselineGeneration ||
			!((row.InputObservationSource == "PARTICIPANT" && row.InputObservationResult == "PRESENT") ||
				(row.InputObservationSource == "ROOM" && row.InputObservationResult == "ABSENT")) {
			return false
		}
	} else if row.InputObservationID != "" || row.InputObservationSource != "" ||
		row.InputObservationResult != "" || row.InputObservationGeneration != 0 {
		return false
	}
	if row.EgressRequired {
		if row.ObservationID == "" {
			if row.ObservationSource != "" || row.ObservationGeneration != 0 {
				return false
			}
		} else if row.ObservationGeneration <= row.BaselineGeneration ||
			(row.ObservationSource != "QUERY" && row.ObservationSource != "ROOM") {
			return false
		}
	} else if row.ObservationID != "" || row.ObservationSource != "" || row.ObservationGeneration != 0 {
		return false
	}
	return true
}

// This local monotonic timer is independent of SQL/provider calls that may be
// blocking at the deadline. Durable timeout is retried by the main loop.
func (e *recoveryEpisode) startDeadlineSignal(parent context.Context) {
	timerCtx, stop := context.WithCancel(parent)
	e.stopDeadline = stop
	go func() {
		wait := time.Until(e.started.Add(90 * time.Second))
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timerCtx.Done():
			return
		case <-timer.C:
			if e.resolvedProof.Load() {
				return
			}
			if e.pendingProof.Load() {
				slog.Warn("media_recovery_witness_pending", "episode_id", e.id)
				return
			}
			if e.alerted.CompareAndSwap(false, true) {
				slog.Error("media_recovery_deadline_missed", "episode_id", e.id,
					"scope_status", "unresolved", "affected_count", -1, "persisted", false)
			}
		}
	}()
}

func runInternalChild(ctx context.Context, getenv func(string) string) error {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return errWorkerConfig
	}
	if !awaitRecoveryRelease(os.Stdin) {
		return errWorkerConfig
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		cancel() // Parent closed the control pipe after release.
	}()
	return runNative(childCtx, getenv, nil)
}

func awaitRecoveryRelease(input io.Reader) bool {
	var release [1]byte
	_, err := io.ReadFull(input, release[:])
	return err == nil && release[0] == 1
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
func superviseNativeChild(ctx context.Context, events chan<- time.Time, releases chan<- chan bool) {
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
			cmd.Stdin = readEnd
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err = cmd.Start(); err == nil {
				_ = readEnd.Close()
				permit := make(chan bool, 1)
				select {
				case releases <- permit:
				case <-ctx.Done():
					_ = writeEnd.Close()
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
					return
				}
				var approved bool
				select {
				case approved = <-permit:
				case <-ctx.Done():
				}
				if !approved {
					_ = writeEnd.Close()
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
					return
				}
				if _, err := writeEnd.Write([]byte{1}); err != nil {
					slog.Warn("media_native_child_release_failed")
					_ = writeEnd.Close()
					_ = cmd.Process.Signal(syscall.SIGTERM)
				} else {
					slog.Info("media_native_child_released")
				}
				done := make(chan struct{})
				go func() { _ = cmd.Wait(); close(done) }()
				select {
				case <-done:
				case <-ctx.Done():
					_ = writeEnd.Close()
					_ = cmd.Process.Signal(syscall.SIGTERM)
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						_ = cmd.Process.Kill()
						<-done
					}
				}
				_ = writeEnd.Close()
				if ctx.Err() != nil {
					return
				}
				slog.Warn("media_native_child_exited")
				select {
				case events <- time.Now():
				case <-ctx.Done():
					return
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

func (e *recoveryEpisode) shouldObserve(member live.RecoveryMember) bool {
	if member.Disposition != "pending" || e.inflight[member.OperationID] {
		return false
	}
	if e.withInput {
		_, pending := e.pending[member.OperationID]
		return !pending && !e.witnessed[member.OperationID]
	}
	return !e.observed[member.OperationID]
}

func (e *recoveryEpisode) noteObserveResult(result observeResult) {
	if result.episodeID != e.id {
		return
	}
	delete(e.inflight, result.operationID)
	// Mixed status is not durable proof: one side can commit while the other
	// fails, and either record acknowledgement can disappear after commit.
	if !e.withInput && result.disposition != "busy" {
		e.observed[result.operationID] = true
	}
}

func runSupervised(ctx context.Context, getenv func(string) string) error {
	if ctx == nil || getenv == nil {
		return errWorkerConfig
	}
	episode, err := newRecoveryEpisode()
	if err != nil {
		return err
	}
	episode.startDeadlineSignal(ctx)
	defer func() {
		if episode.stopDeadline != nil {
			episode.stopDeadline()
		}
	}()
	// Recovery DSN is parsed/opened before material keys or native child config.
	recoveryDSN := getenv("COMMERCE_MEDIA_RECOVERY_DATABASE_URL")
	if len(recoveryDSN) == 0 || len(recoveryDSN) > 8192 || strings.TrimSpace(recoveryDSN) == "" {
		slog.Error("media_recovery_dsn_invalid")
		return errWorkerConfig
	}
	if _, parseErr := pgxpool.ParseConfig(recoveryDSN); parseErr != nil {
		slog.Error("media_recovery_dsn_invalid")
		return errWorkerConfig
	}
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
	childEvents := make(chan time.Time)
	childReleases := make(chan chan bool)
	childDone := make(chan struct{})
	childStarted := false
	nativeReleased := false
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
					episode.markResolved()
					slog.Info("media_recovery_no_work", "episode_id", episode.id)
				}
			} else {
				episode.coverageKnown = false
			}
		}
		if !childStarted {
			childStarted = true
			if workerEnabled && nativeErr == nil {
				go func() { superviseNativeChild(childCtx, childEvents, childReleases); close(childDone) }()
			} else {
				close(childDone)
			}
		}
		if nativeReleased && episode.admitted && !providerChecked && workerEnabled && nativeErr == nil &&
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
					if !episode.shouldObserve(member) {
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
				readbackElapsed := episode.elapsedMS() // one sample after the committed batch
				episode.acceptReadback(readback, readbackElapsed)
			}
		}
		if episode.admitted && !episode.resolved && len(episode.pending) > 0 {
			episode.attemptPending(ctx, ledger)
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
					episode.markResolved()
				}
				if persisted && status != "already_finished" {
					episode.markResolved()
				}
			}
			if status != "already_finished" && episode.alerted.CompareAndSwap(false, true) {
				slog.Error("media_recovery_deadline_missed", "episode_id", episode.id,
					"scope_status", status, "affected_count", count, "persisted", persisted)
			} else if persisted && status != "already_finished" {
				slog.Info("media_recovery_timeout_persisted", "episode_id", episode.id,
					"scope_status", status, "affected_count", count)
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
			episode.noteObserveResult(result)
		case permit := <-childReleases:
			// Each restart waits for this episode's committed admission result,
			// or an explicit degraded DB/config path, before native work.
			allowed := episode.admitted || !episode.coverageKnown || capacityErr != nil
			permit <- allowed
			nativeReleased = allowed
		case exitAt := <-childEvents:
			nativeReleased = false
			if !episode.resolvedAt.IsZero() && !episode.resolvedAt.After(exitAt) {
				if episode.cancelObserve != nil {
					episode.cancelObserve()
				}
				episode, err = newRecoveryEpisodeAt(exitAt)
				if err != nil {
					return errWorkerStart
				}
				episode.startDeadlineSignal(ctx)
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
