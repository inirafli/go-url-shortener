package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/inirafli/go-url-shortener/internal/storage"
)

type Handler struct {
	storage *storage.Storage
}

func NewHandler(s *storage.Storage) *Handler {
	return &Handler{
		storage: s,
	}
}

type ShortenRequest struct {
	LongURL string `json:"long_url"`
}

type ShortenResponse struct {
	ShortURL string `json:"short_url"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isValidURL(urlStr string) bool {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}

	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Handler for URL shortening requests
func (h *Handler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Invalid request method")
		return
	}

	var req ShortenRequest
	// 4KB limit for the long URL
	maxBodyBytes := int64(1024 * 4)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	// Disallow unknown fields in the JSON request to be stricter
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)

	// Request error handling
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at character %d)", syntaxError.Offset)
			writeError(w, http.StatusBadRequest, msg)
		case errors.Is(err, io.ErrUnexpectedEOF):
			writeError(w, http.StatusBadRequest, "Request body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at character %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			writeError(w, http.StatusBadRequest, msg)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			writeError(w, http.StatusBadRequest, msg)
		case errors.Is(err, io.EOF): // Happens with empty body
			writeError(w, http.StatusBadRequest, "Request body must not be empty")
		case errors.As(err, &maxBytesError):
			msg := fmt.Sprintf("Request body must not be larger than %d bytes", maxBodyBytes)
			writeError(w, http.StatusRequestEntityTooLarge, msg)
		default:
			log.Printf("Error decoding JSON: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not decode request body")
		}

		return
	}

	if req.LongURL == "" {
		writeError(w, http.StatusBadRequest, "Missing 'long_url' in request body")
		return
	}

	if !isValidURL(req.LongURL) {
		writeError(w, http.StatusBadRequest, "Invalid 'long_url' format. Must be a valid HTTP/HTTPS URL.")
		return
	}

	shortID, err := h.storage.Save(req.LongURL)
	if err != nil {
		log.Printf("Error saving URL to storage: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to shorten URL")
		return
	}

	// Constructing shortUr;
	scheme := "http"
	fullShortURL := fmt.Sprintf("%s://%s/%s", scheme, r.Host, shortID)

	// Prepare and Send JSON Response
	resp := ShortenResponse{ShortURL: fullShortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// RedirectURL handles requests to redirect a short URL to its original long URL
func (h *Handler) RedirectURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Invalid request method")
		return
	}

	shortID := strings.TrimPrefix(r.URL.Path, "/")
	if shortID == "" {
		writeError(w, http.StatusBadRequest, "Missing short ID in URL path")
		return
	}

	//  Use Storage to Load Long URL
	longURL, err := h.storage.Load(shortID)
	if err != nil {
		log.Printf("Error loading URL for shortID '%s': %v", shortID, err)

		// Check if the error indicates "not found"
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "Short URL not found")
		} else {
			// Some other unexpected storage error occurred
			writeError(w, http.StatusInternalServerError, "Failed to retrieve URL")
		}

		return
	}

	// Perform HTTP Redirect
	http.Redirect(w, r, longURL, http.StatusFound)
}
