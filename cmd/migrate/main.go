package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/alchemorsel/v3/internal/infrastructure/persistence/migrations"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <command>")
		fmt.Println("Commands: up, down, reset, version, status")
		os.Exit(1)
	}

	command := os.Args[1]

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create migrator
	migrator, err := migrations.New(db, logger)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer migrator.Close()

	// Execute command
	switch command {
	case "up":
		if err := migrator.Up(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Migrations completed successfully")

	case "down":
		if err := migrator.Down(); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		fmt.Println("Rollback completed successfully")

	case "reset":
		if err := migrator.Reset(); err != nil {
			log.Fatalf("Reset failed: %v", err)
		}
		fmt.Println("Reset completed successfully")

	case "version":
		version, dirty, err := migrator.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %t)\n", version, dirty)

	case "status":
		status, err := migrator.Status()
		if err != nil {
			log.Fatalf("Failed to get status: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %t)\n", status.Version, status.Dirty)
		fmt.Printf("Applied migrations: %d\n", len(status.Applied))

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: up, down, reset, version, status")
		os.Exit(1)
	}
}
