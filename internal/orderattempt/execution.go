// Purpose: typed responses from the durable order-attempt execution authority.
// Depends on: the standard JSON package and frozen order-attempt-v1 receipts.
// Used by: inline/River orchestration and the later versioned HTTP adapters.
// Invariants: receipt is safe durable evidence, never a plan, token or private contact.
package orderattempt

import "encoding/json"

// ExecutionReceipt separates durable attempt state from an HTTP observation.
// Terminal receipt bytes are replayed from PG; callers never infer absence from
// missing JSON, a transport error or current order commercial state.
type ExecutionReceipt struct {
	State   string          `json:"state"`
	Receipt json.RawMessage `json:"receipt"`
}
