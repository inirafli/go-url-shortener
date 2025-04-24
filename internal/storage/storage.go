package storage

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

const shortIDLength = 6

type Storage struct {
	mu       sync.RWMutex
	urls     map[string]string
	r        *rand.Rand
	filePath string
}

func NewStorage(filePath string) (*Storage, error) {
	source := rand.NewSource(time.Now().UnixNano())
	randomGenerator := rand.New(source)

	s := &Storage{
		urls:     make(map[string]string),
		r:        randomGenerator,
		filePath: filePath,
	}

	err := s.loadFromFile()
	if err != nil {
		log.Printf("Warning: Could not load data from file '%s': %v. Starting with empty storage.", filePath, err)
		s.urls = make(map[string]string)
		return s, nil
	}

	log.Printf("Loaded %d URLs from %s", len(s.urls), filePath)
	return s, nil
}

func (s *Storage) Save(longURL string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var shortID string
	var saveErr error

	foundUniqueID := false
	for range 5 {
		genID := s.generateShortID()
		if _, exists := s.urls[genID]; !exists {
			shortID = genID
			foundUniqueID = true
			break
		}
	}

	if !foundUniqueID {
		return "", fmt.Errorf("failed to generate a unique short ID after multiple attempts")
	}

	s.urls[shortID] = longURL

	saveErr = s.appendToFile(shortID, longURL)
	if saveErr != nil {
		log.Printf("Error: Failed to persist shortID '%s' to file '%s': %v", shortID, s.filePath, saveErr)
		delete(s.urls, shortID)
		return "", fmt.Errorf("failed to save URL persistently: %w", saveErr)
	}

	return shortID, nil
}

func (s *Storage) Load(shortID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	longUrl, exists := s.urls[shortID]
	if !exists {
		return "", fmt.Errorf("short ID not found: %s", shortID)
	}

	return longUrl, nil
}

func (s *Storage) generateShortID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, shortIDLength)
	for i := range b {
		b[i] = charset[s.r.Intn(len(charset))]
	}
	return string(b)
}

func (s *Storage) loadFromFile() error {
	file, err := os.OpenFile(s.filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open storage file: %w", err)
	}

	defer file.Close()

	// Scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	loadedCount := 0
	tempUrls := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Skipping malformed line in %s: %s", s.filePath, line)
			continue
		}

		shortID := parts[0]
		longURL := parts[1]
		tempUrls[shortID] = longURL
		loadedCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading storage file: %w", err)
	}

	s.urls = tempUrls
	return nil
}

func (s *Storage) appendToFile(shortID, longURL string) error {
	file, err := os.OpenFile(s.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open storage file for appending: %w", err)
	}
	defer file.Close()

	record := fmt.Sprintf("%s\t%s\n", shortID, longURL) // TSV format

	_, err = file.WriteString(record)
	if err != nil {
		return fmt.Errorf("Failed to write record to storage file: %w", err)
	}

	return nil
}
