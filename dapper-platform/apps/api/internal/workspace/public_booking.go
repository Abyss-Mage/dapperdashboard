package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A small per-visitor limiter (process-local, like the sign-in throttle). The gateway appends the real client address to
// X-Forwarded-For, so the last entry is the one the gateway saw.
type visitorLimiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	hits   map[string][]time.Time
}

func newVisitorLimiter(max int, window time.Duration) *visitorLimiter {
	return &visitorLimiter{window: window, max: max, hits: map[string][]time.Time{}}
}
func (l *visitorLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	if len(l.hits) > 5000 { // keep memory bounded
		for k, v := range l.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	return true
}
func visitor(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func registerPublicBookingRoutes(mux *http.ServeMux, store *postgres.Store) {
	browse := newVisitorLimiter(240, time.Minute)
	book := newVisitorLimiter(6, 10*time.Minute)
	sendCode := newVisitorLimiter(5, time.Hour)
	checkCode := newVisitorLimiter(20, time.Hour)
	base := "/api/v1/public/booking/{site}"
	limited := func(w http.ResponseWriter, r *http.Request, l *visitorLimiter) bool {
		if l.allow(visitor(r)) {
			return false
		}
		w.Header().Set("Retry-After", "600")
		reply(w, 429, map[string]string{"error": "Too many tries from this connection. Please wait a few minutes and try again."})
		return true
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrPublicSite):
			reply(w, 404, map[string]string{"error": "Online booking is not available right now."})
		case errors.Is(err, postgres.ErrPublicBookingInput), errors.Is(err, postgres.ErrBookingInput), errors.Is(err, postgres.ErrAvailabilityInput), errors.Is(err, postgres.ErrSchedulingRule), errors.Is(err, postgres.ErrSchedulingRuleConflict):
			reply(w, 422, map[string]string{"error": "Please check your choices and try again."})
		case errors.As(err, new(postgres.CouponInvalid)):
			reply(w, 422, map[string]string{"error": err.Error()})
		case errors.Is(err, postgres.ErrPublicBookingPrice):
			reply(w, 422, map[string]string{"error": "This service needs a quote. Please contact us to book it."})
		case errors.Is(err, postgres.ErrPhoneVerificationUnavailable):
			reply(w, 503, map[string]string{"error": "Phone checks are not available yet. You can still book as a new customer."})
		case errors.Is(err, postgres.ErrPhoneVerification):
			reply(w, 422, map[string]string{"error": "That code is wrong or has expired."})
		case errors.Is(err, postgres.ErrPhoneVerificationLimit):
			reply(w, 429, map[string]string{"error": "Too many tries. Please wait a while and try again."})
		case errors.Is(err, postgres.ErrTaxInclusiveUnsupported):
			reply(w, 503, map[string]string{"error": "Online booking is not available right now."})
		default:
			// The time may have just been taken; everything else is an internal error with no details for visitors.
			if isCapacityConflict(err) {
				reply(w, 409, map[string]string{"error": "That time was just taken. Please pick another time."})
				return
			}
			reply(w, 500, map[string]string{"error": "Something went wrong on our side. Nothing was charged. Please try again."})
		}
	}
	mux.HandleFunc("GET "+base+"/options", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, browse) {
			return
		}
		out, err := store.PublicBookingOptions(r.Context(), r.PathValue("site"))
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, out)
	})
	// Only what the card form needs: public identifiers, never credentials.
	mux.HandleFunc("GET "+base+"/square", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, browse) {
			return
		}
		scope, err := store.PublicBookingScope(r.Context(), r.PathValue("site"))
		if err != nil {
			fail(w, err)
			return
		}
		b, err := store.CurrentSquareMerchantBinding(r.Context(), scope, "payments.record")
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 200, map[string]any{"configured": false})
			return
		}
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationID(b.Environment), "binding": map[string]any{"environment": b.Environment, "squareLocationId": b.SquareLocationID}})
	})
	mux.HandleFunc("GET "+base+"/availability", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, browse) {
			return
		}
		q := r.URL.Query()
		version, e1 := strconv.Atoi(q.Get("version"))
		days, e2 := strconv.Atoi(q.Get("days"))
		if !uuid(q.Get("resourceId")) || !uuid(q.Get("ruleId")) || e1 != nil || e2 != nil || version < 1 || days < 1 || days > 31 || len(q.Get("from")) != 10 {
			reply(w, 400, map[string]string{"error": "Valid service, start day and number of days (1-31) required"})
			return
		}
		out, err := store.PublicAvailability(r.Context(), r.PathValue("site"), q.Get("resourceId"), q.Get("ruleId"), version, q.Get("from"), days)
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, out)
	})
	mux.HandleFunc("POST "+base+"/verify/start", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, sendCode) {
			return
		}
		var in struct {
			Phone string `json:"phone"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := store.StartPhoneVerification(r.Context(), r.PathValue("site"), in.Phone); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "sent"})
	})
	mux.HandleFunc("POST "+base+"/verify/check", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, checkCode) {
			return
		}
		var in struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if !decode(w, r, &in) {
			return
		}
		token, err := store.CheckPhoneVerification(r.Context(), r.PathValue("site"), in.Phone, in.Code)
		if err != nil {
			fail(w, err)
			return
		}
		items, err := store.ReturningCustomers(r.Context(), r.PathValue("site"), in.Phone, token)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"token": token, "customers": items})
	})
	mux.HandleFunc("POST "+base+"/book", func(w http.ResponseWriter, r *http.Request) {
		if limited(w, r, book) {
			return
		}
		var in struct {
			postgres.PublicBookingInput
			DepositMethod string `json:"depositMethod"` // "card" or "cash"
			SourceID      string `json:"sourceId"`      // the Square single-use card token
		}
		if !decode(w, r, &in) {
			return
		}
		site := r.PathValue("site")
		if in.DepositMethod != "card" && in.DepositMethod != "cash" || in.DepositMethod == "card" && (len(in.SourceID) < 8 || len(in.SourceID) > 200) {
			reply(w, 422, map[string]string{"error": "Choose how to pay the deposit."})
			return
		}
		res, err := store.PublicBook(r.Context(), site, in.PublicBookingInput)
		if err != nil {
			fail(w, err)
			return
		}
		paid := false
		if in.DepositMethod == "card" {
			scope, err := store.PublicBookingScope(r.Context(), site)
			if err == nil {
				err = takeSquareDeposit(r, store, scope, res.InvoiceID, "dep-"+in.RequestKey, in.SourceID, res.DepositCents)
			}
			if err != nil {
				store.PublicBookingAbort(r.Context(), site, res.BookingID, res.InvoiceID)
				reply(w, 402, map[string]string{"error": depositMessage(err)})
				return
			}
			paid = true
		}
		reply(w, 201, map[string]any{"bookingId": res.BookingID, "invoiceNumber": res.InvoiceNumber, "totalCents": res.TotalCents, "depositCents": res.DepositCents, "depositPaid": paid, "serviceName": res.ServiceName})
	})
}

func isCapacityConflict(err error) bool {
	return errors.Is(err, postgres.ErrBookingReservation) || errors.Is(err, postgres.ErrBookingCapacityRequest)
}

func depositMessage(err error) string {
	switch {
	case errors.Is(err, postgres.ErrSquareBinding):
		return "Card payments are not available yet. Please choose to pay on the day."
	case errors.Is(err, postgres.ErrPaymentProvider):
		return "We could not confirm the card payment, so the booking was not kept. Please check your card and try again."
	default:
		return "The deposit could not be taken, so the booking was not kept. Please try again or pay on the day."
	}
}

// takeSquareDeposit charges the deposit against the invoice through the same prepare + execute path staff payments use.
func takeSquareDeposit(r *http.Request, store *postgres.Store, scope postgres.Scope, invoice, requestKey, sourceID string, amount int64) error {
	resolve := store.SquareCredentialResolver()
	if resolve == nil {
		return postgres.ErrSquareBinding
	}
	binding, err := store.CurrentSquareMerchantBinding(r.Context(), scope, "payments.record")
	if err != nil {
		return err
	}
	attempt, err := store.PrepareSquareInvoiceIntent(r.Context(), scope, postgres.PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: binding.Version, RequestKey: requestKey, AmountCents: amount})
	if err != nil {
		return err
	}
	_, err = store.ExecuteSquareInvoiceIntent(r.Context(), scope, attempt.ID, sourceID, resolve)
	return err
}
