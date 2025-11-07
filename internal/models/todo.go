package models

import "time"

// Todo represents a todo item in the system
type Todo struct {
	ID                   int        `json:"id" db:"id"`
	ExternID             string     `json:"extern_id" db:"extern_id"`
	Todo                 string     `json:"todo" db:"todo"`
	Completed            bool       `json:"completed" db:"completed"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	ProcessingStatus     string     `json:"processing_status" db:"processing_status"`
	ClaimedBy            *string    `json:"claimed_by,omitempty" db:"claimed_by"`
	ClaimedAt            *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`
	LastHeartbeat        *time.Time `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	ProcessingStartedAt  *time.Time `json:"processing_started_at,omitempty" db:"processing_started_at"`
	ProcessingCompletedAt *time.Time `json:"processing_completed_at,omitempty" db:"processing_completed_at"`
}

// ProcessingStatus constants
const (
	StatusPending    = "pending"
	StatusClaimed    = "claimed"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// CreateTodoInput represents the input for creating a new todo
type CreateTodoInput struct {
	ExternID string `json:"extern_id" minLength:"1" maxLength:"80" doc:"External ID for synchronization"`
	Todo     string `json:"todo" minLength:"1" maxLength:"500" doc:"The todo description"`
}

// UpdateTodoInput represents the input for updating a todo
type UpdateTodoInput struct {
	Todo      *string `json:"todo,omitempty" minLength:"1" maxLength:"500" doc:"The todo description"`
	Completed *bool   `json:"completed,omitempty" doc:"Whether the todo is completed"`
}

// ClusterMemberInfo represents cluster member information
type ClusterMemberInfo struct {
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Status string `json:"status"`
}
