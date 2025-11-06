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

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the database schema
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		extern_id TEXT NOT NULL,
		todo TEXT NOT NULL,
		completed BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at);
	CREATE INDEX IF NOT EXISTS idx_todos_completed ON todos(completed);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_todos_extern_id ON todos(extern_id);
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
		"SELECT id, extern_id, todo, completed, created_at FROM todos WHERE id = ?",
		id,
	).Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt)

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
		"SELECT id, extern_id, todo, completed, created_at FROM todos ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var todo models.Todo
		if err := rows.Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt); err != nil {
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
		"SELECT id, extern_id, todo, completed, created_at FROM todos WHERE extern_id = ?",
		externID,
	).Scan(&todo.ID, &todo.ExternID, &todo.Todo, &todo.Completed, &todo.CreatedAt)

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
