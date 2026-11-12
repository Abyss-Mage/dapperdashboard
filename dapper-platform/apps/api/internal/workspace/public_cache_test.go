package workspace

import "testing"

func TestCachedPublicPath(t *testing.T) {
	for p, want := range map[string]bool{"/api/v1/public/sites/dapper": true, "/api/v1/public/portfolio": true, "/api/v1/public/dealers": true,
		"/api/v1/public/sites/dapper/bookings": false, "/api/v1/public/bookings": false, "/api/v1/dealer-portal/invoices": false, "/api/v1/organizations/x/sites/y/seo": false} {
		if cachedPublicPath(p) != want {
			t.Fatalf("%s: want %v", p, want)
		}
	}
}
