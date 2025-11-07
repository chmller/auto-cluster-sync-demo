package api

import (
	"context"
	"net/http"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
	"github.com/danielgtaylor/huma/v2"
)

// Cluster interface for broadcasting events
type Cluster interface {
	BroadcastTodoCreated(todo *models.Todo) error
	BroadcastTodoUpdated(todo *models.Todo) error
	BroadcastTodoDeleted(externID string) error
	IsReady() bool
	LocalNode() string
	MemberCount() int
	GetMemberInfo() []models.ClusterMemberInfo
}

// Server holds the API server dependencies
type Server struct {
	db      *database.DB
	cluster Cluster
}

// NewServer creates a new API server
func NewServer(db *database.DB, cluster Cluster) *Server {
	return &Server{
		db:      db,
		cluster: cluster,
	}
}

// RegisterRoutes registers all API routes with the Huma API
func (s *Server) RegisterRoutes(api huma.API) {
	// GET /health/ready - Health check
	huma.Register(api, huma.Operation{
		OperationID: "health-ready",
		Method:      http.MethodGet,
		Path:        "/health/ready",
		Summary:     "Readiness check",
		Description: "Check if the node is ready to serve requests (fully synced)",
		Tags:        []string{"health"},
	}, s.healthReady)

	// GET /health/info - Cluster info
	huma.Register(api, huma.Operation{
		OperationID: "health-info",
		Method:      http.MethodGet,
		Path:        "/health/info",
		Summary:     "Cluster information",
		Description: "Get information about the cluster status and members",
		Tags:        []string{"health"},
	}, s.healthInfo)

	// GET /todos - List all todos
	huma.Register(api, huma.Operation{
		OperationID: "list-todos",
		Method:      http.MethodGet,
		Path:        "/todos",
		Summary:     "List all todos",
		Description: "Get a list of all todo items",
		Tags:        []string{"todos"},
	}, s.listTodos)

	// GET /todos/{id} - Get a specific todo
	huma.Register(api, huma.Operation{
		OperationID: "get-todo",
		Method:      http.MethodGet,
		Path:        "/todos/{id}",
		Summary:     "Get a todo",
		Description: "Get a specific todo item by ID",
		Tags:        []string{"todos"},
	}, s.getTodo)

	// POST /todos - Create a new todo
	huma.Register(api, huma.Operation{
		OperationID: "create-todo",
		Method:      http.MethodPost,
		Path:        "/todos",
		Summary:     "Create a todo",
		Description: "Create a new todo item",
		Tags:        []string{"todos"},
	}, s.createTodo)

	// PUT /todos/{id} - Update a todo
	huma.Register(api, huma.Operation{
		OperationID: "update-todo",
		Method:      http.MethodPut,
		Path:        "/todos/{id}",
		Summary:     "Update a todo",
		Description: "Update an existing todo item",
		Tags:        []string{"todos"},
	}, s.updateTodo)

	// DELETE /todos/{id} - Delete a todo
	huma.Register(api, huma.Operation{
		OperationID: "delete-todo",
		Method:      http.MethodDelete,
		Path:        "/todos/{id}",
		Summary:     "Delete a todo",
		Description: "Delete a todo item",
		Tags:        []string{"todos"},
	}, s.deleteTodo)
}

// Request/Response types

type ListTodosResponse struct {
	Body []models.Todo
}

type GetTodoRequest struct {
	ID int `path:"id" minimum:"1" doc:"Todo ID"`
}

type GetTodoResponse struct {
	Body models.Todo
}

type CreateTodoRequest struct {
	Body models.CreateTodoInput
}

type CreateTodoResponse struct {
	Body models.Todo
}

type UpdateTodoRequest struct {
	ID   int                     `path:"id" minimum:"1" doc:"Todo ID"`
	Body models.UpdateTodoInput
}

type UpdateTodoResponse struct {
	Body models.Todo
}

type DeleteTodoRequest struct {
	ID int `path:"id" minimum:"1" doc:"Todo ID"`
}

// Handler implementations

func (s *Server) listTodos(ctx context.Context, input *struct{}) (*ListTodosResponse, error) {
	todos, err := s.db.ListTodos()
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to list todos", err)
	}

	// Return empty array instead of nil
	if todos == nil {
		todos = []models.Todo{}
	}

	return &ListTodosResponse{Body: todos}, nil
}

func (s *Server) getTodo(ctx context.Context, input *GetTodoRequest) (*GetTodoResponse, error) {
	todo, err := s.db.GetTodo(input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to get todo", err)
	}

	if todo == nil {
		return nil, huma.Error404NotFound("Todo not found")
	}

	return &GetTodoResponse{Body: *todo}, nil
}

func (s *Server) createTodo(ctx context.Context, input *CreateTodoRequest) (*CreateTodoResponse, error) {
	todo, err := s.db.CreateTodo(input.Body.ExternID, input.Body.Todo)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to create todo", err)
	}

	// Broadcast to cluster (if cluster is enabled)
	if s.cluster != nil {
		if err := s.cluster.BroadcastTodoCreated(todo); err != nil {
			// Log error but don't fail the request
			// Todo is already created locally
			// Cluster sync will retry later
		}
	}

	return &CreateTodoResponse{Body: *todo}, nil
}

