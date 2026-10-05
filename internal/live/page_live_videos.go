// Purpose: A5-3 Page "live videos" picker (MOCK): the merchant-side plan that mints one meta.live_videos operation + default-lane River job in the caller's transaction (integration.plan_meta_live_videos, migrations/0118), and the snapshot read (live.read_page_live_videos). The API process never loads a Page token; the Graph GET happens in cmd/claims-worker.
// Depends on: draft.go (authorize/mapError/readPermission/managePermission), results.go mapReadError, command.Run/Audit, core.InsertOperationJob, integration.plan_meta_live_videos / live.read_page_live_videos (0118), river.river_job (0018).
// Used by: internal/httpapi/live_flow.go POST .../page-live-videos/read and GET .../page-live-videos.
package live

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

// PageLiveVideosReadInput is the exact A5-3 POST body: the bound Facebook Page to list live videos for.
type PageLiveVideosReadInput struct {
	BindingID string `json:"binding_id"`
}

// PageLiveVideosRead is the A5-3 POST response: the planned (or in-flight) operation and its raw
// operation-ledger state (READY when freshly planned).
type PageLiveVideosRead struct {
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
}

// LiveVideoItem is one normalized live video: never a token, never viewer/buyer data. StartedAt is
// the raw Meta creation_time (bounded by the worker, not parsed — its +0000 offset is not RFC 3339).
type LiveVideoItem struct {
	VideoID   string `json:"video_id"`
	PostID    string `json:"post_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
}

// PageLiveVideos is the A5-3 GET response: none until a read is requested, pending while one is in
// flight, then succeeded/failed from the latest snapshot; unknown for a terminal operation with an
// unrecognised state. Items are only present on a terminal snapshot (≤25, LIVE first).
type PageLiveVideos struct {
	BindingID   string          `json:"binding_id"`
	State       string          `json:"state"`
	Code        *string         `json:"code"`
	RequestedAt *time.Time      `json:"requested_at"`
	FetchedAt   *time.Time      `json:"fetched_at"`
	Items       []LiveVideoItem `json:"items"`
}

// ReadPageLiveVideos plans a read-only meta.live_videos operation for the bound Page. It neither
// reads provider credentials nor mutates a Meta asset; the receipt and default-lane job commit
// together, and UNKNOWN is never blindly replayed (I06). jobs is the insert-only main-schema River
// client (Schema "river", like the audience read): the plan SQL admits only a same-transaction
// external_operation_v1 job on that schema.
func ReadPageLiveVideos(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, bindingID string, jobs *river.Client[pgx.Tx]) (PageLiveVideosRead, error) {
	if !command.ValidID(bindingID) {
		return PageLiveVideosRead{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return PageLiveVideosRead{}, err
	}
	hash := sha256.Sum256([]byte(token))
	request := struct {
		PrincipalID string `json:"principal_id"`
		BindingID   string `json:"binding_id"`
	}{scope.PrincipalID, bindingID}
	var out PageLiveVideosRead
	err := command.Run(ctx, tx, scope, "live.page_live_videos.read", key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		// Preflight: reuse an in-flight read for the same binding (I23) or say plan_required.
		var raw json.RawMessage
		if err := tx.QueryRow(ctx, `SELECT integration.plan_meta_live_videos($1,$2::uuid,$3::uuid,NULL,NULL)`,
			hash[:], scope.StoreID, bindingID).Scan(&raw); err != nil {
			return mapReadError(err)
		}
		var preflight struct {
			PlanRequired bool   `json:"plan_required"`
			OperationID  string `json:"operation_id"`
		}
		if err := json.Unmarshal(raw, &preflight); err != nil {
			return mapReadError(err)
		}
		if !preflight.PlanRequired {
			if !command.ValidID(preflight.OperationID) {
				return command.ErrInvalid
			}
			return command.Audit(ctx, tx, scope, "live.page_live_videos.read_requested")
		}
		var op string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&op); err != nil {
			return mapReadError(err)
		}
		if !command.ValidID(op) {
			return command.ErrInvalid
		}
		job, err := core.InsertOperationJob(ctx, jobs, tx, op)
		if err != nil {
			return mapReadError(err)
		}
		if err = tx.QueryRow(ctx, `SELECT integration.plan_meta_live_videos($1,$2::uuid,$3::uuid,$4::uuid,$5)`,
			hash[:], scope.StoreID, bindingID, op, job).Scan(&raw); err != nil {
			return mapReadError(err)
		}
		if err = json.Unmarshal(raw, &out); err != nil {
			return mapReadError(err)
		}
		return command.Audit(ctx, tx, scope, "live.page_live_videos.read_requested")
	})
	if err != nil {
		return PageLiveVideosRead{}, mapReadError(err)
	}
	// A replay remains an authenticated request even after its lock wait.
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return PageLiveVideosRead{}, err
	}
	return out, nil
}

// GetPageLiveVideos reads the latest snapshot (or newest operation state) for one bound Page. The
// definer reads the GUC-scoped store, so no token hash is needed; the authorize fences pin the scope
// and permission on both sides of the projection.
func GetPageLiveVideos(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, bindingID string) (PageLiveVideos, error) {
	if !command.ValidID(bindingID) {
		return PageLiveVideos{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return PageLiveVideos{}, err
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT live.read_page_live_videos($1::uuid)`, bindingID).Scan(&raw); err != nil {
		return PageLiveVideos{}, mapReadError(err)
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return PageLiveVideos{}, err
	}
	return decodePageLiveVideos(raw)
}

// decodePageLiveVideos re-verifies the snapshot projection: closed state vocabulary, canonical
// binding id and a non-null items array.
func decodePageLiveVideos(raw []byte) (PageLiveVideos, error) {
	var out PageLiveVideos
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return PageLiveVideos{}, ErrResultsUnavailable
	}
	switch out.State {
	case "none", "pending", "succeeded", "failed", "unknown":
	default:
		return PageLiveVideos{}, ErrResultsUnavailable
	}
	if !command.ValidID(out.BindingID) || out.Items == nil {
		return PageLiveVideos{}, ErrResultsUnavailable
	}
	return out, nil
}
