package workspace

import "testing"

func TestAdministratorWithoutMFAOnlyReachesSetupRoutes(t *testing.T) {
	for _, allowed := range []string{"/api/v1/account/onboarding", "/api/v1/account/mfa/setup", "/api/v1/account/mfa/confirm", "/api/v1/account/profile", "/api/v1/auth/logout", "/api/v1/auth/me"} {
		if !mfaSetupAllowed(allowed) {
			t.Fatalf("%s must stay reachable so an administrator can finish MFA setup", allowed)
		}
	}
	for _, blocked := range []string{"/api/v1/auth/access", "/api/v1/employee/workspace", "/api/v1/organizations/x/access", "/api/v1/organizations/x/locations/y/invoices", "/api/v1/account", "/api/v1/accounting/x", "/api/v1/auth/login"} {
		if mfaSetupAllowed(blocked) {
			t.Fatalf("%s must be blocked until MFA is on", blocked)
		}
	}
}
