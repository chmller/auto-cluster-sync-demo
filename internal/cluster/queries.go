package cluster

import (
	"encoding/json"
	"log"
	"time"

	"github.com/hashicorp/serf/serf"
)

// handleQuery handles incoming Serf queries
func (c *Cluster) handleQuery(query *serf.Query) {
	switch query.Name {
	case QueryFullState:
		c.handleFullStateQuery(query)
	case QueryCount:
		c.handleCountQuery(query)
	default:
		log.Printf("[WARN] Unknown query: %s", query.Name)
	}
}

// handleFullStateQuery broadcasts todos individually to avoid size limits
func (c *Cluster) handleFullStateQuery(query *serf.Query) {
	log.Printf("[INFO] Received full state query from %s", query.SourceNode())

	// Get all todos from database
	todos, err := c.db.ListTodos()
	if err != nil {
		log.Printf("[ERROR] Failed to list todos: %v", err)
		return
	}

	// Acknowledge the query with count only
	countData := map[string]int{"count": len(todos)}
	data, err := json.Marshal(countData)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal count: %v", err)
		return
	}

	if err := query.Respond(data); err != nil {
		log.Printf("[ERROR] Failed to respond to query: %v", err)
		return
	}

	log.Printf("[INFO] Acknowledged full state request from %s, will broadcast %d todos", query.SourceNode(), len(todos))

	// Broadcast each todo individually via user events to avoid size limit
	go func() {
		for _, todo := range todos {
			event := TodoSyncEvent{
				Type:      "created",
				ExternID:  todo.ExternID,
				Todo:      todo.Todo,
				Completed: &todo.Completed,
				NodeID:    c.nodeID,
				Timestamp: time.Now().Unix(),
			}

			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("[ERROR] Failed to marshal todo %s: %v", todo.ExternID, err)
				continue
			}

			if err := c.serf.UserEvent(EventTodoCreated, payload, false); err != nil {
				log.Printf("[ERROR] Failed to broadcast todo %s: %v", todo.ExternID, err)
				continue
			}

			// Small delay to avoid overwhelming the network
			time.Sleep(10 * time.Millisecond)
		}
		log.Printf("[INFO] Finished broadcasting %d todos to %s", len(todos), query.SourceNode())
	}()
}

// handleCountQuery responds with the count of todos
func (c *Cluster) handleCountQuery(query *serf.Query) {
	log.Printf("[INFO] Received count query from %s", query.SourceNode())

	// Count todos
	count, err := c.db.CountTodos()
	if err != nil {
		log.Printf("[ERROR] Failed to count todos: %v", err)
		return
	}

	// Create response
	response := CountResponse{
		Count:  count,
		NodeID: c.nodeID,
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal count response: %v", err)
		return
	}

	// Send response
	if err := query.Respond(data); err != nil {
		log.Printf("[ERROR] Failed to respond to query: %v", err)
		return
	}

	log.Printf("[INFO] Sent count (%d) to %s", count, query.SourceNode())
}

// requestFullSync requests full state from all nodes in the cluster
func (c *Cluster) requestFullSync() {
	defer c.markReady() // Always mark as ready when done, even on error

	log.Printf("[INFO] Requesting full sync from cluster...")

	// Create query params
	params := &serf.QueryParam{
		FilterNodes: nil, // Query all nodes
		RequestAck:  true,
		Timeout:     10 * time.Second,
	}

	// Send query
	resp, err := c.serf.Query(QueryFullState, nil, params)
	if err != nil {
		log.Printf("[ERROR] Failed to send full sync query: %v", err)
		return
	}

	// Collect responses (now just acknowledgments with counts)
	expectedCount := 0
	respondingNodes := 0

	for r := range resp.ResponseCh() {
		var countData map[string]int
		if err := json.Unmarshal(r.Payload, &countData); err != nil {
			log.Printf("[ERROR] Failed to unmarshal response from %s: %v", r.From, err)
			continue
		}

		count := countData["count"]
		log.Printf("[INFO] Node %s will broadcast %d todos", r.From, count)
		expectedCount += count
		respondingNodes++
	}

	log.Printf("[INFO] Received acknowledgments from %d node(s), expecting ~%d todos via broadcast", respondingNodes, expectedCount)

	// Wait a bit for broadcasts to arrive
	// The actual syncing happens via handleTodoCreated events
	if expectedCount > 0 {
		waitTime := time.Duration(expectedCount/10+5) * time.Second // ~10 todos/sec + 5s buffer
		if waitTime > 30*time.Second {
			waitTime = 30 * time.Second
		}
		log.Printf("[INFO] Waiting %v for broadcasts to complete...", waitTime)
		time.Sleep(waitTime)
	}

	// Check final count
	finalCount, err := c.db.CountTodos()
	if err != nil {
		log.Printf("[ERROR] Failed to count synced todos: %v", err)
	} else {
		log.Printf("[INFO] Full sync complete: %d todos synced", finalCount)
	}
}
