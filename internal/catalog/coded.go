package catalog

import "net/http"

// Error is a coded refusal of the catalog routes (product-editor §f): Status is the HTTP status, Code the contract
// error code. The httperror message table must list every Code used here, or the merchant sees "internal"
// (mirrors merchanttools.Error and promotions.Coded).
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return "catalog: " + e.Code }

// refusal builds a coded refusal. amount_not_whole_twd and keyword_taken are the merchant 422/409 codes; live_window_open
// is a per-item bulk-status refusal (not a whole-request error).
func refusal(status int, code string) *Error { return &Error{Status: status, Code: code} }

// Sentinels the HTTP layer classifies (httpapi.catalogClassify checks *Error first, then the shared table).
var (
	ErrAmountNotWholeTWD = refusal(http.StatusUnprocessableEntity, "amount_not_whole_twd")
	ErrKeywordTaken      = refusal(http.StatusConflict, "keyword_taken")
	ErrLiveWindowOpen    = refusal(http.StatusConflict, "live_window_open")
)
