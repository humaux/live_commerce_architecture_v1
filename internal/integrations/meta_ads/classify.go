package metaads

// classify.go: the contract §3 classification tables, verbatim. The single rule behind all of them:
// a Graph create/status POST has NO idempotency key (F9), so the only response that may end an
// operation as FAILED_FINAL is a 4xx whose body is a parseable Graph error (the request was
// rejected); every doubt (timeout, 5xx, transport error, unparseable body, 2xx without the expected
// shape) is UNKNOWN, and the only follow-up is the query-only reconcile, never a second POST.

import (
	"encoding/json"
	"strconv"

	"livecommerce/internal/integrations/core"
)

// Graph error codes that mean "throttled", not "rejected" (contract §3: FAILED_FINAL `rate_limited`, a retry is a new
// publish attempt, §5.3). Throttling is not an ad-policy refusal, so ads-graph Amendment 2 (pass Meta's wording through)
// does not replace it; Meta's message, if any, still rides along. Sources: F13 insights best practices (code 4), F21 rate
// limiting (BUC 80004); codes 17 and 613 are named by contract §3 (retrieved 2026-09-29).
const (
	codeAppLimit    = 4
	codeUserLimit   = 17
	codeCustomLimit = 613
	codeBUCLimit    = 80004
)

func unknown(code string) core.Outcome     { return core.Outcome{State: "UNKNOWN", Code: code} }
func failedFinal(code string) core.Outcome { return core.Outcome{State: "FAILED_FINAL", Code: code} }
func unconfirmed() core.Outcome            { return unknown("graph_unconfirmed") }

// Only the numeric code and user-facing message leave the Graph envelope.
func graphErrorCode(body []byte) (int, string, bool) {
	var doc struct {
		Error *struct {
			Code        int             `json:"code"`
			UserMessage json.RawMessage `json:"error_user_msg"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &doc) != nil || doc.Error == nil || doc.Error.Code < 1 || doc.Error.Code > 999999 {
		return 0, "", false
	}
	var message string
	_ = json.Unmarshal(doc.Error.UserMessage, &message)
	return doc.Error.Code, PlainUserMessage(message), true
}

// rejection is the FAILED_FINAL outcome for a 4xx Graph error body, or ok=false when the response is
// not a proven rejection (then the caller records UNKNOWN).
func rejection(rep reply, err error) (core.Outcome, bool) {
	if err != nil || rep.status < 400 || rep.status > 499 {
		return core.Outcome{}, false
	}
	code, message, ok := graphErrorCode(rep.body)
	if !ok {
		return core.Outcome{}, false
	}
	out := failedFinal("graph_" + strconv.Itoa(code))
	switch code {
	case codeAppLimit, codeUserLimit, codeCustomLimit, codeBUCLimit:
		out = failedFinal("rate_limited")
	}
	if message != "" {
		out.Detail = GraphRefusal{UserMessage: message}
	}
	return out, true
}

// failureOutcome classifies a response that is not the expected 2xx for a create or a read.
func failureOutcome(rep reply, err error) core.Outcome {
	if out, ok := rejection(rep, err); ok {
		return out
	}
	if err == nil && !rep.ok() {
		if code, message, ok := graphErrorCode(rep.body); ok {
			out := unknown("graph_" + strconv.Itoa(code))
			if message != "" {
				out.Detail = GraphRefusal{UserMessage: message}
			}
			return out
		}
	}
	return unconfirmed()
}

// A reconcile refusal may carry merchant-facing evidence, but cannot prove that
// an earlier write had no effect. Never promote it to FAILED_FINAL.
func reconcileFailure(rep reply, err error) core.Outcome {
	out := failureOutcome(rep, err)
	out.State = "UNKNOWN"
	return out
}

// classifyCreate: 2xx with a numeric `id` is SUCCEEDED with the id as provider_reference (it becomes
// ads.remote_objects.remote_id, `^[0-9]{1,40}$`); 2xx without one is UNKNOWN (the object may exist).
func classifyCreate(rep reply, err error) core.Outcome {
	if err == nil && rep.ok() {
		if id, ok := numericField(rep.body, "id"); ok {
			return core.Outcome{State: "SUCCEEDED", Code: "graph_created", ProviderReference: id}
		}
		return unconfirmed()
	}
	return failureOutcome(rep, err)
}

// classifyStatusPost is activate/pause: `{"success":true}` is SUCCEEDED and EVERYTHING else is
// UNKNOWN, never FAILED_FINAL: a rate-limited or rejected status POST does not prove the campaign's
// state, and the status GET (reconcile) is the only proof.
func classifyStatusPost(rep reply, err error) core.Outcome {
	if err == nil && rep.ok() {
		var doc struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(rep.body, &doc) == nil && doc.Success {
			return core.Outcome{State: "SUCCEEDED", Code: "graph_success"}
		}
	}
	return reconcileFailure(rep, err)
}

// numericField returns a string-typed digits-only member of a JSON object.
func numericField(body []byte, name string) (string, bool) {
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) != nil {
		return "", false
	}
	var s string
	if json.Unmarshal(doc[name], &s) != nil || !numericIDPattern.MatchString(s) {
		return "", false
	}
	return s, true
}
