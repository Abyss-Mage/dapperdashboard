// Trusted first-run operator tool; no HTTP signup or default credential.
package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	organization := flag.String("organization", "", "actual organization display name")
	email := flag.String("email", "", "authorized initial administrator email")
	flag.Parse()
	if os.Getenv("DAPPER_INITIAL_ADMIN") != "1" {
		log.Fatal("Explicit initial administrator opt-in required")
	}
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 2049))
	if err != nil || len(input) > 1026 {
		log.Fatal("Bounded password input required on standard input")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(input), "\n"), "\r")
	clear(input)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, os.Getenv("DAPPER_DATABASE_URL"))
	if err != nil {
		log.Fatal("Private application database configuration unavailable")
	}
	defer store.Close()
	if err = store.ValidateSchema(ctx); err != nil {
		log.Fatal("Explicit reviewed schema migration required")
	}
	administrator, err := store.InitializeAdministrator(ctx, *organization, *email, password)
	password = ""
	if err != nil {
		log.Fatal("Initial administrator provisioning refused or failed; existing accounts and data were preserved")
	}
	if err = json.NewEncoder(os.Stdout).Encode(administrator); err != nil {
		log.Fatal("Administrator provisioned, but result output failed; inspect application access before retrying")
	}
}
