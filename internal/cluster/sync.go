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

	log.Printf("[INFO] Broadcasted %s: %s", eventName, event.ExternID)
	return nil
}

// broadcastJobEvent sends a job event to the cluster
func (c *Cluster) broadcastJobEvent(eventName string, event JobEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal job event: %w", err)
	}

	err = c.serf.UserEvent(eventName, payload, false)
	if err != nil {
		return fmt.Errorf("failed to broadcast job event: %w", err)
	}

	log.Printf("[INFO] Broadcasted %s: %s (node: %s)", eventName, event.ExternID, event.NodeID)
	return nil
}

// BroadcastJobClaimed broadcasts that a job was claimed by this node
func (c *Cluster) BroadcastJobClaimed(todo *models.Todo) error {
	event := JobEvent{
		ExternID:  todo.ExternID,
		TodoID:    todo.ID,
		NodeID:    c.nodeID,
		Status:    models.StatusClaimed,
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobClaimed, event)
}

// BroadcastJobStarted broadcasts that a job started processing
func (c *Cluster) BroadcastJobStarted(todo *models.Todo) error {
	event := JobEvent{
		ExternID:  todo.ExternID,
		TodoID:    todo.ID,
		NodeID:    c.nodeID,
		Status:    models.StatusProcessing,
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobStarted, event)
}

// BroadcastJobHeartbeat broadcasts a heartbeat for an active job
func (c *Cluster) BroadcastJobHeartbeat(externID string) error {
	event := JobEvent{
		ExternID:  externID,
		NodeID:    c.nodeID,
		Status:    "heartbeat",
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobHeartbeat, event)
}

// BroadcastJobCompleted broadcasts that a job completed successfully
func (c *Cluster) BroadcastJobCompleted(todo *models.Todo) error {
	event := JobEvent{
		ExternID:  todo.ExternID,
		TodoID:    todo.ID,
		NodeID:    c.nodeID,
		Status:    models.StatusCompleted,
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobCompleted, event)
}

// BroadcastJobFailed broadcasts that a job failed
func (c *Cluster) BroadcastJobFailed(todo *models.Todo) error {
	event := JobEvent{
		ExternID:  todo.ExternID,
		TodoID:    todo.ID,
		NodeID:    c.nodeID,
		Status:    models.StatusFailed,
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobFailed, event)
}

// BroadcastJobReleased broadcasts that a job was released back to pending
func (c *Cluster) BroadcastJobReleased(todo *models.Todo) error {
	event := JobEvent{
		ExternID:  todo.ExternID,
		TodoID:    todo.ID,
		NodeID:    c.nodeID,
		Status:    models.StatusPending,
		Timestamp: time.Now().Unix(),
	}
	return c.broadcastJobEvent(EventJobReleased, event)
}
