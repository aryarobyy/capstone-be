.PHONY: run dev build tidy clean

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
