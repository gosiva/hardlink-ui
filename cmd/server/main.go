package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gosiva/hardlink-ui/internal/api"
	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/scanner"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting hardlink-ui server...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Configuration loaded: DataRoot=%s, Port=%s", cfg.DataRoot, cfg.Port)

	// Validate data root exists
	if _, err := os.Stat(cfg.DataRoot); err != nil {
		log.Fatalf("Data root does not exist: %s - %v", cfg.DataRoot, err)
	}

	// Initialize database
	db, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize admin user if needed
	if err := initAdminUser(db, cfg); err != nil {
		log.Fatalf("Failed to initialize admin user: %v", err)
	}

	// Create scanner
	scan := scanner.NewScanner(db, cfg.DataRoot)

	// Determine web path
	webPath := os.Getenv("WEB_PATH")
	if webPath == "" {
		// Default to /app/web in Docker, or ./web for local development
		if _, err := os.Stat("/app/web"); err == nil {
			webPath = "/app/web"
		} else {
			webPath = "./web"
		}
	}

	absWebPath, err := filepath.Abs(webPath)
	if err != nil {
		log.Fatalf("Failed to resolve web path: %v", err)
	}
	log.Printf("Using web path: %s", absWebPath)

	// Verify templates exist
	templatesPath := filepath.Join(absWebPath, "templates")
	if _, err := os.Stat(templatesPath); err != nil {
		log.Fatalf("Templates directory not found: %s", templatesPath)
	}

	// Initialize router
	router, err := api.Router(db, cfg, scan, absWebPath)
	if err != nil {
		log.Fatalf("Failed to initialize router: %v", err)
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
		// ReadTimeout: 30s for request headers/body
		ReadTimeout: 30 * time.Second,
		// WriteTimeout must be 0 (disabled) to support SSE streaming
		// SSE endpoints can stream for up to 10 minutes
		WriteTimeout: 0,
		// IdleTimeout handles cleanup of idle connections
		IdleTimeout: 120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start cleanup goroutine for expired sessions
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := db.CleanupExpiredSessions(cfg.SessionTimeout); err != nil {
				log.Printf("Error cleaning up sessions: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func initAdminUser(db *storage.DB, cfg *config.Config) error {
	if cfg.AdminUser == "" || cfg.AdminPassword == "" || cfg.TOTPSecret == "" {
		log.Println("Warning: Admin credentials not provided in environment")
		return nil
	}

	// Check if user already exists
	exists, err := db.UserExists(cfg.AdminUser)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		log.Printf("Admin user '%s' already exists", cfg.AdminUser)
		return nil
	}

	// Create admin user
	if err := db.CreateUser(cfg.AdminUser, cfg.AdminPassword, cfg.TOTPSecret); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Printf("Admin user '%s' created successfully", cfg.AdminUser)
	return nil
}
