package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/api"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

func main() {
	// Command line flags
	portFlag := flag.String("port", "", "HTTP server port (default: 8080 or PORT env var)")
	dbPathFlag := flag.String("db", "", "Database file path (default: ./todos.db or DB_PATH env var)")
	flag.Parse()

	// Configuration: flags take precedence over environment variables
	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	dbPath := *dbPathFlag
	if dbPath == "" {
		dbPath = os.Getenv("DB_PATH")
	}
	if dbPath == "" {
		dbPath = "./todos.db"
	}

	// Initialize database
	log.Printf("Initializing database at %s", dbPath)
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create Chi router
	router := chi.NewMux()

	// Create Huma API
	humaAPI := humachi.New(router, huma.DefaultConfig("Todo API", "1.0.0"))

	// Register routes
	apiServer := api.NewServer(db)
	apiServer.RegisterRoutes(humaAPI)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		log.Printf("API documentation available at http://localhost:%s/docs", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Gracefully shutdown with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
