package cluster

// Event types for todo synchronization
const (
	EventTodoCreated = "todo:created"
	EventTodoUpdated = "todo:updated"
	EventTodoDeleted = "todo:deleted"
)

// Event types for job management
const (
	EventJobClaimed   = "job:claimed"
	EventJobStarted   = "job:started"
	EventJobHeartbeat = "job:heartbeat"
	EventJobCompleted = "job:completed"
	EventJobFailed    = "job:failed"
	EventJobReleased  = "job:released"
)

// Query types for cluster communication
const (
	QueryFullState  = "sync:full-state"
	QueryCount      = "sync:count"
	QueryActiveLocks = "locks:active"
)

// TodoSyncEvent represents a todo synchronization event
type TodoSyncEvent struct {
	Type      string `json:"type"`       // "created", "updated", "deleted"
	ExternID  string `json:"extern_id"`
	Todo      string `json:"todo,omitempty"`
	Completed *bool  `json:"completed,omitempty"`
	NodeID    string `json:"node_id"`
	Timestamp int64  `json:"timestamp"`
}

// CountResponse represents a response to a count query
type CountResponse struct {
	Count  int    `json:"count"`
	NodeID string `json:"node_id"`
}

// JobEvent represents a job management event
type JobEvent struct {
	ExternID  string `json:"extern_id"`
	TodoID    int    `json:"todo_id"`
	NodeID    string `json:"node_id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}
