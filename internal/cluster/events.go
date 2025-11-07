package cluster

import (
	"encoding/json"
	"log"

	"github.com/hashicorp/serf/serf"
)

// handleEvents processes Serf events from the event channel
func (c *Cluster) handleEvents() {
	for {
		select {
		case event := <-c.eventCh:
			switch e := event.(type) {
			case serf.MemberEvent:
				c.handleMemberEvent(e)
			case serf.UserEvent:
				c.handleUserEvent(e)
			case *serf.Query:
				c.handleQuery(e)
			default:
				log.Printf("[WARN] Unknown event type: %T", e)
			}
		case <-c.shutdown:
			log.Printf("[INFO] Event handler shutting down")
			return
		}
	}
}

// handleMemberEvent handles cluster membership events
func (c *Cluster) handleMemberEvent(event serf.MemberEvent) {
	for _, member := range event.Members {
		switch event.Type {
		case serf.EventMemberJoin:
			log.Printf("[INFO] Node joined: %s (%s)", member.Name, member.Addr)

			// If I'm the new node, request full sync
			if member.Name == c.nodeID {
				log.Printf("[INFO] I'm the new node, requesting full sync...")
				go c.requestFullSync()
			}

		case serf.EventMemberLeave:
			log.Printf("[INFO] Node left gracefully: %s", member.Name)

		case serf.EventMemberFailed:
			log.Printf("[WARN] Node failed: %s", member.Name)
			// Reclaim jobs from failed node
			go c.reclaimJobsFromNode(member.Name)

		case serf.EventMemberUpdate:
			log.Printf("[INFO] Node updated: %s", member.Name)

		case serf.EventMemberReap:
			log.Printf("[INFO] Node reaped: %s", member.Name)
		}
	}
}

// handleUserEvent handles custom user events (todo sync and job management)
func (c *Cluster) handleUserEvent(event serf.UserEvent) {
	// Skip events from myself
	if event.Name == c.nodeID {
		return
	}

	switch event.Name {
	case EventTodoCreated:
		c.handleTodoCreated(event.Payload)
	case EventTodoUpdated:
		c.handleTodoUpdated(event.Payload)
	case EventTodoDeleted:
		c.handleTodoDeleted(event.Payload)
	case EventJobClaimed:
		c.handleJobClaimed(event.Payload)
	case EventJobStarted:
		c.handleJobStarted(event.Payload)
	case EventJobHeartbeat:
		c.handleJobHeartbeat(event.Payload)
	case EventJobCompleted:
		c.handleJobCompleted(event.Payload)
	case EventJobFailed:
		c.handleJobFailed(event.Payload)
	case EventJobReleased:
		c.handleJobReleased(event.Payload)
	default:
		log.Printf("[WARN] Unknown user event: %s", event.Name)
	}
}

// handleTodoCreated processes a todo created event
func (c *Cluster) handleTodoCreated(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal todo created event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Received todo created: %s from %s", event.ExternID, event.NodeID)

	// Check if todo already exists (idempotency)
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("[ERROR] Failed to check existing todo: %v", err)
		return
	}

	if existing != nil {
		log.Printf("[INFO] Todo %s already exists, skipping", event.ExternID)
		return
	}

	// Create todo in local database
	_, err = c.db.CreateTodo(event.ExternID, event.Todo)
	if err != nil {
		log.Printf("[ERROR] Failed to create todo: %v", err)
		return
	}

	log.Printf("[INFO] Todo %s synced successfully", event.ExternID)
}

