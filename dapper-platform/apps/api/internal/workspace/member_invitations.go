package workspace

import (
	"context"
	"log"
	"crypto/sha256"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func invitationFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrInvitationInput):
		reply(w, 422, map[string]string{"error": "Provide a valid email, explicit expiry of 15–1440 minutes, or a new password of 15–1024 bytes as applicable"})
	case errors.Is(err, postgres.ErrInvitationUnavailable):
		reply(w, 410, map[string]string{"error": "Invitation unavailable, expired, revoked or already accepted"})
	case errors.Is(err, postgres.ErrInvitationPending):
		reply(w, 409, map[string]string{"error": "A pending invitation already exists; revoke it before creating a replacement"})
	case errors.Is(err, postgres.ErrInvitationIdentity):
		reply(w, 409, map[string]string{"error": "For an existing account, sign in to the invited account in another tab and accept without a new password. For a new account, sign out of any other active account first. Invitations cannot reset passwords."})
	case errors.Is(err, postgres.ErrAuthenticationBusy):
		w.Header().Set("Retry-After", "2")
		reply(w, 503, map[string]string{"error": "Enrollment is busy; retry shortly"})
	default:
		failure(w, err)
	}
}

// DeliverInvitations sends whatever is waiting in the invitation queue, once a minute and only when outgoing email works.
func DeliverInvitations(service *postgres.InvitationService, sender postgres.EmailSender) {
	for {
		if emailReady(sender) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			results, err := service.ProcessDeliveries(ctx, sender, 20)
			cancel()
			if err != nil {
				log.Print("Queued invitations could not be sent; will retry")
			} else if len(results) > 0 {
				log.Printf("Invitation queue: %d processed", len(results))
			}
		}
		time.Sleep(time.Minute)
	}
}

