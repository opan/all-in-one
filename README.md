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
├── internal/            # Internal packages (not for external use)
│   ├── common/          # Shared code across domains
│   │   └── common.go    # Common response types and errors
│   └── listing/         # Listing domain
│       ├── service.go   # Main service integration point
│       └── pkg/         # Domain-specific packages
│           ├── model/   # Data models and interfaces
│           │   └── model.go
│           ├── storage/ # Storage implementations
│           │   ├── memory.go
│           │   └── sqlite.go
│           └── handler/ # HTTP handlers
│               └── handler.go
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

## Configuration

The application uses Viper for configuration management with the following priority order:
1. Environment variables (highest priority)
2. Configuration file (`config.yaml`)
3. Default values (lowest priority)

### Configuration Options

| Setting | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| Server Port | `ALLINONE_SERVER_PORT` | `:8080` | Port for the HTTP server |
| Storage Type | `ALLINONE_STORAGE_TYPE` | `memory` | Storage backend (`memory` or `sqlite`) |
| Storage Path | `ALLINONE_STORAGE_PATH` | `./data/listings.db` | SQLite database file path |

### Configuration File

Create a `config.yaml` file in the project root:

```yaml
server:
  port: ":8080"

storage:
  type: "memory"  # Options: "memory" or "sqlite"
  path: "./data/listings.db"  # Only used when type is "sqlite"
```

## Storage Options

The application supports multiple storage backends:

1. **In-memory Storage** (default)
   - Stores data in memory, resets on server restart
   - Fast but not persistent

2. **SQLite Storage** 
   - Stores data in SQLite database files
   - Persistent between server restarts
   - Configurable via config file or environment variables

## Running the Server

### Basic Usage

```bash
# Install dependencies
go mod tidy

# Run with default settings (memory storage, port 8080)
go run main.go
```

### With Configuration File

1. Create `config.yaml` (see Configuration section above)
2. Run the server:
```bash
go run main.go
```

### With Environment Variables

```bash
# Use SQLite storage
ALLINONE_STORAGE_TYPE=sqlite go run main.go

# Use custom port
ALLINONE_SERVER_PORT=:9000 go run main.go

# Use both custom storage and port
ALLINONE_STORAGE_TYPE=sqlite ALLINONE_STORAGE_PATH=./custom.db ALLINONE_SERVER_PORT=:3000 go run main.go
```

### Running the Frontend (Svelte)

```bash
# Navigate to UI directory
cd ui

# Install dependencies
npm install

# Start development server
npm run dev
```

The frontend will be available at `http://localhost:5173` and automatically proxy API requests to the backend.

## Frontend Integration

The API is CORS-enabled and includes:
- Vite proxy configuration for seamless frontend-backend communication
- Svelte routes for listing data at `/listing`
- Automatic data loading and table display

## Adding a New Domain

To add a new domain (e.g., `user`):

1. Create a new directory structure under `internal/user/`:
   ```
   internal/user/
   ├── service.go        # Main service integration point
   └── pkg/              # Domain-specific packages
       ├── model/        # Data models and interfaces
       │   └── model.go
       ├── storage/      # Storage implementations
       │   ├── memory.go
       │   └── sqlite.go
       └── handler/      # HTTP handlers
           └── handler.go
   ```

2. Implement the required components:
   - `model.go` - Define the entity and storage interface
   - `memory.go` and `sqlite.go` - Implement storage backends
   - `handler.go` - Implement HTTP handlers
   - `service.go` - Create the service that ties everything together

3. Update `main.go` to initialize and wire up the new domain service

## Frontend Integration

The API is CORS-enabled for integration with a frontend application (e.g., Svelte).