// handleTodoUpdated processes a todo updated event
func (c *Cluster) handleTodoUpdated(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal todo updated event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Received todo updated: %s from %s", event.ExternID, event.NodeID)

	// Find todo by extern_id
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("[ERROR] Failed to find todo: %v", err)
		return
	}

	if existing == nil {
		// Todo doesn't exist, create it
		log.Printf("[WARN] Todo %s doesn't exist, creating", event.ExternID)
		_, err = c.db.CreateTodo(event.ExternID, event.Todo)
		if err != nil {
			log.Printf("[ERROR] Failed to create todo: %v", err)
		}
		return
	}

	// Update todo
	var todo *string
	if event.Todo != "" {
		todo = &event.Todo
	}

	_, err = c.db.UpdateTodo(existing.ID, todo, event.Completed)
	if err != nil {
		log.Printf("[ERROR] Failed to update todo: %v", err)
		return
	}

	log.Printf("[INFO] Todo %s updated successfully", event.ExternID)
}

// handleTodoDeleted processes a todo deleted event
func (c *Cluster) handleTodoDeleted(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal todo deleted event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Received todo deleted: %s from %s", event.ExternID, event.NodeID)

	// Find todo by extern_id
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("[ERROR] Failed to find todo: %v", err)
		return
	}

	if existing == nil {
		log.Printf("[INFO] Todo %s doesn't exist, nothing to delete", event.ExternID)
		return
	}

	// Delete todo
	err = c.db.DeleteTodo(existing.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to delete todo: %v", err)
		return
	}

	log.Printf("[INFO] Todo %s deleted successfully", event.ExternID)
}

// handleJobClaimed processes a job claimed event
func (c *Cluster) handleJobClaimed(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job claimed event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Job claimed: %s by node %s", event.ExternID, event.NodeID)
}

// handleJobStarted processes a job started event
func (c *Cluster) handleJobStarted(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job started event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Job started: %s by node %s", event.ExternID, event.NodeID)
}

// handleJobHeartbeat processes a job heartbeat event
func (c *Cluster) handleJobHeartbeat(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job heartbeat event: %v", err)
		return
	}

	// Skip if from myself (or log at debug level only)
	if event.NodeID == c.nodeID {
		return
	}

	// Heartbeats are frequent, only log in debug mode
	// log.Printf("[DEBUG] Job heartbeat: %s from node %s", event.ExternID, event.NodeID)
}

// handleJobCompleted processes a job completed event
func (c *Cluster) handleJobCompleted(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job completed event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Job completed: %s by node %s", event.ExternID, event.NodeID)
}

// handleJobFailed processes a job failed event
func (c *Cluster) handleJobFailed(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job failed event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[ERROR] Job failed: %s on node %s", event.ExternID, event.NodeID)
}

// handleJobReleased processes a job released event
func (c *Cluster) handleJobReleased(payload []byte) {
	var event JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[ERROR] Failed to unmarshal job released event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("[INFO] Job released: %s (was on node %s)", event.ExternID, event.NodeID)
}

// reclaimJobsFromNode reclaims all jobs from a failed node
func (c *Cluster) reclaimJobsFromNode(nodeID string) {
	log.Printf("[INFO] Reclaiming jobs from failed node: %s", nodeID)

	jobs, err := c.db.GetJobsByNode(nodeID)
	if err != nil {
		log.Printf("[ERROR] Failed to get jobs from node %s: %v", nodeID, err)
		return
	}

	if len(jobs) == 0 {
		log.Printf("[INFO] No jobs to reclaim from node %s", nodeID)
		return
	}

	log.Printf("[INFO] Found %d job(s) to reclaim from node %s", len(jobs), nodeID)

	for _, job := range jobs {
		err := c.db.ReleaseJob(job.ExternID)
		if err != nil {
			log.Printf("[ERROR] Failed to release job %s: %v", job.ExternID, err)
			continue
		}

		log.Printf("[INFO] Released job %s back to pending", job.ExternID)

		// Broadcast release event
		if err := c.BroadcastJobReleased(&job); err != nil {
			log.Printf("[WARN] Failed to broadcast job released: %v", err)
		}
	}

	log.Printf("[INFO] Reclaimed %d job(s) from failed node %s", len(jobs), nodeID)
}
