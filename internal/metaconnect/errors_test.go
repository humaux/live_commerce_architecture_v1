package metaconnect

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"livecommerce/internal/httperror"
)

// Every code this package can answer must survive httperror.Write; an unlisted code is rewritten to "internal" (found by the
// meta-connect PG gate before this guard existed).
func TestFrozenCodesSurviveHTTPError(t *testing.T) {
	codes := []string{"meta_connect_failed"}
	for code := range frozenStatus {
		codes = append(codes, code)
	}
	for _, code := range codes {
		w := httptest.NewRecorder()
		httperror.Write(w, 409, code)
		var env httperror.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != code {
			t.Errorf("code %q is rewritten to %q by httperror: add it to the message table", code, env.Code)
		}
	}
}
