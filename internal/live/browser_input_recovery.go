// Responsibility: mixed recovery observation under the original fenced lease.
// Dependencies: immutable runtime maps, LiveKit reads and private recovery SQL.
// Callers: MediaRecoveryObserver.Observe. Invariants: durable proof precedes retry;
// independent required sides share one provider budget; no lifecycle mutations.

package live

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/platform"
)

// NewMediaRecoveryObserverWithBrowserInput admits provider I/O only after exact
// configuration, recovery-role and both readiness checks have succeeded.
func NewMediaRecoveryObserverWithBrowserInput(ctx context.Context, pool *pgxpool.Pool, projects []MediaProject, runtime *BrowserInputRuntime) (*MediaRecoveryObserver, error) {
	if ctx == nil || pool == nil {
		return nil, ErrMediaRecovery
	}
	media, input, err := recoveryProjectMaps(projects, runtime)
	if err != nil {
		return nil, err
	}
	if platform.ValidateMediaRecoveryPool(ctx, pool) != nil {
		return nil, ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ready bool
	if pool.QueryRow(bounded, `SELECT live.media_recovery_ready() AND live.media_browser_input_recovery_ready()`).Scan(&ready) != nil || !ready {
		return nil, ErrMediaRecovery
	}
	return &MediaRecoveryObserver{pool: pool, browserInput: true, providerAdmitted: true, projects: media, input: input}, nil
}

func recoveryProjectMaps(projects []MediaProject, runtime *BrowserInputRuntime) (map[mediaProjectKey]mediaEndpoint, *BrowserInputRuntime, error) {
	if len(projects) < 1 || len(projects) > 128 || runtime == nil || len(runtime.projects) < 1 || len(runtime.projects) > 128 {
		return nil, nil, ErrMediaRecovery
	}
	selected := make(map[mediaProjectKey]mediaEndpoint, len(projects))
	for _, p := range projects {
		if !mediaProjectPattern.MatchString(p.ProjectID) || p.CredentialVersion < 1 || p.Config.Environment != "MOCK" || p.Transport == nil {
			return nil, nil, ErrMediaRecovery
		}
		key := mediaProjectKey{p.ProjectID, p.CredentialVersion}
		if _, ok := selected[key]; ok {
			return nil, nil, ErrMediaRecovery
		}
		c, e := livekit.New(p.Config, p.Transport)
		if e != nil {
			return nil, nil, ErrMediaRecovery
		}
		selected[key] = mediaEndpoint{identity: p.Config.Endpoint, client: c}
	}
	copied := &BrowserInputRuntime{projects: make(map[mediaProjectKey]browserInputEndpoint, len(runtime.projects))}
	for key, in := range runtime.projects {
		media, ok := selected[key]
		if !ok || in.client == nil || in.identity != media.identity || !canonicalBrowserInputURL(in.browserURL) {
			return nil, nil, ErrMediaRecovery
		}
		copied.projects[key] = in
	}
	return selected, copied, nil
}

// Resolve independently: a missing or mismatched side never suppresses the other.
func (r *MediaRecoveryObserver) recoveryEndpoints(key mediaProjectKey, identity string) (mediaEndpoint, browserInputEndpoint, bool, bool) {
	media, ok := r.projects[key]
	mediaOK := ok && media.client != nil && media.identity == identity
	var input browserInputEndpoint
	if r.input != nil {
		input = r.input.projects[key]
	}
	return media, input, mediaOK, input.client != nil && input.identity == identity
}
func recoveryMissingSides(inputRequired, egressRequired bool, inputID, egressID sql.NullString) (bool, bool) {
	return inputRequired && !inputID.Valid, egressRequired && !egressID.Valid
}
func recoveryTurn(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	limit := time.Now().Add(25 * time.Second)
	if deadline.Before(limit) {
		limit = deadline
	}
	return context.WithDeadline(ctx, limit)
}

// This returns provider evidence only; SQL must still qualify and commit it.
func observeRecoveryInput(ctx context.Context, client *livekit.Client, room, publisher string) (string, string, any, any, error) {
	if ctx.Err() != nil {
		return "", "", nil, nil, ctx.Err()
	}
	observed, err := client.ObserveInput(ctx, livekit.InputTarget{RoomName: room, Identity: publisher})
	if err == nil && observed.RoomName == room && observed.Identity == publisher {
		return "PARTICIPANT", "PRESENT", observed.ParticipantID, observed.State, nil
	}
	if ctx.Err() != nil {
		return "", "", nil, nil, ctx.Err()
	}
	if _, err = client.ObserveInputRoom(ctx, room); errors.Is(err, livekit.ErrNotObserved) {
		return "ROOM", "ABSENT", nil, nil, nil
	}
	return "", "", nil, nil, livekit.ErrUnavailable
}

func observeRecoveryEgress(ctx context.Context, client *livekit.Client, room, egressID string) (string, livekit.Observation, error) {
	if ctx.Err() != nil {
		return "", livekit.Observation{}, ctx.Err()
	}
	if egressID == "" {
		observation, err := client.FindByRoom(ctx, room)
		return "ROOM", observation, err
	}
	observation, err := client.Query(ctx, livekit.Target{RoomName: room, EgressID: egressID})
	return "QUERY", observation, err
}

// Observe on a mixed observer reads durable receipts before each claim. A
// record ACK can disappear after commit, so claims and readback, never the
// provider response alone, decide which side remains unproven.
func (r *MediaRecoveryObserver) observeWithInput(ctx context.Context, episode string, member RecoveryMember) (string, error) {
	if r == nil || r.pool == nil || !r.browserInput || !r.providerAdmitted || ctx == nil ||
		member.EpisodeID != episode || episode == "" || member.BaselineGeneration < 0 || member.Disposition != "pending" || member.OperationID == "" || member.JobID < 1 ||
		(!member.InputRequired && !member.EgressRequired) ||
		(member.ExecutionProfile != "PROVIDER_MOCK" && member.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS") ||
		(member.ExecutionProfile == "PROVIDER_MOCK" && (member.InputRequired || !member.EgressRequired)) {
		return "", ErrMediaRecovery
	}
	turn, stop := recoveryTurn(ctx, member.DeadlineAt)
	defer stop()
	if turn.Err() != nil {
		return "overdue", nil
	}
	// This read is mandatory after a possible lost record acknowledgement.
	readback, err := r.Read(turn, episode)
	if err != nil {
		return "", ErrMediaRecovery
	}
	for _, row := range readback {
		if row.OperationID == member.OperationID && row.EpisodeID == episode && row.JobID == member.JobID &&
			row.ExecutionProfile == member.ExecutionProfile && row.InputRequired == member.InputRequired &&
			row.EgressRequired == member.EgressRequired && row.BaselineGeneration == member.BaselineGeneration &&
			(!member.InputRequired || (row.InputObservationID != "" && row.InputObservationGeneration > member.BaselineGeneration &&
				((row.InputObservationSource == "PARTICIPANT" && row.InputObservationResult == "PRESENT") ||
					(row.InputObservationSource == "ROOM" && row.InputObservationResult == "ABSENT")))) &&
			(!member.EgressRequired || (row.ObservationID != "" && row.ObservationGeneration > member.BaselineGeneration &&
				(row.ObservationSource == "QUERY" || row.ObservationSource == "ROOM"))) {
			return "already_observed", nil
		}
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(turn, 5*time.Second)
	var status string
	var generation, version sql.NullInt64
	var projectID, endpoint, room, publisher, egress, profile, inputID, egressID sql.NullString
	var inputRequired, egressRequired bool
	err = r.pool.QueryRow(bounded, `SELECT disposition,generation,project_id,credential_version,
	 endpoint_identity,room_name,publisher_identity,egress_id,execution_profile,input_required,egress_required,
	 input_observation_id::text,egress_observation_id::text
	 FROM live.claim_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea)`,
		episode, member.OperationID, member.JobID, token[:]).Scan(&status, &generation, &projectID,
		&version, &endpoint, &room, &publisher, &egress, &profile, &inputRequired, &egressRequired,
		&inputID, &egressID)
	cancel()
	if err != nil {
		return "", ErrMediaRecovery
	}
	if status != "claimed" {
		return status, nil
	}
	if !generation.Valid || generation.Int64 < 1 || generation.Int64 <= member.BaselineGeneration ||
		!projectID.Valid || !version.Valid || !endpoint.Valid || !room.Valid || !profile.Valid ||
		profile.String != member.ExecutionProfile || inputRequired != member.InputRequired ||
		egressRequired != member.EgressRequired ||
		(inputRequired && !publisher.Valid) {
		return "", ErrMediaRecovery
	}
	finish := func(code string) {
		short, done := context.WithTimeout(turn, 5*time.Second)
		defer done()
		var released string
		_ = r.pool.QueryRow(short, `SELECT live.finish_media_recovery_observation_with_input(
		 $1::uuid,$2::uuid,$3::bigint,$4::bytea,$5::text)`, episode, member.OperationID,
			generation.Int64, token[:], code).Scan(&released)
	}
	media, input, mediaConfigured, inputConfigured := r.recoveryEndpoints(mediaProjectKey{projectID.String, version.Int64}, endpoint.String)
	missingInput, missingEgress := recoveryMissingSides(inputRequired, egressRequired, inputID, egressID)
	observeInput, observeEgress := missingInput && inputConfigured, missingEgress && mediaConfigured
	inputFailed := missingInput && !inputConfigured
	inputCommitted := inputRequired && inputID.Valid
	egressFailed := missingEgress && !mediaConfigured
	provider, done := context.WithTimeout(turn, 10*time.Second)
	defer done()
	if observeInput {
		source, result, sid, state, observeErr := observeRecoveryInput(provider, input.client, room.String, publisher.String)
		if observeErr == nil && source != "" && provider.Err() == nil {
			short, end := context.WithTimeout(turn, 5*time.Second)
			var disposition string
			var persistedID string
			err = r.pool.QueryRow(short, `SELECT disposition,input_observation_id::text
			 FROM live.record_browser_input_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,
			 $5::text,$6::text,$7::text,$8::text)`, episode, member.OperationID,
				generation.Int64, token[:], source, result, sid, state).Scan(&disposition, &persistedID)
			end()
			if err != nil || disposition != "checked" || persistedID == "" {
				// The transaction may have committed. Never issue a second input RPC.
				inputFailed = true
			} else {
				inputCommitted = true
			}
		} else {
			inputFailed = true
		}
	}
	if observeEgress && provider.Err() == nil {
		source, observation, observeErr := observeRecoveryEgress(provider, media.client, room.String, egress.String)
		if observeErr == nil && provider.Err() == nil {
			short, end := context.WithTimeout(turn, 5*time.Second)
			var disposition, persistedID string
			err = r.pool.QueryRow(short, `SELECT disposition,observation_id::text
			 FROM live.record_media_recovery_observation_with_input($1::uuid,$2::uuid,$3::bigint,$4::bytea,
			 $5::text,$6::text,$7::text,$8::text,$9::bigint,$10::bigint,$11::bigint)`,
				episode, member.OperationID, generation.Int64, token[:], source, observation.EgressID,
				observation.RoomName, observation.Status, observation.StartedAtNS,
				observation.UpdatedAtNS, observation.EndedAtNS).Scan(&disposition, &persistedID)
			end()
			if err == nil && persistedID != "" && (disposition == "checked" || disposition == "terminal") {
				if inputFailed {
					return "partial", nil
				}
				return disposition, nil
			}
		}
		finish("remote_unknown")
		if inputCommitted {
			return "partial", nil
		}
		return "remote_unknown", nil
	}
	if egressFailed || (missingEgress && provider.Err() != nil) {
		finish("credential_unavailable")
		if inputCommitted {
			return "partial", nil
		}
		return "credential_unavailable", nil
	}
	if inputFailed {
		finish("credential_unavailable")
		if egressRequired && egressID.Valid {
			return "partial", nil
		}
		return "credential_unavailable", nil
	}
	// Input-only proof leaves the reconcile lease to be released explicitly.
	finish("not_observed")
	return "checked", nil
}
