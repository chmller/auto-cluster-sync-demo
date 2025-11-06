# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a distributed todo service demonstrating auto-clustering and data synchronization between service instances without requiring an external database. The service uses:

- **SQLite** (`modernc.org/sqlite`) as the embedded database (pure Go, no CGO required)
- **Huma v2** (`github.com/danielgtaylor/huma/v2`) for the REST API framework
- **Chi v5** (`github.com/go-chi/chi/v5`) as the HTTP router
- Auto-clustering for service discovery and coordination (planned)
- Auto-failover for high availability (planned)
- Data synchronization across all service instances (planned)

## Dependencies

Core dependencies:
- `github.com/danielgtaylor/huma/v2` - Modern REST API framework with auto-validation and OpenAPI generation
- `github.com/danielgtaylor/huma/v2/adapters/humachi` - Chi adapter for Huma
- `github.com/go-chi/chi/v5` - Lightweight HTTP router
- `modernc.org/sqlite` - Pure Go SQLite implementation (no CGO)

## Current Implementation Status

**Completed:**
1. **REST API Layer**: Huma-based HTTP server exposing todo CRUD operations
2. **Data Layer**: SQLite database for local storage with full CRUD operations

**Planned (not yet implemented):**
3. **Clustering Layer**: Service discovery and membership management
4. **Sync Layer**: Replicates database changes across all cluster members
5. **Failover Logic**: Detects node failures and maintains cluster health

## Architecture

### Implemented Components

**Package Structure:**
- `cmd/server/main.go` - Entry point, HTTP server setup with graceful shutdown
- `internal/api/api.go` - Huma API handlers and route registration
- `internal/database/database.go` - SQLite operations and schema management
- `internal/models/todo.go` - Data models and request/response types

**Data Flow:**
1. HTTP request arrives at Chi router
2. Huma validates request and deserializes to typed input structs
3. API handler calls database layer
4. Database executes SQLite query and returns model
5. API handler returns typed response
6. Huma serializes response to JSON

**Database Schema:**
```sql
CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    todo TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Future Architectural Considerations

When implementing clustering and sync:
- Each instance runs independently with its own SQLite database
- Changes to one instance's database will be propagated to all other instances
- The cluster should be self-organizing (no manual configuration of peers)
- Split-brain scenarios and conflict resolution must be handled

## Development Commands

### Building
```bash
go build -o auto-cluster-sync ./cmd/server
```

### Running
```bash
# Default configuration
go run ./cmd/server

# With command line flags
go run ./cmd/server -port 3000 -db /tmp/todos.db

# With environment variables
PORT=3000 DB_PATH=/tmp/todos.db go run ./cmd/server

# Show help
go run ./cmd/server -h
```

### Testing
```bash
go test ./...
```

Run a specific test:
```bash
go test ./path/to/package -run TestName
```

### Running Multiple Instances
To test clustering, run multiple instances on different ports:
```bash
# Terminal 1
go run ./cmd/server -port 8080 -db /tmp/todos1.db

# Terminal 2
go run ./cmd/server -port 8081 -db /tmp/todos2.db

# Terminal 3
go run ./cmd/server -port 8082 -db /tmp/todos3.db
```

## Configuration

The application supports both command line flags and environment variables for configuration.

**Command Line Flags** (take precedence):
- `-port` - HTTP server port (default: `8080`)
- `-db` - Path to SQLite database file (default: `./todos.db`)

**Environment Variables**:
- `PORT` - HTTP server port (default: `8080`)
- `DB_PATH` - Path to SQLite database file (default: `./todos.db`)

Priority order: Command line flags > Environment variables > Defaults

## API Endpoints

All endpoints are implemented using Huma v2 with automatic validation and OpenAPI documentation.

**Implemented Endpoints:**
- `GET /todos` - List all todos (returns empty array if none exist)
- `GET /todos/{id}` - Get a specific todo (404 if not found)
- `POST /todos` - Create a new todo
  - Request body: `{"todo": "description"}` (1-500 characters)
  - Returns: Created todo with generated ID and timestamp
- `PUT /todos/{id}` - Update a todo (partial updates supported)
  - Request body: `{"todo": "...", "completed": true}` (either field optional)
  - Returns: Updated todo (404 if not found)
- `DELETE /todos/{id}` - Delete a todo (204 on success, 404 if not found)

**API Documentation:**
Interactive OpenAPI documentation is automatically generated at `/docs`

## Database Operations

**Implemented in `internal/database/database.go`:**
- `New(dbPath)` - Creates database connection and initializes schema
- `CreateTodo(todo)` - Inserts new todo, returns created record
- `GetTodo(id)` - Retrieves single todo by ID
- `ListTodos()` - Returns all todos ordered by created_at DESC
- `UpdateTodo(id, todo, completed)` - Partial update support
- `DeleteTodo(id)` - Removes todo by ID

**Future Schema Extensions:**
When implementing sync, consider adding:
- Metadata for conflict resolution (version vectors, timestamps, or similar)
- Sync log/changelog table to track operations
- Node ID to track origin of changes

## Synchronization Strategy

Consider implementing one of:
- **Event Log Replication**: Each change is logged and replayed on other nodes
- **State-based CRDT**: Use conflict-free replicated data types
- **Operation-based CRDT**: Broadcast operations that commute
- **Last-Write-Wins with Vector Clocks**: Simple but may lose updates

## Clustering Technology Options

Potential libraries for auto-clustering:
- **Hashicorp Memberlist** (gossip protocol, SWIM-based)
- **Hashicorp Serf** (higher-level than memberlist)
- **etcd/raft** (consensus-based, stronger consistency)
- Custom gossip implementation

## Key Implementation Challenges

1. **Conflict Resolution**: When two nodes modify the same todo simultaneously
2. **Network Partitions**: Handling split-brain scenarios
3. **Bootstrap Problem**: First node vs. joining existing cluster
4. **Tombstones**: Handling deletes in a distributed system
5. **Compaction**: Managing sync log growth over time