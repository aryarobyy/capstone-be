package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"capstone-be/config"
	"capstone-be/internal/database"
)

const migrationsDir = "migrations"

func main() {
	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := createMigrationsTable(db); err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	switch action {
	case "up":
		if err := runUp(db); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	case "down":
		if err := runDown(db); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
	default:
		log.Fatalf("Unknown action: %s. Use 'up' or 'down'", action)
	}
}

func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`
	_, err := db.Exec(query)
	return err
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func runUp(db *sql.DB) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to retrieve applied migrations: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	appliedCount := 0
	for _, file := range upFiles {
		version := strings.TrimSuffix(file, ".up.sql")
		if applied[version] {
			continue
		}

		filePath := filepath.Join(migrationsDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		log.Printf("Applied migration: %s", file)
		appliedCount++
	}

	if appliedCount == 0 {
		log.Println("Database schema is already up to date. No new migrations to apply.")
	} else {
		log.Printf("Successfully applied %d migration(s)", appliedCount)
	}
	return nil
}

func runDown(db *sql.DB) error {
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1")
	if err != nil {
		return fmt.Errorf("failed to retrieve last migration: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		log.Println("No migrations to rollback.")
		return nil
	}

	var lastVersion string
	if err := rows.Scan(&lastVersion); err != nil {
		return err
	}

	downFile := filepath.Join(migrationsDir, lastVersion+".down.sql")
	content, err := os.ReadFile(downFile)
	if err != nil {
		return fmt.Errorf("down migration file not found: %s: %w", downFile, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to execute down migration %s: %w", downFile, err)
	}

	if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", lastVersion); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to remove migration record %s: %w", lastVersion, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit down migration: %w", err)
	}

	log.Printf("Successfully rolled back migration: %s", lastVersion)
	return nil
}
