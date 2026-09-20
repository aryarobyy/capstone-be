package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"capstone-be/config"
	"capstone-be/internal/database"
)

const migrationsDir = "migrations"

type Migration struct {
	Version  int
	Name     string
	UpFile   string
	DownFile string
}

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
	case "status":
		if err := runStatus(db); err != nil {
			log.Fatalf("Migration status failed: %v", err)
		}
	default:
		log.Fatalf("Unknown action: %s. Use 'up', 'down', or 'status'", action)
	}
}

func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			version VARCHAR(255) PRIMARY KEY,
			failed VARCHAR(100),
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	if count > 1 {
		rows, err := db.Query("SELECT version FROM migrations")
		if err == nil {
			maxVer := 0
			for rows.Next() {
				var v string
				if err := rows.Scan(&v); err == nil {
					if n, err := parseVersion(v); err == nil && n > maxVer {
						maxVer = n
					}
				}
			}
			rows.Close()

			if maxVer > 0 {
				_, _ = db.Exec("DELETE FROM migrations")
				_, _ = db.Exec("INSERT INTO migrations (version, failed, applied_at) VALUES ($1, $2, NOW())", strconv.Itoa(maxVer), "false")
			}
		}
	}

	return nil
}

func parseVersion(str string) (int, error) {
	str = strings.TrimSpace(str)
	if idx := strings.Index(str, "_"); idx != -1 {
		str = str[:idx]
	}
	return strconv.Atoi(str)
}

func loadMigrationFiles() ([]Migration, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	upMap := make(map[int]string)
	downMap := make(map[int]string)
	nameMap := make(map[int]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			ver, err := parseVersion(name)
			if err != nil {
				continue
			}
			upMap[ver] = name
			base := strings.TrimSuffix(name, ".up.sql")
			if idx := strings.Index(base, "_"); idx != -1 {
				nameMap[ver] = base[idx+1:]
			} else {
				nameMap[ver] = base
			}
		} else if strings.HasSuffix(name, ".down.sql") {
			ver, err := parseVersion(name)
			if err != nil {
				continue
			}
			downMap[ver] = name
		}
	}

	var list []Migration
	for ver, upFile := range upMap {
		list = append(list, Migration{
			Version:  ver,
			Name:     nameMap[ver],
			UpFile:   upFile,
			DownFile: downMap[ver],
		})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Version < list[j].Version
	})

	return list, nil
}

func getCurrentVersion(db *sql.DB) (int, string, error) {
	var verStr string
	var failed sql.NullString
	err := db.QueryRow("SELECT version, failed FROM migrations LIMIT 1").Scan(&verStr, &failed)
	if err == sql.ErrNoRows {
		return 0, "false", nil
	}
	if err != nil {
		return 0, "", err
	}

	ver, err := parseVersion(verStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid version '%s' in migrations table: %w", verStr, err)
	}

	failedStr := "false"
	if failed.Valid && failed.String != "" {
		failedStr = failed.String
	}

	return ver, failedStr, nil
}

func setCurrentVersionTx(tx *sql.Tx, version int, failed string) error {
	if _, err := tx.Exec("DELETE FROM migrations"); err != nil {
		return err
	}
	if version <= 0 {
		return nil
	}
	_, err := tx.Exec("INSERT INTO migrations (version, failed, applied_at) VALUES ($1, $2, NOW())", strconv.Itoa(version), failed)
	return err
}

func setCurrentVersionDB(db *sql.DB, version int, failed string) error {
	if _, err := db.Exec("DELETE FROM migrations"); err != nil {
		return err
	}
	if version <= 0 {
		return nil
	}
	_, err := db.Exec("INSERT INTO migrations (version, failed, applied_at) VALUES ($1, $2, NOW())", strconv.Itoa(version), failed)
	return err
}

func runUp(db *sql.DB) error {
	currentVer, failed, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if failed != "false" && failed != "" {
		log.Printf("WARNING: Current migration version %d is marked as failed (%s).", currentVer, failed)
	}

	migrations, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	appliedCount := 0
	for _, m := range migrations {
		if m.Version <= currentVer {
			continue // Skip already applied migrations
		}

		filePath := filepath.Join(migrationsDir, m.UpFile)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", m.UpFile, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			_ = setCurrentVersionDB(db, m.Version, "true")
			return fmt.Errorf("failed to execute migration %s: %w", m.UpFile, err)
		}

		if err := setCurrentVersionTx(tx, m.Version, "false"); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration version: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", m.UpFile, err)
		}

		log.Printf("Applied migration version %d: %s", m.Version, m.UpFile)
		appliedCount++
		currentVer = m.Version
	}

	if appliedCount == 0 {
		log.Printf("Database schema is already up to date (current version: %d). No new migrations to apply.", currentVer)
	} else {
		log.Printf("Successfully applied %d migration(s). Database is now at version %d.", appliedCount, currentVer)
	}
	return nil
}

func runDown(db *sql.DB) error {
	currentVer, _, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if currentVer <= 0 {
		log.Println("No migrations to rollback (current version is 0).")
		return nil
	}

	migrations, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	var currentMigration *Migration
	prevVersion := 0
	for _, m := range migrations {
		if m.Version == currentVer {
			cm := m
			currentMigration = &cm
			break
		}
		if m.Version < currentVer {
			prevVersion = m.Version
		}
	}

	if currentMigration == nil || currentMigration.DownFile == "" {
		return fmt.Errorf("down migration file not found for version %d", currentVer)
	}

	downPath := filepath.Join(migrationsDir, currentMigration.DownFile)
	content, err := os.ReadFile(downPath)
	if err != nil {
		return fmt.Errorf("failed to read down migration file %s: %w", currentMigration.DownFile, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		_ = tx.Rollback()
		_ = setCurrentVersionDB(db, currentVer, "true")
		return fmt.Errorf("failed to execute down migration %s: %w", currentMigration.DownFile, err)
	}

	// Update version to previous version (e.g. 10 -> 9)
	if err := setCurrentVersionTx(tx, prevVersion, "false"); err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update migration version: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit down migration: %w", err)
	}

	log.Printf("Successfully rolled back migration version %d: %s. Current version is now %d.", currentVer, currentMigration.DownFile, prevVersion)
	return nil
}

func runStatus(db *sql.DB) error {
	currentVer, failed, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	migrations, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	fmt.Printf("Current Database Version: %d | Failed: %s\n", currentVer, failed)
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-8s %-45s %-12s\n", "Version", "Migration File", "Status")
	fmt.Println(strings.Repeat("-", 70))

	appliedCount := 0
	for _, m := range migrations {
		status := "PENDING"
		if m.Version <= currentVer {
			status = "APPLIED"
			appliedCount++
		}
		fmt.Printf("%-8d %-45s %-12s\n", m.Version, m.UpFile, status)
	}
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Total: %d | Applied: %d | Pending: %d\n", len(migrations), appliedCount, len(migrations)-appliedCount)

	return nil
}
