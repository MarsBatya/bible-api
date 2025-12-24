package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// Translation configuration
var translations = map[string]string{
	"KJV": "assets/KJV+.Sqlite3",
	"RST": "assets/RST+.Sqlite3",
}

// Database connection pool for each translation
var dbPool = make(map[string]*sql.DB)
var dbMutex sync.RWMutex

// Response structures
type VerseResponse struct {
	Translation     string `json:"translation"`
	BookNumber      int    `json:"book_number"`
	BookTitle       string `json:"book_title"`
	BookTitleShort  string `json:"book_title_short"`
	Chapter         int    `json:"chapter"`
	Verse           int    `json:"verse"`
	Text            string `json:"text"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Regex for cleaning text (matches Python version)
var textCleanRegex = regexp.MustCompile(`(<S>\d+</S>|</?[^ai <>]+/?>)`)
var whitespaceRegex = regexp.MustCompile(`\s+`)

// Clean text function (Python equivalent)
func clearText(text string) string {
	cleaned := textCleanRegex.ReplaceAllString(text, "")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = whitespaceRegex.ReplaceAllString(cleaned, " ")
	return cleaned
}

// Initialize database connections
func initDatabases() error {
	for name, path := range translations {
		// Check if file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("Warning: Database file not found for %s: %s", name, path)
			continue
		}

		// Open database with read-only and connection pooling
		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&cache=shared", path))
		if err != nil {
			return fmt.Errorf("failed to open database %s: %v", name, err)
		}

		// Set connection pool settings for concurrent reads
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)

		// Test connection
		if err := db.Ping(); err != nil {
			db.Close()
			log.Printf("Warning: Failed to ping database %s: %v", name, err)
			continue
		}

		dbPool[name] = db
		log.Printf("Successfully connected to %s database", name)
	}

	if len(dbPool) == 0 {
		return fmt.Errorf("no valid databases could be loaded")
	}

	return nil
}

// Get random verse handler
func getRandomVerseHandler(w http.ResponseWriter, r *http.Request) {
	// Extract translation name from URL path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		respondWithError(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	translationName := parts[1]

	// Check if translation exists in configuration
	if _, exists := translations[translationName]; !exists {
		respondWithError(w, fmt.Sprintf("Translation '%s' not found", translationName), http.StatusNotFound)
		return
	}

	// Get database connection
	dbMutex.RLock()
	db, exists := dbPool[translationName]
	dbMutex.RUnlock()

	if !exists {
		respondWithError(w, fmt.Sprintf("Database for translation '%s' is not available", translationName), http.StatusServiceUnavailable)
		return
	}

	// Execute query
	var verse VerseResponse
	var rawText string

	query := `
		SELECT v.book_number, v.chapter, v.verse, v.text, b.short_name, b.long_name
		FROM verses v
		JOIN books b ON v.book_number = b.book_number
		ORDER BY RANDOM()
		LIMIT 1
	`

	err := db.QueryRow(query).Scan(
		&verse.BookNumber,
		&verse.Chapter,
		&verse.Verse,
		&rawText,
		&verse.BookTitleShort,
		&verse.BookTitle,
	)

	if err != nil {
		log.Printf("Database query error for %s: %v", translationName, err)
		respondWithError(w, "Failed to retrieve verse", http.StatusInternalServerError)
		return
	}

	// Clean text and set translation name
	verse.Text = clearText(rawText)
	verse.Translation = translationName

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(verse)
}

// Helper function to respond with errors
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	dbMutex.RLock()
	availableTranslations := make([]string, 0, len(dbPool))
	for name := range dbPool {
		availableTranslations = append(availableTranslations, name)
	}
	dbMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"translations": availableTranslations,
	})
}

// Logging middleware
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}
		log.Printf("[%s] %s from %s", r.Method, r.URL.Path, ip)
		next(w, r)
	}
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next(w, r)
	}
}

func main() {
	// Initialize databases
	log.Println("Initializing databases...")
	if err := initDatabases(); err != nil {
		log.Fatalf("Failed to initialize databases: %v", err)
	}

	// Defer closing all database connections
	defer func() {
		for name, db := range dbPool {
			log.Printf("Closing database connection for %s", name)
			db.Close()
		}
	}()

	// Setup routes
	http.HandleFunc("/get-random-verse/", corsMiddleware(loggingMiddleware(getRandomVerseHandler)))
	http.HandleFunc("/health", corsMiddleware(loggingMiddleware(healthHandler)))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s...", port)
	log.Printf("Available translations: %v", func() []string {
		keys := make([]string, 0, len(dbPool))
		for k := range dbPool {
			keys = append(keys, k)
		}
		return keys
	}())

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
