// Package install holds the first-run setup for a fresh checkout: the configuration file it writes, the checks it runs against
// PostgreSQL and SMTP, and the HTTP wizard that collects the answers. Nothing here ships a default account or default secret.
package install

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const ConfigVersion = 1

// TLS modes: the gateway always terminates HTTPS itself, or sits behind a proxy that does.
const (
	TLSSelfSigned = "self-signed" // generated here for the site host; browsers will warn until it is trusted
	TLSFiles      = "files"       // the operator's own certificate and key
	TLSProxy      = "proxy"       // plain HTTP on the listen address, behind a reverse proxy that serves the public HTTPS URL
)

type Config struct {
	Version  int      `json:"version"`
	Created  string   `json:"created"`
	Site     Site     `json:"site"`
	Database Database `json:"database"`
	SMTP     SMTP     `json:"smtp"`
	Keys     Keys     `json:"keys"`
	Session  Session  `json:"session"`
}
type Site struct {
	Name     string `json:"name"`
	URL      string `json:"url"` // public origin, for example https://dapper.example.com
	Timezone string `json:"timezone"`
	Listen   string `json:"listen"` // gateway address, for example 127.0.0.1:8443 or 0.0.0.0:443
	TLS      TLS    `json:"tls"`
}
type TLS struct {
	Mode     string `json:"mode"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
}
type Database struct {
	URL string `json:"url"` // contains the password; this file is written 0600 and is git-ignored
}
type SMTP struct {
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	FromAddress string `json:"fromAddress,omitempty"`
	FromName    string `json:"fromName,omitempty"`
}
type Keys struct {
	MFA        string `json:"mfa"`
	Invitation string `json:"invitation"`
}
type Session struct {
	Lifetime string `json:"lifetime"`
}

// Dir is the per-installation data directory (git-ignored): config, certificate, uploads and the install lock.
func Dir(home string) string          { return filepath.Join(home, ".dapper") }
func ConfigPath(home string) string   { return filepath.Join(Dir(home), "config.json") }
func lockPath(home string) string     { return filepath.Join(Dir(home), "installed.lock") }
func Installed(home string) bool      { _, err := os.Stat(lockPath(home)); return err == nil }
func UploadDir(home string) string    { return filepath.Join(Dir(home), "uploads") }
func CertDir(home string) string      { return filepath.Join(Dir(home), "tls") }
func BinDir(home string) string       { return filepath.Join(Dir(home), "bin") }
func randomKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// NewKeys generates the two server-held 256-bit keys (authenticator-secret encryption and invitation encryption).
func NewKeys() (Keys, error) {
	m, err := randomKey()
	if err != nil {
		return Keys{}, err
	}
	i, err := randomKey()
	return Keys{MFA: m, Invitation: i}, err
}

func Load(home string) (Config, error) {
	var c Config
	raw, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, errors.New("configuration file is not valid JSON")
	}
	if c.Version != ConfigVersion {
		return c, fmt.Errorf("configuration version %d is not supported", c.Version)
	}
	return c, nil
}

// Save writes the configuration with owner-only permissions. It refuses to replace an installation that is already locked.
func Save(home string, c Config) error {
	if Installed(home) {
		return errors.New("this installation is already set up; remove .dapper to run setup again")
	}
	if err := os.MkdirAll(Dir(home), 0o700); err != nil {
		return err
	}
	restrict(Dir(home))
	c.Version, c.Created = ConfigVersion, time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(ConfigPath(home), raw, 0o600); err != nil {
		return err
	}
	return os.WriteFile(lockPath(home), []byte(c.Created+"\n"), 0o600)
}

// Origin returns the validated public origin (scheme and host only).
func (c Config) Origin() (*url.URL, error) { return parseOrigin(c.Site.URL) }

func parseOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return nil, errors.New("site address must look like https://example.com (no path)")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

// Env is the environment the API, gateway and web app are started with.
func (c Config) Env(home string) []string {
	origin, _ := c.Origin()
	env := []string{
		"DAPPER_WORKSPACE=1", "DAPPER_HTTPS_WORKSPACE=1", "DAPPER_PASSWORD_LOGIN=1",
		"DAPPER_DATABASE_URL=" + c.Database.URL, "DAPPER_CLIENT_ORIGIN=" + origin.String(),
		"DAPPER_SESSION_LIFETIME=" + c.Session.Lifetime,
		"DAPPER_MFA_KEY=" + c.Keys.MFA, "DAPPER_INVITATION_ENCRYPTION_KEY=" + c.Keys.Invitation,
		"DAPPER_UPLOAD_DIR=" + UploadDir(home), "DAPPER_SOP_DIR=" + filepath.Join(home, "dapper-platform", "apps", "api", "data", "sops"),
		"DAPPER_LISTEN=" + c.Site.Listen, "DAPPER_PUBLIC_HOST=" + origin.Host, "DAPPER_TIMEZONE=" + c.Site.Timezone, "DAPPER_BUSINESS_NAME=" + c.Site.Name,
	}
	switch c.Site.TLS.Mode {
	case TLSProxy:
		env = append(env, "DAPPER_BEHIND_PROXY=1")
	default:
		env = append(env, "DAPPER_TLS_CERT="+c.Site.TLS.CertFile, "DAPPER_TLS_KEY="+c.Site.TLS.KeyFile)
	}
	if c.SMTP.Host != "" {
		env = append(env, "DAPPER_SMTP_HOST="+c.SMTP.Host, "DAPPER_SMTP_PORT="+strconv.Itoa(c.SMTP.Port), "DAPPER_SMTP_USERNAME="+c.SMTP.Username, "DAPPER_SMTP_PASSWORD="+c.SMTP.Password,
			"DAPPER_SMTP_FROM_ADDRESS="+c.SMTP.FromAddress, "DAPPER_SMTP_FROM_NAME="+c.SMTP.FromName)
	}
	return env
}

// restrict limits the folder to the current user. POSIX modes already do this; on Windows the inherited permissions are replaced.
func restrict(dir string) {
	if runtime.GOOS != "windows" {
		return
	}
	if user := os.Getenv("USERNAME"); user != "" {
		_ = exec.Command("icacls", dir, "/inheritance:r", "/grant:r", user+":(OI)(CI)F").Run()
	}
}
