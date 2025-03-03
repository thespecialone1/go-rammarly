package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from the appropriate .env file
func LoadEnv() error {
	// Try loading environment-specific .env files
	envs := []string{
		".env." + strings.ToLower(os.Getenv("ENVIRONMENT")), // .env.production, .env.render, etc.
		".env.local",                                         // Default local env
		".env",                                               // Fallback to .env
	}

	var loaded bool
	for _, env := range envs {
		if err := godotenv.Load(env); err == nil {
			log.Printf("Loaded environment from %s", env)
			loaded = true
			break
		}
	}

	if !loaded {
		log.Println("No .env file loaded, using system environment variables")
	}

	// Verify critical environment variables
	checkEnvVar("GOOGLE_CLIENT_ID")
	checkEnvVar("GOOGLE_CLIENT_SECRET")
	checkEnvVar("SESSION_SECRET_KEY")

	return nil
}

// checkEnvVar verifies if an important environment variable is set
func checkEnvVar(name string) {
	if value := os.Getenv(name); value == "" {
		log.Printf("WARNING: Environment variable %s is not set", name)
	} else {
		// For sensitive variables, just log that they are set
		if strings.Contains(strings.ToLower(name), "secret") {
			masked := fmt.Sprintf("%s is set", name)
			log.Println(masked)
		} else {
			log.Printf("%s = %s", name, value)
		}
	}
}