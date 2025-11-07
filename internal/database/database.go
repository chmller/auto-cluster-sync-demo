package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
	_ "modernc.org/sqlite"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool for optimal SQLite performance
	// SQLite doesn't benefit from many open connections for writes
	conn.SetMaxOpenConns(5)  // Limit concurrent connections
	conn.SetMaxIdleConns(2)  // Keep few idle connections
	conn.SetConnMaxLifetime(0) // Reuse connections indefinitely

	db := &DB{conn: conn}

	// Apply SQLite optimizations for better concurrency and performance
	if err := db.optimizeSQLite(); err != nil {
		return nil, fmt.Errorf("failed to optimize SQLite: %w", err)
	}

	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// optimizeSQLite applies performance and concurrency optimizations
func (db *DB) optimizeSQLite() error {
	pragmas := []string{
		// Enable WAL mode for better concurrency (allows parallel reads during writes)
		"PRAGMA journal_mode=WAL",

		// Set busy timeout to 5 seconds (SQLite will retry instead of immediately failing)
		"PRAGMA busy_timeout=5000",

		// Use NORMAL synchronous mode (faster, still safe for most applications)
		"PRAGMA synchronous=NORMAL",

		// Increase cache size to ~40MB (10000 pages * 4KB)
		"PRAGMA cache_size=-40000",

		// Store temporary tables and indices in memory
		"PRAGMA temp_store=MEMORY",

		// Enable memory-mapped I/O (faster reads)
		"PRAGMA mmap_size=268435456", // 256MB
	}

	for _, pragma := range pragmas {
		if _, err := db.conn.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	return nil
}

// initSchema creates the database schema
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		extern_id TEXT NOT NULL,
		todo TEXT NOT NULL,
		completed BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		processing_status TEXT NOT NULL DEFAULT 'pending',
		claimed_by TEXT,
		claimed_at TIMESTAMP,
		last_heartbeat TIMESTAMP,
		processing_started_at TIMESTAMP,
		processing_completed_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at);
	CREATE INDEX IF NOT EXISTS idx_todos_completed ON todos(completed);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_todos_extern_id ON todos(extern_id);
	CREATE INDEX IF NOT EXISTS idx_todos_processing_status ON todos(processing_status);
	CREATE INDEX IF NOT EXISTS idx_todos_claimed_by ON todos(claimed_by);
	CREATE INDEX IF NOT EXISTS idx_todos_last_heartbeat ON todos(last_heartbeat);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateTodo creates a new todo item
