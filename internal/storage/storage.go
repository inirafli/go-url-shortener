package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const shortIDLength = 6
const uniqueViolationCode = "23505"

type Storage struct {
	db *sql.DB
	r  *rand.Rand
}

func NewStorage(dsn string) (*Storage, error) {
	// Open database connection
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established successfully.")

	source := rand.NewSource(time.Now().UnixNano())
	randomGenerator := rand.New(source)

	return &Storage{
		db: db,
		r:  randomGenerator,
	}, nil
}

// Close releases the database connection pool.
func (s *Storage) Close() error {
	if s.db != nil {
		log.Println("Closing database connection pool.")
		return s.db.Close()
	}
	return nil
}

func (s *Storage) Save(ctx context.Context, longURL string) (string, error) {
	for i := 0; i < 5; i++ {
		shortID := s.generateShortID()

		stmt := `INSERT INTO urls (short_id, long_url) VALUES ($1, $2)`
		// Execute the INSERT statement
		_, err := s.db.ExecContext(ctx, stmt, shortID, longURL)
		if err == nil {
			return shortID, nil
		}

		// Check if the error is a unique key violation (collision)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			log.Printf("Collision detected for short ID '%s', retrying...", shortID)
			continue
		}

		// Other database error occurred
		log.Printf("Error saving URL to database: %v", err)
		return "", fmt.Errorf("failed to save URL to database: %w", err)
	}

	return "", errors.New("failed to generate a unique short ID after multiple attempts")
}

func (s *Storage) Load(ctx context.Context, shortID string) (string, error) {
	var longURL string

	stmt := `SELECT long_url FROM urls WHERE short_id = $1`
	row := s.db.QueryRowContext(ctx, stmt, shortID)

	err := row.Scan(&longURL)
	if err != nil {
		// shortID is not found
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("short ID not found: %s", shortID)
		}
		// Other database error occurred
		log.Printf("Error loading URL from database: %v", err)
		return "", fmt.Errorf("failed to load URL from database: %w", err)
	}

	return longURL, nil
}

func (s *Storage) generateShortID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, shortIDLength)
	for i := range b {
		b[i] = charset[s.r.Intn(len(charset))]
	}
	return string(b)
}
