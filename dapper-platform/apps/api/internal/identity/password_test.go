package identity

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type syntheticVerifier struct {
	calls int
	err   error
}

func (s *syntheticVerifier) SignIn(context.Context, string, string, time.Duration) (string, error) {
	s.calls++
	return "synthetic-session", s.err
}
func TestPasswordLoginBoundary(t *testing.T) {
	verifier := &syntheticVerifier{}
	login, err := NewPasswordLogin(verifier, "https://127.0.0.1:3006", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://127.0.0.1:3006/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()
		login.ServeHTTP(w, r)
		return w
	}
	body := `{"email":"synthetic@example.invalid","password":"synthetic fixture only"}`
	if w := request("GET", "https://127.0.0.1:3006", body); w.Code != 405 {
		t.Fatal("GET accepted")
	}
	if w := request("POST", "https://other.invalid", body); w.Code != 403 {
		t.Fatal("cross origin accepted")
	}
	if w := request("POST", "https://127.0.0.1:3006", body+`{}`); w.Code != 400 {
		t.Fatal("multiple bodies accepted")
	}
	if verifier.calls != 0 {
		t.Fatal("invalid boundary reached password verifier")
	}
	w := request("POST", "https://127.0.0.1:3006", body)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("successful sign-in response wrong")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookieName || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].MaxAge != 3600 {
		t.Fatal("protected session cookie missing")
	}
	verifier.err = postgres.ErrSession
	for i := 0; i < 9; i++ {
		if request("POST", "https://127.0.0.1:3006", body).Code != 401 {
			t.Fatal("generic denial missing")
		}
	}
	before := verifier.calls
	w = request("POST", "https://127.0.0.1:3006", body)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || verifier.calls != before {
		t.Fatal("account throttle missing")
	}
	if _, err = NewPasswordLogin(verifier, "http://127.0.0.1:3006", time.Hour); err == nil {
		t.Fatal("HTTP configuration accepted")
	}
	if _, err = NewPasswordLogin(verifier, "https://127.0.0.1:3006", 0); err == nil {
		t.Fatal("implicit lifetime accepted")
	}
}
