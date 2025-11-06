# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a distributed todo service demonstrating auto-clustering and data synchronization between service instances without requiring an external database. The service uses:

- **SQLite** (`modernc.org/sqlite`) as the embedded database (pure Go, no CGO required)
- **Huma v2** (`github.com/danielgtaylor/huma/v2`) for the REST API framework
- **Chi v5** (`github.com/go-chi/chi/v5`) as the HTTP router
- **Serf** (`github.com/hashicorp/serf`) for auto-clustering, service discovery, and coordination
- Auto-failover for high availability via SWIM failure detection
- Data synchronization across all service instances via gossip protocol

## Dependencies

Core dependencies:
- `github.com/danielgtaylor/huma/v2` - Modern REST API framework with auto-validation and OpenAPI generation
- `github.com/danielgtaylor/huma/v2/adapters/humachi` - Chi adapter for Huma
- `github.com/go-chi/chi/v5` - Lightweight HTTP router
- `modernc.org/sqlite` - Pure Go SQLite implementation (no CGO)
- `github.com/hashicorp/serf` - Service discovery and orchestration via gossip protocol
- `gopkg.in/yaml.v3` - YAML configuration file parsing

## Current Implementation Status

**Completed:**
1. **REST API Layer**: Huma-based HTTP server exposing todo CRUD operations
2. **Data Layer**: SQLite database for local storage with full CRUD operations
3. **Clustering Layer**: Serf-based service discovery and membership management
4. **Sync Layer**: Automatic replication of database changes across all cluster members
5. **Failover Logic**: Automatic node failure detection and removal via SWIM protocol
6. **Configuration**: YAML-based config with seed node discovery and log level control
7. **Startup Guarantee**: Blocking synchronization on startup before accepting requests
8. **Health Endpoints**: Readiness probe and cluster info endpoints for monitoring
9. **Graceful Shutdown**: Idempotent cluster stop with proper cleanup

## Architecture

### Implemented Components

**Package Structure:**
- `cmd/server/main.go` - Entry point, config loading, cluster & HTTP server setup
- `internal/config/config.go` - YAML configuration loading and validation
- `internal/cluster/` - Serf cluster management
  - `cluster.go` - Serf initialization, join, leave logic
  - `events.go` - Event handlers (member join/leave, user events)
  - `sync.go` - Broadcast methods for todo synchronization
  - `queries.go` - Query handlers for full state transfer
  - `types.go` - Event and message type definitions
- `internal/api/api.go` - Huma API handlers with cluster integration
- `internal/database/database.go` - SQLite operations and schema management
- `internal/models/todo.go` - Data models, request/response types, and cluster types (ClusterMemberInfo)

**Data Flow (with Clustering):**
1. HTTP request arrives at Chi router
2. Huma validates request and deserializes to typed input structs
3. API handler calls database layer
4. Database executes SQLite query and returns model
5. API handler broadcasts event to cluster (via Serf UserEvent)
6. Other nodes receive event and update their local databases
7. API handler returns typed response
8. Huma serializes response to JSON

**Database Schema:**
```sql
CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    extern_id TEXT NOT NULL,
    todo TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_todos_created_at ON todos(created_at);
CREATE INDEX idx_todos_completed ON todos(completed);
CREATE UNIQUE INDEX idx_todos_extern_id ON todos(extern_id);
```

### Clustering Architecture (Serf)

**Serf Protocol:**
- SWIM (Scalable Weakly-consistent Infection-style Process Group Membership)
- Gossip-based communication for service discovery and event propagation
- UDP-based for lightweight messaging
- Eventual consistency model

**Cluster Discovery:**
- Seed nodes configured in YAML config file
- Nodes attempt to join via seed list on startup
- Only one reachable seed needed - gossip handles the rest
- Automatic full mesh topology

**Event Types:**
1. **Member Events** (automatic):
   - `MemberJoin` - New node joined cluster
   - `MemberLeave` - Node left gracefully
   - `MemberFailed` - Node failed (timeout)
   - `MemberUpdate` - Node metadata changed

2. **User Events** (custom):
   - `todo:created` - Todo created on a node
   - `todo:updated` - Todo updated on a node
   - `todo:deleted` - Todo deleted on a node

3. **Queries** (request/response):
   - `sync:full-state` - Request all todos from nodes
   - `sync:count` - Request todo count for consistency check

**Synchronization Flow:**
1. User creates todo via POST /todos on Node 1
2. Node 1 writes to local SQLite
3. Node 1 broadcasts `todo:created` event via Serf
4. Gossip protocol propagates event to all nodes (typically <1s)
5. Node 2 & 3 receive event
6. Event handler checks `extern_id` for idempotency
7. If new: write to local SQLite
8. If duplicate: skip (already synced)

**Full Sync (New Node):**
- New node detects `MemberJoin` event for itself
- Sends `sync:full-state` Query to all nodes
- Collects all todos from responses
- Deduplicates via `extern_id`
- Bulk insert into local database

