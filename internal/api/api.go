package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
	"github.com/danielgtaylor/huma/v2"
)

// Server holds the API server dependencies
type Server struct {
	db *database.DB
}

// NewServer creates a new API server
func NewServer(db *database.DB) *Server {
	return &Server{db: db}
}

// RegisterRoutes registers all API routes with the Huma API
func (s *Server) RegisterRoutes(api huma.API) {
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
	todo, err := s.db.CreateTodo(input.Body.Todo)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to create todo", err)
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

	return &UpdateTodoResponse{Body: *todo}, nil
}

func (s *Server) deleteTodo(ctx context.Context, input *DeleteTodoRequest) (*struct{}, error) {
	err := s.db.DeleteTodo(input.ID)
	if err == sql.ErrNoRows {
		return nil, huma.Error404NotFound("Todo not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to delete todo", err)
	}

	return nil, nil
}
