# All-in-One API Server

A modular Go API server with a flexible storage backend implementation.

## Architecture

The application is designed with modularity in mind, using the following structure:

- **main.go**: The entry point that wires everything together
- **pkg/models**: Shared data structures across the application
- **pkg/storage**: Storage interfaces and implementations
  - `interface.go`: Defines the storage interface
  - `memory.go`: In-memory implementation
  - `sqlite.go`: SQLite implementation (optional)
- **pkg/listing**: The listing service implementation
  - `handler.go`: HTTP handlers for listing API endpoints

## Storage Backend

The application uses a flexible storage backend pattern through interfaces. You can switch between:

1. **Memory Storage** (default): Data is stored in memory and lost when the server stops
2. **SQLite Storage**: Data is persisted to an SQLite database file

To switch the storage backend, modify the `main.go` file:

```go
// For in-memory storage (default)
itemStore := storage.NewMemoryStorage()

// For SQLite storage (uncomment to use)
// itemStore, err := storage.NewSQLiteStorage("./data.db")
// if err != nil {
//     log.Fatalf("Failed to initialize SQLite storage: %v", err)
// }
// defer itemStore.(*storage.SQLiteStorage).Close()
```

## API Endpoints

The server exposes the following REST API endpoints:

- `GET /api/v1/health`: Health check endpoint
- `GET /api/v1/items`: Get all items
- `POST /api/v1/items`: Create a new item
- `GET /api/v1/items/{id}`: Get an item by ID
- `PUT /api/v1/items/{id}`: Update an item
- `DELETE /api/v1/items/{id}`: Delete an item

## Running the Server

```bash
go run main.go
```

The server will start on port 8080 by default.

## Extending the Application

To add more modules:

1. Create a new package under `/pkg`
2. Implement the necessary functionality
3. Wire it up in `main.go`

For example, to add a new "products" module:

1. Create `/pkg/products/` with handlers and models
2. Implement the storage interface for products
3. Register the routes in `main.go`
