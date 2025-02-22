package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Environment types
const (
	EnvLocal      = "local"
	EnvRender     = "render"
	EnvProduction = "production"
)

// LoadEnv loads the appropriate environment configuration
func LoadEnv() error {
	// Check if we're running on Render.com
	if os.Getenv("RENDER") != "" {
		log.Println("Running on Render.com, using environment variables")
		return nil // Use existing env vars
	}

	// Check if we're running in production (DuckDNS)
	if os.Getenv("PRODUCTION") != "" {
		log.Println("Running in production, using environment variables")
		return nil // Use existing env vars
	}

	// Default to local environment if not specified
	envFile := ".env.local"

	// Check if environment is explicitly set
	switch strings.ToLower(os.Getenv("GO_ENV")) {
	case EnvLocal:
		envFile = ".env.local"
	case EnvRender:
		envFile = ".env.render"
	case EnvProduction:
		envFile = ".env.production"
	}

	// Load the environment file
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("error loading %s file: %w", envFile, err)
	}

	log.Printf("Loaded environment from %s", envFile)
	return nil
}