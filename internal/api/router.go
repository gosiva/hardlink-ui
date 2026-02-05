package api

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/scanner"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// Router sets up all HTTP routes
func Router(db *storage.DB, cfg *config.Config, scan *scanner.Scanner, webPath string) (http.Handler, error) {
	r := chi.NewRouter()

	// Create handlers
	templatesPath := filepath.Join(webPath, "templates")
	
	authHandler, err := NewAuthHandler(db, cfg, templatesPath)
	if err != nil {
		return nil, err
	}

	explorerHandler, err := NewExplorerHandler(db, cfg, templatesPath)
	if err != nil {
		return nil, err
	}

	hardlinkHandler := NewHardlinkHandler(db, cfg)
	duplicatesHandler := NewDuplicatesHandler(db, cfg, scan)

	// Middleware
	middleware := NewMiddleware(db, cfg)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS)

	// Static files
	staticPath := filepath.Join(webPath, "static")
	fileServer := http.FileServer(http.Dir(staticPath))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Public routes (no auth required)
	r.Get("/login", authHandler.ShowLogin)
	r.Post("/login", authHandler.ShowLogin)
	r.Get("/2fa", authHandler.Show2FA)
	r.Post("/2fa", authHandler.Show2FA)
	r.Get("/logout", authHandler.Logout)

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)

		// Main explorer page
		r.Get("/", explorerHandler.ShowExplorer)

		// API routes
		r.Route("/api", func(r chi.Router) {
			// Explorer
			r.Get("/list", explorerHandler.ListDirectory)
			r.Get("/details", explorerHandler.GetDetails)
			r.Post("/create-folder", explorerHandler.CreateFolder)

			// Hardlinks
			r.Post("/create-hardlink", hardlinkHandler.CreateHardlink)
			r.Post("/create-hardlinks-folder", hardlinkHandler.CreateHardlinksFolder)
			r.Post("/delete-hardlink", hardlinkHandler.DeleteHardlink)

			// Duplicates
			r.Get("/duplicates/scan", duplicatesHandler.StartScan)
			r.Get("/duplicates/progress/{job_id}", duplicatesHandler.GetProgress)
			r.Get("/duplicates/progress", duplicatesHandler.GetProgress) // with query param
			r.Get("/duplicates/results/{job_id}", duplicatesHandler.GetResults)
			r.Get("/duplicates/results", duplicatesHandler.GetResults) // with query param
			r.Post("/duplicates/convert", duplicatesHandler.ConvertDuplicates)
		})
	})

	log.Println("Router initialized successfully")
	return r, nil
}
