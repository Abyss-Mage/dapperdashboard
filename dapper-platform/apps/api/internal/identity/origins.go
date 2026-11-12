package identity

import (
	"net/url"
	"os"
	"strings"
	"sync"
)

var extraOrigins = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, raw := range strings.Split(os.Getenv("DAPPER_EXTRA_ORIGINS"), ",") {
		if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Scheme == "https" && u.Host != "" && (u.Path == "" || u.Path == "/") {
			out["https://"+u.Host] = true
		}
	}
	return out
})

// OriginAllowed says whether a browser Origin header is this app: its main address or one of DAPPER_EXTRA_ORIGINS (the dashboard
// and workspace subdomains, owner 2026-11-13).
func OriginAllowed(got, primary string) bool {
	return got != "" && (got == primary || extraOrigins()[got])
}

// ExtraHosts are the host names of DAPPER_EXTRA_ORIGINS, for the gateway's host check.
func ExtraHosts() []string {
	var out []string
	for o := range extraOrigins() {
		out = append(out, strings.TrimPrefix(o, "https://"))
	}
	return out
}
