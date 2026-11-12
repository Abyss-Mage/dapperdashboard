package workspace

import (
	"errors"
	"log"
	"net/http"
	"time"

	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
)

// The shop iPad's own routes (error404 #20, owner decisions 2026-11-11). They sit outside the normal sign-in: the iPad signs in with the
// franchise's shop username and password (its own cookie), lists the workers, and a worker's 6-digit PIN opens their own dashboard
// session there. Wrong passwords and PINs are limited per visitor, per worker and per iPad.
// shortcut: the limits live in this process (like the sign-in throttle), so a restart clears them; move them to the database if more than one API process ever runs.

const shopCookieName = "__Host-dapper_shop"

// shopWorkerMark is readable by the page so the app shows the shop bar (clock buttons, Done) while a worker uses the iPad. It grants nothing.
const shopWorkerMark = "dapper_shop_worker"

func registerShopDeviceRoutes(mux *http.ServeMux, store *postgres.Store) {
	wrongLogin := newVisitorLimiter(10, 15*time.Minute)
	wrongPinWorker, wrongPinDevice := newVisitorLimiter(5, 15*time.Minute), newVisitorLimiter(30, 15*time.Minute)
	base := "/api/v1/shop"

	device := func(w http.ResponseWriter, r *http.Request) (postgres.ShopDevice, bool) {
		c, err := r.Cookie(shopCookieName)
		if err == nil {
			var d postgres.ShopDevice
			if d, err = store.ShopDeviceOf(r.Context(), c.Value); err == nil {
				return d, true
			}
		}
		if err != nil && !errors.Is(err, postgres.ErrShopSession) && !errors.Is(err, http.ErrNoCookie) {
			failure(w, err)
			return postgres.ShopDevice{}, false
		}
		reply(w, 401, map[string]string{"error": "This iPad is not signed in to a shop.", "code": "shop_signed_out"})
		return postgres.ShopDevice{}, false
	}
	// clockDevice is the clock page's device: the shop login, or else anyone signed in to the workspace (their own franchise; owner,
	// 2026-11-12). viaShop says which, since only the shop iPad ends a worker's session when the names are shown.
	clockDevice := func(w http.ResponseWriter, r *http.Request) (d postgres.ShopDevice, viaShop, ok bool) {
		if c, err := r.Cookie(shopCookieName); err == nil {
			if d, err = store.ShopDeviceOf(r.Context(), c.Value); err == nil {
				return d, true, true
			} else if !errors.Is(err, postgres.ErrShopSession) {
				failure(w, err)
				return d, false, false
			}
		}
		if c, err := r.Cookie(identity.SessionCookieName); err == nil {
			if d, err = store.ShopDeviceForStaff(r.Context(), c.Value); err == nil {
				return d, false, true
			} else if !errors.Is(err, postgres.ErrShopSession) {
				failure(w, err)
				return d, false, false
			}
		}
		reply(w, 401, map[string]string{"error": "This iPad is not signed in to a shop.", "code": "shop_signed_out"})
		return d, false, false
	}
	workerCookie := func(r *http.Request) string {
		if c, err := r.Cookie(identity.SessionCookieName); err == nil {
			return c.Value
		}
		return ""
	}
	endWorker := func(w http.ResponseWriter, r *http.Request) {
		if token := workerCookie(r); token != "" {
			_ = store.ShopRelease(r.Context(), token)
		}
		// Only a session the iPad opened is ended above; the cookies go either way, since this device is the shop's.
		http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: shopWorkerMark, Value: "", Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}

	mux.HandleFunc("POST "+base+"/login", func(w http.ResponseWriter, r *http.Request) {
		key := "shoplogin:" + visitor(r)
		if wrongLogin.full(key) {
			reply(w, 429, map[string]string{"error": "Too many tries. Wait 15 minutes and try again."})
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(w, r, &input) {
			return
		}
		token, err := store.ShopSignIn(r.Context(), input.Username, input.Password)
		if errors.Is(err, postgres.ErrShopSession) {
			wrongLogin.allow(key)
			reply(w, 401, map[string]string{"error": "That shop username and password do not match."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{Name: shopCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int((90 * 24 * time.Hour).Seconds())})
		// The shop sees every workspace page (owner, 2026-11-12): the workspace opens as the franchise's Shop account.
		if d, e := store.ShopDeviceOf(r.Context(), token); e == nil {
			if ws, e := store.ShopWorkspaceSession(r.Context(), d); e == nil {
				setWorkspaceCookie(w, ws)
			} else {
				log.Printf("The shop workspace could not be opened: %v", e)
			}
		}
		reply(w, 200, map[string]bool{"signedIn": true})
	})
	mux.HandleFunc("POST "+base+"/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(shopCookieName); err == nil {
			_ = store.ShopSignOut(r.Context(), c.Value)
		}
		endWorker(w, r)
		http.SetCookie(w, &http.Cookie{Name: shopCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(204)
	})
	// The name list. Opening it also ends any worker session left open on the iPad, so the next person never lands in someone else's dashboard.
	mux.HandleFunc("GET "+base+"/workers", func(w http.ResponseWriter, r *http.Request) {
		d, viaShop, ok := clockDevice(w, r)
		if !ok {
			return
		}
		// The shop iPad's own name list (?release=1) ends any worker left signed in there; the clock page inside a dashboard does not.
		if viaShop && r.URL.Query().Get("release") == "1" {
			endWorker(w, r)
		}
		workers, err := store.ShopWorkers(r.Context(), d)
		if err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"name": d.Name, "workers": workers, "shopLogin": viaShop})
	})
	mux.HandleFunc("POST "+base+"/workers/{principal}/unlock", func(w http.ResponseWriter, r *http.Request) {
		d, ok := device(w, r)
		if !ok {
			return
		}
		principal := r.PathValue("principal")
		var input struct {
			Pin string `json:"pin"`
		}
		if !uuid(principal) || !decode(w, r, &input) {
			if !uuid(principal) {
				reply(w, 400, map[string]string{"error": "Choose your name."})
			}
			return
		}
		who, where := "pin:"+d.LocationID+":"+principal, "pin:"+d.LocationID
		if wrongPinWorker.full(who) || wrongPinDevice.full(where) {
			reply(w, 429, map[string]string{"error": "Too many wrong PINs. Wait 15 minutes, or ask the owner to check your PIN."})
			return
		}
		token, err := store.ShopUnlock(r.Context(), d, principal, input.Pin)
		if errors.Is(err, postgres.ErrClockPinUnknown) {
			wrongPinWorker.allow(who)
			wrongPinDevice.allow(where)
			reply(w, 401, map[string]string{"error": "That PIN is not right. Try again."})
			return
		}
		if errors.Is(err, postgres.ErrShopManagerPin) {
			reply(w, 403, map[string]string{"error": "Owners and administrators sign in with their own login, not a shop PIN."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		life := int((30 * time.Minute).Seconds())
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: life})
		http.SetCookie(w, &http.Cookie{Name: shopWorkerMark, Value: "1", Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: life})
		reply(w, 200, map[string]bool{"unlocked": true})
	})
	// The shop iPad asks for a new Shop workspace session when the last one (24 hours at most) has ended.
	mux.HandleFunc("POST "+base+"/session", func(w http.ResponseWriter, r *http.Request) {
		d, ok := device(w, r)
		if !ok {
			return
		}
		ws, err := store.ShopWorkspaceSession(r.Context(), d)
		if errors.Is(err, postgres.ErrShopSession) {
			reply(w, 401, map[string]string{"error": "This iPad is not signed in to a shop.", "code": "shop_signed_out"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		setWorkspaceCookie(w, ws)
		w.WriteHeader(204)
	})
	// Availability on the shop iPad: name + PIN, then that worker's week (POST reads it, PUT submits it).
	availability := func(w http.ResponseWriter, r *http.Request, write bool) {
		d, _, ok := clockDevice(w, r)
		if !ok {
			return
		}
		principal := r.PathValue("principal")
		var input struct {
			Pin       string                          `json:"pin"`
			Week      string                          `json:"week"`
			WeekStart string                          `json:"weekStart"`
			Intervals []postgres.AvailabilityInterval `json:"intervals"`
			Notes     string                          `json:"notes"`
			Version   int                             `json:"version"`
		}
		if !uuid(principal) || !decode(w, r, &input) {
			if !uuid(principal) {
				reply(w, 400, map[string]string{"error": "Choose your name."})
			}
			return
		}
		who, where := "pin:"+d.LocationID+":"+principal, "pin:"+d.LocationID
		if wrongPinWorker.full(who) || wrongPinDevice.full(where) {
			reply(w, 429, map[string]string{"error": "Too many wrong PINs. Wait 15 minutes, or ask the owner to check your PIN."})
			return
		}
		var in *postgres.EmployeeAvailability
		if write {
			in = &postgres.EmployeeAvailability{WeekStart: input.WeekStart, Intervals: input.Intervals, Notes: input.Notes, Version: input.Version}
		}
		data, err := store.ShopAvailability(r.Context(), d, principal, input.Pin, input.Week, in)
		if errors.Is(err, postgres.ErrClockPinUnknown) {
			wrongPinWorker.allow(who)
			wrongPinDevice.allow(where)
			reply(w, 401, map[string]string{"error": "That PIN is not right. Try again."})
			return
		}
		if err != nil {
			employeeFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, data)
	}
	mux.HandleFunc("POST "+base+"/workers/{principal}/availability", func(w http.ResponseWriter, r *http.Request) { availability(w, r, false) })
	mux.HandleFunc("PUT "+base+"/workers/{principal}/availability", func(w http.ResponseWriter, r *http.Request) { availability(w, r, true) })
	// The workspace's clock page on the shop iPad: name + PIN, then the punch, without opening the worker's dashboard.
	mux.HandleFunc("POST "+base+"/workers/{principal}/punch", func(w http.ResponseWriter, r *http.Request) {
		d, _, ok := clockDevice(w, r)
		if !ok {
			return
		}
		principal := r.PathValue("principal")
		var input struct {
			Pin    string `json:"pin"`
			Action string `json:"action"`
		}
		if !uuid(principal) || !decode(w, r, &input) {
			if !uuid(principal) {
				reply(w, 400, map[string]string{"error": "Choose your name."})
			}
			return
		}
		who, where := "pin:"+d.LocationID+":"+principal, "pin:"+d.LocationID
		if wrongPinWorker.full(who) || wrongPinDevice.full(where) {
			reply(w, 429, map[string]string{"error": "Too many wrong PINs. Wait 15 minutes, or ask the owner to check your PIN."})
			return
		}
		state, err := store.ShopPinPunch(r.Context(), d, principal, input.Pin, input.Action)
		if errors.Is(err, postgres.ErrClockPinUnknown) {
			wrongPinWorker.allow(who)
			wrongPinDevice.allow(where)
			reply(w, 401, map[string]string{"error": "That PIN is not right. Try again."})
			return
		}
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, state)
	})
	// The shop bar: the worker's shift (GET) and punches (POST). Both need the iPad's sign-in and a worker session the iPad opened.
	me := func(w http.ResponseWriter, r *http.Request, action string) {
		if _, ok := device(w, r); !ok {
			return
		}
		state, err := store.ShopPunch(r.Context(), workerCookie(r), action)
		if errors.Is(err, postgres.ErrShopWorkerSession) {
			reply(w, 401, map[string]string{"error": "Tap your name on the shop iPad to continue.", "code": "shop_worker_signed_out"})
			return
		}
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, state)
	}
	mux.HandleFunc("GET "+base+"/me", func(w http.ResponseWriter, r *http.Request) { me(w, r, "status") })
	mux.HandleFunc("POST "+base+"/me", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Action string `json:"action"`
		}
		if !decode(w, r, &input) {
			return
		}
		me(w, r, input.Action)
	})
	mux.HandleFunc("POST "+base+"/done", func(w http.ResponseWriter, r *http.Request) {
		endWorker(w, r)
		w.WriteHeader(204)
	})
}

func setWorkspaceCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int((24 * time.Hour).Seconds())})
}
