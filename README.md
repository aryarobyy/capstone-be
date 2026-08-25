# Capstone Backend (capstone-be)

A REST API backend built in Go using a clean **Modular Monolith Architecture**. This project structure isolates domains/features into their own packages, enabling scalability, testability, and clear separation of concerns without GORM (utilizing Go's native `database/sql` package and the `github.com/lib/pq` driver for PostgreSQL).

## Architecture & Directory Structure

```
capstone-be/
├── cmd/
│   └── api/
│       └── main.go              # Entrypoint and module wiring
├── config/                      # Environmental configuration loading
│   └── config.go
├── internal/
│   ├── database/                # database/sql PostgreSQL pool connection
│   │   └── postgres.go
│   ├── middleware/              # Global router middlewares
│   │   ├── cors.go
│   │   ├── logger.go
│   │   └── recovery.go
│   ├── modules/                 # Modular domain features
│   │   ├── health/              # Healthcheck Module
│   │   │   ├── handler.go
│   │   │   └── router.go
│   │   ├── auth/                # Auth Module (Register, Login)
│   │   │   ├── dto.go           # Request & Response structs
│   │   │   ├── entity.go        # Domain model
│   │   │   ├── handler.go       # HTTP controllers
│   │   │   ├── repository.go    # Data Access Layer
│   │   │   ├── service.go       # Business Logic Layer
│   │   │   └── router.go        # Module routes mapping
│   │   └── user/                # User Module (Profile & User CRUD)
│   │       ├── dto.go           # Request & Response structs
│   │       ├── entity.go        # Domain model
│   │       ├── handler.go       # HTTP controllers
│   │       ├── repository.go    # Data Access Layer
│   │       ├── service.go       # Business Logic Layer
│   │       └── router.go        # Module routes mapping
│   └── utils/                   # Standardized helpers & response handler
│       └── response_handler.go
├── .env.example                 # Config template
├── .gitignore                   # Standard Go Gitignore
├── go.mod                       # Go modules configuration
└── Makefile                     # CLI shortcuts
```

## Features

1. **Modular Monolith**: Code is grouped by domain (features). Adding a new feature is as easy as creating a directory under `internal/modules/` and registering its routes in `main.go`.
2. **Clean Dependency Injection**: Modules receive their database dependencies through constructors, making it easy to mock/stub database layers for testing.
3. **Structured API Response**: Every success and error response adheres to a strict JSON structure.
4. **Middleware-Infused Router**: Standardized CORS configuration, request logger, and automatic recovery from panics (without crashing the server).
5. **No ORM (Raw SQL)**: Direct, high-performance querying using native `database/sql` with Postgres drivers.

## Getting Started

### Prerequisites

- Go 1.22+ installed
- PostgreSQL database instance running

### Installation

1. Clone the repository and navigate to the project directory:
   ```bash
   cd capstone-be
   ```
2. Copy the environment template:
   ```bash
   cp .env.example .env
   ```
3. Update `.env` with your database credentials.
4. Download dependencies:
   ```bash
   go mod tidy
   ```

### Running the App

Using the Makefile:
- Run in development mode: `make run`
- Build the production binary: `make build`

Or using raw Go commands:
- Run: `go run cmd/api/main.go`

---

## API Endpoints

### Health Check
- `GET /api/health` - Checks server and database connectivity status.

### Authentication
- `POST /api/auth/register` - Register a new user account.
- `POST /api/auth/login` - Authenticate user credentials.

### Users
- `GET /api/users` - Retrieve all users.
- `GET /api/users/:id` - Retrieve a user by ID.
- `PUT /api/users/:id` - Update user details.
- `DELETE /api/users/:id` - Delete a user.

### Database Table Schema (Users)

To run the User CRUD showcase, create the `users` table in your PostgreSQL database:

```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```
# capstone-be
