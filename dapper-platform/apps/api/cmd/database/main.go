// Explicit database migration utility. It does not start an HTTP server or
// upgrade the local demo's session into production authorization.
package main

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"flag"
	"log"
	"os"
	"time"
)

func main() {
	migrate := flag.Bool("migrate", false, "apply reviewed database migrations")
	flag.Parse()
	if !*migrate {
		log.Fatal("Use -migrate to explicitly apply reviewed migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, os.Getenv("DAPPER_DATABASE_URL"))
	if err != nil {
		log.Fatal("Database unavailable or configuration invalid; connection details are not logged")
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		log.Fatal("Migration failed; review schema compatibility and migration history")
	}
	log.Print("Database migrations applied successfully")
}