**Startup Guarantee:**
- HTTP server **blocks** during startup until full sync is complete
- `Cluster.Start()` waits for `requestFullSync()` to finish (max 30s timeout)
- Ready state is tracked via internal channel (`readyCh`)
- If timeout occurs, node continues anyway (fail-open behavior)
- Health endpoint `/health/ready` returns 503 until node is ready
- Prevents serving incomplete data to clients during startup

**Conflict Resolution:**
- `extern_id` is globally unique (provided by client)
- UNIQUE constraint in database prevents duplicates
- Last-write-wins for updates (based on timestamp)
- Tombstones for deletes (event propagation)

## Development Commands

### Building
```bash
go build -o auto-cluster-sync ./cmd/server
```

### Running

**Single Node (no clustering):**
```bash
# Default configuration
go run ./cmd/server

# With command line flags
go run ./cmd/server -port 3000 -db /tmp/todos.db -node-name my-node -serf-addr 0.0.0.0:7946
```

**Cluster (with config files):**
```bash
# Terminal 1 - Node 1
go run ./cmd/server -config configs/local_1.yaml

# Terminal 2 - Node 2
go run ./cmd/server -config configs/local_2.yaml

# Terminal 3 - Node 3
go run ./cmd/server -config configs/local_3.yaml
```

**Override config with flags:**
```bash
go run ./cmd/server -config configs/local_1.yaml -port 9090
```

### Testing
```bash
go test ./...
```

Run a specific test:
```bash
go test ./path/to/package -run TestName
```

### Testing Cluster Sync

1. Start 3 nodes using configs
2. Check cluster status:
```bash
# Check readiness
curl http://localhost:8080/health/ready
curl http://localhost:8081/health/ready
curl http://localhost:8082/health/ready

# Get detailed cluster info
curl http://localhost:8080/health/info | jq
curl http://localhost:8081/health/info | jq
curl http://localhost:8082/health/info | jq
```

3. Create a todo on node 1:
```bash
curl -X POST http://localhost:8080/todos \
  -H "Content-Type: application/json" \
  -d '{"extern_id": "test-1", "todo": "Test cluster sync"}'
```

4. Check that it's synced to other nodes:
```bash
curl http://localhost:8081/todos  # Node 2
curl http://localhost:8082/todos  # Node 3

# Verify todo counts match across all nodes
curl http://localhost:8080/health/info | jq '.todo_count'
curl http://localhost:8081/health/info | jq '.todo_count'
curl http://localhost:8082/health/info | jq '.todo_count'
```

## Configuration

The application supports YAML config files and command line flags.

**Command Line Flags:**
- `-config` - Path to YAML configuration file
- `-port` - HTTP server port (overrides config)
- `-db` - Path to SQLite database file (overrides config)
- `-node-name` - Node name for cluster (overrides config)
- `-serf-addr` - Serf bind address (overrides config)

**YAML Configuration Format:**
```yaml
# Log level: debug, info, warn, error (default: info)
log_level: "info"

node:
  name: "node-1"
  serf:
    bind_addr: "127.0.0.1:7946"
    advertise_addr: ""  # Optional: external address
  http:
    port: 8080
  database:
    path: "./todos-node1.db"

cluster:
  seeds:
    - "127.0.0.1:7946"
    - "127.0.0.1:7947"
    - "127.0.0.1:7948"
  join_timeout: 10  # seconds
  encrypt_key: ""   # Optional: Serf encryption key
```

**Priority order:** Command line flags > Config file > Defaults

**Config Files Location:** `configs/` directory contains example configs:
- `local_1.yaml`, `local_2.yaml`, `local_3.yaml` - 3-node local cluster

**Logging Configuration:**
- Supported log levels: `debug`, `info`, `warn`, `error`
- Default: `info`
- Uses structured logging (slog) with text format
- Example: Set `log_level: "debug"` in config for verbose logging

## API Endpoints

All endpoints are implemented using Huma v2 with automatic validation and OpenAPI documentation.

**Implemented Endpoints:**
- `GET /health/ready` - Health check / readiness probe
  - Returns: 200 OK with `{"ready": true}` when node is fully synced
  - Returns: 503 Service Unavailable with `{"ready": false}` when still syncing
  - Use case: Load balancer health checks, Kubernetes readiness probes
- `GET /health/info` - Cluster status and member information
  - Returns: Cluster status including node name, ready state, member count, member list, and todo count
  - Response includes: `node_name`, `ready`, `cluster_mode`, `member_count`, `members[]`, `todo_count`
  - Use case: Monitoring, debugging, cluster overview dashboards
- `GET /todos` - List all todos (returns empty array if none exist)
- `GET /todos/{id}` - Get a specific todo (404 if not found)
- `POST /todos` - Create a new todo
  - Request body: `{"extern_id": "unique-id", "todo": "description"}`
    - `extern_id`: External ID for synchronization (1-80 characters, required)
    - `todo`: Todo description (1-500 characters, required)
  - Returns: Created todo with generated ID and timestamp
