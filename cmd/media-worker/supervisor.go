// Parent recovery supervision depends on live ledger DTOs and native child lifecycle.
// Mixed composition is private and opt-in; admission and whole batches precede state writes.
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
	// Fixed by construction alongside the ledger mode; never inferred from readback.
	mixed         bool
	id            string
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
	observationID      string
	elapsedMS          int64
	inputObservationID string
	member             live.RecoveryMember
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
	if e.mixed && !e.validMixedAdmission() {
		return
	}
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

// The concrete ledger implements both methods. This private seam permits a
// deterministic retry spy without SQL/provider calls or an exported ABI.
type recoveryWitnessLedger interface {
	Witness(context.Context, string, string, string, int64) (string, error)
	WitnessWithInput(context.Context, string, string, string, string, int64) (string, error)
}

func (e *recoveryEpisode) attemptPending(ctx context.Context, ledger recoveryWitnessLedger) {
	if e.mixed {
		if !e.validMixedAdmission() {
			return
		}
		// Validate every cached binding before making any witness call.
		for id, proof := range e.pending {
			found := false
			for _, member := range e.members {
				if member.OperationID == id && member == proof.member {
					found = true
					break
				}
			}
			if !found || proof.elapsedMS < 0 || proof.elapsedMS > 90000 ||
				(proof.inputObservationID != "") != proof.member.InputRequired ||
				(proof.observationID != "") != proof.member.EgressRequired {
				return
			}
		}
	}
	ids := make([]string, 0, len(e.pending))
	for id := range e.pending {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		proof := e.pending[id]
		var status string
		var err error
		if e.mixed {
			status, err = ledger.WitnessWithInput(ctx, e.id, id, proof.inputObservationID, proof.observationID, proof.elapsedMS)
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
	if e.mixed {
		e.acceptMixedReadback(rows, elapsedMS)
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

// validMixedAdmission checks the immutable Begin snapshot, including the SQL
// capacity bound. A scope-only empty row is not a member or an observation.
func (e *recoveryEpisode) validMixedAdmission() bool {
	if !e.admitted || !e.coverageKnown || e.id == "" || len(e.members) == 0 {
		return false
	}
	if len(e.members) == 1 && e.members[0].Disposition == "empty" {
		m := e.members[0]
		return m.EpisodeID == e.id && m.CoverageKnown && m.CandidateCount == 0 &&
			m.BlockedByEpisodeID == "" && m.OperationID == "" && m.JobID == 0 &&
			m.BaselineGeneration == 0 && m.ExecutionProfile == "" && !m.InputRequired && !m.EgressRequired
	}
	if len(e.members) > 32 {
		return false
	} // Begin's maximum admitted capacity.
	seen := make(map[string]bool, len(e.members))
	for _, m := range e.members {
		if m.EpisodeID != e.id || !m.CoverageKnown || m.CandidateCount != len(e.members) ||
			m.BlockedByEpisodeID != "" || m.OperationID == "" || seen[m.OperationID] ||
			m.JobID <= 0 || m.BaselineGeneration < 0 || !(m.InputRequired || m.EgressRequired) {
			return false
		}
		switch m.ExecutionProfile {
		case "PROVIDER_MOCK":
			if m.InputRequired || !m.EgressRequired {
				return false
			}
		case "LOCAL_SFU_MOCK_EGRESS":
		default:
			return false
		}
		switch m.Disposition {
		case "pending", "overdue", "terminal_at_lock", "native_ineligible", "ceiling":
		default:
			return false
		}
		seen[m.OperationID] = true
	}
	return true
}

// mixedProofShape distinguishes an absent required receipt (valid partial)
// from impossible payloads. Sources and optional zero values follow SQL Read.
func mixedProofShape(r live.RecoveryReadback) (complete, valid bool) {
	inputAbsent := r.InputObservationID == "" && r.InputObservationSource == "" &&
		r.InputObservationResult == "" && r.InputObservationGeneration == 0
	egressAbsent := r.ObservationID == "" && r.ObservationSource == "" && r.ObservationGeneration == 0
	inputValid := r.InputObservationID != "" && r.InputObservationGeneration > r.BaselineGeneration &&
		(r.InputObservationSource == "PARTICIPANT" && r.InputObservationResult == "PRESENT" ||
			r.InputObservationSource == "ROOM" && r.InputObservationResult == "ABSENT")
	egressValid := r.ObservationID != "" && r.ObservationGeneration > r.BaselineGeneration &&
		(r.ObservationSource == "QUERY" || r.ObservationSource == "ROOM")
	if (!r.InputRequired && !inputAbsent) || (!r.EgressRequired && !egressAbsent) ||
		(r.InputRequired && !inputAbsent && !inputValid) || (r.EgressRequired && !egressAbsent && !egressValid) {
		return false, false
	}
	return (!r.InputRequired || inputValid) && (!r.EgressRequired || egressValid), true
}

func (e *recoveryEpisode) acceptMixedReadback(rows []live.RecoveryReadback, elapsedMS int64) {
	if elapsedMS < 0 || !e.validMixedAdmission() || len(rows) != len(e.members) {
		return
	}
	if e.members[0].Disposition == "empty" {
		// Comparing to the complete zero-member DTO prevents proof/status smuggling.
		want := live.RecoveryReadback{EpisodeID: e.id, ScopeStatus: "empty", CoverageKnown: true}
		if rows[0] == want {
			e.markResolved()
		}
		return
	}
	members := make(map[string]live.RecoveryMember, len(e.members))
	for _, m := range e.members {
		members[m.OperationID] = m
	}
	qualified := make(map[string]bool, len(rows))
	scope := rows[0].ScopeStatus
	if scope != "pending" && scope != "finished" && scope != "overdue" {
		return
	}
	witnessCount, timeoutCount := 0, 0
	// Stage validation for the entire snapshot before touching any episode state.
	for _, r := range rows {
		m, exists := members[r.OperationID]
		if !exists || r.EpisodeID != e.id || r.ScopeStatus != scope || !r.CoverageKnown ||
			r.CandidateCount != len(e.members) || r.BlockedByEpisodeID != "" ||
			r.JobID != m.JobID || r.ExecutionProfile != m.ExecutionProfile ||
			r.InputRequired != m.InputRequired || r.EgressRequired != m.EgressRequired ||
			r.BaselineGeneration != m.BaselineGeneration {
			return
		}
		if _, duplicate := qualified[r.OperationID]; duplicate {
			return
		}
		complete, valid := mixedProofShape(r)
		if !valid {
			return
		}
		qualified[r.OperationID] = complete
		if r.Disposition == "timeout" {
			timeoutCount++
		}
		if (r.Disposition == "timeout") != !r.TimeoutAt.IsZero() {
			return
		}
		if scope == "finished" && r.Disposition != "witnessed" {
			return
		}
		if scope == "pending" && r.Disposition == "timeout" {
			return
		}
		switch r.Disposition {
		case "witnessed":
			witnessCount++
			if !complete || r.WitnessElapsedMS < 0 || r.WitnessElapsedMS > 90000 {
				return
			}
			if cached, ok := e.pending[r.OperationID]; ok &&
				(cached.observationID != r.ObservationID || cached.inputObservationID != r.InputObservationID || cached.elapsedMS != r.WitnessElapsedMS) {
				return
			}
		case "checked", "terminal":
			if !complete || r.WitnessElapsedMS != 0 {
				return
			}
		case "pending":
			if complete || r.WitnessElapsedMS != 0 {
				return
			}
		case "timeout", "terminal_at_lock", "native_ineligible", "ceiling":
			if r.WitnessElapsedMS != 0 {
				return
			}
		default:
			return
		}
	}
	if (scope == "finished") != (witnessCount == len(rows)) ||
		(scope == "overdue" && (timeoutCount == 0 || timeoutCount+witnessCount != len(rows))) {
		return
	}
	for _, r := range rows {
		id := r.OperationID
		if r.Disposition == "timeout" {
			delete(e.pending, id)
			delete(e.witnessed, id)
			e.rejected[id] = true
			continue
		}
		if e.rejected[id] {
			continue
		}
		if r.Disposition == "witnessed" {
			e.witnessed[id] = true
			delete(e.pending, id)
			continue // Durable SQL witness includes its original timely elapsed.
		}
		if scope != "overdue" && qualified[id] && elapsedMS <= 90000 && !e.witnessed[id] {
			if _, exists := e.pending[id]; !exists {
				e.pending[id] = recoveryAttestation{observationID: r.ObservationID,
					inputObservationID: r.InputObservationID, elapsedMS: elapsedMS, member: members[id]}
			}
		}
	}
	e.updateProofState()
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

func runSupervised(ctx context.Context, getenv func(string) string) error {
	if ctx == nil || getenv == nil {
		return errWorkerConfig
	}
	episode, err := newRecoveryEpisode()
	if err != nil {
		return err
	}
	mixed := requestedBrowserInput(getenv)
	episode.mixed = mixed
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
			ledger = recoveryLedger(pool, mixed)
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
	if mixed && (projectsErr != nil || nativeErr != nil || !workerEnabled || capacityErr != nil) {
		episode.coverageKnown = false
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
				ledger = recoveryLedger(pool, mixed)
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
				if !mixed && len(members) == 1 && members[0].Disposition == "empty" && members[0].CoverageKnown {
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
			if result.episodeID == episode.id {
				delete(episode.inflight, result.operationID)
				if result.disposition != "busy" {
					episode.observed[result.operationID] = true
				}
			}
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
				episode.mixed = mixed
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
	configured, input, err := workerRuntimes(projects, env.BrowserInputEnabled)
	if err != nil {
		return nil
	}
	// Construct the same native client for readiness without starting it.
	if env.BrowserInputEnabled {
		if _, err = live.NewMediaClientWithBrowserInput(startup, worker, executor, env.Keys, configured, input, env.Concurrency); err != nil {
			return nil
		}
		observer, err := live.NewMediaRecoveryObserverWithBrowserInput(startup, recoveryPool, configured, input)
		if err != nil {
			return nil
		}
		return observer
	}
	observer, err := live.NewMediaRecoveryObserver(startup, recoveryPool, configured)
	if err != nil {
		return nil
	}
	return observer
}

// Requested scope survives malformed configuration and selects diagnostic SQL
// before the strict loader can fail. Only empty/0 are legacy requests.
func requestedBrowserInput(getenv func(string) string) bool {
	flag := getenv("COMMERCE_MEDIA_BROWSER_INPUT_ENABLED")
	return flag != "" && flag != "0"
}
func recoveryLedger(pool *pgxpool.Pool, mixed bool) *live.MediaRecoveryObserver {
	if mixed {
		return live.NewMediaRecoveryLedgerWithBrowserInput(pool)
	}
	return live.NewMediaRecoveryLedger(pool)
}
