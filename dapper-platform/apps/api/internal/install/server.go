package install

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"dapper.local/platform/internal/postgres"
)

var franchiseCode = regexp.MustCompile(`^[A-Z0-9-]{2,12}$`)

//go:embed wizard/index.html
var wizardPage []byte

// Server is the browser wizard. It only runs while the installation is not locked, only answers requests that carry the one-time
// code printed in the terminal, and stops itself once the installation has been written.
type Server struct {
	Home, Token, Addr string
	Done              chan struct{}
	mu                sync.Mutex
	fails             int
	until             time.Time
	finished          bool
}

func NewServer(home, addr string) (*Server, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return &Server{Home: home, Addr: addr, Token: code[:4] + "-" + code[4:8] + "-" + code[8:12], Done: make(chan struct{})}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		_, _ = w.Write(wizardPage)
	})
	mux.HandleFunc("POST /api/unlock", s.guard(func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]any{"ok": true}) }))
	mux.HandleFunc("POST /api/check", s.guard(s.check))
	mux.HandleFunc("POST /api/db/test", s.guard(s.dbTest))
	mux.HandleFunc("POST /api/mail/test", s.guard(s.mailTest))
	mux.HandleFunc("POST /api/install", s.guard(s.install))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			http.Error(w, "unexpected host", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		mux.ServeHTTP(w, r)
	})
}

// hostAllowed blocks DNS-rebinding: on a loopback listener only loopback names are accepted.
func (s *Server) hostAllowed(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	listenHost, _, _ := net.SplitHostPort(s.Addr)
	if ip := net.ParseIP(listenHost); ip != nil && ip.IsLoopback() {
		return h == "localhost" || h == "127.0.0.1" || h == "[::1]" || h == "::1"
	}
	return true
}

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, msg string) { reply(w, code, map[string]any{"ok": false, "error": msg}) }

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if Installed(s.Home) {
			fail(w, 410, "This installation is already set up.")
			return
		}
		s.mu.Lock()
		locked := time.Now().Before(s.until)
		s.mu.Unlock()
		if locked {
			fail(w, 429, "Too many wrong codes. Wait a minute and try again.")
			return
		}
		if subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(r.Header.Get("X-Setup-Code")))), []byte(s.Token)) != 1 {
			s.mu.Lock()
			if s.fails++; s.fails >= 8 {
				s.fails, s.until = 0, time.Now().Add(time.Minute)
			}
			s.mu.Unlock()
			fail(w, 401, "That setup code is not right. Use the code printed in the terminal where setup is running.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		next(w, r)
	}
}

