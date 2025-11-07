# auto-cluster-sync-demo

Run multiple instances of a service and sync all data between all service instances.

## Features

* No external database required
* Auto-clustering with Serf gossip protocol
* Automatic data synchronization across all nodes
* Auto failover and failure detection
* Startup synchronization guarantee
* Health check endpoints for monitoring
* Configurable log levels
* Easy to use
* Minimal configuration

This service is a distributed todo list REST API built with [Huma](https://huma.rocks), SQLite, and [Serf](https://www.serf.io/).

## Quick Start

### Build

```bash
go build -o auto-cluster-sync ./cmd/server
```

### Run

```bash
# Standalone mode (port 8080, database: ./todos.db)
./auto-cluster-sync

# With command line flags
./auto-cluster-sync -port 3000 -db /tmp/todos.db

# With configuration file
./auto-cluster-sync -config configs/local_1.yaml

# Show help
./auto-cluster-sync -h
```

### API Documentation

Interactive API documentation is available at:
```
http://localhost:8080/docs
```

## API Endpoints

### Health Check
```bash
# Check if node is ready (fully synced)
curl http://localhost:8080/health/ready
```

Returns 200 OK when ready, 503 Service Unavailable when still syncing.

### Cluster Info
```bash
# Get cluster status and member information
curl http://localhost:8080/health/info
```

Returns detailed information about:
- Node name and ready status
- Cluster mode (enabled/disabled)
- Number of cluster members
- List of all members with their status
- Number of todos in local database

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
  -d '{"extern_id": "unique-id-123", "todo": "Buy groceries"}'
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

### Configuration File (YAML)

Create a YAML config file with the following structure:

```yaml
# Log level: debug, info, warn, error (default: info)
log_level: "info"

node:
  name: "node-1"
  serf:
    bind_addr: "127.0.0.1:7946"
  http:
    port: 8080
  database:
    path: "./todos-node1.db"

cluster:
  seeds:
    - "127.0.0.1:7946"
  join_timeout: 10
```

### Command Line Flags

- `-config` - Path to YAML configuration file
- `-port` - HTTP server port (overrides config, default: `8080`)
- `-db` - Database file path (overrides config, default: `./todos.db`)
- `-node-name` - Node name (overrides config)
- `-serf-addr` - Serf bind address (overrides config)
- `-keygen` - Generate encryption key for Serf cluster and exit

**Note:** Command line flags take precedence over config file values.

## Project Structure

```
.
├── cmd/
│   └── server/          # Main application entry point
│       └── main.go
├── internal/
│   ├── api/             # HTTP API handlers and routes
│   │   └── api.go
│   ├── cluster/         # Serf cluster management
│   │   ├── cluster.go   # Cluster lifecycle and state
│   │   ├── events.go    # Event handlers
│   │   ├── sync.go      # Broadcasting operations
│   │   ├── queries.go   # Full state sync
│   │   └── types.go     # Event type definitions
│   ├── config/          # Configuration loading
│   │   └── config.go
│   ├── database/        # Database layer and CRUD operations
│   │   └── database.go
│   └── models/          # Data models
│       └── todo.go
├── configs/             # Example configuration files
│   ├── local_1.yaml
│   ├── local_2.yaml
│   └── local_3.yaml
└── go.mod
```

## Database Schema

The SQLite database contains a single `todos` table:

| Column     | Type      | Description                               |
|------------|-----------|-------------------------------------------|
| id         | INTEGER   | Primary key (auto-increment)              |
| extern_id  | TEXT      | External ID for synchronization (unique)  |
| todo       | TEXT      | Todo description                          |
| completed  | BOOLEAN   | Whether the todo is completed             |
| created_at | TIMESTAMP | When the todo was created                 |

## Clustering

The application supports clustering using Hashicorp Serf for service discovery and todo synchronization.

### Generating Encryption Keys (Optional)

For production deployments, you can enable Serf encryption by generating a secure encryption key:

```bash
# Generate a new encryption key
./auto-cluster-sync -keygen
```

This command generates a cryptographically secure 32-byte (256-bit) encryption key and displays it with usage instructions. Add the generated key to the `cluster.encrypt_key` field in your configuration file.

**Important:** All nodes in the cluster must use the same encryption key to communicate securely.

### Running a Cluster

Start multiple nodes using the provided configuration files:

```bash
# Terminal 1 - Node 1
./auto-cluster-sync -config configs/local_1.yaml

# Terminal 2 - Node 2
./auto-cluster-sync -config configs/local_2.yaml

# Terminal 3 - Node 3
./auto-cluster-sync -config configs/local_3.yaml
```

Each node will:
1. Initialize its local SQLite database
2. Join the cluster via seed nodes
3. Request full sync from existing nodes
4. Wait for sync to complete (max 30s)
5. Start HTTP server and accept requests

### How It Works

- **Service Discovery**: Nodes discover each other via Serf gossip protocol
- **Data Sync**: Todo CRUD operations are automatically synchronized across all nodes
- **Idempotency**: `extern_id` ensures todos are not duplicated across nodes
- **Full Sync**: New nodes automatically request full state from existing nodes
- **Startup Guarantee**: HTTP server only starts after full sync is complete (max 30s timeout)
- **Failure Detection**: Failed nodes are automatically detected and removed
- **Health Check**: `/health/ready` endpoint returns 503 until node is fully synced

### Creating Todos in a Cluster

When you create a todo on any node, it's automatically synced to all other nodes:

```bash
# Create on node 1
curl -X POST http://localhost:8080/todos \
  -H "Content-Type: application/json" \
  -d '{"extern_id": "todo-1", "todo": "Buy milk"}'

# Todo is now available on all nodes:
curl http://localhost:8081/todos  # Node 2
curl http://localhost:8082/todos  # Node 3
```

## Roadmap

- [x] Basic REST API with CRUD operations
- [x] SQLite database integration
- [x] Auto-clustering with service discovery (Serf)
- [x] Data synchronization between instances
- [x] Auto failover support (Serf failure detection)
- [x] Startup synchronization guarantee
- [x] Health check endpoints (`/health/ready`, `/health/info`)
- [x] Configurable log levels
- [x] YAML configuration support
- [x] Serf encryption key generation tool
- [ ] Metrics and monitoring (Prometheus)
- [ ] TLS support for HTTP API