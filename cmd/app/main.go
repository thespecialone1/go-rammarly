package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/thespecialone1/go-rammerly/auth"
	"github.com/thespecialone1/go-rammerly/db"
	"github.com/thespecialone1/go-rammerly/handlers"
	_ "github.com/joho/godotenv"
	"github.com/thespecialone1/go-rammerly/config"
)

func main() {
	// Load environment configuration
	if err := config.LoadEnv(); err != nil {
		log.Fatalf("Failed to load environment: %v", err)
	}

	// Get the project root directory
	cmdDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	
	// Navigate to project root (assuming cmd/app is one level deep)
	projectRoot := filepath.Dir(filepath.Dir(cmdDir))
	
	// Construct full path to database file
	dbPath := filepath.Join(projectRoot, "./db.sqlite")
	
	log.Printf("Attempting to open database at: %s", dbPath)

	// Open the SQLite database 
	dbConn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer dbConn.Close()

	// Verify database connection
	if err := dbConn.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create the users table if it doesn't exist.
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		google_id TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		picture TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := dbConn.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Verify table creation
	var tableName string
	err = dbConn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		log.Fatalf("Failed to verify users table creation: %v", err)
	}
	log.Println("Users table verified successfully")

	// Initialize sqlc-generated queries.
	queries := db.New(dbConn)
	// Set the global Queries variable for the auth package.
	auth.Queries = queries

	// Initialize templates.
	handlers.InitTemplates()

	// Set up HTTP routes.
	http.HandleFunc("/", handlers.HandleHome)
	http.HandleFunc("/grammar", handlers.HandleGrammar)
	http.HandleFunc("/image-analysis", handlers.HandleImageAnalysis)
	http.HandleFunc("/generate", handlers.HandleGenerate)
	http.HandleFunc("/analyze-image", handlers.HandleAnalyzeImage)
	http.HandleFunc("/logout", auth.LogoutHandler)

	// Google OAuth routes.
	http.HandleFunc("/auth/google", auth.HandleGoogleLogin)
	http.HandleFunc("/auth/google/callback", auth.HandleGoogleCallback)

	// Serve static assets.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server is running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}