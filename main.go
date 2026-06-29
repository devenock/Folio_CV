package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"foliocv/db"
	"foliocv/handlers"
	"foliocv/middleware"
	"foliocv/services"
)

func main() {
	_ = godotenv.Load()

	required := []string{"DATABASE_URL", "SECRET_KEY", "ANTHROPIC_API_KEY"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			log.Fatalf("Required env var %s is not set", key)
		}
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}
	defer pool.Close()

	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./data/uploads"
	}
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads dir: %v", err)
	}

	claude := services.NewClaudeClient()

	authHandler := handlers.NewAuthHandler(pool)
	uploadHandler := handlers.NewUploadHandler(pool, claude)
	portfolioHandler := handlers.NewPortfolioHandler(pool)
	sectionHandler := handlers.NewSectionHandler(pool)
	itemHandler := handlers.NewItemHandler(pool)
	themeHandler := handlers.NewThemeHandler(pool)
	blockHandler := handlers.NewBlockHandler(pool)
	imageHandler := handlers.NewImageHandler(pool)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	r.Handle("/static/*", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))
	r.Handle("/media/*", http.StripPrefix("/media/",
		http.FileServer(http.Dir(filepath.Join(uploadsDir, "images")))))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/", handlers.LandingHandler)
	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)
	r.Get("/register", authHandler.RegisterPage)
	r.Post("/register", authHandler.Register)
	r.Post("/logout", authHandler.Logout)

	r.Get("/p/{slug}", portfolioHandler.PublicView)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(pool))

		r.Get("/upload", uploadHandler.Page)
		r.Post("/upload", uploadHandler.Handle)
		r.Get("/upload/processing", uploadHandler.ProcessingPage)
		r.Get("/upload/status", uploadHandler.Status)

		r.Get("/dashboard", portfolioHandler.Dashboard)
		r.Get("/dashboard/profile/{field}/edit", portfolioHandler.EditField)
		r.Put("/dashboard/profile/{field}", portfolioHandler.UpdateField)
		r.Post("/dashboard/visibility", portfolioHandler.ToggleVisibility)
		r.Post("/dashboard/theme", themeHandler.Switch)

		r.Get("/dashboard/sections/new", sectionHandler.NewPage)
		r.Post("/dashboard/sections", sectionHandler.Create)
		r.Post("/dashboard/sections/reorder", sectionHandler.Reorder)
		r.Get("/dashboard/sections/{id}/edit", sectionHandler.Edit)
		r.Post("/dashboard/sections/{id}/title", sectionHandler.UpdateTitle)
		r.Post("/dashboard/sections/{id}/toggle", sectionHandler.Toggle)
		r.Delete("/dashboard/sections/{id}", sectionHandler.Delete)

		r.Get("/dashboard/sections/{sectionID}/items/new", itemHandler.NewForm)
		r.Post("/dashboard/sections/{sectionID}/items", itemHandler.Create)
		r.Post("/dashboard/sections/{sectionID}/items/reorder", itemHandler.Reorder)
		r.Get("/dashboard/sections/{sectionID}/items/{id}/edit", itemHandler.EditForm)
		r.Put("/dashboard/sections/{sectionID}/items/{id}", itemHandler.Update)
		r.Delete("/dashboard/sections/{sectionID}/items/{id}", itemHandler.Delete)

		// Canvas builder (hidden — not linked from nav yet, per rollout plan)
		r.Get("/dashboard/canvas", blockHandler.Canvas)
		r.Post("/dashboard/blocks", blockHandler.Create)
		r.Get("/dashboard/blocks/{id}", blockHandler.View)
		r.Get("/dashboard/blocks/{id}/edit", blockHandler.EditForm)
		r.Put("/dashboard/blocks/{id}/position", blockHandler.UpdatePosition)
		r.Put("/dashboard/blocks/{id}", blockHandler.UpdateContent)
		r.Post("/dashboard/blocks/{id}/toggle", blockHandler.Toggle)
		r.Delete("/dashboard/blocks/{id}", blockHandler.Delete)

		r.Post("/dashboard/images", imageHandler.Upload)
		r.Delete("/dashboard/images/{id}", imageHandler.Delete)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 150 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("FolioCV running on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