func (s *Server) updateTodo(ctx context.Context, input *UpdateTodoRequest) (*UpdateTodoResponse, error) {
	todo, err := s.db.UpdateTodo(input.ID, input.Body.Todo, input.Body.Completed)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to update todo", err)
	}

	if todo == nil {
		return nil, huma.Error404NotFound("Todo not found")
	}

	// Broadcast to cluster (if cluster is enabled)
	if s.cluster != nil {
		if err := s.cluster.BroadcastTodoUpdated(todo); err != nil {
			// Log error but don't fail the request
		}
	}

	return &UpdateTodoResponse{Body: *todo}, nil
}

func (s *Server) deleteTodo(ctx context.Context, input *DeleteTodoRequest) (*struct{}, error) {
	// Get todo first to get extern_id for cluster broadcast
	todo, err := s.db.GetTodo(input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to get todo", err)
	}
	if todo == nil {
		return nil, huma.Error404NotFound("Todo not found")
	}

	// Delete from database
	err = s.db.DeleteTodo(input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to delete todo", err)
	}

	// Broadcast to cluster (if cluster is enabled)
	if s.cluster != nil {
		if err := s.cluster.BroadcastTodoDeleted(todo.ExternID); err != nil {
			// Log error but don't fail the request
		}
	}

	return nil, nil
}

type HealthReadyResponse struct {
	Body struct {
		Ready   bool   `json:"ready" doc:"Whether the node is ready to serve requests"`
		Message string `json:"message,omitempty" doc:"Optional status message"`
	}
}

func (s *Server) healthReady(ctx context.Context, input *struct{}) (*HealthReadyResponse, error) {
	resp := &HealthReadyResponse{}

	if s.cluster == nil {
		// No cluster, always ready
		resp.Body.Ready = true
		resp.Body.Message = "Running in standalone mode"
		return resp, nil
	}

	if s.cluster.IsReady() {
		resp.Body.Ready = true
		resp.Body.Message = "Node is ready"
		return resp, nil
	}

	// Not ready yet (still syncing)
	resp.Body.Ready = false
	resp.Body.Message = "Node is syncing, not ready yet"
	return resp, huma.Error503ServiceUnavailable("Node is syncing, not ready yet")
}

type HealthInfoResponse struct {
	Body struct {
		NodeName      string                      `json:"node_name" doc:"Name of this node"`
		Ready         bool                        `json:"ready" doc:"Whether the node is ready to serve requests"`
		ClusterMode   bool                        `json:"cluster_mode" doc:"Whether clustering is enabled"`
		MemberCount   int                         `json:"member_count" doc:"Number of cluster members"`
		Members       []models.ClusterMemberInfo  `json:"members,omitempty" doc:"List of cluster members"`
		TodoCount     int                         `json:"todo_count" doc:"Number of todos in local database"`
		JobsPending   int                         `json:"jobs_pending" doc:"Number of pending jobs"`
		JobsClaimed   int                         `json:"jobs_claimed" doc:"Number of claimed jobs"`
		JobsProcessing int                        `json:"jobs_processing" doc:"Number of jobs being processed"`
		JobsCompleted int                         `json:"jobs_completed" doc:"Number of completed jobs"`
		JobsFailed    int                         `json:"jobs_failed" doc:"Number of failed jobs"`
	}
}

func (s *Server) healthInfo(ctx context.Context, input *struct{}) (*HealthInfoResponse, error) {
	resp := &HealthInfoResponse{}

	// Get todo count from database
	todoCount, err := s.db.CountTodos()
	if err != nil {
		todoCount = -1 // Indicate error
	}
	resp.Body.TodoCount = todoCount

	// Get job statistics
	jobsPending, _ := s.db.CountJobsByStatus(models.StatusPending)
	jobsClaimed, _ := s.db.CountJobsByStatus(models.StatusClaimed)
	jobsProcessing, _ := s.db.CountJobsByStatus(models.StatusProcessing)
	jobsCompleted, _ := s.db.CountJobsByStatus(models.StatusCompleted)
	jobsFailed, _ := s.db.CountJobsByStatus(models.StatusFailed)

	resp.Body.JobsPending = jobsPending
	resp.Body.JobsClaimed = jobsClaimed
	resp.Body.JobsProcessing = jobsProcessing
	resp.Body.JobsCompleted = jobsCompleted
	resp.Body.JobsFailed = jobsFailed

	if s.cluster == nil {
		// Standalone mode
		resp.Body.NodeName = "standalone"
		resp.Body.Ready = true
		resp.Body.ClusterMode = false
		resp.Body.MemberCount = 1
		resp.Body.Members = nil
		return resp, nil
	}

	// Cluster mode
	resp.Body.NodeName = s.cluster.LocalNode()
	resp.Body.Ready = s.cluster.IsReady()
	resp.Body.ClusterMode = true
	resp.Body.MemberCount = s.cluster.MemberCount()
	resp.Body.Members = s.cluster.GetMemberInfo()

	return resp, nil
}
