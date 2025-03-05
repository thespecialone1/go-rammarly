package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/thespecialone1/go-rammerly/db"
)

const (
	// SessionName is the name used for the cookie session store
	SessionName = "user-session"

	// SessionMaxAge defines how long the session should last
	SessionMaxAge = 86400 // 1 day in seconds

	// StateTokenSize is the number of random bytes for the state token
	StateTokenSize = 32

	// Environment URLs
	localDevURL   = "http://localhost:8080"
	renderDevURL  = "https://go-rammarly.onrender.com"
	productionURL = "https://go-rammarly.duckdns.org"
)

// Environment variables
const (
	EnvEnvironment        = "ENVIRONMENT"
	EnvGoogleClientID     = "GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "GOOGLE_CLIENT_SECRET"
	EnvSessionSecret      = "SESSION_SECRET_KEY"
	EnvRedirectURL        = "OAUTH_REDIRECT_URL"
)

// Environment types
const (
	EnvLocalDev   = "local"
	EnvRenderDev  = "render"
	EnvProduction = "production"
)

// SessionStore provides access to the session storage
var SessionStore *sessions.CookieStore

// Queries is a global pointer to the sqlc–generated queries.
// It will be set from main.go once the database connection is established.
var Queries *db.Queries

// persistentSecretKey holds the secret key to ensure it remains consistent
// var persistentSecretKey string

// GetEnvironment determines the current environment
func GetEnvironment() string {
	env := strings.ToLower(os.Getenv(EnvEnvironment))

	switch env {
	case EnvLocalDev:
		return EnvLocalDev
	case EnvRenderDev:
		return EnvRenderDev
	case EnvProduction:
		return EnvProduction
	default:
		// Default to local development if not specified
		return EnvLocalDev
	}
}

// GoogleUserInfo represents the user information returned from Google OAuth
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// IsProduction returns true if the current environment is production
func IsProduction() bool {
	return GetEnvironment() == EnvProduction
}

// GetBaseURL returns the appropriate base URL based on environment
func GetBaseURL() string {
	switch GetEnvironment() {
	case EnvLocalDev:
		return localDevURL
	case EnvRenderDev:
		return renderDevURL
	case EnvProduction:
		return productionURL
	default:
		return localDevURL
	}
}

// InitSessionStore sets up the session store with proper configuration
func InitSessionStore() {
	// Use a consistent, long-lived secret key
	secretKey := []byte(os.Getenv("SESSION_SECRET_KEY"))
	if len(secretKey) == 0 {
		secretKey = []byte("fallback-very-long-secret-key-at-least-32-bytes-long")
		log.Println("WARNING: Using fallback session secret")
	}

	SessionStore = sessions.NewCookieStore(secretKey)
	SessionStore.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Consider environment-based setting
		MaxAge:   86400, // 1 day
		SameSite: http.SameSiteLaxMode,
	}
}

