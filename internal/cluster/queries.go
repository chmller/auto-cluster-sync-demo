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
		log.Printf("Unknown query: %s", query.Name)
	}
}

// handleFullStateQuery responds with all todos in the database
func (c *Cluster) handleFullStateQuery(query *serf.Query) {
	log.Printf("📤 Received full state query from %s", query.SourceNode())

	// Get all todos from database
	todos, err := c.db.ListTodos()
	if err != nil {
		log.Printf("❌ Failed to list todos: %v", err)
		return
	}

	// Marshal todos to JSON
	data, err := json.Marshal(todos)
	if err != nil {
		log.Printf("❌ Failed to marshal todos: %v", err)
		return
	}

	// Send response
	if err := query.Respond(data); err != nil {
		log.Printf("❌ Failed to respond to query: %v", err)
		return
	}

	log.Printf("✅ Sent %d todos to %s", len(todos), query.SourceNode())
}

// handleCountQuery responds with the count of todos
func (c *Cluster) handleCountQuery(query *serf.Query) {
	log.Printf("📤 Received count query from %s", query.SourceNode())

	// Count todos
	count, err := c.db.CountTodos()
	if err != nil {
		log.Printf("❌ Failed to count todos: %v", err)
		return
	}

	// Create response
	response := CountResponse{
		Count:  count,
		NodeID: c.nodeID,
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("❌ Failed to marshal count response: %v", err)
		return
	}

	// Send response
	if err := query.Respond(data); err != nil {
		log.Printf("❌ Failed to respond to query: %v", err)
		return
	}

	log.Printf("✅ Sent count (%d) to %s", count, query.SourceNode())
}

// requestFullSync requests full state from all nodes in the cluster
func (c *Cluster) requestFullSync() {
	defer c.markReady() // Always mark as ready when done, even on error

	log.Println("🔄 Requesting full sync from cluster...")

	// Create query params
	params := &serf.QueryParam{
		FilterNodes: nil, // Query all nodes
		RequestAck:  true,
		Timeout:     10 * time.Second,
	}

	// Send query
	resp, err := c.serf.Query(QueryFullState, nil, params)
	if err != nil {
		log.Printf("❌ Failed to send full sync query: %v", err)
		return
	}

	// Collect responses
	seenExternIDs := make(map[string]bool)
	totalSynced := 0

	for r := range resp.ResponseCh() {
		var todos []struct {
			ExternID  string `json:"extern_id"`
			Todo      string `json:"todo"`
			Completed bool   `json:"completed"`
		}

		if err := json.Unmarshal(r.Payload, &todos); err != nil {
			log.Printf("❌ Failed to unmarshal response from %s: %v", r.From, err)
			continue
		}

		log.Printf("📦 Received %d todos from %s", len(todos), r.From)

		for _, todo := range todos {
			// Skip duplicates
			if seenExternIDs[todo.ExternID] {
				continue
			}

			// Check if todo already exists
			existing, err := c.db.GetTodoByExternID(todo.ExternID)
			if err != nil {
				log.Printf("❌ Failed to check todo %s: %v", todo.ExternID, err)
				continue
			}

			if existing != nil {
				// Already exists, skip
				seenExternIDs[todo.ExternID] = true
				continue
			}

			// Create todo in local database
			_, err = c.db.CreateTodo(todo.ExternID, todo.Todo)
			if err != nil {
				log.Printf("❌ Failed to sync todo %s: %v", todo.ExternID, err)
				continue
			}

			seenExternIDs[todo.ExternID] = true
			totalSynced++
		}
	}

	log.Printf("✅ Full sync complete: %d todos synced", totalSynced)
}
