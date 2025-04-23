package storage

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const shortIDLength = 6

type Storage struct {
	mu   sync.RWMutex
	urls map[string]string
	r    *rand.Rand
}

func NewStorage() *Storage {
	source := rand.NewSource(time.Now().UnixNano())
	randomGenerator := rand.New(source)

	return &Storage{
		urls: make(map[string]string),
		r:    randomGenerator,
	}
}

func (s *Storage) Save(longURL string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := 0; i < 5; i++ {
		shortID := s.generateShortID()
		if _, exists := s.urls[shortID]; !exists {
			s.urls[shortID] = longURL
			return shortID, nil
		}
	}

	return "", fmt.Errorf("failed to generate a unique short ID after multiple attempts")
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
