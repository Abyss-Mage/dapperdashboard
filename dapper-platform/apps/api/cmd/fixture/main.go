package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	if os.Getenv("DAPPER_SYNTHETIC_FIXTURE") != "1" {
		log.Fatal("Explicit synthetic fixture opt-in required")
	}
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
	fixture, err := store.CreateSyntheticFixture(ctx, password)
	password = ""
	if err != nil {
		log.Fatal("Synthetic fixture creation failed; existing schemas are never reused. No secret details are logged")
	}
	if err = json.NewEncoder(os.Stdout).Encode(fixture); err != nil {
		log.Fatal("Could not write synthetic fixture identifiers")
	}
}
