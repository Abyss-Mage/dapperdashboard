package install

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dapper.local/platform/internal/email"
	"github.com/jackc/pgx/v5"
)

// DB is what the wizard collects for the PostgreSQL step.
type DB struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"sslMode"`
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

func (d DB) validate() error {
	if strings.TrimSpace(d.Host) == "" || strings.ContainsAny(d.Host, " /?#@") || d.Port < 1 || d.Port > 65535 || !identifier.MatchString(d.Name) || strings.TrimSpace(d.User) == "" {
		return errors.New("enter a host, a port, a database name (letters, digits, underscores) and a user")
	}
	switch d.SSLMode {
	case "disable", "require", "verify-full":
		return nil
	}
	return errors.New("choose a TLS mode for the database connection")
}

// URL builds the connection string for a database on that server.
func (d DB) URL(database string) string {
	u := url.URL{Scheme: "postgres", User: url.UserPassword(d.User, d.Password), Host: net.JoinHostPort(d.Host, fmt.Sprint(d.Port)), Path: "/" + database}
	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// DBReport is the plain-language result of looking at the server; it never echoes the password back.
type DBReport struct {
	Reachable      bool   `json:"reachable"`
	Version        string `json:"version,omitempty"`
	DatabaseExists bool   `json:"databaseExists"`
	Empty          bool   `json:"empty"`
	CanCreate      bool   `json:"canCreate"`
	Message        string `json:"message"`
}

func connect(ctx context.Context, url string) (*pgx.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return pgx.Connect(ctx, url)
}

// Probe checks the server, whether the database exists, whether it is empty and whether this user may create it.
func Probe(ctx context.Context, d DB) DBReport {
	if err := d.validate(); err != nil {
		return DBReport{Message: err.Error()}
	}
	admin, err := connect(ctx, d.URL("postgres"))
	if err != nil {
		// A restricted user may only be able to reach its own database.
		own, ownErr := connect(ctx, d.URL(d.Name))
		if ownErr != nil {
			return DBReport{Message: "Could not connect: " + friendly(err)}
		}
		defer own.Close(ctx)
		return reportFor(ctx, own, DBReport{Reachable: true, DatabaseExists: true})
	}
	defer admin.Close(ctx)
	r := DBReport{Reachable: true}
	_ = admin.QueryRow(ctx, `SELECT version()`).Scan(&r.Version)
	_ = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, d.Name).Scan(&r.DatabaseExists)
	_ = admin.QueryRow(ctx, `SELECT rolcreatedb OR rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&r.CanCreate)
	if !r.DatabaseExists {
		r.Empty = true
		if r.CanCreate {
			r.Message = "The server is reachable and the database does not exist yet; setup can create it."
		} else {
			r.Message = "The server is reachable but the database does not exist and this user may not create it. Ask your database administrator to create it, then test again."
		}
		return r
	}
	target, err := connect(ctx, d.URL(d.Name))
	if err != nil {
		r.Message = "The database exists but this user cannot open it: " + friendly(err)
		return r
	}
	defer target.Close(ctx)
	return reportFor(ctx, target, r)
}

func reportFor(ctx context.Context, c *pgx.Conn, r DBReport) DBReport {
	if r.Version == "" {
		_ = c.QueryRow(ctx, `SELECT version()`).Scan(&r.Version)
	}
	r.Empty = UsableDatabase(ctx, c)
	switch {
	case r.Empty:
		r.Message = "Connected. The database is empty and ready for Dapper."
	default:
		r.Message = "This database already contains tables. Setup only installs into an empty database, so choose a new database name."
	}
	return r
}

// CreateDatabase creates the database when it does not exist. The name is validated as a plain identifier first.
func CreateDatabase(ctx context.Context, d DB) error {
	if err := d.validate(); err != nil {
		return err
	}
	admin, err := connect(ctx, d.URL("postgres"))
	if err != nil {
		return errors.New("could not connect to create the database: " + friendly(err))
	}
	defer admin.Close(ctx)
	var exists bool
	if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, d.Name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{d.Name}.Sanitize())
	return err
}

// friendly keeps driver text but removes anything that could carry the connection string.
func friendly(err error) string {
	s := err.Error()
	if i := strings.Index(s, "postgres://"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 220 {
		s = s[:220]
	}
	return strings.TrimSpace(s)
}

// Mail is what the wizard collects for the SMTP step.
type Mail struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
}

// SendTest sends one message through the configured server so a wrong password shows up now, not at the first invitation.
func SendTest(ctx context.Context, m Mail, to string) error {
	sender, err := email.NewSMTPSender(email.Config{Host: m.Host, Port: m.Port, Username: m.Username, Password: m.Password, FromAddress: m.FromAddress, FromName: m.FromName})
	if err != nil {
		return errors.New("check the server name and the From address")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if _, err = sender.SendEmail(ctx, to, "Dapper setup test", "<p>This is the test message from your Dapper setup. Email is working.</p>", "This is the test message from your Dapper setup. Email is working."); err != nil {
		return errors.New("the mail server refused the message or could not be reached")
	}
	return nil
}

// GenerateSelfSigned writes a certificate for the site host (and localhost) into dir. It never changes any trust store.
func GenerateSelfSigned(dir, host string) (cert, key string, err error) {
	name := host
	if h, _, e := net.SplitHostPort(host); e == nil {
		name = h
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return
	}
	t := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(2, 0, 0),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	for _, n := range []string{name, "localhost"} {
		if ip := net.ParseIP(n); ip != nil {
			t.IPAddresses = append(t.IPAddresses, ip)
		} else {
			t.DNSNames = append(t.DNSNames, n)
		}
	}
	t.IPAddresses = append(t.IPAddresses, net.ParseIP("127.0.0.1"))
	der, err := x509.CreateCertificate(rand.Reader, &t, &t, &priv.PublicKey, priv)
	if err != nil {
		return
	}
	cert, key = filepath.Join(dir, "certificate.pem"), filepath.Join(dir, "private-key.pem")
	if err = os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		return
	}
	err = os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0o600)
	return
}

// UsableDatabase is true for an empty database, or one an earlier setup migrated but never finished (no organisation or account yet).
func UsableDatabase(ctx context.Context, c *pgx.Conn) bool {
	var tables int
	if c.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables) != nil {
		return false
	}
	if tables == 0 {
		return true
	}
	var used bool
	if c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organizations) OR EXISTS(SELECT 1 FROM principals)`).Scan(&used) != nil {
		return false
	}
	return !used
}
