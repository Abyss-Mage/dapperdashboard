package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Runs as a plain database owner, not a superuser, because that is what the server's account is.
func TestBackupDatabaseRoundTrip(t *testing.T) {
	raw := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("DAPPER_TEST_DATABASE_URL not set")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Skip("pg_dump and pg_restore are not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := fmt.Sprintf("dapper_backup_%d", time.Now().UnixNano())
	for _, stmt := range []string{`CREATE ROLE ` + name + ` LOGIN PASSWORD 'synthetic-backup-test'`, `CREATE DATABASE ` + name + ` OWNER ` + name} {
		if _, err = admin.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.Path, u.User = "/"+name, url.UserPassword(name, "synthetic-backup-test")
	store, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		store.Close()
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+name+` WITH (FORCE)`)
		_, _ = admin.Exec(context.Background(), `DROP ROLE `+name)
	}()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	subject := func(s string) {
		t.Helper()
		if _, err := store.pool.Exec(ctx, `INSERT INTO principals(external_subject) VALUES($1)`, s); err != nil {
			t.Fatal(err)
		}
	}
	has := func(s string) bool {
		t.Helper()
		var found bool
		if err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE external_subject=$1)`, s).Scan(&found); err != nil {
			t.Fatal(err)
		}
		return found
	}
	subject("synthetic:before")
	dump := filepath.Join(t.TempDir(), "d.dump")
	if err = store.DumpDatabase(ctx, dump); err != nil {
		t.Fatal(err)
	}
	subject("synthetic:after")

	bad := filepath.Join(t.TempDir(), "bad.dump")
	if err = os.WriteFile(bad, []byte("not a dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = store.RestoreDatabase(ctx, bad); err == nil {
		t.Fatal("garbage restored")
	}
	if !has("synthetic:after") {
		t.Fatal("failed restore changed live data")
	}

	if err = store.RestoreDatabase(ctx, dump); err != nil {
		t.Fatal(err)
	}
	if !has("synthetic:before") || has("synthetic:after") {
		t.Fatal("restore did not bring back the backed-up data")
	}
	if err = store.ValidateSchema(ctx); err != nil {
		t.Fatalf("restored schema invalid: %v", err)
	}
}
