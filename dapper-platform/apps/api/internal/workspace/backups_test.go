package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dapper.local/platform/internal/postgres"
	"github.com/jackc/pgx/v5"
)

// The whole path, over HTTP, against its own database: make, download, refuse bad restores, restore.
func TestBackupDownloadAndRestore(t *testing.T) {
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
	name := fmt.Sprintf("dapper_backup_http_%d", time.Now().UnixNano())
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
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	db, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = db.Close(context.Background())
		store.Close()
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+name+` WITH (FORCE)`)
		_, _ = admin.Exec(context.Background(), `DROP ROLE `+name)
	}()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	if err = os.MkdirAll(uploads, 0o700); err != nil {
		t.Fatal(err)
	}
	store.SetUploadDirectory(uploads)
	store.SetSOPDirectory(filepath.Join(root, "sops"))
	write := func(file string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(uploads, file), []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("before.jpg")

	var org, owner, stranger string
	if err = db.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic backup org') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	principal := func(subject string) string {
		t.Helper()
		var id string
		if err := db.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, subject).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	owner, stranger = principal("synthetic:owner"), principal("synthetic:stranger")
	if _, err = db.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, owner); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage'),($1,$2,'invoices.configure')`, owner, org); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerBackupRoutes(mux, store, nil)
	base := "/api/v1/organizations/" + org + "/backups"
	call := func(method, path, contentType string, body []byte, actor string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, sessionActor{principalID: actor}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	const passphrase = "synthetic long passphrase"
	make := func(p, actor string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"passphrase": p})
		return call("POST", base, "application/json", body, actor)
	}
	if got := make(passphrase, stranger).Code; got != 403 {
		t.Fatalf("stranger made a backup: %d", got)
	}
	if got := make("short", owner).Code; got != 422 {
		t.Fatalf("short passphrase accepted: %d", got)
	}
	made := make(passphrase, owner)
	var out struct {
		ID string `json:"id"`
	}
	if made.Code != 201 || json.Unmarshal(made.Body.Bytes(), &out) != nil {
		t.Fatalf("backup failed: %d %s", made.Code, made.Body.String())
	}
	if got := call("GET", base+"/"+out.ID, "", nil, stranger).Code; got != 403 {
		t.Fatalf("stranger downloaded: %d", got)
	}
	got := call("GET", base+"/"+out.ID, "", nil, owner)
	if got.Code != 200 || !strings.HasPrefix(got.Body.String(), "DAPPERBK1") || strings.Contains(got.Body.String(), "synthetic:owner") {
		t.Fatalf("download wrong: %d", got.Code)
	}
	file := got.Body.Bytes()
	if again := call("GET", base+"/"+out.ID, "", nil, owner).Code; again != 404 {
		t.Fatalf("file left on the server after download: %d", again)
	}

	// Things that happen after the backup, which a restore must take away.
	principal("synthetic:after")
	write("after.jpg")
	if err = os.Remove(filepath.Join(uploads, "before.jpg")); err != nil {
		t.Fatal(err)
	}
	restore := func(p, confirm string, data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		_ = form.WriteField("passphrase", p)
		_ = form.WriteField("confirm", confirm)
		part, _ := form.CreateFormFile("file", "b.dapbk")
		_, _ = part.Write(data)
		_ = form.Close()
		return call("POST", base+"/restore", form.FormDataContentType(), body.Bytes(), owner)
	}
	has := func(subject string) bool {
		t.Helper()
		var found bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE external_subject=$1)`, subject).Scan(&found); err != nil {
			t.Fatal(err)
		}
		return found
	}
	exists := func(file string) bool {
		_, err := os.Stat(filepath.Join(uploads, file))
		return err == nil
	}
	for label, res := range map[string]*httptest.ResponseRecorder{
		"wrong passphrase": restore("not the right passphrase", "RESTORE", file),
		"no confirmation":  restore(passphrase, "yes", file),
		"cut short":        restore(passphrase, "RESTORE", file[:len(file)-100]),
	} {
		if res.Code != 422 {
			t.Fatalf("%s: %d %s", label, res.Code, res.Body.String())
		}
		if !has("synthetic:after") || !exists("after.jpg") {
			t.Fatalf("%s changed live data", label)
		}
	}

	done := restore(passphrase, "RESTORE", file)
	if done.Code != 200 {
		t.Fatalf("restore failed: %d %s", done.Code, done.Body.String())
	}
	if err = db.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if db, err = pgx.Connect(ctx, u.String()); err != nil {
		t.Fatal(err)
	}
	if !has("synthetic:owner") || has("synthetic:after") {
		t.Fatal("database not restored")
	}
	if !exists("before.jpg") || exists("after.jpg") {
		t.Fatal("photos not restored")
	}
	if safety, _ := filepath.Glob(filepath.Join(root, "backups", "safety-*.dapbk")); len(safety) != 1 {
		t.Fatalf("expected one safety copy, found %d", len(safety))
	}
	if left, _ := filepath.Glob(filepath.Join(root, "backups", "*.dump")); len(left) != 0 {
		t.Fatal("a plain database dump was left on disk")
	}
	if err = store.ValidateSchema(ctx); err != nil {
		t.Fatalf("restored schema invalid: %v", err)
	}
}
