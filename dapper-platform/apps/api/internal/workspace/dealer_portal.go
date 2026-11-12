package workspace

import (
	"archive/zip"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
)

// The dealership portal (error404 #26–#32). Owner routes manage the portal logins; the portal's own routes (/api/v1/dealer-portal/...)
// sit outside the staff sign-in and check the portal cookie, which belongs to exactly one dealership.

const dealerCookieName = "__Host-dapper_dealer"

func registerDealerPortalRoutes(mux *http.ServeMux, store *postgres.Store) {
	// Owner: the dealership's portal logins.
	users := "/api/v1/organizations/{organization}/locations/{location}/dealerships/{dealership}/portal-users"
	ownerScope := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc, dl := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealership")
		if !uuid(org) || !uuid(loc) || !uuid(dl) || (r.PathValue("user") != "" && !uuid(r.PathValue("user"))) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, action)
		if err != nil {
			failure(w, err)
			return scope, false
		}
		return scope, true
	}
	userFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrPortalInput):
			reply(w, 422, map[string]string{"error": "Enter a name (2 to 120 characters), a valid email and a password of at least 10 characters."})
		case errors.Is(err, postgres.ErrPortalEmail):
			reply(w, 409, map[string]string{"error": "That email already has a dealership portal login."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+users, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := ownerScope(w, r, "dealership.read")
		if !ok {
			return
		}
		items, err := store.DealerPortalUsers(r.Context(), scope, r.PathValue("dealership"))
		if err != nil {
			userFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+users, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := ownerScope(w, r, "dealership.manage")
		if !ok {
			return
		}
		var in struct{ Name, Email, Password string }
		if !decode(w, r, &in) {
			return
		}
		u, err := store.CreateDealerPortalUser(r.Context(), scope, r.PathValue("dealership"), in.Name, in.Email, in.Password)
		if err != nil {
			userFailure(w, err)
			return
		}
		reply(w, 201, u)
	})
	mux.HandleFunc("PUT "+users+"/{user}/password", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := ownerScope(w, r, "dealership.manage")
		if !ok {
			return
		}
		var in struct{ Password string }
		if !decode(w, r, &in) {
			return
		}
		if err := store.SetDealerPortalPassword(r.Context(), scope, r.PathValue("user"), in.Password); err != nil {
			userFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+users+"/{user}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := ownerScope(w, r, "dealership.manage")
		if !ok {
			return
		}
		if err := store.DeleteDealerPortalUser(r.Context(), scope, r.PathValue("user")); err != nil {
			userFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})

	// The portal.
	base := "/api/v1/dealer-portal"
	wrongLogin, wrongLink := newVisitorLimiter(10, 15*time.Minute), newVisitorLimiter(20, 15*time.Minute)
	setCookie := func(w http.ResponseWriter, token string, life time.Duration) {
		http.SetCookie(w, &http.Cookie{Name: dealerCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(life.Seconds())})
	}
	who := func(w http.ResponseWriter, r *http.Request) (postgres.PortalWho, bool) {
		c, err := r.Cookie(dealerCookieName)
		if err == nil {
			var p postgres.PortalWho
			if p, err = store.DealerPortalWho(r.Context(), c.Value); err == nil {
				w.Header().Set("Cache-Control", "private, no-store")
				return p, true
			}
		}
		if err != nil && !errors.Is(err, postgres.ErrPortalSession) && !errors.Is(err, http.ErrNoCookie) {
			failure(w, err)
			return postgres.PortalWho{}, false
		}
		reply(w, 401, map[string]string{"error": "Sign in to your dealership account.", "code": "dealer_signed_out"})
		return postgres.PortalWho{}, false
	}

	mux.HandleFunc("POST "+base+"/login", func(w http.ResponseWriter, r *http.Request) {
		key := "dealerlogin:" + visitor(r)
		if wrongLogin.full(key) {
			reply(w, 429, map[string]string{"error": "Too many tries. Wait 15 minutes and try again."})
			return
		}
		var in struct{ Email, Password string }
		if !decode(w, r, &in) {
			return
		}
		token, err := store.DealerPortalSignIn(r.Context(), in.Email, in.Password)
		if errors.Is(err, postgres.ErrPortalSession) {
			wrongLogin.allow(key)
			reply(w, 401, map[string]string{"error": "That email and password do not match."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		setCookie(w, token, 7*24*time.Hour)
		reply(w, 200, map[string]bool{"signedIn": true})
	})
	// The link in an invoice email: opens the portal for up to 30 days and says which invoice it was for.
	mux.HandleFunc("POST "+base+"/link", func(w http.ResponseWriter, r *http.Request) {
		key := "dealerlink:" + visitor(r)
		if wrongLink.full(key) {
			reply(w, 429, map[string]string{"error": "Too many tries. Wait 15 minutes and open the link from your email again."})
			return
		}
		var in struct{ Token string }
		if !decode(w, r, &in) {
			return
		}
		token, invoice, err := store.DealerPortalFromLink(r.Context(), in.Token)
		if errors.Is(err, postgres.ErrPayLink) {
			wrongLink.allow(key)
			reply(w, 404, map[string]string{"error": "This link has expired or is not valid. Sign in to your dealership account instead.", "code": "link_expired"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		setCookie(w, token, 30*24*time.Hour)
		w.Header().Set("Referrer-Policy", "no-referrer")
		reply(w, 200, map[string]string{"invoiceId": invoice})
	})
	mux.HandleFunc("POST "+base+"/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(dealerCookieName); err == nil {
			_ = store.DealerPortalSignOut(r.Context(), c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: dealerCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET "+base+"/me", func(w http.ResponseWriter, r *http.Request) {
		if p, ok := who(w, r); ok {
			reply(w, 200, p)
		}
	})
	mux.HandleFunc("GET "+base+"/invoices", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		items, err := store.PortalInvoices(r.Context(), p)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	invoiceOf := func(w http.ResponseWriter, r *http.Request) (postgres.PortalWho, postgres.DealerInvoice, bool) {
		p, ok := who(w, r)
		if !ok {
			return p, postgres.DealerInvoice{}, false
		}
		id := r.PathValue("invoice")
		if !uuid(id) {
			reply(w, 404, map[string]string{"error": "Invoice not found"})
			return p, postgres.DealerInvoice{}, false
		}
		inv, err := store.DealerPortalInvoice(r.Context(), p, id)
		if err != nil {
			invoiceFailure(w, err)
			return p, inv, false
		}
		return p, inv, true
	}
	mux.HandleFunc("GET "+base+"/invoices/{invoice}", func(w http.ResponseWriter, r *http.Request) {
		if _, inv, ok := invoiceOf(w, r); ok {
			reply(w, 200, inv)
		}
	})
	mux.HandleFunc("GET "+base+"/invoices/{invoice}/pdf", func(w http.ResponseWriter, r *http.Request) {
		if _, inv, ok := invoiceOf(w, r); ok {
			name, pdf := store.InvoicePDF(r.Context(), inv.Invoice)
			replyPDF(w, name, pdf)
		}
	})
	// The ticked invoices (?ids=a,b) as one ZIP of PDFs.
	mux.HandleFunc("GET "+base+"/zip", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		if len(ids) == 0 || len(ids) > 200 {
			reply(w, 422, map[string]string{"error": "Choose 1 to 200 invoices."})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="Invoices-`+time.Now().Format("2006-01-02")+`.zip"`)
		zw := zip.NewWriter(w)
		seen := map[string]int{}
		for _, id := range ids {
			if !uuid(id) {
				continue
			}
			inv, err := store.DealerPortalInvoice(r.Context(), p, id)
			if err != nil {
				continue
			}
			name, pdf := store.InvoicePDF(r.Context(), inv.Invoice)
			if seen[name]++; seen[name] > 1 {
				name = fmt.Sprintf("%s-%d.pdf", name[:len(name)-4], seen[name])
			}
			f, err := zw.Create(name)
			if err != nil {
				break
			}
			if _, err = f.Write(pdf); err != nil {
				break
			}
		}
		_ = zw.Close()
	})
	mux.HandleFunc("GET "+base+"/payments", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		items, err := store.PortalPayments(r.Context(), p)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	statement := func(w http.ResponseWriter, r *http.Request) (postgres.PortalWho, postgres.Statement, bool) {
		p, ok := who(w, r)
		if !ok {
			return p, postgres.Statement{}, false
		}
		q := r.URL.Query()
		st, err := store.PortalStatement(r.Context(), p, q.Get("from"), q.Get("to"), q.Get("status"))
		if errors.Is(err, postgres.ErrPortalInput) {
			reply(w, 422, map[string]string{"error": "Choose a From date on or before the To date."})
			return p, st, false
		}
		if err != nil {
			failure(w, err)
			return p, st, false
		}
		return p, st, true
	}
	mux.HandleFunc("GET "+base+"/statement", func(w http.ResponseWriter, r *http.Request) {
		if _, st, ok := statement(w, r); ok {
			reply(w, 200, st)
		}
	})
	mux.HandleFunc("GET "+base+"/statement/pdf", func(w http.ResponseWriter, r *http.Request) {
		if p, st, ok := statement(w, r); ok {
			q := r.URL.Query()
			name, pdf := store.PortalStatementPDF(r.Context(), p, st, q.Get("payments") == "1", q.Get("aging") == "1")
			replyPDF(w, name, pdf)
		}
	})

	// Cards: the Square card form's settings, the saved cards, saving, default, removing.
	binding := func(w http.ResponseWriter, r *http.Request, p postgres.PortalWho) (postgres.SquareMerchantBinding, bool) {
		b, err := store.CurrentSquareMerchantBinding(r.Context(), p.Scope, "payments.record")
		if errors.Is(err, postgres.ErrSquareBinding) || (err == nil && store.SquareCredentialResolver() == nil) {
			reply(w, 503, map[string]string{"error": "Card payments are not set up for this business yet. Contact them to pay another way."})
			return b, false
		}
		if err != nil {
			failure(w, err)
			return b, false
		}
		return b, true
	}
	vaultFor := func(r *http.Request, b postgres.SquareMerchantBinding) (square.CardVault, error) {
		client, err := store.SquareCredentialResolver()(r.Context(), b.Environment, b.CredentialReference)
		if err != nil || client == nil {
			return nil, postgres.ErrPaymentProvider
		}
		v, ok := client.(square.CardVault)
		if !ok {
			return nil, postgres.ErrPaymentProvider
		}
		return v, nil
	}
	// saveCard stores a card from the card form (a one-time token) at Square for the dealership and records it here.
	saveCard := func(r *http.Request, p postgres.PortalWho, b postgres.SquareMerchantBinding, sourceID, key string) (postgres.SavedCardView, string, string, error) {
		vault, err := vaultFor(r, b)
		if err != nil {
			return postgres.SavedCardView{}, "", "", err
		}
		customer, err := store.PortalSquareCustomer(r.Context(), p, b.Environment)
		if err != nil {
			return postgres.SavedCardView{}, "", "", err
		}
		if customer == "" {
			if customer, err = vault.CreateCustomer(r.Context(), "dc-"+p.DealershipID, p.DealershipID, p.DealershipName, p.ContactEmail); err != nil {
				return postgres.SavedCardView{}, "", "", err
			}
			if err = store.SavePortalSquareCustomer(r.Context(), p, b.Environment, customer); err != nil {
				return postgres.SavedCardView{}, "", "", err
			}
			// Another request may have saved one first; use whichever is stored.
			if customer, err = store.PortalSquareCustomer(r.Context(), p, b.Environment); err != nil {
				return postgres.SavedCardView{}, "", "", err
			}
		}
		// Square idempotency keys are at most 45 characters: "dk-" + the browser's UUID (input capped at 42).
		card, err := vault.CreateCard(r.Context(), "dk-"+key, sourceID, customer)
		if err != nil {
			return postgres.SavedCardView{}, "", "", err
		}
		view, err := store.AddPortalCard(r.Context(), p, b.Environment, customer, postgres.SavedCardView{Brand: card.Brand, Last4: card.Last4, ExpMonth: card.ExpMonth, ExpYear: card.ExpYear}, card.ID)
		return view, card.ID, customer, err
	}
	cardFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, square.ErrSquareDeclined), errors.Is(err, postgres.ErrPaymentDeclined):
			reply(w, 402, map[string]string{"error": "The card was declined. Check the details or try another card."})
		case errors.Is(err, postgres.ErrPortalInput):
			reply(w, 404, map[string]string{"error": "That card was not found."})
		case errors.Is(err, square.ErrSquareScope):
			reply(w, 409, map[string]string{"error": "Saving cards is not switched on for this business yet. You can still pay with your card without saving it."})
		case errors.Is(err, square.ErrSquarePaymentFailed):
			reply(w, 502, map[string]string{"error": "Square could not save the card: " + squareReason(err)})
		case errors.Is(err, postgres.ErrPaymentProvider):
			reply(w, 502, map[string]string{"error": "The card processor did not answer. Try again in a minute."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+base+"/square", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		b, err := store.CurrentSquareMerchantBinding(r.Context(), p.Scope, "payments.record")
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 200, map[string]any{"configured": false})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationIDFor(r.Context(), p.Scope, b.Environment), "binding": map[string]any{"environment": b.Environment, "squareLocationId": b.SquareLocationID}})
	})
	mux.HandleFunc("GET "+base+"/cards", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		b, err := store.CurrentSquareMerchantBinding(r.Context(), p.Scope, "payments.record")
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 200, map[string]any{"items": []any{}, "configured": false})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.PortalCards(r.Context(), p, b.Environment)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "configured": store.SquareCredentialResolver() != nil})
	})
	mux.HandleFunc("POST "+base+"/cards", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		var in struct{ SourceID, IdempotencyKey string }
		if !decodeLimited(w, r, &in, 16*1024) {
			return
		}
		if strings.TrimSpace(in.SourceID) == "" || len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 42 {
			reply(w, 422, map[string]string{"error": "The card details are not complete."})
			return
		}
		b, ok := binding(w, r, p)
		if !ok {
			return
		}
		view, _, _, err := saveCard(r, p, b, in.SourceID, in.IdempotencyKey)
		if err != nil {
			cardFailure(w, err)
			return
		}
		reply(w, 201, view)
	})
	mux.HandleFunc("PUT "+base+"/cards/{card}/default", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		if err := store.SetPortalDefaultCard(r.Context(), p, r.PathValue("card")); err != nil {
			cardFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/cards/{card}", func(w http.ResponseWriter, r *http.Request) {
		p, ok := who(w, r)
		if !ok {
			return
		}
		sqCard, _, env, err := store.PortalCardSource(r.Context(), p, r.PathValue("card"))
		if err != nil {
			cardFailure(w, err)
			return
		}
		if err = store.RemovePortalCard(r.Context(), p, r.PathValue("card")); err != nil {
			cardFailure(w, err)
			return
		}
		// Also disable it at Square; it is already gone from the portal either way.
		if b, e := store.CurrentSquareMerchantBinding(r.Context(), p.Scope, "payments.record"); e == nil && b.Environment == env && store.SquareCredentialResolver() != nil {
			if v, e := vaultFor(r, b); e == nil {
				if e = v.DisableCard(r.Context(), sqCard); e != nil {
					log.Printf("A removed dealership card could not be disabled at Square")
				}
			}
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	// Pay an invoice's balance: with a saved card, or a new card (optionally saved first).
	// Pay several invoices at once (error404 #51): one charge per invoice from one card. A new card can be charged only once by Square,
	// so for several invoices it is saved first and each invoice is charged from the saved card. It stops at the first one that fails.
	mux.HandleFunc("POST "+base+"/pay-many", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			InvoiceIDs     []string `json:"invoiceIds"`
			SourceID       string   `json:"sourceId"`
			CardID         string   `json:"cardId"`
			IdempotencyKey string   `json:"idempotencyKey"`
		}
		if !decodeLimited(w, r, &in, 32*1024) {
			return
		}
		p, ok := who(w, r)
		if !ok {
			return
		}
		if len(in.InvoiceIDs) == 0 || len(in.InvoiceIDs) > 50 || len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 42 || (in.CardID == "") == (strings.TrimSpace(in.SourceID) == "") {
			reply(w, 422, map[string]string{"error": "Choose the invoices and a saved or new card."})
			return
		}
		invs := []postgres.DealerInvoice{}
		for _, id := range in.InvoiceIDs {
			if !uuid(id) {
				reply(w, 404, map[string]string{"error": "Invoice not found"})
				return
			}
			inv, err := store.DealerPortalInvoice(r.Context(), p, id)
			if err != nil {
				invoiceFailure(w, err)
				return
			}
			if inv.DueCents > 0 && inv.Status != "void" {
				invs = append(invs, inv)
			}
		}
		if len(invs) == 0 {
			reply(w, 422, map[string]string{"error": "Those invoices have nothing left to pay."})
			return
		}
		waitOnSquare(w)
		b, ok := binding(w, r, p)
		if !ok {
			return
		}
		var source, customer string
		var err error
		switch {
		case in.CardID != "":
			var env string
			if source, customer, env, err = store.PortalCardSource(r.Context(), p, in.CardID); err == nil && env != b.Environment {
				err = postgres.ErrPortalInput
			}
		case len(invs) == 1:
			source = in.SourceID
		default:
			_, source, customer, err = saveCard(r, p, b, in.SourceID, in.IdempotencyKey)
			if errors.Is(err, square.ErrSquareScope) {
				reply(w, 409, map[string]string{"error": "Paying several invoices with a new card needs the card saved, and saving cards is not switched on for this business yet. Pay each invoice on its own instead."})
				return
			}
		}
		if err != nil {
			cardFailure(w, err)
			return
		}
		type result struct {
			InvoiceID     string `json:"invoiceId"`
			InvoiceNumber string `json:"invoiceNumber"`
			AmountCents   int64  `json:"amountCents"`
			Paid          bool   `json:"paid"`
			Error         string `json:"error,omitempty"`
		}
		results := []result{}
		var paid int64
		for _, inv := range invs {
			store.ReleaseStuckSquareIntents(r.Context(), p.Scope, inv.ID, store.SquareCredentialResolver())
			attempt, e := store.PrepareSquareInvoiceIntent(r.Context(), p.Scope, postgres.PrepareSquareInvoiceIntentInput{InvoiceID: inv.ID, BindingVersion: b.Version, RequestKey: "dealer-many-" + in.IdempotencyKey + "-" + inv.ID[:8], AmountCents: inv.DueCents})
			if e == nil {
				_, e = store.ExecuteSquareInvoiceIntentFor(r.Context(), p.Scope, attempt.ID, source, customer, store.SquareCredentialResolver())
			}
			res := result{InvoiceID: inv.ID, InvoiceNumber: inv.InvoiceNumber, AmountCents: inv.DueCents, Paid: e == nil}
			if e != nil {
				switch {
				case errors.Is(e, postgres.ErrPaymentDeclined):
					res.Error = "The card was declined."
				case errors.Is(e, square.ErrSquareRejected):
					res.Error = "Square refused the payment: " + squareReason(e)
				case errors.Is(e, postgres.ErrPaymentOverpay):
					res.Error = "An earlier card payment is still being confirmed for this invoice."
				default:
					res.Error = "The card processor did not answer. Nothing more was charged; try again in a minute."
				}
				results = append(results, res)
				break
			}
			paid += inv.DueCents
			results = append(results, res)
		}
		reply(w, 200, map[string]any{"results": results, "paidCents": paid})
	})
	mux.HandleFunc("POST "+base+"/invoices/{invoice}/pay", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SourceID       string `json:"sourceId"`
			CardID         string `json:"cardId"`
			SaveCard       bool   `json:"saveCard"`
			AmountCents    int64  `json:"amountCents"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if !decodeLimited(w, r, &in, 16*1024) {
			return
		}
		p, inv, ok := invoiceOf(w, r)
		if !ok {
			return
		}
		if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 42 || (in.CardID == "") == (strings.TrimSpace(in.SourceID) == "") {
			reply(w, 422, map[string]string{"error": "Choose a saved card or enter a card."})
			return
		}
		waitOnSquare(w)
		b, ok := binding(w, r, p)
		if !ok {
			return
		}
		source, customer := in.SourceID, ""
		var err error
		cardNotSaved, notSavedWhy := false, ""
		switch {
		case in.CardID != "":
			var env string
			if source, customer, env, err = store.PortalCardSource(r.Context(), p, in.CardID); err == nil && env != b.Environment {
				err = postgres.ErrPortalInput
			}
		case in.SaveCard:
			var saved, owner string
			if _, saved, owner, err = saveCard(r, p, b, in.SourceID, in.IdempotencyKey); err == nil {
				source, customer = saved, owner
			} else if !errors.Is(err, square.ErrSquareDeclined) {
				// Saving failed but the card itself was not refused: still take the payment with it, unsaved (error404 #42).
				log.Printf("Dealership card could not be saved, paying without saving: %v", err)
				cardNotSaved, notSavedWhy, err = true, squareReason(err), nil
			}
		}
		if err != nil {
			cardFailure(w, err)
			return
		}
		// An earlier attempt whose answer never came back must not keep blocking this invoice: check it with Square first.
		checked := store.ReleaseStuckSquareIntents(r.Context(), p.Scope, inv.ID, store.SquareCredentialResolver())
		attempt, err := store.PrepareSquareInvoiceIntent(r.Context(), p.Scope, postgres.PrepareSquareInvoiceIntentInput{InvoiceID: inv.ID, BindingVersion: b.Version, RequestKey: "dealer-portal-" + in.IdempotencyKey, AmountCents: in.AmountCents})
		if err == nil {
			_, err = store.ExecuteSquareInvoiceIntentFor(r.Context(), p.Scope, attempt.ID, source, customer, store.SquareCredentialResolver())
		}
		if err == nil {
			inv, err = store.DealerPortalInvoice(r.Context(), p, inv.ID)
		}
		switch {
		case err == nil:
			reply(w, 201, map[string]any{"payment": map[string]any{"amountCents": in.AmountCents}, "invoice": inv, "cardNotSaved": cardNotSaved, "cardNotSavedReason": notSavedWhy})
		case errors.Is(err, postgres.ErrPaymentDeclined):
			reply(w, 402, map[string]string{"error": "The card was declined. Check the details or try another card."})
		case errors.Is(err, square.ErrSquareRejected):
			reply(w, 422, map[string]string{"error": "Square refused the payment: " + squareReason(err) + " Nothing was charged."})
		case errors.Is(err, postgres.ErrSquareIntentState):
			reply(w, 409, map[string]string{"error": "This payment is already being processed."})
		case errors.Is(err, postgres.ErrInvoiceStatusConflict):
			reply(w, 409, map[string]string{"error": "This invoice cannot be paid in its current status."})
		case errors.Is(err, postgres.ErrPaymentOverpay):
			// Say what holds the balance, so a stuck earlier attempt can be seen and sorted out.
			msg := "The amount is more than the balance owing."
			if held := store.HeldSquareIntents(r.Context(), p.Scope, inv.ID, true); len(held) > 0 {
				msg += " An earlier card payment is still holding it: " + strings.Join(held, "; ") + "."
			}
			if len(checked) > 0 {
				msg += " Checked with Square: " + strings.Join(checked, "; ") + ". Ask the business to check it in Square and release it if nothing was charged."
			}
			reply(w, 422, map[string]string{"error": msg})
		case errors.Is(err, postgres.ErrPaymentInput):
			reply(w, 422, map[string]string{"error": "The payment details are not valid. Reload the page and try again."})
		case errors.Is(err, postgres.ErrPaymentRequest):
			reply(w, 409, map[string]string{"error": "This payment was already sent. Reload the page to see the balance."})
		case errors.Is(err, postgres.ErrPaymentProvider):
			reply(w, 502, map[string]string{"error": "The card processor did not answer. Try again in a minute."})
		default:
			failure(w, err)
		}
	})
}

// squareReason is what Square said, for the person in front of the card form (error404 follow-up 2026-11-12: cards were not saving and
// the cause was hidden). Square's messages describe the request, never the card number.
func squareReason(err error) string {
	switch {
	case errors.Is(err, square.ErrSquareScope):
		return "this business's Square connection does not allow saving cards yet (the owner reconnects Square to allow it)."
	case errors.Is(err, postgres.ErrPaymentProvider):
		return "the card service is not set up for this business."
	}
	why := err.Error()
	if i := strings.LastIndex(why, square.ErrSquarePaymentFailed.Error()+":"); i >= 0 {
		why = why[i+len(square.ErrSquarePaymentFailed.Error())+1:]
	}
	why = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(why), "square refused the request:"))
	if why == "" || len(why) > 300 {
		return "Square did not accept the request. Try again in a minute."
	}
	return why
}
