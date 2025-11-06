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
				log.Printf("Unknown event type: %T", e)
			}
		case <-c.shutdown:
			log.Println("Event handler shutting down")
			return
		}
	}
}

// handleMemberEvent handles cluster membership events
func (c *Cluster) handleMemberEvent(event serf.MemberEvent) {
	for _, member := range event.Members {
		switch event.Type {
		case serf.EventMemberJoin:
			log.Printf("🎉 Node joined: %s (%s)", member.Name, member.Addr)

			// If I'm the new node, request full sync
			if member.Name == c.nodeID {
				log.Println("ℹ️  I'm the new node, requesting full sync...")
				go c.requestFullSync()
			}

		case serf.EventMemberLeave:
			log.Printf("👋 Node left gracefully: %s", member.Name)

		case serf.EventMemberFailed:
			log.Printf("💀 Node failed: %s", member.Name)

		case serf.EventMemberUpdate:
			log.Printf("🔄 Node updated: %s", member.Name)

		case serf.EventMemberReap:
			log.Printf("🗑️  Node reaped: %s", member.Name)
		}
	}
}

// handleUserEvent handles custom user events (todo sync)
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
	default:
		log.Printf("Unknown user event: %s", event.Name)
	}
}

// handleTodoCreated processes a todo created event
func (c *Cluster) handleTodoCreated(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("❌ Failed to unmarshal todo created event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("📥 Received todo created: %s from %s", event.ExternID, event.NodeID)

	// Check if todo already exists (idempotency)
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("❌ Failed to check existing todo: %v", err)
		return
	}

	if existing != nil {
		log.Printf("⏭️  Todo %s already exists, skipping", event.ExternID)
		return
	}

	// Create todo in local database
	_, err = c.db.CreateTodo(event.ExternID, event.Todo)
	if err != nil {
		log.Printf("❌ Failed to create todo: %v", err)
		return
	}

	log.Printf("✅ Todo %s synced successfully", event.ExternID)
}

// handleTodoUpdated processes a todo updated event
func (c *Cluster) handleTodoUpdated(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("❌ Failed to unmarshal todo updated event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("📥 Received todo updated: %s from %s", event.ExternID, event.NodeID)

	// Find todo by extern_id
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("❌ Failed to find todo: %v", err)
		return
	}

	if existing == nil {
		// Todo doesn't exist, create it
		log.Printf("⚠️  Todo %s doesn't exist, creating", event.ExternID)
		_, err = c.db.CreateTodo(event.ExternID, event.Todo)
		if err != nil {
			log.Printf("❌ Failed to create todo: %v", err)
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
		log.Printf("❌ Failed to update todo: %v", err)
		return
	}

	log.Printf("✅ Todo %s updated successfully", event.ExternID)
}

// handleTodoDeleted processes a todo deleted event
func (c *Cluster) handleTodoDeleted(payload []byte) {
	var event TodoSyncEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("❌ Failed to unmarshal todo deleted event: %v", err)
		return
	}

	// Skip if from myself
	if event.NodeID == c.nodeID {
		return
	}

	log.Printf("📥 Received todo deleted: %s from %s", event.ExternID, event.NodeID)

	// Find todo by extern_id
	existing, err := c.db.GetTodoByExternID(event.ExternID)
	if err != nil {
		log.Printf("❌ Failed to find todo: %v", err)
		return
	}

	if existing == nil {
		log.Printf("⏭️  Todo %s doesn't exist, nothing to delete", event.ExternID)
		return
	}

	// Delete todo
	err = c.db.DeleteTodo(existing.ID)
	if err != nil {
		log.Printf("❌ Failed to delete todo: %v", err)
		return
	}

	log.Printf("✅ Todo %s deleted successfully", event.ExternID)
}
