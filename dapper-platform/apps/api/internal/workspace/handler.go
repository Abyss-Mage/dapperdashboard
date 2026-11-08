// Package workspace exposes scoped PostgreSQL operations. It cannot issue a
// session: identity verification/provisioning must occur through a trusted flow.
package workspace

import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func uuid(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	return decodeLimited(w, r, value, 8192)
}
func decodeLimited(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		reply(w, 415, map[string]string{"error": "Use application/json"})
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		reply(w, 400, map[string]string{"error": "Send one valid JSON object with supported fields"})
		return false
	}
	return true
}

func failure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrForbidden) || errors.Is(err, postgres.ErrScope) {
		reply(w, 403, map[string]string{"error": "This action is not granted for the selected location"})
		return
	}
	if errors.Is(err, postgres.ErrCustomerDuplicate) {
		reply(w, 409, map[string]string{"error": "Another customer already uses this email or phone number. Find that customer instead of adding a new one."})
		return
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23503" {
		reply(w, 422, map[string]any{"errors": map[string]string{"customerId": "Choose a customer in the selected location"}})
		return
	}
	reply(w, 503, map[string]string{"error": "The workspace could not complete this request"})
}

type customerInput struct {
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Phone   string  `json:"phone"`
	Address *string `json:"address"`
}
type vehicleInput struct {
	CustomerID  string `json:"customerId"`
	Description string `json:"description"`
	Category    string `json:"categoryDescription"`
	Identifier  string `json:"identifier"`
}

// NewHandler is loopback-only until production identity, deployment and client
// visibility policies are approved. Browser writes require the exact origin.
type sessionKey struct{}
type sessionActor struct{ principalID, token string }

func requestSession(r *http.Request) sessionActor {
	value, _ := r.Context().Value(sessionKey{}).(sessionActor)
	return value
}

func NewHandler(store *postgres.Store, origin string, loginRoutes ...http.Handler) http.Handler {
	return NewHandlerWithFullServices(store, origin, nil, nil, nil, "", "", loginRoutes...)
}

func NewHandlerWithInvitations(store *postgres.Store, origin string, invitations *postgres.InvitationService, loginRoutes ...http.Handler) http.Handler {
	return NewHandlerWithFullServices(store, origin, invitations, nil, nil, "", "", loginRoutes...)
}

func NewHandlerWithServices(store *postgres.Store, origin string, invitations *postgres.InvitationService, sqClient square.Client, loginRoutes ...http.Handler) http.Handler {
	return NewHandlerWithFullServices(store, origin, invitations, sqClient, nil, "", "", loginRoutes...)
}

