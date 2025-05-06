package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inirafli/go-url-shortener/internal/handler"
	"github.com/inirafli/go-url-shortener/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration from env
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "shortener_user")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "url_shortener_db")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	log.Printf("Attempting to connect to database: %s:%s/%s", dbHost, dbPort, dbName)

	// Initialize storage
	urlStorage, err := storage.NewStorage(dsn)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	urlHandler := handler.NewHandler(urlStorage)

	mux := http.NewServeMux()
	mux.HandleFunc("/shorten", urlHandler.ShortenURL)

	// Handler for the root path "/" and any other paths.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		urlHandler.ShortenURL(w, r.WithContext(r.Context()))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "Welcome to the Go URL Shortener! (with PostgreSQL)")
			fmt.Fprintln(w, "\nUsage:")
			fmt.Fprintln(w, "  POST /shorten   - with JSON body {\"long_url\": \"...\"}")
			fmt.Fprintln(w, "  GET /{shortID} - redirects to the original URL")
			return
		}

		urlHandler.RedirectURL(w, r.WithContext(r.Context()))
	})

	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Starting URL Shortener server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v\n", port, err)
		}
	}()

	// Wait for interrupt signal
	<-stopChan
	log.Println("Shutting down server...")

	// Create a deadline context for shutdown
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	// Attempt shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	// Close the database connection
	if err := urlStorage.Close(); err != nil {
		log.Printf("Error closing database connection pool: %v", err)
	}

	log.Println("Server stopped")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Printf("Environment variable %s not set, using default: %s", key, fallback)
	return fallback
}