func registerMemberInvitationRoutes(mux *http.ServeMux, store *postgres.Store, service *postgres.InvitationService, sender postgres.EmailSender) {
	base := "/api/v1/organizations/{organization}/access/invitations"
	available := func(w http.ResponseWriter, r *http.Request) bool {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return false
		}
		if service == nil {
			if _, err := store.ManagedAccess(r.Context(), requestSession(r).principalID, org); err != nil {
				failure(w, err)
			} else {
				reply(w, 503, map[string]string{"error": "Invitation delivery encryption is not configured"})
			}
			return false
		}
		return true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		if !available(w, r) {
			return
		}
		items, err := service.List(r.Context(), requestSession(r).principalID, r.PathValue("organization"))
		if err != nil {
			invitationFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		if !available(w, r) {
			return
		}
		var input struct {
			Email            string `json:"email"`
			ExpiresInMinutes int    `json:"expiresInMinutes"`
			RoleID           string `json:"roleId"`
			LocationID       string `json:"locationId"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.ExpiresInMinutes < 15 || input.ExpiresInMinutes > 1440 {
			invitationFailure(w, postgres.ErrInvitationInput)
			return
		}
		var preset *postgres.InvitationRole
		if input.RoleID != "" || input.LocationID != "" {
			if !uuid(input.RoleID) || !uuid(input.LocationID) {
				invitationFailure(w, postgres.ErrInvitationInput)
				return
			}
			preset = &postgres.InvitationRole{RoleID: input.RoleID, LocationID: input.LocationID}
		}
		invitation, err := service.CreateWithRole(r.Context(), requestSession(r).principalID, r.PathValue("organization"), input.Email, time.Duration(input.ExpiresInMinutes)*time.Minute, preset)
		if err != nil {
			invitationFailure(w, err)
			return
		}
		reply(w, 202, invitation)
	})
	// A franchise owner's own team: they can invite Employees to their franchise, and nothing wider.
	teamBase := "/api/v1/organizations/{organization}/locations/{location}/team"
	teamSelectors := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return "", "", false
		}
		return org, loc, true
	}
	mux.HandleFunc("GET "+teamBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := teamSelectors(w, r)
		if !ok {
			return
		}
		team, err := store.FranchiseTeam(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, team)
	})
	mux.HandleFunc("POST "+teamBase+"/invitations", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := teamSelectors(w, r)
		if !ok {
			return
		}
		if service == nil {
			reply(w, 503, map[string]string{"error": "Invitation delivery encryption is not configured"})
			return
		}
		var input struct {
			Email string `json:"email"`
		}
		if !decode(w, r, &input) {
			return
		}
		invitation, err := service.CreateEmployeeInvitation(r.Context(), requestSession(r).principalID, org, loc, input.Email)
		if err != nil {
			invitationFailure(w, err)
			return
		}
		status := "not_configured"
		if emailReady(sender) {
			status = "queued"
			if results, e := service.ProcessDeliveries(r.Context(), sender, 10); e == nil {
				for _, result := range results {
					if result.InvitationID == invitation.ID {
						status = result.State
					}
				}
			}
		}
		reply(w, 202, map[string]any{"invitation": invitation, "emailStatus": status})
	})
	mux.HandleFunc("PUT "+teamBase+"/employees/{member}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := teamSelectors(w, r)
		if !ok {
			return
		}
		if !uuid(r.PathValue("member")) {
			reply(w, 400, map[string]string{"error": "Valid employee required"})
			return
		}
		var input struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		}
		if !decode(w, r, &input) {
			return
		}
		err := store.UpdateFranchiseEmployee(r.Context(), requestSession(r).principalID, org, loc, r.PathValue("member"), input.Name, input.Phone)
		if errors.Is(err, postgres.ErrProfileInput) {
			reply(w, 422, map[string]string{"error": "Enter a name. The phone number is optional; if you add one, include the country code, for example +1 705 555 0100."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	for action, active := range map[string]bool{"remove": false, "reinstate": true} {
		mux.HandleFunc("POST "+teamBase+"/employees/{member}/"+action, func(w http.ResponseWriter, r *http.Request) {
			org, loc, ok := teamSelectors(w, r)
			if !ok {
				return
			}
			if !uuid(r.PathValue("member")) {
				reply(w, 400, map[string]string{"error": "Valid employee required"})
				return
			}
			if err := store.SetFranchiseEmployeeActive(r.Context(), requestSession(r).principalID, org, loc, r.PathValue("member"), active); err != nil {
				failure(w, err)
				return
			}
			reply(w, 200, map[string]bool{"active": active})
		})
	}
	mux.HandleFunc("POST "+teamBase+"/invitations/{invitation}/revoke", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := teamSelectors(w, r)
		if !ok {
			return
		}
		if service == nil || !uuid(r.PathValue("invitation")) {
			reply(w, 400, map[string]string{"error": "Valid invitation required"})
			return
		}
		if err := service.RevokeEmployeeInvitation(r.Context(), requestSession(r).principalID, org, loc, r.PathValue("invitation")); err != nil {
			invitationFailure(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST "+base+"/{invitation}/revoke", func(w http.ResponseWriter, r *http.Request) {
		if !available(w, r) {
			return
		}
		if !uuid(r.PathValue("invitation")) {
			reply(w, 400, map[string]string{"error": "Valid invitation selector required"})
			return
		}
		if err := service.Revoke(r.Context(), requestSession(r).principalID, r.PathValue("organization"), r.PathValue("invitation")); err != nil {
			invitationFailure(w, err)
			return
		}
		w.WriteHeader(204)
	})
	// error404 #91: take an accepted, revoked or expired invitation off the list.
	mux.HandleFunc("POST "+base+"/{invitation}/hide", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("invitation")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invitation selector required"})
			return
		}
		err := store.HideMemberInvitation(r.Context(), requestSession(r).principalID, org, id)
		switch {
		case errors.Is(err, postgres.ErrInvitationStillOpen):
			reply(w, 409, map[string]string{"error": "This invitation can still be accepted. Revoke it first, then remove it."})
		case err != nil:
			invitationFailure(w, err)
		default:
			w.WriteHeader(204)
		}
	})
	mux.HandleFunc("POST "+base+"/dispatch", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		if _, err := store.ManagedAccess(r.Context(), requestSession(r).principalID, org); err != nil {
			failure(w, err)
			return
		}
		if service == nil {
			reply(w, 503, map[string]string{"error": "Invitation delivery encryption is not configured"})
			return
		}
		if !emailReady(sender) {
			reply(w, 503, map[string]string{"error": "Email delivery provider is not configured"})
			return
		}
		var input struct {
			Limit int `json:"limit"`
		}
		if r.ContentLength > 0 {
			if !decode(w, r, &input) {
				return
			}
		}
		results, err := service.ProcessDeliveries(r.Context(), sender, input.Limit)
		if err != nil {
			invitationFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"results": results, "processed": len(results)})
	})
}

type enrollmentBucket struct {
	count   int
	expires time.Time
}
type invitationEnrollment struct {
	service  *postgres.InvitationService
	mu       sync.Mutex
	attempts map[[32]byte]enrollmentBucket
}

func (e *invitationEnrollment) allow(address string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	for key, bucket := range e.attempts {
		if !now.Before(bucket.expires) {
			delete(e.attempts, key)
		}
	}
	key := sha256.Sum256([]byte(address))
	bucket := e.attempts[key]
	if bucket.count >= 30 || (bucket.count == 0 && len(e.attempts) >= 10000) {
		return false
	}
	if bucket.count == 0 {
		bucket.expires = now.Add(15 * time.Minute)
	}
	bucket.count++
	e.attempts[key] = bucket
	return true
}
func (e *invitationEnrollment) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		reply(w, 405, map[string]string{"error": "Use POST to accept an invitation"})
		return
	}
	if e.service == nil {
		reply(w, 503, map[string]string{"error": "Invitation enrollment is not configured"})
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !e.allow(ip) {
		w.Header().Set("Retry-After", "900")
		reply(w, 429, map[string]string{"error": "Too many enrollment attempts; retry later"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	var sessionToken string
	headers := r.Header.Values("Authorization")
	if len(headers) == 0 {
		if cookie, err := r.Cookie(identity.SessionCookieName); err == nil {
			sessionToken = cookie.Value
		}
	} else {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(headers) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			reply(w, 401, map[string]string{"error": "Valid existing session required"})
			return
		}
		sessionToken = parts[1]
	}
	err = e.service.Accept(r.Context(), input.Token, input.Password, sessionToken)
	input.Password = ""
	input.Token = ""
	if err != nil {
		invitationFailure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(204)
}