func NewHandlerWithFullServices(store *postgres.Store, origin string, invitations *postgres.InvitationService, sqClient square.Client, emailSender postgres.EmailSender, sqWebhookURL, sqSignatureKey string, loginRoutes ...http.Handler) http.Handler {
	enrollment := &invitationEnrollment{service: invitations, attempts: make(map[[32]byte]enrollmentBucket)}
	var login http.Handler
	if len(loginRoutes) > 0 {
		login = loginRoutes[0]
	}
	mux := http.NewServeMux()
	registerJobRoutes(mux, store)
	registerServiceRoutes(mux, store)
	registerPublicContentRoutes(mux, store)
	registerMapTileRoutes(mux, origin)
	registerSquareAppCredentialRoutes(mux, store, origin)
	registerCentralServiceLinkRoutes(mux, store)
	registerStructureRoutes(mux, store)
	registerSchedulingRoutes(mux, store)
	registerSchedulingHoursRoutes(mux, store)
	registerFranchiseTaxRoutes(mux, store)
	registerCustomerManageRoutes(mux, store)
	registerEmployeeInvoiceRoutes(mux, store)
	registerSchedulingRuleRoutes(mux, store)
	registerServiceReservationRoutes(mux, store)
	registerReservationReadRoutes(mux, store)
	registerCentralServiceRoutes(mux, store)
	registerAttendanceRoutes(mux, store)
	registerGPSRoutes(mux, store)
	registerBookingRoutes(mux, store)
	registerBookingOfferRoutes(mux, store)
	registerCustomerBookingRequestRoutes(mux, store)
	registerInvoiceRoutes(mux, store, sqClient)
	registerCalendarRoutes(mux, store)
	registerSOPRoutes(mux, store)
	registerCustomerPortalRoutes(mux, store)
	registerBackupRoutes(mux, store, emailSender)
	registerAccessManagementRoutes(mux, store)
	registerInvoiceIssuanceRoutes(mux, store)
	registerInvoiceTemplateRoutes(mux, store)
	registerRuleAvailabilityRoutes(mux, store)
	registerPublicBookingRoutes(mux, store, emailSender, origin)
	registerSiteAssetRoutes(mux, store)
	installBookingEmails(store, emailSender, origin)
	registerMemberInvitationRoutes(mux, store, invitations, emailSender)
	registerAccountCredentialRoutes(mux, store, emailSender, origin)
	registerMessagingRoutes(mux, store, emailSender)
	registerHomeRoute(mux, store)
	registerWebsiteTrackingRoutes(mux, store)
	registerCustomScriptRoutes(mux, store)
	registerCalendarFeedRoutes(mux, store, origin)
	registerWebsiteEnquiryRoutes(mux, store)
	registerActivityLogRoutes(mux, store)
	registerMemberTerminationRoutes(mux, store)
	registerCouponRoutes(mux, store)
	registerReportRoutes(mux, store)
	registerCustomerAccountManagementRoutes(mux, store)
	registerQuoteRoutes(mux, store)
	registerSquareIntentRoutes(mux, store)
	registerSquareOAuthRoutes(mux, store, origin)
	registerEmployeeSquareRoutes(mux, store)
	registerEmployeeUploadRoutes(mux, store)
	registerUploadReviewRoutes(mux, store)
	registerEmployeeDocumentRoutes(mux, store)
	registerAccountRoutes(mux, store)
	registerSquareWebhookRoutes(mux, store, sqClient, sqWebhookURL, sqSignatureKey)
	registerDealershipRoutes(mux, store)
	registerMarketingRoutes(mux, store)
	registerOwnerOverviewRoutes(mux, store)
	registerRoleRoutes(mux, store)
	registerEmployeeWorkspaceRoutes(mux, store)
	mux.HandleFunc("GET /api/v1/auth/access", func(w http.ResponseWriter, r *http.Request) {
		locations, organizations, err := store.Access(r.Context(), requestSession(r).principalID)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"locations": locations, "organizations": organizations, "limit": 100})
	})
	mux.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"principalId": requestSession(r).principalID})
	})
	mux.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := store.RevokeSession(r.Context(), requestSession(r).token); err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(204)
	})
	base := "/api/v1/organizations/{organization}/locations/{location}/"
	for _, resource := range []string{"customers", "vehicles"} {
		for _, method := range []string{"GET", "POST"} {
			mux.HandleFunc(method+" "+base+resource, func(w http.ResponseWriter, r *http.Request) {
				organization, location := r.PathValue("organization"), r.PathValue("location")
				if !uuid(organization) || !uuid(location) {
					reply(w, 400, map[string]string{"error": "Valid organization and location selectors are required"})
					return
				}
				action := resource + ".read"
				if r.Method == "POST" {
					action = resource + ".create"
				}
				scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, action)
				if err != nil {
					failure(w, err)
					return
				}
				if r.Method == "GET" {
					if resource == "customers" {
						items, err := store.Customers(r.Context(), scope)
						if err != nil {
							failure(w, err)
							return
						}
						reply(w, 200, map[string]any{"items": items, "limit": 100})
					} else {
						items, err := store.Vehicles(r.Context(), scope)
						if err != nil {
							failure(w, err)
							return
						}
						reply(w, 200, map[string]any{"items": items, "limit": 100})
					}
					return
				}
				if resource == "customers" {
					var input customerInput
					if !decode(w, r, &input) {
						return
					}
					input.Name = strings.TrimSpace(input.Name)
					input.Email = strings.TrimSpace(input.Email)
					input.Phone = strings.TrimSpace(input.Phone)
					fields := map[string]string{}
					if size := utf8.RuneCountInString(input.Name); size < 2 || size > 100 {
						fields["name"] = "Enter a name between 2 and 100 characters"
					}
					if input.Email != "" {
						address, err := mail.ParseAddress(input.Email)
						if err != nil || address.Address != input.Email || len(input.Email) > 254 {
							fields["email"] = "Enter a valid email address"
						}
					}
					if utf8.RuneCountInString(input.Phone) > 30 {
						fields["phone"] = "Phone number must be 30 characters or fewer"
					}
					if len(fields) > 0 {
						reply(w, 422, map[string]any{"errors": fields})
						return
					}
					customer, err := store.CreateCustomer(r.Context(), scope, input.Name, input.Email, input.Phone)
					if err != nil {
						failure(w, err)
						return
					}
					reply(w, 201, customer)
					return
				}
				var input vehicleInput
				if !decode(w, r, &input) {
					return
				}
				fields := map[string]string{}
				if !uuid(input.CustomerID) {
					fields["customerId"] = "Choose a customer in the selected location"
				}
				if size := utf8.RuneCountInString(strings.TrimSpace(input.Description)); size < 2 || size > 120 {
					fields["description"] = "Enter a description between 2 and 120 characters"
				}
				if utf8.RuneCountInString(input.Category) > 60 {
					fields["categoryDescription"] = "Category description must be 60 characters or fewer"
				}
				if utf8.RuneCountInString(input.Identifier) > 60 {
					fields["identifier"] = "Identifier must be 60 characters or fewer"
				}
				if len(fields) > 0 {
					reply(w, 422, map[string]any{"errors": fields})
					return
				}
				vehicle, err := store.CreateVehicle(r.Context(), scope, postgres.Vehicle{CustomerID: input.CustomerID, Description: input.Description, CategoryDescription: input.Category, Identifier: input.Identifier})
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 201, vehicle)
			})
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Data changes as people work: never let a browser reuse an earlier answer (routes that serve files set their own caching).
		w.Header().Set("Cache-Control", "no-store")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || host != "127.0.0.1" {
			reply(w, 403, map[string]string{"error": "Loopback host required"})
			return
		}
		if r.URL.Path == "/api/v1/health" && r.Method == http.MethodGet {
			reply(w, 200, map[string]any{"status": "ok", "mode": "scoped-postgres-foundation", "productionReady": false, "identityProviderConfigured": login != nil})
			return
		}
		if r.URL.Path == "/api/v1/integrations/square/webhooks" || r.URL.Path == "/api/v1/integrations/square/oauth/callback" && r.Method == http.MethodGet {
			mux.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("Origin") != origin {
			reply(w, 403, map[string]string{"error": "Same-origin request required"})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/public/") {
			mux.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/auth/enroll" {
			if !strings.HasPrefix(origin, "https://") {
				reply(w, 503, map[string]string{"error": "HTTPS enrollment origin required"})
				return
			}
			enrollment.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/callback" {
			if login == nil {
				reply(w, 503, map[string]string{"error": "Identity provider is not configured"})
				return
			}
			login.ServeHTTP(w, r)
			return
		}
		headers := r.Header.Values("Authorization")
		parts := strings.Fields(r.Header.Get("Authorization"))
		var token string
		if len(headers) == 0 {
			if sessionCookie, err := r.Cookie(identity.SessionCookieName); err == nil {
				token = sessionCookie.Value
			}
		} else if len(headers) == 1 && len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = parts[1]
		}
		if token == "" {
			reply(w, 401, map[string]string{"error": "A verified workspace session is required"})
			return
		}
		actorID, err := store.SessionPrincipal(r.Context(), token)
		if errors.Is(err, postgres.ErrSession) {
			reply(w, 401, map[string]string{"error": "Session expired or revoked"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		// Administrators must have authenticator-app MFA. Until it is on they can only reach the account/security routes.
		if blocked, e := store.MFASetupBlocks(r.Context(), actorID); e != nil {
			failure(w, e)
			return
		} else if blocked && !mfaSetupAllowed(r.URL.Path) {
			reply(w, 403, map[string]string{"error": "Set up two-factor authentication to continue", "code": "mfa_setup_required"})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/organizations/") {
			restricted, e := store.EmployeeUsesRestrictedAPI(r.Context(), actorID)
			if e != nil {
				failure(w, e)
				return
			}
			if restricted {
				reply(w, 403, map[string]string{"error": "Use your assigned dealership employee workspace. Management access is not granted."})
				return
			}
		}
		recorder := &statusRecorder{ResponseWriter: w, status: 200}
		mux.ServeHTTP(recorder, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sessionActor{principalID: actorID, token: token})))
		// Every change a signed-in person makes is written to the activity log (who, when, from where, on what device).
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			method, path, status := r.Method, r.URL.Path, recorder.status
			ip, agent := postgres.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")), r.UserAgent()
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = store.RecordActivity(ctx, actorID, "", "write", method, path, status, ip, agent)
			}()
		}
	})
}
