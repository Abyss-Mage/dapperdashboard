package email

import "testing"

// Microsoft 365 answers the LOGIN sign-in with these two prompts; the replies must be the user name, then the password.
func TestLoginAuthAnswersTheTwoPrompts(t *testing.T) {
	a := loginAuth{"me@example.test", "secret"}
	if u, err := a.Next([]byte("Username:"), true); err != nil || string(u) != "me@example.test" {
		t.Fatal("username prompt", string(u), err)
	}
	if p, err := a.Next([]byte("Password:"), true); err != nil || string(p) != "secret" {
		t.Fatal("password prompt", err)
	}
	if _, err := a.Next([]byte("Something else:"), true); err == nil {
		t.Fatal("an unknown prompt must not be answered")
	}
}