func (db *DB) CreateTodo(externID, todo string) (*models.Todo, error) {
	result, err := db.conn.Exec(
		"INSERT INTO todos (extern_id, todo, completed, created_at) VALUES (?, ?, ?, ?)",
		externID, todo, false, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create todo: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return db.GetTodo(int(id))
}

// GetTodo retrieves a todo by ID
func (db *DB) GetTodo(id int) (*models.Todo, error) {
	var todo models.Todo
	err := db.conn.QueryRow(
		`SELECT id, extern_id, todo, completed, created_at, processing_status,
		 claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		 FROM todos WHERE id = ?`,
		id,
	).Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
		&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
		&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	return &todo, nil
}

// ListTodos retrieves all todos
func (db *DB) ListTodos() ([]models.Todo, error) {
	rows, err := db.conn.Query(
		`SELECT id, extern_id, todo, completed, created_at, processing_status,
		 claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		 FROM todos ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var todo models.Todo
		if err := rows.Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
			&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
			&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		todos = append(todos, todo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating todos: %w", err)
	}

	return todos, nil
}

// UpdateTodo updates a todo item
func (db *DB) UpdateTodo(id int, todo *string, completed *bool) (*models.Todo, error) {
	// First check if the todo exists
	existing, err := db.GetTodo(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Build dynamic update query
	query := "UPDATE todos SET "
	args := []interface{}{}
	updates := []string{}

	if todo != nil {
		updates = append(updates, "todo = ?")
		args = append(args, *todo)
	}
	if completed != nil {
		updates = append(updates, "completed = ?")
		args = append(args, *completed)
	}

	if len(updates) == 0 {
		// No updates, return existing
		return existing, nil
	}

	query += updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = ?"
	args = append(args, id)

	_, err = db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update todo: %w", err)
	}

	return db.GetTodo(id)
}

// DeleteTodo deletes a todo by ID
func (db *DB) DeleteTodo(id int) error {
	result, err := db.conn.Exec("DELETE FROM todos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetTodoByExternID retrieves a todo by external ID
func (db *DB) GetTodoByExternID(externID string) (*models.Todo, error) {
	var todo models.Todo
	err := db.conn.QueryRow(
		`SELECT id, extern_id, todo, completed, created_at, processing_status,
		 claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		 FROM todos WHERE extern_id = ?`,
		externID,
	).Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
		&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
		&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo by extern_id: %w", err)
	}

	return &todo, nil
}

// CountTodos returns the total number of todos
func (db *DB) CountTodos() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM todos").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count todos: %w", err)
	}
	return count, nil
}

// ClaimNextPendingTodo atomically claims the next pending todo (FIFO)
func (db *DB) ClaimNextPendingTodo(nodeID string) (*models.Todo, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Find oldest pending todo (FIFO)
	var todo models.Todo
	err = tx.QueryRow(`
		SELECT id, extern_id, todo, completed, created_at, processing_status,
		       claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		FROM todos
		WHERE processing_status = ? AND completed = 0
		ORDER BY created_at ASC
		LIMIT 1
	`, models.StatusPending).Scan(
		&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
		&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
		&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No pending jobs
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find pending todo: %w", err)
	}

	// Atomic claim
	now := time.Now()
	result, err := tx.Exec(`
		UPDATE todos
		SET processing_status = ?,
		    claimed_by = ?,
		    claimed_at = ?,
		    last_heartbeat = ?
		WHERE id = ? AND processing_status = ?
	`, models.StatusClaimed, nodeID, now, now, todo.ID, models.StatusPending)

	if err != nil {
		return nil, fmt.Errorf("failed to claim todo: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Someone else claimed it (race condition)
		return nil, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update local struct
	todo.ProcessingStatus = models.StatusClaimed
	todo.ClaimedBy = &nodeID
	todo.ClaimedAt = &now
	todo.LastHeartbeat = &now

	return &todo, nil
}

// UpdateJobStatus updates the processing status of a todo
func (db *DB) UpdateJobStatus(externID, status string) error {
	now := time.Now()
	_, err := db.conn.Exec(`
		UPDATE todos
		SET processing_status = ?,
		    processing_completed_at = CASE WHEN ? IN (?, ?) THEN ? ELSE processing_completed_at END
		WHERE extern_id = ?
	`, status, status, models.StatusCompleted, models.StatusFailed, now, externID)

	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	return nil
}

// SendHeartbeat updates the last_heartbeat timestamp for a job
func (db *DB) SendHeartbeat(externID, nodeID string) error {
	now := time.Now()
	result, err := db.conn.Exec(`
		UPDATE todos
		SET last_heartbeat = ?
		WHERE extern_id = ? AND claimed_by = ? AND processing_status IN (?, ?)
	`, now, externID, nodeID, models.StatusClaimed, models.StatusProcessing)

	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no job found or job not owned by this node")
	}

	return nil
}

// GetStaleJobs returns jobs that haven't sent a heartbeat within the timeout
func (db *DB) GetStaleJobs(timeout time.Duration) ([]models.Todo, error) {
	cutoff := time.Now().Add(-timeout)
	rows, err := db.conn.Query(`
		SELECT id, extern_id, todo, completed, created_at, processing_status,
		       claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		FROM todos
		WHERE processing_status IN (?, ?)
		  AND last_heartbeat < ?
	`, models.StatusClaimed, models.StatusProcessing, cutoff)

	if err != nil {
		return nil, fmt.Errorf("failed to get stale jobs: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var todo models.Todo
		if err := rows.Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
			&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
			&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt); err != nil {
			return nil, fmt.Errorf("failed to scan stale job: %w", err)
		}
		todos = append(todos, todo)
	}

	return todos, rows.Err()
}

// ReleaseJob releases a job back to pending status
func (db *DB) ReleaseJob(externID string) error {
	_, err := db.conn.Exec(`
		UPDATE todos
		SET processing_status = ?,
		    claimed_by = NULL,
		    claimed_at = NULL,
		    last_heartbeat = NULL,
		    processing_started_at = NULL
		WHERE extern_id = ?
	`, models.StatusPending, externID)

	if err != nil {
		return fmt.Errorf("failed to release job: %w", err)
	}
	return nil
}

// GetJobsByNode returns all jobs claimed by a specific node
func (db *DB) GetJobsByNode(nodeID string) ([]models.Todo, error) {
	rows, err := db.conn.Query(`
		SELECT id, extern_id, todo, completed, created_at, processing_status,
		       claimed_by, claimed_at, last_heartbeat, processing_started_at, processing_completed_at
		FROM todos
		WHERE claimed_by = ? AND processing_status IN (?, ?)
	`, nodeID, models.StatusClaimed, models.StatusProcessing)

	if err != nil {
		return nil, fmt.Errorf("failed to get jobs by node: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var todo models.Todo
		if err := rows.Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt,
			&todo.ProcessingStatus, &todo.ClaimedBy, &todo.ClaimedAt, &todo.LastHeartbeat,
			&todo.ProcessingStartedAt, &todo.ProcessingCompletedAt); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		todos = append(todos, todo)
	}

	return todos, rows.Err()
}

// MarkJobProcessing marks a job as actively processing
func (db *DB) MarkJobProcessing(externID string) error {
	now := time.Now()
	_, err := db.conn.Exec(`
		UPDATE todos
		SET processing_status = ?,
		    processing_started_at = ?
		WHERE extern_id = ? AND processing_status = ?
	`, models.StatusProcessing, now, externID, models.StatusClaimed)

	if err != nil {
		return fmt.Errorf("failed to mark job as processing: %w", err)
	}
	return nil
}

// CountJobsByStatus returns the count of jobs by processing status
func (db *DB) CountJobsByStatus(status string) (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM todos WHERE processing_status = ?", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs by status: %w", err)
	}
	return count, nil
}
