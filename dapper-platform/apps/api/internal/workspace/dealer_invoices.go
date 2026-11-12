package workspace

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
)

// Dealership (B2B) invoices: drafts with item lines, dates, PO number and discount, issued into the dealership number series (or a typed
// number) and emailed with a private link to view and pay; the owner's list with filters, and the filtered set as one ZIP of PDFs.
func registerDealerInvoiceRoutes(mux *http.ServeMux, store *postgres.Store, emailSender postgres.EmailSender, origin string) {
	base := "/api/v1/organizations/{organization}/locations/{location}/dealer-invoices"
	scopeFor := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) || (r.PathValue("invoice") != "" && !uuid(r.PathValue("invoice"))) {
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
	fail := func(w http.ResponseWriter, err error) {
		if errors.Is(err, postgres.ErrDealerInvoiceInput) {
			reply(w, 422, map[string]string{"error": "Check the invoice: choose an active dealership and add at least one item (2 to 255 characters, quantity 1 or more). The due date must be on or after the invoice date and within a year; the discount cannot be more than the subtotal; an invoice number is letters, numbers, spaces and . / # - (up to 40); a PO number up to 40 characters."})
			return
		}
		if errors.Is(err, postgres.ErrDealerNumberTaken) {
			reply(w, 409, map[string]string{"error": "That invoice number is already used. Type another, or leave it empty to use the next number."})
			return
		}
		invoiceFailure(w, err)
	}
	filterOf := func(r *http.Request) postgres.DealerInvoiceFilter {
		q := r.URL.Query()
		return postgres.DealerInvoiceFilter{DealershipID: q.Get("dealershipId"), From: q.Get("from"), To: q.Get("to"), Status: q.Get("status")}
	}

	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		items, err := store.ListDealerInvoices(r.Context(), scope, filterOf(r))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.create")
		if !ok {
			return
		}
		var input postgres.DealerInvoiceInput
		if !decodeLimited(w, r, &input, 256<<10) {
			return
		}
		inv, err := store.CreateDealerInvoice(r.Context(), scope, input)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 201, inv)
	})
	// The editor's running totals with the franchise's tax, while typing (error404 #24).
	mux.HandleFunc("POST "+base+"/preview", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		var input postgres.DealerInvoiceInput
		if !decodeLimited(w, r, &input, 256<<10) {
			return
		}
		out, err := store.PreviewDealerInvoice(r.Context(), scope, input)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	// Back to a draft with the same number, to edit it (error404 #23); only while nothing has been paid.
	mux.HandleFunc("POST "+base+"/{invoice}/reopen", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.manage")
		if !ok {
			return
		}
		inv, err := store.ReopenDealerInvoice(r.Context(), scope, r.PathValue("invoice"))
		if errors.Is(err, postgres.ErrInvoiceStatusConflict) {
			reply(w, 409, map[string]string{"error": "Only an unpaid issued invoice can be edited. Once something is paid, void it and make a new one."})
			return
		}
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, inv)
	})
	// Card attempts still holding an invoice's balance, and the owner's release of them after checking Square.
	mux.HandleFunc("GET "+base+"/{invoice}/held-payments", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		items := store.HeldSquareIntents(r.Context(), scope, r.PathValue("invoice"), false)
		if items == nil {
			items = []string{}
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base+"/{invoice}/release-held-payments", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.manage")
		if !ok {
			return
		}
		n, err := store.ReleaseHeldSquareIntents(r.Context(), scope, r.PathValue("invoice"))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]int{"released": n})
	})
	mux.HandleFunc("GET "+base+"/{invoice}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		inv, err := store.GetDealerInvoice(r.Context(), scope, r.PathValue("invoice"))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, inv)
	})
	mux.HandleFunc("PUT "+base+"/{invoice}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.create")
		if !ok {
			return
		}
		var input postgres.DealerInvoiceInput
		if !decodeLimited(w, r, &input, 256<<10) {
			return
		}
		inv, err := store.UpdateDealerInvoice(r.Context(), scope, r.PathValue("invoice"), input)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, inv)
	})
	send := func(w http.ResponseWriter, r *http.Request, scope postgres.Scope, inv postgres.DealerInvoice) {
		// The quick checks answer now; the PDF and the mail server run in the background, so the popup never waits on them (a slow
		// mail server outlasted the server's 10-second reply limit and left "Issue and email" stuck on "Saving…").
		status, to, why := "not_configured", "", ""
		if emailReady(emailSender) {
			var err error
			if to, _, err = store.DealerContact(r.Context(), scope, inv.DealershipID); err != nil {
				status, why = "failed", "the dealership's contact email could not be read ("+err.Error()+")"
				log.Printf("Dealership invoice %s: %s", inv.ID, why)
			} else if to == "" {
				status = "no_email"
			} else {
				status = "sending"
				go func() {
					// Not tied to the request: staff leaving the page must not cut the email off half way.
					mailCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
					defer cancel()
					if s, _ := emailDealerInvoice(mailCtx, store, emailSender, origin, scope, inv); s != "sent" {
						log.Printf("Dealership invoice %s: email to the dealership ended as %s", inv.ID, s)
					}
				}()
			}
		}
		reply(w, 200, struct {
			postgres.DealerInvoice
			EmailStatus string `json:"emailStatus"`
			EmailedTo   string `json:"emailedTo,omitempty"`
			EmailError  string `json:"emailError,omitempty"`
		}{inv, status, to, why})
	}
	mux.HandleFunc("POST "+base+"/{invoice}/issue", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.issue")
		if !ok {
			return
		}
		id := r.PathValue("invoice")
		if _, err := store.GetDealerInvoice(r.Context(), scope, id); err != nil {
			fail(w, err)
			return
		}
		if _, err := store.IssueInvoice(r.Context(), scope, id); err != nil {
			fail(w, err)
			return
		}
		inv, err := store.GetDealerInvoice(r.Context(), scope, id)
		if err != nil {
			fail(w, err)
			return
		}
		send(w, r, scope, inv)
	})
	mux.HandleFunc("POST "+base+"/{invoice}/email", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.issue")
		if !ok {
			return
		}
		inv, err := store.GetDealerInvoice(r.Context(), scope, r.PathValue("invoice"))
		if err != nil {
			fail(w, err)
			return
		}
		if inv.Status == "draft" || inv.Status == "void" {
			reply(w, 409, map[string]string{"error": "Only an issued invoice can be emailed."})
			return
		}
		send(w, r, scope, inv)
	})
	mux.HandleFunc("POST "+base+"/{invoice}/void", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.manage")
		if !ok {
			return
		}
		id := r.PathValue("invoice")
		if _, err := store.GetDealerInvoice(r.Context(), scope, id); err != nil {
			fail(w, err)
			return
		}
		if _, err := store.VoidInvoice(r.Context(), scope, id, "Voided from dealership invoices"); err != nil {
			fail(w, err)
			return
		}
		inv, err := store.GetDealerInvoice(r.Context(), scope, id)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, inv)
	})
	mux.HandleFunc("GET "+base+"/{invoice}/pdf", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		inv, err := store.GetDealerInvoice(r.Context(), scope, r.PathValue("invoice"))
		if err != nil {
			fail(w, err)
			return
		}
		name, pdf := store.InvoicePDF(r.Context(), inv.Invoice)
		replyPDF(w, name, pdf)
	})
	// Every invoice the filters show, as one ZIP of PDFs (at most 500; narrow the filters for more).
	mux.HandleFunc("GET "+base+"/zip", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		items, err := store.ListDealerInvoices(r.Context(), scope, filterOf(r))
		if err != nil {
			fail(w, err)
			return
		}
		if len(items) == 0 {
			reply(w, 404, map[string]string{"error": "No invoices match these filters."})
			return
		}
		if len(items) > 500 {
			reply(w, 413, map[string]string{"error": "More than 500 invoices match. Narrow the dates or pick one dealership, then download again."})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="Dealership-invoices-`+time.Now().Format("2006-01-02")+`.zip"`)
		w.Header().Set("Cache-Control", "private, no-store")
		zw := zip.NewWriter(w)
		seen := map[string]int{}
		for _, it := range items {
			inv, err := store.GetDealerInvoice(r.Context(), scope, it.ID)
			if err != nil {
				log.Printf("Dealership invoice %s left out of the ZIP: %v", it.ID, err)
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

	series := "/api/v1/organizations/{organization}/dealer-invoice-series"
	mux.HandleFunc("GET "+series, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		out, err := store.DealerSeries(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+series, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input postgres.DealerInvoiceSeries
		if !decode(w, r, &input) {
			return
		}
		out, err := store.SaveDealerSeries(r.Context(), requestSession(r).principalID, org, input)
		if errors.Is(err, postgres.ErrDealerInvoiceInput) {
			reply(w, 422, map[string]string{"error": "The prefix is capital letters, numbers and dashes (up to 24); the next number is 1 or more."})
			return
		}
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
}

// emailDealerInvoice sends the dealership's contact the invoice PDF with a link to view and pay it in their account, and says what
// happened ("sent", "no_email", "not_configured" or "failed"). The invoice is already issued, so a failed email never undoes it.
func emailDealerInvoice(ctx context.Context, store *postgres.Store, sender postgres.EmailSender, origin string, scope postgres.Scope, inv postgres.DealerInvoice) (status, to string) {
	if !emailReady(sender) {
		return "not_configured", ""
	}
	to, name, err := store.DealerContact(ctx, scope, inv.DealershipID)
	if err != nil {
		log.Printf("Dealership invoice %s: the dealership's contact could not be read", inv.ID)
		return "failed", ""
	}
	if to == "" {
		return "no_email", ""
	}
	due := "on receipt"
	if inv.DueDate != "" {
		if t, e := time.Parse("2006-01-02", inv.DueDate); e == nil {
			due = t.Format("January 2, 2006")
		}
	} else if inv.DueAt != nil {
		due = inv.DueAt.In(documentZone()).Format("January 2, 2006")
	}
	vars := map[string]string{"name": name, "number": inv.InvoiceNumber, "total": fmt.Sprintf("$%.2f", float64(inv.TotalCents)/100), "due_date": due, "link": ""}
	token, err := store.CreateInvoicePayLink(ctx, scope, inv.ID)
	if err != nil {
		log.Printf("Dealership invoice %s: the pay link could not be made: %v", inv.ID, err)
		return "failed", to
	}
	vars["link"] = strings.TrimRight(origin, "/") + "/pay/" + token
	subject, body, text := store.RenderEmail(ctx, "dealer_invoice_issued", vars)
	if s, ok := sender.(interface {
		SendEmailWithAttachment(context.Context, string, string, string, string, *email.Attachment) (string, error)
	}); ok {
		file, pdf := store.InvoicePDF(ctx, inv.Invoice)
		_, err = s.SendEmailWithAttachment(ctx, to, subject, body, text, &email.Attachment{Name: file, ContentType: "application/pdf", Data: pdf})
	} else {
		_, err = sender.SendEmail(ctx, to, subject, body, text)
	}
	if err != nil {
		log.Printf("Dealership invoice %s could not be emailed: %v", inv.ID, err)
		return "failed", to
	}
	return "sent", to
}

func documentZone() *time.Location {
	zone, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return time.UTC
	}
	return zone
}
