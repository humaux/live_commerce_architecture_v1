// Purpose: literal API-mux fixture; never executed as a server.
// Depends on: net/http only.
// Used by: CI-DRIFT AST tests.
package httpapi

import "net/http"

const rootPrefix = "/v1/admin/stores/{store_id}"

func register(mux *http.ServeMux) {
	base := rootPrefix
	alias := base + "/widgets/{widget_id}"
	mux.HandleFunc(http.MethodGet+" "+(alias), handler)
	mux.HandleFunc(pattern(), handler)
}
func handler(w http.ResponseWriter, r *http.Request) {}
