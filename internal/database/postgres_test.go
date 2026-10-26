package database

import (
	"os"
	"strings"
	"testing"

	"capstone-be/config"
)

func TestInitDBSuccess(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	cfg := &config.Config{DBName: "postgres", DBSSLMode: "disable", DBTimeZone: "UTC"}
	for _, f := range strings.Fields(dsn) {
		k, v, _ := strings.Cut(f, "=")
		switch k {
		case "host":
			cfg.DBHost = v
		case "port":
			cfg.DBPort = v
		case "user":
			cfg.DBUser = v
		case "password":
			cfg.DBPassword = v
		case "dbname":
			cfg.DBName = v
		}
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if s := db.Stats(); s.MaxOpenConnections != 100 {
		t.Errorf("MaxOpenConnections = %d, want 100", s.MaxOpenConnections)
	}
	var one int
	if err := db.QueryRow(`SELECT 1`).Scan(&one); err != nil || one != 1 {
		t.Fatalf("query: %v", err)
	}
}

func TestInitDBUnreachable(t *testing.T) {
	cfg := &config.Config{DBHost: "127.0.0.1", DBPort: "1", DBUser: "u", DBPassword: "p", DBName: "d", DBSSLMode: "disable", DBTimeZone: "UTC"}
	db, err := InitDB(cfg)
	if err == nil || db != nil || !strings.HasPrefix(err.Error(), "database unavailable") {
		t.Fatalf("got db=%v err=%v", db, err)
	}
}
