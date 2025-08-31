# All-in-One API Server

A modular Go API server with multiple domain packages and flexible storage backends.

## Architecture

This project demonstrates a modular, domain-driven design approach for a Go API server:

- Each domain (e.g., `listing`) is in its own package
- Each domain has its own storage interface with multiple implementations
- Main application wires everything together with dependency injection

### Project Structure

```
all-in-one/
├── main.go              # Application entry point
├── go.mod               # Go module definition
├── pkg/                 # Package directory
│   ├── common/          # Shared code across domains
│   │   └── common.go    # Common response types and errors
│   └── listing/         # Listing domain
│       ├── model.go     # Listing entity and storage interface
│       ├── memory.go    # In-memory storage implementation
│       ├── sqlite.go    # SQLite storage implementation
│       └── handler.go   # HTTP handlers for listing API
└── data/                # Database files (created at runtime)
```

## Features

- RESTful API with JSON responses
- Multiple storage backends (in-memory and SQLite)
- Domain-driven design with separate modules
- CORS support for frontend integration
- Easily extensible with new domains

## Available Endpoints

- Health Check:
  - `GET /api/v1/health` - API health status

- Listing API:
  - `GET /api/v1/items` - Get all items
  - `POST /api/v1/items` - Create new item
  - `GET /api/v1/items/{id}` - Get item by ID
  - `PUT /api/v1/items/{id}` - Update item
  - `DELETE /api/v1/items/{id}` - Delete item

## Storage Options

The application supports multiple storage backends:

1. **In-memory Storage** (default)
   - Stores data in memory, resets on server restart
   - Fast but not persistent

2. **SQLite Storage** (commented in main.go)
   - Stores data in SQLite database files
   - Persistent between server restarts
   - To use, uncomment the SQLite initialization in main.go

## Running the Server

```bash
go run main.go
```

The server will start on port 8080.

## Adding a New Domain

To add a new domain (e.g., `user`):

1. Create a new package `pkg/user/`
2. Create the following files:
   - `model.go` - Define the entity and storage interface
   - `memory.go` - Implement in-memory storage
   - `sqlite.go` - Implement SQLite storage
   - `handler.go` - Implement HTTP handlers
3. Update `main.go` to initialize and wire up the new domain

## Frontend Integration

The API is CORS-enabled for integration with a frontend application (e.g., Svelte).
