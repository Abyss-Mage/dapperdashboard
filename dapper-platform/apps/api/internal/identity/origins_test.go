package identity

import "testing"

func TestOriginAllowed(t *testing.T) {
	t.Setenv("DAPPER_EXTRA_ORIGINS", " https://dashboard.example.com ,http://plain.example.com,https://x.example.com/path,https://workspace.example.com")
	main := "https://201.0.0.1"
	for got, want := range map[string]bool{main: true, "https://dashboard.example.com": true, "https://workspace.example.com": true, "http://plain.example.com": false, "https://x.example.com": false, "https://evil.example.com": false, "": false} {
		if OriginAllowed(got, main) != want {
			t.Fatalf("%q: want %v", got, want)
		}
	}
	if len(ExtraHosts()) != 2 {
		t.Fatalf("hosts %v", ExtraHosts())
	}
}
