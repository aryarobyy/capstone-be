.PHONY: help run dev build clean tidy migrate migrate-up migrate-down migrate-status migrate-create up down

# Allow "make migrate up", "make migrate down", "make migrate status"
ifeq (migrate,$(firstword $(MAKECMDGOALS)))
  MIGRATE_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  $(eval $(MIGRATE_ARGS):;@:)
else
# Aliases when not running "make migrate ..."
up: migrate-up
down: migrate-down
endif

# Default target: display help
help:
	@echo "Available commands:"
	@echo "  make run             - Run the backend application"
	@echo "  make dev             - Run with hot reload (Air)"
	@echo "  make build           - Build executable binary into bin/api"
	@echo "  make clean           - Remove built binaries"
	@echo "  make tidy            - Download and tidy Go modules"
	@echo "  make migrate-up      - Run all pending migrations (alias: make migrate up, make up)"
	@echo "  make migrate-down    - Rollback last migration (alias: make migrate down, make down)"
	@echo "  make migrate-status  - View migration status (alias: make migrate status)"
	@echo "  make migrate-create name=<name> - Create a new migration SQL file pair"

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

# Flexible migrate command: handles "make migrate", "make migrate up", "make migrate down", "make migrate status"
migrate:
	@action="$(if $(MIGRATE_ARGS),$(MIGRATE_ARGS),up)"; \
	go run cmd/migrate/main.go $$action

# Run database migrations (apply pending migrations)
migrate-up:
	go run cmd/migrate/main.go up

# Rollback last database migration
migrate-down:
	go run cmd/migrate/main.go down

# Check migration status
migrate-status:
	go run cmd/migrate/main.go status

# Create a new migration file: make migrate-create name=create_example_table
migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=<migration_name>"; exit 1; fi
	@next_num=$$(printf "%06d" $$(expr $$(ls -1 migrations/*.up.sql 2>/dev/null | wc -l) + 1)); \
	up_file="migrations/$${next_num}_$(name).up.sql"; \
	down_file="migrations/$${next_num}_$(name).down.sql"; \
	touch "$$up_file"; \
	touch "$$down_file"; \
	echo "Created $$up_file"; \
	echo "Created $$down_file"
