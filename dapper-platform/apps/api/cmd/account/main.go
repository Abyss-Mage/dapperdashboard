// Trusted operator tool. No public registration, principal creation or grants.
package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	principal := flag.String("principal", "", "existing principal UUID")
	email := flag.String("email", "", "managed account email")
	flag.Parse()
	if os.Getenv("DAPPER_ACCOUNT_PROVISION") != "1" || *principal == "" || *email == "" {
		log.Fatal("Explicit account provisioning opt-in and existing principal/email required")
	}
	// Password arrives on standard input, never argv, flags or log output.
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 2049))
	if err != nil || len(input) > 1026 {
		log.Fatal("Bounded password input required")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(input), "\n"), "\r")
	clear(input)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, os.Getenv("DAPPER_DATABASE_URL"))
	if err != nil {
		log.Fatal("Private database configuration unavailable")
	}
	defer store.Close()
	if err = store.ValidateSchema(ctx); err != nil {
		log.Fatal("Explicit schema migration/verification required before provisioning")
	}
	verifier, err := postgres.NewPasswordAuthenticator(store)
	if err != nil {
		log.Fatal("Password verification initialization failed")
	}
	err = verifier.SetPassword(ctx, *principal, *email, password)
	password = ""
	if err != nil {
		log.Fatal("Account credential provisioning failed; no input or database details are logged")
	}
	log.Print("Managed credential saved for the existing active principal; existing sessions revoked, no grants changed")
}
