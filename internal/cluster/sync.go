package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
)

// BroadcastTodoCreated broadcasts a todo created event to the cluster
func (c *Cluster) BroadcastTodoCreated(todo *models.Todo) error {
	event := TodoSyncEvent{
		Type:      "created",
		ExternID:  todo.ExternID,
		Todo:      todo.Todo,
		Completed: &todo.Completed,
		NodeID:    c.nodeID,
		Timestamp: time.Now().Unix(),
	}

	return c.broadcastEvent(EventTodoCreated, event)
}

// BroadcastTodoUpdated broadcasts a todo updated event to the cluster
func (c *Cluster) BroadcastTodoUpdated(todo *models.Todo) error {
	event := TodoSyncEvent{
		Type:      "updated",
		ExternID:  todo.ExternID,
		Todo:      todo.Todo,
		Completed: &todo.Completed,
		NodeID:    c.nodeID,
		Timestamp: time.Now().Unix(),
	}

	return c.broadcastEvent(EventTodoUpdated, event)
}

// BroadcastTodoDeleted broadcasts a todo deleted event to the cluster
func (c *Cluster) BroadcastTodoDeleted(externID string) error {
	event := TodoSyncEvent{
		Type:      "deleted",
		ExternID:  externID,
		NodeID:    c.nodeID,
		Timestamp: time.Now().Unix(),
	}

	return c.broadcastEvent(EventTodoDeleted, event)
}

// broadcastEvent sends a user event to the cluster
func (c *Cluster) broadcastEvent(eventName string, event TodoSyncEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = c.serf.UserEvent(eventName, payload, false)
	if err != nil {
		return fmt.Errorf("failed to broadcast event: %w", err)
	}

	log.Printf("📤 Broadcasted %s: %s", eventName, event.ExternID)
	return nil
}
