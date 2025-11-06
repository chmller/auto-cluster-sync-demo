package models

import "time"

// Todo represents a todo item in the system
type Todo struct {
	ID        int       `json:"id" db:"id"`
	Todo      string    `json:"todo" db:"todo"`
	Completed bool      `json:"completed" db:"completed"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreateTodoInput represents the input for creating a new todo
type CreateTodoInput struct {
	Todo string `json:"todo" minLength:"1" maxLength:"500" doc:"The todo description"`
}

// UpdateTodoInput represents the input for updating a todo
type UpdateTodoInput struct {
	Todo      *string `json:"todo,omitempty" minLength:"1" maxLength:"500" doc:"The todo description"`
	Completed *bool   `json:"completed,omitempty" doc:"Whether the todo is completed"`
}
