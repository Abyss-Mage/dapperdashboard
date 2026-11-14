package workspace

import (
	"log"
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

func registerMemberTerminationRoutes(mux *http.ServeMux, store *postgres.Store) {
	set := func(active bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			org, member := r.PathValue("organization"), r.PathValue("member")
			if !uuid(org) || !uuid(member) {
				reply(w, 400, map[string]string{"error": "Valid selectors required"})
				return
			}
			err := store.SetMemberActive(r.Context(), requestSession(r).principalID, org, member, active)
			switch {
			case errors.Is(err, postgres.ErrMemberSelf):
				reply(w, 409, map[string]string{"error": "You cannot terminate your own account."})
			case errors.Is(err, postgres.ErrMemberLastAdmin):
				reply(w, 409, map[string]string{"error": "At least one active administrator must remain."})
			case errors.Is(err, postgres.ErrMemberNotFound):
				reply(w, 404, map[string]string{"error": "That team member was not found."})
			case err != nil:
				failure(w, err)
			default:
				reply(w, 200, map[string]bool{"active": active})
			}
		}
	}
	mux.HandleFunc("POST /api/v1/organizations/{organization}/members/{member}/terminate", set(false))
	mux.HandleFunc("POST /api/v1/organizations/{organization}/members/{member}/reinstate", set(true))
	// error404 #90: remove a person from the team (terminated, roles and membership removed; their records stay).
	mux.HandleFunc("POST /api/v1/organizations/{organization}/members/{member}/remove", func(w http.ResponseWriter, r *http.Request) {
		org, member := r.PathValue("organization"), r.PathValue("member")
		if !uuid(org) || !uuid(member) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		err := store.RemoveMember(r.Context(), requestSession(r).principalID, org, member)
		switch {
		case errors.Is(err, postgres.ErrMemberSelf):
			reply(w, 409, map[string]string{"error": "You cannot remove your own account."})
		case errors.Is(err, postgres.ErrMemberLastAdmin):
			reply(w, 409, map[string]string{"error": "At least one active administrator must remain."})
		case errors.Is(err, postgres.ErrMemberNotFound):
			reply(w, 404, map[string]string{"error": "That team member was not found."})
		case err != nil:
			failure(w, err)
		default:
			reply(w, 200, map[string]bool{"removed": true})
		}
	})
	// error404 #118: delete a person completely; their email (or name) must be typed to confirm.
	mux.HandleFunc("POST /api/v1/organizations/{organization}/members/{member}/erase", func(w http.ResponseWriter, r *http.Request) {
		org, member := r.PathValue("organization"), r.PathValue("member")
		if !uuid(org) || !uuid(member) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var in struct {
			Confirm string `json:"confirm"`
		}
		if !decode(w, r, &in) {
			return
		}
		err := store.EraseMember(r.Context(), requestSession(r).principalID, org, member, in.Confirm)
		switch {
		case errors.Is(err, postgres.ErrPurgeConfirm):
			reply(w, 422, map[string]string{"error": "That does not match. Type their email exactly as shown."})
		case errors.Is(err, postgres.ErrMemberElsewhere):
			reply(w, 409, map[string]string{"error": "This person also belongs to another organization, so their account can't be deleted from here. Use Remove from team."})
		case errors.Is(err, postgres.ErrMemberSelf):
			reply(w, 409, map[string]string{"error": "You cannot delete your own account."})
		case errors.Is(err, postgres.ErrMemberLastAdmin):
			reply(w, 409, map[string]string{"error": "At least one active administrator must remain."})
		case errors.Is(err, postgres.ErrMemberNotFound):
			reply(w, 404, map[string]string{"error": "That team member was not found."})
		case err != nil:
			log.Printf("Erase of member %s failed: %v", member, err)
			failure(w, err)
		default:
			reply(w, 200, map[string]bool{"erased": true})
		}
	})
}
