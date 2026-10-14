// Purpose: REAL_PG tests-first proof that a rendered notification format refusal
// commits the admitted order with definitive not_sent before any send-job insert.
// Depends on: isolated DB, signed inbound, actual C1/C2 and complete VOID/replay oracle.
// Used by: parent-owned OA1 CI; no fake executor, handcrafted PREPARED DTO or dispatcher.
// Invariants: trusted template content faults are distinct from malformed authenticated
// inbound and AEAD dependency faults, which must retain retryable PENDING custody.
package foundation_test

import (
	"context"
	"strings"
	"testing"
)

func TestOrderAttemptNotificationRenderedFormatRefusalCommitsVoid(t *testing.T) {
	// Establish actual signed ingress/current capability/positive C1 custody first.
	// The helper creates a database exclusively for this scenario, not a shared DB.
	s := oaNotificationResumeNative(t)

	// Defensive trusted-template fault injection ONLY: merchants cannot publish or
	// replace order-pay-link/v1. Preserve its fixed identity/version/DM-only metadata
	// and one placeholder; the body alone fits 2000 runes, but the native order URL
	// replaces six placeholder runes with more than six and exceeds Facebook's cap.
	// No inbound ciphertext, recipient, crypto key, plan or native C2 stage is edited.
	body := strings.Repeat("好", 1994) + "{{連結}}"
	var injected bool
	err := s.e.p.f.owner.QueryRow(context.Background(), `UPDATE msgtemplates.fixed_templates
	 SET body=$1 WHERE template_id='order-pay-link/v1' AND version=1
	 RETURNING body=$1 AND char_length(body)=2000
	 AND (char_length(body)-char_length(replace(body,'{{連結}}','')))/char_length('{{連結}}')=1
	 AND kinds=ARRAY['dm']::text[] AND NOT public_safe`, body).Scan(&injected)
	if err != nil || !injected {
		t.Fatalf("isolated trusted fixed-template content fault sentinel: injected=%v err=%v", injected, err)
	}
	var facebook bool
	err = s.e.p.f.owner.QueryRow(context.Background(), `SELECT object='page'
	 FROM social.conversations WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		s.e.tenant(), s.e.store(), s.conversation).Scan(&facebook)
	if err != nil || !facebook {
		t.Fatalf("native Facebook rendered-length prerequisite: facebook=%v err=%v", facebook, err)
	}

	// Full actual Executor and SQL C2, not a classifier-only check. The shared
	// oracle requires one complete COMMITTED order/quote/hold/claim/pay credential,
	// own VOID(format_invalid), NULL operation/outbound/binding, no own send artifact
	// and the exact unchanged external-job ID set; replay preserves all those rows.
	oaNotificationResumeExecuteVoid(t, s, "format_invalid")
}
