package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/api"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/cluster"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/config"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// slogWriter adapts slog to io.Writer interface for standard log package
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}

// generateEncryptionKey generates a 32-byte encryption key for Serf
func generateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func main() {
	// Command line flags
	configFlag := flag.String("config", "", "Path to configuration file (YAML)")
	portFlag := flag.String("port", "", "HTTP server port (overrides config)")
	dbPathFlag := flag.String("db", "", "Database file path (overrides config)")
	nodeNameFlag := flag.String("node-name", "", "Node name (overrides config)")
	serfAddrFlag := flag.String("serf-addr", "", "Serf bind address (overrides config)")
	keygenFlag := flag.Bool("keygen", false, "Generate encryption key for Serf cluster and exit")
	flag.Parse()

	// Handle keygen mode
	if *keygenFlag {
		key, err := generateEncryptionKey()
		if err != nil {
			log.Fatalf("Failed to generate encryption key: %v", err)
		}

		fmt.Println("==============================================")
		fmt.Println("Generated Serf Encryption Key:")
		fmt.Println(key)
		fmt.Println("==============================================")
		fmt.Println("")
		fmt.Println("IMPORTANT: Add this key to the 'encrypt_key' field")
		fmt.Println("in the configuration file of EVERY node in your cluster.")
		fmt.Println("")
		fmt.Println("Example YAML configuration:")
		fmt.Println("")
		fmt.Println("  cluster:")
		fmt.Println("    encrypt_key: \"" + key + "\"")
		fmt.Println("    seeds:")
		fmt.Println("      - \"127.0.0.1:7946\"")
		fmt.Println("")
		fmt.Println("All nodes must use the SAME key to communicate securely.")
		fmt.Println("==============================================")
		os.Exit(0)
	}

	var cfg *config.Config
	var err error

	// Load config file if provided
	if *configFlag != "" {
		log.Printf("Loading configuration from %s", *configFlag)
		cfg, err = config.LoadConfig(*configFlag)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		// Use defaults
		cfg = &config.Config{
			Node: config.NodeConfig{
				Name: "node-1",
				Serf: config.SerfConfig{
					BindAddr: "0.0.0.0:7946",
				},
				HTTP: config.HTTPConfig{
					Port: 8080,
				},
				Database: config.DBConfig{
					Path: "./todos.db",
				},
			},
			Cluster: config.ClusterConfig{
				Seeds:       []string{},
				JoinTimeout: 10,
			},
		}
	}

	// Override with command line flags
	if *portFlag != "" {
		port, err := strconv.Atoi(*portFlag)
		if err != nil {
			log.Fatalf("Invalid port: %v", err)
		}
		cfg.Node.HTTP.Port = port
	}
	if *dbPathFlag != "" {
		cfg.Node.Database.Path = *dbPathFlag
	}
	if *nodeNameFlag != "" {
		cfg.Node.Name = *nodeNameFlag
	}
	if *serfAddrFlag != "" {
		cfg.Node.Serf.BindAddr = *serfAddrFlag
	}

	// Setup logger with configured level
	logLevel := config.ParseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(&slogWriter{logger: logger})

	slog.Info("Starting auto-cluster-sync", "log_level", cfg.LogLevel, "node", cfg.Node.Name)

	// Initialize database
	log.Printf("Initializing database at %s", cfg.Node.Database.Path)
	db, err := database.New(cfg.Node.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize cluster
	log.Printf("Initializing cluster (node: %s, serf: %s)", cfg.Node.Name, cfg.Node.Serf.BindAddr)
	clusterInstance, err := cluster.New(cfg.Node.Name, cfg.Node.Serf.BindAddr, db)
	if err != nil {
		log.Fatalf("Failed to initialize cluster: %v", err)
	}
	defer clusterInstance.Stop()

	// Start cluster
	joinTimeout := time.Duration(cfg.Cluster.JoinTimeout) * time.Second
	if err := clusterInstance.Start(cfg.Cluster.Seeds, joinTimeout); err != nil {
		log.Fatalf("Failed to start cluster: %v", err)
	}

	// Create Chi router
	router := chi.NewMux()

	// Create Huma API
	humaAPI := humachi.New(router, huma.DefaultConfig("Todo API", "1.0.0"))

	// Register routes with cluster support
	apiServer := api.NewServer(db, clusterInstance)
	apiServer.RegisterRoutes(humaAPI)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Node.HTTP.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.Node.HTTP.Port)
		log.Printf("API documentation available at http://localhost:%d/docs", cfg.Node.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Gracefully shutdown cluster first
	if err := clusterInstance.Stop(); err != nil {
		log.Printf("Error stopping cluster: %v", err)
	}

	// Then shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