// GetGoogleOAuthConfig returns the OAuth2 configuration for Google.
func GetGoogleOAuthConfig() *oauth2.Config {
	// Check for custom redirect URL first
	redirectURL := os.Getenv(EnvRedirectURL)

	// If not set, construct from base URL
	if redirectURL == "" {
		redirectURL = GetBaseURL() + "/auth/google/callback"
	}

	clientID := os.Getenv(EnvGoogleClientID)
	clientSecret := os.Getenv(EnvGoogleClientSecret)

	if clientID == "" || clientSecret == "" {
		log.Println("WARNING: Google OAuth credentials missing or incomplete")
	}

	return &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// generateStateToken creates a random token to mitigate CSRF attacks.
func generateStateToken() (string, error) {
	log.Println("Generating state token")
	b := make([]byte, StateTokenSize)
	n, err := rand.Read(b)
	if err != nil {
		log.Printf("Random read error: %v", err)
		return "", err
	}
	log.Printf("Generated %d random bytes", n)
	return base64.URLEncoding.EncodeToString(b), nil
}

// ClearInvalidSession removes any invalid session cookies
func ClearInvalidSession(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     SessionName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   GetEnvironment() != EnvLocalDev,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// GetSessionSafe retrieves the session and handles invalid sessions gracefully
func GetSessionSafe(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, err := SessionStore.Get(r, SessionName)
	if err != nil {
		log.Printf("Session retrieval error: %v", err)
		// Clear the invalid cookie
		ClearInvalidSession(w)
		// Create a fresh session
		session = sessions.NewSession(SessionStore, SessionName)
		session.Options = SessionStore.Options
		session.IsNew = true
	}
	return session
}

// HandleGoogleLogin initiates the Google OAuth login process.
func HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	// Ensure session store is initialized
	if SessionStore == nil {
		InitSessionStore()
	}

	config := GetGoogleOAuthConfig()

	// Generate state token
	state, err := generateStateToken()
	if err != nil {
		log.Printf("State token generation error: %v", err)
		http.Error(w, "Authentication initialization failed", http.StatusInternalServerError)
		return
	}

	// Create a new session if needed
	session, err := SessionStore.Get(r, SessionName)
	if err != nil {
		log.Printf("Session retrieval error: %v", err)
		session = sessions.NewSession(SessionStore, SessionName)
	}

	// Store state in session
	session.Values["state"] = state
	session.Values["state_expiry"] = time.Now().Add(5 * time.Minute).Unix()

	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Redirect to Google's OAuth consent page
	authURL := config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleGoogleCallback handles the OAuth callback, inserts user info into the database, and saves the session.
func HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Get the session safely
	session := GetSessionSafe(w, r)

	// Verify state parameter to prevent CSRF
	if err := validateState(r, session); err != nil {
		log.Printf("State validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for a token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	config := GetGoogleOAuthConfig()
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("Token exchange error: %v", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Get user info from Google
	userInfo, err := fetchGoogleUserInfo(token.AccessToken)
	if err != nil {
		log.Printf("User info fetch error: %v", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	// Process user in database
	user, err := processUser(r.Context(), userInfo)
	if err != nil {
		log.Printf("User processing error: %v", err)
		http.Error(w, "User account processing failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Login successful for user: %s (ID: %d)", user.Name, user.ID)

	// Save user session
	if err := saveUserSession(r, w, session, user); err != nil {
		log.Printf("Session save error: %v", err)
		http.Error(w, "Failed to save user session", http.StatusInternalServerError)
		return
	}

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// validateState verifies the state parameter to prevent CSRF attacks
func validateState(r *http.Request, session *sessions.Session) error {
    // More robust state validation
    storedState, ok := session.Values["state"].(string)
    if !ok {
        return fmt.Errorf("no state token found in session")
    }

    // Compare state tokens
    incomingState := r.URL.Query().Get("state")
    if incomingState == "" {
        return fmt.Errorf("no state parameter in request")
    }

    if incomingState != storedState {
        log.Printf("State mismatch. Stored: %s, Incoming: %s", storedState, incomingState)
        return fmt.Errorf("state token mismatch")
    }

    return nil
}

// fetchGoogleUserInfo retrieves the user information from Google's userinfo endpoint
func fetchGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info JSON: %w", err)
	}

	return &userInfo, nil
}

// processUser checks if the user exists in the database and creates a new record if necessary
func processUser(ctx context.Context, userInfo *GoogleUserInfo) (*db.User, error) {
	// Log incoming user information for debugging
	log.Printf("Processing user - GoogleID: %s, Email: %s, Name: %s",
		userInfo.ID, userInfo.Email, userInfo.Name)

	// Existing user lookup
	existingUser, err := Queries.GetUserByGoogleID(ctx, userInfo.ID)
	if err == nil {
		log.Printf("Existing user found: %d", existingUser.ID)
		return &existingUser, nil
	}

	if err != sql.ErrNoRows {
		// Log the specific database error
		log.Printf("Database query error: %v", err)
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// User creation
	newUser, err := Queries.CreateUser(ctx, db.CreateUserParams{
		GoogleID: userInfo.ID,
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Picture: sql.NullString{
			String: userInfo.Picture,
			Valid:  userInfo.Picture != "",
		},
	})
	if err != nil {
		// Log detailed error information
		log.Printf("User creation failed: %v", err)
		return nil, fmt.Errorf("failed to create user: detailed database error: %w", err)
	}

	log.Printf("Created new user with ID: %d", newUser.ID)
	return &newUser, nil
}

// saveUserSession saves the user information in the session
func saveUserSession(r *http.Request, w http.ResponseWriter, session *sessions.Session, user *db.User) error {
	// Save user data in session
	session.Values["user_id"] = user.ID
	session.Values["user_email"] = user.Email
	session.Values["user_name"] = user.Name
	session.Values["picture"] = user.Picture.String
	session.Values["google_id"] = user.GoogleID // Store Google ID for GetCurrentUser function

	// Clear the state after successful authentication
	delete(session.Values, "state")
	delete(session.Values, "state_expiry")

	// Save the session
	return session.Save(r, w)
}

// GetSession retrieves the user session for the given request.
func GetSession(r *http.Request) (*sessions.Session, error) {
	return SessionStore.Get(r, SessionName)
}

// GetCurrentUser returns the current authenticated user from the session, or nil if not logged in
func GetCurrentUser(r *http.Request) (*db.User, error) {
	session, err := GetSession(r)
	if err != nil {
		return nil, fmt.Errorf("session error: %w", err)
	}

	userID, ok := session.Values["user_id"].(int64)
	if !ok || userID == 0 {
		return nil, nil // No user logged in
	}

	// Get the Google ID from the session
	googleID, ok := session.Values["google_id"].(string)
	if !ok || googleID == "" {
		return nil, fmt.Errorf("user session data incomplete")
	}

	user, err := Queries.GetUserByGoogleID(r.Context(), googleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return &user, nil
}

// RequireAuthentication is a middleware that ensures a user is logged in
func RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := GetSession(r)
		if err != nil {
			log.Printf("Authentication middleware - session error: %v", err)
			ClearInvalidSession(w)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		if _, ok := session.Values["user_id"].(int64); !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LogoutHandler clears the user session and invalidates the session cookie.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Use safe session getter instead
	session := GetSessionSafe(w, r)

	// Clear session values and set MaxAge to -1 to expire the cookie immediately.
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		log.Printf("Session clear error: %v", err)
		http.Error(w, "Failed to log out", http.StatusInternalServerError)
		return
	}

	log.Println("User successfully logged out.")
	// Redirect to the home page after logout.
	http.Redirect(w, r, "/", http.StatusFound)
}

// init initializes the session store with environment-specific settings
func init() {
	InitSessionStore()

	// Log environment information
	env := GetEnvironment()
	log.Printf("Running in %s environment", strings.ToUpper(env))
	log.Printf("Using base URL: %s", GetBaseURL())

	// Check if session secret is set
	if os.Getenv(EnvSessionSecret) == "" && IsProduction() {
		log.Println("WARNING: No session secret key set in production environment. Set SESSION_SECRET_KEY for better security.")
	}
}
