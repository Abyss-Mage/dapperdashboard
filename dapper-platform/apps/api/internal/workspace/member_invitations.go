package workspace

import (
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
