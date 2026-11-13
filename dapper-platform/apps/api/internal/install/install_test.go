package install

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledLockAndSaveOnce(t *testing.T) {
	home := t.TempDir()
	cfg := Config{Site: Site{Name: "Test", URL: "https://example.test", Timezone: "America/Toronto", Listen: "127.0.0.1:8443", TLS: TLS{Mode: TLSProxy}}, Database: Database{URL: "postgres://u:p@h/d"}, Keys: Keys{MFA: "a", Invitation: "b"}, Session: Session{Lifetime: "8h"}}
	if Installed(home) {
		t.Fatal("fresh folder must not look installed")
	}
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	if !Installed(home) {
		t.Fatal("lock missing after save")
	}
	if err := Save(home, cfg); err == nil {
		t.Fatal("second save must be refused")
	}
	loaded, err := Load(home)
	if err != nil || loaded.Site.Name != "Test" || loaded.Version != ConfigVersion {
		t.Fatalf("load failed: %v %+v", err, loaded)
	}
	// Settings added to .env by hand afterwards reach the services (owner 2026-11-13): indexing and the Square webhook.
	f, err := os.OpenFile(ConfigPath(home), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("DAPPER_PUBLIC_INDEXING=enabled\nDAPPER_SQUARE_WEBHOOK_URL=https://dapper.example.com/api/v1/integrations/square/webhooks\nDAPPER_REDIS_URL=redis://127.0.0.1:6379/0\n")
	f.Close()
	if loaded, err = Load(home); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(loaded.Env(home), "\n")
	for _, want := range []string{"DAPPER_PUBLIC_INDEXING=enabled", "DAPPER_SQUARE_WEBHOOK_URL=https://dapper.example.com/api/v1/integrations/square/webhooks", "DAPPER_REDIS_URL=redis://127.0.0.1:6379/0"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(env, "DAPPER_SQUARE_WEBHOOK_SIGNATURE_KEY") {
		t.Error("an unset optional setting must not appear")
	}
}

func TestEnvMapsProxyAndTLS(t *testing.T) {
	c := Config{Site: Site{URL: "https://dapper.example.com", Listen: "0.0.0.0:8443", TLS: TLS{Mode: TLSProxy}, Timezone: "America/Toronto", Name: "X"}, Database: Database{URL: "postgres://x"}, Keys: Keys{MFA: "a", Invitation: "b"}, Session: Session{Lifetime: "8h"}}
	env := strings.Join(c.Env(t.TempDir()), "\n")
	for _, want := range []string{"DAPPER_CLIENT_ORIGIN=https://dapper.example.com", "DAPPER_PUBLIC_HOST=dapper.example.com", "DAPPER_BEHIND_PROXY=1", "DAPPER_LISTEN=0.0.0.0:8443", "DAPPER_PASSWORD_LOGIN=1"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(env, "DAPPER_TLS_CERT") || strings.Contains(env, "DAPPER_SMTP_HOST") {
		t.Error("proxy mode without SMTP must not set certificate or SMTP variables")
	}
	c.Site.TLS = TLS{Mode: TLSFiles, CertFile: "c.pem", KeyFile: "k.pem"}
	if env = strings.Join(c.Env(t.TempDir()), "\n"); !strings.Contains(env, "DAPPER_TLS_CERT=c.pem") || strings.Contains(env, "DAPPER_BEHIND_PROXY") {
		t.Error("certificate mode must set certificate paths only")
	}
}

func TestOriginMustBeBare(t *testing.T) {
	for _, bad := range []string{"", "ftp://x.test", "https://x.test/path", "https://u:p@x.test", "https://x.test?q=1", "x.test"} {
		if _, err := parseOrigin(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if u, err := parseOrigin("https://x.test:8443/"); err != nil || u.String() != "https://x.test:8443" {
		t.Errorf("good origin rejected: %v %v", u, err)
	}
}

func TestDatabaseInputAndURL(t *testing.T) {
	good := DB{Host: "db.internal", Port: 5432, Name: "dapper", User: "app", Password: "p@ss/word?#", SSLMode: "require"}
	if err := good.validate(); err != nil {
		t.Fatal(err)
	}
	u := good.URL("dapper")
	if !strings.HasPrefix(u, "postgres://app:") || !strings.Contains(u, "@db.internal:5432/dapper") || !strings.Contains(u, "sslmode=require") || strings.Contains(u, "p@ss/word?#") {
		t.Errorf("password must be escaped in the URL: %s", u)
	}
	for _, bad := range []DB{{Host: "h", Port: 5432, Name: "bad name; drop", User: "u", SSLMode: "disable"}, {Host: "h/x", Port: 5432, Name: "d", User: "u", SSLMode: "disable"}, {Host: "h", Port: 0, Name: "d", User: "u", SSLMode: "disable"}, {Host: "h", Port: 5432, Name: "d", User: "u", SSLMode: "weird"}} {
		if bad.validate() == nil {
			t.Errorf("%+v should be rejected", bad)
		}
	}
}

func TestWizardNeedsSetupCodeAndLocksOut(t *testing.T) {
	home := t.TempDir()
	srv, err := NewServer(home, "127.0.0.1:8099")
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	post := func(code, host string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/check", nil)
		r.Host = host
		if code != "" {
			r.Header.Set("X-Setup-Code", code)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if got := post("", "127.0.0.1:8099"); got != 401 {
		t.Errorf("no code: %d", got)
	}
	if got := post(srv.Token, "evil.example"); got != 403 {
		t.Errorf("rebinding host must be refused, got %d", got)
	}
	if got := post(strings.ToLower(srv.Token), "localhost:8099"); got != 200 {
		t.Errorf("right code (any case) from localhost: %d", got)
	}
	for i := 0; i < 8; i++ {
		post("WRONG", "127.0.0.1:8099")
	}
	if got := post(srv.Token, "127.0.0.1:8099"); got != 429 {
		t.Errorf("after repeated wrong codes even the right code waits: %d", got)
	}
}

func TestWizardRefusesWhenInstalled(t *testing.T) {
	home := t.TempDir()
	cfg := Config{Site: Site{Name: "Test", URL: "https://example.test", Timezone: "America/Toronto", Listen: "127.0.0.1:8443", TLS: TLS{Mode: TLSProxy}}, Database: Database{URL: "postgres://u:p@h/d"}, Keys: Keys{MFA: "a", Invitation: "b"}, Session: Session{Lifetime: "8h"}}
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	srv, _ := NewServer(home, "127.0.0.1:8099")
	r := httptest.NewRequest(http.MethodPost, "/api/check", nil)
	r.Host, r.Header = "127.0.0.1:8099", http.Header{"X-Setup-Code": {srv.Token}}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != 410 {
		t.Errorf("installed checkout must answer 410, got %d", w.Code)
	}
}

// Setup opens first until a completed .env exists; a hand-made or half-written .env does not count.
func TestEnvFileIsTheInstallMarker(t *testing.T) {
	home := t.TempDir()
	if Installed(home) {
		t.Fatal("no .env must mean not installed")
	}
	for _, partial := range []string{"DAPPER_DATABASE_URL=postgres://x\n", "DAPPER_INSTALLED=1\n", "# only a comment\n"} {
		if err := os.WriteFile(filepath.Join(home, ".env"), []byte(partial), 0o600); err != nil {
			t.Fatal(err)
		}
		if Installed(home) {
			t.Fatalf("partial .env counted as installed: %q", partial)
		}
	}
	_ = os.Remove(filepath.Join(home, ".env"))
	cfg := Config{Site: Site{Name: `Quote "Garage" \ #1`, URL: "https://example.test", Timezone: "America/Toronto", Listen: "127.0.0.1:8443", TLS: TLS{Mode: TLSFiles, CertFile: `C:\certs\c.pem`, KeyFile: "k.pem"}},
		Database: Database{URL: "postgres://u:p%40ss%23@h:5432/d?sslmode=disable"}, SMTP: SMTP{Host: "smtp.example.test", Port: 587, Password: `pa ss"word=1`, FromAddress: "a@example.test"}, Keys: Keys{MFA: "bWZh", Invitation: "aW52"}, Session: Session{Lifetime: "8h"}}
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	if !Installed(home) {
		t.Fatal("saved setup not recognised")
	}
	got, err := Load(home)
	if err != nil || got.Site.Name != cfg.Site.Name || got.Database.URL != cfg.Database.URL || got.SMTP.Password != cfg.SMTP.Password || got.Site.TLS.CertFile != cfg.Site.TLS.CertFile || got.Keys.Invitation != "aW52" {
		t.Fatalf("round trip lost data: %+v %v", got, err)
	}
	if err = Save(home, cfg); err == nil {
		t.Fatal("setup overwrote a completed .env")
	}
}

func TestInstallRequestValidation(t *testing.T) {
	var q InstallRequest
	q.Site.Name, q.Site.URL, q.Site.Timezone, q.Site.Listen = "Garage", "https://x.test", "America/Toronto", "127.0.0.1:8443"
	q.Site.TLS.Mode = TLSSelfSigned
	q.DB = DB{Host: "h", Port: 5432, Name: "d", User: "u", SSLMode: "disable"}
	q.Owner.Email, q.Owner.Password = "o@x.test", "short"
	if q.validate() == nil {
		t.Error("short owner password must be rejected")
	}
	q.Owner.Password = "a-long-enough-passphrase"
	if q.validate() == nil {
		t.Error("a request without a first franchise must be rejected")
	}
	q.Franchise.Name, q.Franchise.Code, q.Franchise.Street, q.Franchise.City, q.Franchise.Region, q.Franchise.PostalCode, q.Franchise.Country = "Garage", "gar", "1 Main St", "Town", "ON", "A1A 1A1", "Canada"
	if err := q.validate(); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}
	q.Franchise.Code = "g"
	if q.validate() == nil {
		t.Error("a one-letter franchise code must be rejected")
	}
	q.Franchise.Code = "gar"
	q.Franchise.Street = ""
	if q.validate() == nil {
		t.Error("a franchise without a street must be rejected")
	}
	q.Franchise.Street = "1 Main St"
	q.Site.Timezone = "Mars/Olympus"
	if q.validate() == nil {
		t.Error("unknown timezone must be rejected")
	}
}