- `PUT /todos/{id}` - Update a todo (partial updates supported)
  - Request body: `{"todo": "...", "completed": true}` (either field optional)
  - Returns: Updated todo (404 if not found)
  - Note: `extern_id` is immutable and cannot be updated
- `DELETE /todos/{id}` - Delete a todo (204 on success, 404 if not found)

**API Documentation:**
Interactive OpenAPI documentation is automatically generated at `/docs`

## Database Operations

**Implemented in `internal/database/database.go`:**
- `New(dbPath)` - Creates database connection and initializes schema
- `CreateTodo(externID, todo)` - Inserts new todo with external ID, returns created record
- `GetTodo(id)` - Retrieves single todo by ID
- `GetTodoByExternID(externID)` - Retrieves todo by extern_id (for cluster sync idempotency)
- `ListTodos()` - Returns all todos ordered by created_at DESC
- `UpdateTodo(id, todo, completed)` - Partial update support (extern_id is immutable)
- `DeleteTodo(id)` - Removes todo by ID
- `CountTodos()` - Returns total count (for consistency checks)

**Schema Notes:**
- `extern_id` has a UNIQUE index for fast lookups during synchronization
- All records must have a unique `extern_id` (enforced by database constraint)
- `extern_id` is provided by the client and must be globally unique
- Each node has its own SQLite database with identical schema

## Cluster Operations

**Implemented in `internal/cluster/`:**

**Broadcasting (sync.go):**
- `BroadcastTodoCreated(todo)` - Broadcasts todo creation to all nodes
- `BroadcastTodoUpdated(todo)` - Broadcasts todo update to all nodes
- `BroadcastTodoDeleted(externID)` - Broadcasts todo deletion to all nodes

**Event Handling (events.go):**
- `handleTodoCreated()` - Receives and processes todo created events
- `handleTodoUpdated()` - Receives and processes todo updated events
- `handleTodoDeleted()` - Receives and processes todo deleted events
- Idempotency via `GetTodoByExternID()` check

**Queries (queries.go):**
- `handleFullStateQuery()` - Responds with all todos for new nodes
- `handleCountQuery()` - Responds with todo count for consistency checks
- `requestFullSync()` - Requests full state from all nodes on join

**State Management (cluster.go):**
- `IsReady()` - Returns true if node is ready to serve requests (fully synced)
- `markReady()` - Marks node as ready and signals waiting goroutines
- `Start()` - Blocks until full sync complete or 30s timeout
- `Stop()` - Idempotent graceful shutdown (can be called multiple times safely)
- `LocalNode()` - Returns the name of the local node
- `MemberCount()` - Returns the number of cluster members
- `GetMemberInfo()` - Returns detailed information about all cluster members (name, address, status)

**Technical Implementation Details:**
- **Bind Address Parsing**: `New()` parses "IP:Port" format using `net.SplitHostPort()` and sets `BindAddr` and `BindPort` separately for Memberlist config
- **Idempotent Shutdown**: `Stop()` uses a `stopped` boolean flag to prevent double-close of channels
- **Structured Logging**: Uses Go 1.21+ `log/slog` with configurable levels (debug/info/warn/error)
- **Blocking Startup**: `Start()` waits on `readyCh` channel until `requestFullSync()` completes

## Synchronization Strategy (Implemented)

**Event-based Replication via Serf:**
1. Changes broadcast as User Events (`todo:created`, `todo:updated`, `todo:deleted`)
2. Gossip protocol ensures eventual consistency (typically <1 second)
3. Idempotency via `extern_id` UNIQUE constraint
4. Last-Write-Wins for conflict resolution (timestamp-based)

**Advantages:**
- No central coordinator required
- Scales well (gossip is O(log N))
- Automatic failure detection
- Low latency synchronization

**Trade-offs:**
- Eventual consistency (not strong consistency)
- Possible temporary inconsistencies during network partitions
- Requires globally unique `extern_id` from clients

## Production Considerations

**Security:**
- Add Serf encryption via `encrypt_key` in config for production
- Use TLS for HTTP API endpoints
- Validate and sanitize `extern_id` input

**Monitoring:**
- Use `/health/ready` endpoint for readiness probes (Kubernetes, load balancers)
- Use `/health/info` endpoint for cluster status monitoring
- Monitor `todo_count` consistency across nodes via `/health/info`
- Serf provides internal cluster health metrics
- Monitor gossip convergence time
- Track sync event latency
- Adjust `log_level` to `debug` for troubleshooting

**Split-Brain Scenarios:**
- Network partitions can cause temporary divergence
- Nodes in different partitions continue to accept writes
- Once network heals, events propagate and converge
- `extern_id` prevents duplicate creates
- Last-write-wins for updates (based on timestamp)

**Performance:**
- Gossip scales logarithmically with cluster size
- SQLite suitable for <100k todos per node
- For larger datasets: Consider partitioning or distributed DB