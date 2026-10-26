package config

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env here
	for _, k := range []string{"PORT", "ENV", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "DB_TIMEZONE", "JWT_SECRET", "JWT_ACCESS_MINUTES", "JWT_EXPIRATION_HOURS"} {
		t.Setenv(k, "") // registers restore
		os.Unsetenv(k)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Port: "8080", Env: "development", DBHost: "127.0.0.1", DBPort: "5432", DBUser: "postgres", DBPassword: "postgres", DBName: "capstone_db", DBSSLMode: "disable", DBTimeZone: "Asia/Jakarta", JWTAccessMinutes: 15, JWTExpirationHours: 168}
	if *cfg != want {
		t.Fatalf("got %+v\nwant %+v", *cfg, want)
	}
}

func TestLoadConfigFromEnvAndDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DB_NAME", "") // godotenv does not override existing vars
	os.Unsetenv("DB_NAME")
	if err := os.WriteFile(".env", []byte("DB_NAME=from_dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", "9999")
	t.Setenv("JWT_SECRET", "s3cret")
	t.Setenv("JWT_ACCESS_MINUTES", "30")
	t.Setenv("JWT_EXPIRATION_HOURS", "not-a-number") // falls back to 168
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9999" || cfg.JWTSecret != "s3cret" || cfg.JWTAccessMinutes != 30 || cfg.JWTExpirationHours != 168 || cfg.DBName != "from_dotenv" {
		t.Fatalf("unexpected config %+v", *cfg)
	}
}

func TestLoadConfigInvalidAccessMinutes(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("JWT_ACCESS_MINUTES", "fifteen")
	if cfg, err := LoadConfig(); err == nil {
		t.Fatalf("expected error, got %+v", cfg)
	}
}
