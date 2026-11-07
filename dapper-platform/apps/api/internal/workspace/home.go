package workspace

import (
	"context"
	"net/http"
)

// registerHomeRoute tells the dashboard whether the signed-in person works only in the employee workspace (their only role is
// Employee), so the dashboard can send them there.
func registerHomeRoute(mux *http.ServeMux, store interface {
	WorkspaceOnly(ctx context.Context, principal string) (bool, error)
}) {
	mux.HandleFunc("GET /api/v1/account/home", func(w http.ResponseWriter, r *http.Request) {
		only, err := store.WorkspaceOnly(r.Context(), requestSession(r).principalID)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"workspaceOnly": only})
	})
}
