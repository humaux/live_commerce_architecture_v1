package core

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
)

// InsertOperationJob inserts the external_operation_v1 River job for an operation UUID the
// caller minted, inside the caller's transaction. It uses exactly the Plan shape (args
// {operation_id, version:1}, default queue, no InsertOpts) that the meta-claims-intake §5.4
// river_job guard admits from the claims-intake login. The caller inserts the matching
// integration.operations row (integration.plan_claim_reply) in the same transaction.
func InsertOperationJob(ctx context.Context, jobs *river.Client[pgx.Tx], tx pgx.Tx, operationID string) (int64, error) {
	if ctx == nil || jobs == nil || tx == nil || !command.ValidID(operationID) {
		return 0, command.ErrInvalid
	}
	job, err := jobs.InsertTx(ctx, tx, externalOperationArgs{OperationID: operationID, Version: 1}, nil)
	if err != nil {
		return 0, err
	}
	return job.Job.ID, nil
}
