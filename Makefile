.PHONY: run dev build tidy clean migrate-up migrate-down

# Run the backend application
run:
	go run cmd/api/main.go

# Run with hot reload (Air)
dev:
	air

# Build executable binary
build:
	mkdir -p bin
	go build -o bin/api cmd/api/main.go

# Clean up binaries
clean:
	rm -rf bin

# Clean and tidy dependencies
tidy:
	go mod tidy

# Run database migrations (apply pending migrations)
migrate-up:
	go run cmd/migrate/main.go up

# Rollback last database migration
migrate-down:
	go run cmd/migrate/main.go down
