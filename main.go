package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/inirafli/go-url-shortener/internal/handler"
	"github.com/inirafli/go-url-shortener/internal/storage"
)

const urlStoragePath = "urls.tsv"

func main() {
	// Initialize depedencies
	urlStorage, err := storage.NewStorage(urlStoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	urlHandler := handler.NewHandler(urlStorage)

	// Handler for the /shorten endpoint.
	http.HandleFunc("/shorten", urlHandler.ShortenURL)

	// Handler for the root path "/" and any other paths.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Handle the root path specifically (e.g., show a welcome page or info)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "Welcome to the Go URL Shortener!")
			fmt.Fprintln(w, "\nUsage:")
			fmt.Fprintln(w, "  POST /shorten   - with JSON body {\"long_url\": \"...\"}")
			fmt.Fprintln(w, "  GET /{shortID} - redirects to the original URL")
			return
		}

		// Pass the request to the RedirectURL handler
		urlHandler.RedirectURL(w, r)
	})

	port := ":8080"
	fmt.Printf("Starting URL Shortener server on port %s\n", port)
	fmt.Printf("Using storage file: %s\n", urlStoragePath)

	// Start the HTTP server using the DefaultServeMux
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
