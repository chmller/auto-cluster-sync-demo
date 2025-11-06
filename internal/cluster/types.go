package cluster

// Event types for todo synchronization
const (
	EventTodoCreated = "todo:created"
	EventTodoUpdated = "todo:updated"
	EventTodoDeleted = "todo:deleted"
)

// Query types for cluster communication
const (
	QueryFullState = "sync:full-state"
	QueryCount     = "sync:count"
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
