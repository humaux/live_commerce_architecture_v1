package live

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"livecommerce/internal/integrations/livekit"
)

func (r *MediaRecoveryObserver) recoveryEndpoints(key mediaProjectKey, identity string, inputRequired bool) (mediaEndpoint, browserInputEndpoint, bool) {
	media, ok := r.projects[key]
	if !ok || media.client == nil || media.identity != identity {
		return mediaEndpoint{}, browserInputEndpoint{}, false
	}
	if !inputRequired {
		return media, browserInputEndpoint{}, true
	}
	if r.input == nil {
		return mediaEndpoint{}, browserInputEndpoint{}, false
	}
	input, ok := r.input.projects[key]
	return media, input, ok && input.client != nil && input.identity == identity
}

func observeRecoveryEgress(ctx context.Context, client *livekit.Client, room, egressID string) (string, livekit.Observation, error) {
	if egressID == "" {
		observation, err := client.FindByRoom(ctx, room)
		return "ROOM", observation, err
	}
	observation, err := client.Query(ctx, livekit.Target{RoomName: room, EgressID: egressID})
	return "QUERY", observation, err
}

// WitnessWithInput attests only IDs returned by a committed mixed readback.
func (r *MediaRecoveryObserver) WitnessWithInput(ctx context.Context, episode, operation,
	inputID, egressID string, elapsedMS int64) (string, error) {
	if r == nil || !r.withInput || r.pool == nil || ctx == nil || elapsedMS < 0 || elapsedMS > 90000 {
		return "", ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	var input, egress any
	if inputID != "" {
		input = inputID
	}
	if egressID != "" {
		egress = egressID
	}
	if r.pool.QueryRow(bounded, `SELECT live.witness_media_recovery_episode_with_input(
	 $1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::bigint)`, episode, operation, input, egress, elapsedMS).Scan(&result) != nil {
		return "", ErrMediaRecovery
	}
	return result, nil
}

// Observe on a mixed observer reads durable receipts before each claim. A
// record ACK can disappear after commit, so claims and readback, never the
// provider response alone, decide which side remains unproven.
func (r *MediaRecoveryObserver) observeWithInput(ctx context.Context, episode string, member RecoveryMember) (string, error) {
	if r == nil || r.pool == nil || len(r.projects) == 0 || ctx == nil ||
		member.Disposition != "pending" || member.OperationID == "" || member.JobID < 1 ||
		(!member.InputRequired && !member.EgressRequired) ||
		(member.ExecutionProfile != "PROVIDER_MOCK" && member.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS") ||
		(member.ExecutionProfile == "PROVIDER_MOCK" && (member.InputRequired || !member.EgressRequired)) {
		return "", ErrMediaRecovery
	}
	turn, stop := context.WithTimeout(ctx, 25*time.Second)
	defer stop()
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
	if !generation.Valid || generation.Int64 <= member.BaselineGeneration ||
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
	media, input, configured := r.recoveryEndpoints(mediaProjectKey{projectID.String, version.Int64}, endpoint.String, inputRequired)
	if !configured {
		finish("credential_unavailable")
		return "credential_unavailable", nil
	}
	provider, done := context.WithTimeout(turn, 10*time.Second)
	defer done()
	inputFailed := false
	if inputRequired && !inputID.Valid {
		observed, observeErr := input.client.ObserveInput(provider,
			livekit.InputTarget{RoomName: room.String, Identity: publisher.String})
		var source, result string
		var sid, state any
		if observeErr == nil {
			if observed.RoomName == room.String && observed.Identity == publisher.String {
				source, result, sid, state = "PARTICIPANT", "PRESENT", observed.ParticipantID, observed.State
			}
		} else {
			_, roomErr := input.client.ObserveInputRoom(provider, room.String)
			if errors.Is(roomErr, livekit.ErrNotObserved) {
				source, result = "ROOM", "ABSENT"
			}
		}
		if source != "" {
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
			}
		} else {
			inputFailed = true
		}
	}
	if egressRequired && !egressID.Valid {
		source, observation, observeErr := observeRecoveryEgress(provider, media.client, room.String, egress.String)
		if observeErr == nil {
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
		return "remote_unknown", nil
	}
	if inputFailed {
		finish("remote_unknown")
		return "remote_unknown", nil
	}
	// Input-only proof leaves the reconcile lease to be released explicitly.
	finish("not_observed")
	return "checked", nil
}
