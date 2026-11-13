package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"dapper.local/platform/internal/postgres"
)

// sendCustomerAccountSetup gives a customer a sign-in account and emails them a link to choose their password (owner 2026-11-14,
// error404 #85). With force=false (after a booking) it only emails a customer who had no account; force=true (the staff button) always
// emails a fresh 72-hour link. It answers "sent", "exists", "no_email", "not_configured" or "failed". Set up with the credential routes.
var sendCustomerAccountSetup func(ctx context.Context, org, loc, actor, customer string, force bool) (status, email string)

func setUpCustomerAccountSetup(mux *http.ServeMux, store *postgres.Store, auth *postgres.PasswordAuthenticator, sender postgres.EmailSender, origin string) {
	sendCustomerAccountSetup = func(ctx context.Context, org, loc, actor, customer string, force bool) (string, string) {
		principal, email, name, fresh, err := store.PrepareCustomerAccount(ctx, org, loc, actor, customer)
		switch {
		case errors.Is(err, postgres.ErrNoCustomerEmail):
			return "no_email", ""
		case err != nil:
			log.Printf("Customer %s: the sign-in account could not be prepared: %v", customer, err)
			return "failed", ""
		}
		if fresh {
			// A password nobody knows, so the account exists; the customer replaces it through the emailed link.
			secret := make([]byte, 24)
			if _, err = rand.Read(secret); err != nil {
				return "failed", email
			}
			if err = auth.SetPassword(ctx, principal, email, hex.EncodeToString(secret)); err != nil {
				log.Printf("Customer %s: the sign-in account could not be finished: %v", customer, err)
				return "failed", email
			}
		} else if !force {
			return "exists", email
		}
		if !emailReady(sender) {
			return "not_configured", email
		}
		token, err := auth.AccountSetupToken(ctx, principal)
		if err != nil {
			log.Printf("Customer %s: the setup link could not be made: %v", customer, err)
			return "failed", email
		}
		subject, body, text := store.RenderEmail(ctx, "account_setup", map[string]string{"name": name, "link": strings.TrimRight(origin, "/") + "/reset-password?token=" + token})
		if _, err = sender.SendEmail(ctx, email, subject, body, text); err != nil {
			log.Printf("Customer %s: the account setup email could not be sent: %v", customer, err)
			return "failed", email
		}
		return "sent", email
	}

	// Staff send (or resend) the account setup email to a customer. With sendAccountSetup in the dashboard's booking wizard, it runs
	// right after a new customer is created there.
	mux.HandleFunc("POST /api/v1/organizations/{organization}/locations/{location}/customers/{customer}/account-setup", func(w http.ResponseWriter, r *http.Request) {
		org, loc, customer := r.PathValue("organization"), r.PathValue("location"), r.PathValue("customer")
		if !uuid(org) || !uuid(loc) || !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var in struct {
			OnlyIfNew bool `json:"onlyIfNew"`
		}
		if r.ContentLength > 0 && !decode(w, r, &in) {
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "customers.create")
		if err != nil {
			failure(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 40*time.Second)
		defer cancel()
		status, email := sendCustomerAccountSetup(ctx, org, loc, scope.ActorID, customer, !in.OnlyIfNew)
		reply(w, 200, map[string]string{"status": status, "email": email})
	})
}

// accountSetupAfterWalkIn emails a walk-in customer who has no account yet, in the background so the booking answers at once.
func accountSetupAfterWalkIn(b postgres.Booking, actor string) {
	if sendCustomerAccountSetup == nil || b.CustomerID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if status, _ := sendCustomerAccountSetup(ctx, b.OrganizationID, b.LocationID, actor, b.CustomerID, false); status == "failed" || status == "not_configured" {
			log.Printf("Walk-in booking %s: customer account setup email %s", b.ID, status)
		}
	}()
}
