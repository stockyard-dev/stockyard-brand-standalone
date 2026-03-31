// Stockyard Brand — Compliance audit trail as a service.
// SHA-256 hash-chained ledger. Tamper-evident. Self-hosted.
// Single binary, embedded SQLite, zero external dependencies.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/stockyard-dev/stockyard-brand-standalone/internal/server"
	"github.com/stockyard-dev/stockyard-brand-standalone/internal/license"
	"github.com/stockyard-dev/stockyard-brand-standalone/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("brand %s\n", version)
			os.Exit(0)
		case "--health", "health":
			fmt.Println("ok")
			os.Exit(0)
		}
	}

	log.SetFlags(log.Ltime | log.Lshortfile)

	port := 8750
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	dataDir := "./data"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dataDir = d
	}

	adminKey := os.Getenv("BRAND_ADMIN_KEY")
	if adminKey == "" {
		log.Printf("[brand] BRAND_ADMIN_KEY not set — read/verify endpoints are open")
	}

	// License validation — offline Ed25519 check, no network call
	licenseKey := os.Getenv("BRAND_LICENSE_KEY")
	licInfo, licErr := license.Validate(licenseKey, "brand")
	if licenseKey != "" && licErr != nil {
		log.Printf("[license] WARNING: %v — running in free tier", licErr)
		licInfo = nil
	}
	limits := server.LimitsFor(licInfo)
	if licInfo != nil && licInfo.IsPro() {
		log.Printf("  License:   Pro (%s)", licInfo.CustomerID)
	} else {
		log.Printf("  License:   Free tier (set BRAND_LICENSE_KEY to unlock Pro)")
	}

	db, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	log.Printf("")
	log.Printf("  Stockyard Brand %s", version)
	log.Printf("  Ingest:  POST http://localhost:%d/api/events", port)
	log.Printf("  Verify:  GET  http://localhost:%d/api/verify", port)
	log.Printf("  Export:  GET  http://localhost:%d/api/evidence/export", port)
	log.Printf("  Health:  GET  http://localhost:%d/health", port)
	log.Printf("")

	srv := server.New(db, port, adminKey, limits)
	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