type checkItem struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	items := []checkItem{}
	probe := os.MkdirAll(Dir(s.Home), 0o700)
	items = append(items, checkItem{"Write access to the install folder", probe == nil, detailOf(probe, Dir(s.Home))})
	for _, tool := range []struct{ bin, why string }{{"node", "runs the web application"}, {"go", "builds the services"}} {
		p, err := exec.LookPath(tool.bin)
		items = append(items, checkItem{tool.bin + " is installed", err == nil, map[bool]string{true: p + " (" + tool.why + ")", false: "not found on PATH (" + tool.why + ")"}[err == nil]})
	}
	_, err := os.Stat(s.Home + "/dapper-platform/apps/web/.next/BUILD_ID")
	items = append(items, checkItem{"Web application is built", err == nil, map[bool]string{true: "found", false: "not built yet: it is built automatically the first time you start Dapper (a few minutes)"}[err == nil]})
	reply(w, 200, map[string]any{"ok": true, "items": items})
}
func detailOf(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func (s *Server) dbTest(w http.ResponseWriter, r *http.Request) {
	var d DB
	if json.NewDecoder(r.Body).Decode(&d) != nil {
		fail(w, 400, "unreadable request")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	reply(w, 200, map[string]any{"ok": true, "report": Probe(ctx, d)})
}

func (s *Server) mailTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mail
		To string `json:"to"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.To) == "" {
		fail(w, 400, "enter the address to send the test to")
		return
	}
	if err := SendTest(r.Context(), req.Mail, strings.TrimSpace(req.To)); err != nil {
		fail(w, 200, err.Error())
		return
	}
	reply(w, 200, map[string]any{"ok": true})
}

// InstallRequest is everything the wizard collected.
type InstallRequest struct {
	Site struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Timezone string `json:"timezone"`
		Listen   string `json:"listen"`
		TLS      TLS    `json:"tls"`
	} `json:"site"`
	DB       DB   `json:"db"`
	CreateDB bool `json:"createDb"`
	Mail     Mail `json:"mail"`
	Owner    struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	} `json:"owner"`
	// The first franchise. The owner is made Super Admin and the franchise gets its public website entry.
	Franchise struct {
		Name       string `json:"name"`
		Code       string `json:"code"`
		Street     string `json:"street"`
		City       string `json:"city"`
		Region     string `json:"region"`
		PostalCode string `json:"postalCode"`
		Country    string `json:"country"`
		Phone      string `json:"phone"`
	} `json:"franchise"`
}

func (q InstallRequest) validate() error {
	if n := len([]rune(strings.TrimSpace(q.Site.Name))); n < 2 || n > 120 {
		return errors.New("business name must be 2 to 120 characters")
	}
	if _, err := parseOrigin(q.Site.URL); err != nil {
		return err
	}
	if _, err := time.LoadLocation(q.Site.Timezone); err != nil || q.Site.Timezone == "" || q.Site.Timezone == "Local" {
		return errors.New("choose a valid timezone")
	}
	if _, _, err := net.SplitHostPort(q.Site.Listen); err != nil {
		return errors.New("listen address must look like 127.0.0.1:8443")
	}
	switch q.Site.TLS.Mode {
	case TLSSelfSigned, TLSProxy:
	case TLSFiles:
		for _, f := range []string{q.Site.TLS.CertFile, q.Site.TLS.KeyFile} {
			if _, err := os.Stat(f); err != nil {
				return errors.New("certificate or key file not found: " + f)
			}
		}
	default:
		return errors.New("choose how HTTPS is provided")
	}
	if err := q.DB.validate(); err != nil {
		return err
	}
	if q.Mail.Host != "" && (q.Mail.Port < 1 || q.Mail.Port > 65535 || !strings.Contains(q.Mail.FromAddress, "@")) {
		return errors.New("email needs a server, a port and a From address (or leave email blank)")
	}
	if !strings.Contains(q.Owner.Email, "@") || len(q.Owner.Password) < 15 || len(q.Owner.Password) > 1024 {
		return errors.New("owner needs an email address and a password of at least 15 characters")
	}
	f := q.Franchise
	if n := len([]rune(strings.TrimSpace(f.Name))); n < 2 || n > 120 {
		return errors.New("the first franchise needs a name of 2 to 120 characters")
	}
	if !franchiseCode.MatchString(strings.ToUpper(strings.TrimSpace(f.Code))) {
		return errors.New("the franchise code is 2 to 12 letters, digits or dashes, for example DDP")
	}
	if strings.TrimSpace(f.Street) == "" || strings.TrimSpace(f.City) == "" || strings.TrimSpace(f.Region) == "" || strings.TrimSpace(f.PostalCode) == "" || strings.TrimSpace(f.Country) == "" {
		return errors.New("enter the franchise street, city, province or state, postal code and country")
	}
	return nil
}

type stepResult struct {
	Step   string `json:"step"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

func (s *Server) install(w http.ResponseWriter, r *http.Request) {
	var q InstallRequest
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		fail(w, 400, "unreadable request")
		return
	}
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		fail(w, 409, "Setup is already running or finished.")
		return
	}
	s.mu.Unlock()
	if err := q.validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	var steps []stepResult
	bad := func(step string, err error) {
		steps = append(steps, stepResult{step, false, friendly(err)})
		reply(w, 200, map[string]any{"ok": false, "steps": steps, "error": step + " failed: " + friendly(err)})
	}
	good := func(step, detail string) { steps = append(steps, stepResult{step, true, detail}) }
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	if q.CreateDB {
		if err := CreateDatabase(ctx, q.DB); err != nil {
			bad("Create the database", err)
			return
		}
		good("Create the database", q.DB.Name)
	}
	target, err := connect(ctx, q.DB.URL(q.DB.Name))
	if err != nil {
		bad("Connect to the database", err)
		return
	}
	usable := UsableDatabase(ctx, target)
	target.Close(ctx)
	if !usable {
		bad("Check the database", errors.New("it already holds data; choose a new, empty database"))
		return
	}
	good("Connect to the database", "empty and reachable")

	store, err := postgres.Open(ctx, q.DB.URL(q.DB.Name))
	if err != nil {
		bad("Open the database", err)
		return
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		bad("Create the tables", err)
		return
	}
	good("Create the tables", "reviewed migrations applied")
	admin, err := store.InitializeAdministrator(ctx, q.Site.Name, strings.TrimSpace(q.Owner.Email), q.Owner.Password)
	if err != nil {
		bad("Create the owner account", errors.New("the owner account could not be created (check the email and that the password is at least 15 characters)"))
		return
	}
	good("Create the owner account", strings.TrimSpace(q.Owner.Email))
	if _, err = store.ProvisionFirstFranchise(ctx, admin, postgres.FirstFranchiseInput{Name: q.Franchise.Name, Code: q.Franchise.Code, Phone: q.Franchise.Phone,
		Address: postgres.Address{Street: q.Franchise.Street, City: q.Franchise.City, Region: q.Franchise.Region, PostalCode: q.Franchise.PostalCode, Country: q.Franchise.Country}}); err != nil {
		bad("Create the first franchise", errors.New("the franchise could not be created (check the name, code and address)"))
		return
	}
	good("Create the first franchise", strings.TrimSpace(q.Franchise.Name))
	good("Make the owner Super Admin", "the Super Admin role holds every permission and is assigned to "+strings.TrimSpace(q.Owner.Email))

	origin, _ := parseOrigin(q.Site.URL)
	cfg := Config{Site: Site{Name: strings.TrimSpace(q.Site.Name), URL: origin.String(), Timezone: q.Site.Timezone, Listen: q.Site.Listen, TLS: q.Site.TLS},
		Database: Database{URL: q.DB.URL(q.DB.Name)}, Session: Session{Lifetime: "8h"}}
	if cfg.Site.TLS.Mode == TLSSelfSigned {
		cert, key, err := GenerateSelfSigned(CertDir(s.Home), origin.Host)
		if err != nil {
			bad("Create the certificate", err)
			return
		}
		cfg.Site.TLS.CertFile, cfg.Site.TLS.KeyFile = cert, key
		good("Create the certificate", "self-signed; browsers will warn until it is trusted")
	}
	if q.Mail.Host != "" {
		cfg.SMTP = SMTP{Host: strings.TrimSpace(q.Mail.Host), Port: q.Mail.Port, Username: q.Mail.Username, Password: q.Mail.Password, FromAddress: strings.TrimSpace(q.Mail.FromAddress), FromName: strings.TrimSpace(q.Mail.FromName)}
		good("Save email settings", q.Mail.Host)
	} else {
		good("Email", "skipped; invitations stay queued until it is configured")
	}
	if cfg.Keys, err = NewKeys(); err != nil {
		bad("Generate the secret keys", err)
		return
	}
	good("Generate the secret keys", "two 256-bit keys, stored only in the private configuration file")
	_ = os.MkdirAll(UploadDir(s.Home), 0o700)
	if err = Save(s.Home, cfg); err != nil {
		bad("Write the configuration", err)
		return
	}
	good("Write the configuration", ConfigPath(s.Home))
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
	reply(w, 200, map[string]any{"ok": true, "steps": steps, "loginUrl": origin.String() + "/dashboard", "startCommand": "scripts/start.ps1 (Windows)  or  sh scripts/start.sh (macOS, Linux)"})
	go func() { time.Sleep(1500 * time.Millisecond); close(s.Done) }()
}
