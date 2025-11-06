# auto-cluster-sync-demo

Run multiple instances of a service and sync all data between all service instances.

## Features

* No external database required
* Auto clustering (planned)
* Auto failover (planned)
* Easy to use
* Minimal configuration

This service is a todo list REST API built with [Huma](https://huma.rocks) and SQLite.

## Quick Start

### Build

```bash
go build -o auto-cluster-sync ./cmd/server
```

### Run

```bash
# Default (port 8080, database: ./todos.db)
go run ./cmd/server

# With command line flags
go run ./cmd/server -port 3000 -db /tmp/todos.db

# Or with environment variables
PORT=3000 DB_PATH=/tmp/todos.db go run ./cmd/server

# Show help
go run ./cmd/server -h
```

### API Documentation

Interactive API documentation is available at:
```
http://localhost:8080/docs
```

## API Endpoints

### List all todos
```bash
curl http://localhost:8080/todos
```

### Get a specific todo
```bash
curl http://localhost:8080/todos/1
```

### Create a new todo
```bash
curl -X POST http://localhost:8080/todos \
  -H "Content-Type: application/json" \
  -d '{"todo": "Buy groceries"}'
```

### Update a todo
```bash
# Update text
curl -X PUT http://localhost:8080/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"todo": "Buy groceries and cook dinner"}'

# Mark as completed
curl -X PUT http://localhost:8080/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"completed": true}'

# Update both
curl -X PUT http://localhost:8080/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"todo": "Buy groceries and cook dinner", "completed": true}'
```

### Delete a todo
```bash
curl -X DELETE http://localhost:8080/todos/1
```

## Configuration

### Command Line Flags

- `-port` - HTTP server port (default: `8080`)
- `-db` - Database file path (default: `./todos.db`)

### Environment Variables

- `PORT` - HTTP port (default: `8080`)
- `DB_PATH` - SQLite database file path (default: `./todos.db`)

**Note:** Command line flags take precedence over environment variables.

## Project Structure

```
.
├── cmd/
│   └── server/          # Main application entry point
│       └── main.go
├── internal/
│   ├── api/             # HTTP API handlers and routes
│   │   └── api.go
│   ├── database/        # Database layer and CRUD operations
│   │   └── database.go
│   └── models/          # Data models
│       └── todo.go
└── go.mod
```

## Database Schema

The SQLite database contains a single `todos` table:

| Column     | Type      | Description                    |
|------------|-----------|--------------------------------|
| id         | INTEGER   | Primary key (auto-increment)   |
| todo       | TEXT      | Todo description               |
| completed  | BOOLEAN   | Whether the todo is completed  |
| created_at | TIMESTAMP | When the todo was created      |

## Roadmap

- [x] Basic REST API with CRUD operations
- [x] SQLite database integration
- [ ] Auto-clustering with service discovery
- [ ] Data synchronization between instances
- [ ] Auto failover support