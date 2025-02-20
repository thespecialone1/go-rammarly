package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type PageData struct {
	CurrentPage   string
	OriginalText  string
	CorrectedText string
	ImageAnalysis string
}

// Create a session store (use a strong key in production)
var store = sessions.NewCookieStore([]byte("your-very-secret-key"))
func init() {
    if os.Getenv("GO_ENV") != "production" {
        if err := godotenv.Load(); err != nil {
            log.Fatal("Error loading .env file")
        }
    }
    // Check if the client ID is set
    if os.Getenv("GOOGLE_CLIENT_ID") == "" {
        log.Fatal("GOOGLE_CLIENT_ID is not set")
    }
}

// Set up the OAuth2 configuration using your Google credentials and desired scopes.
// Change this from a variable to a function
func getGoogleOAuthConfig() *oauth2.Config {
    return &oauth2.Config{
        RedirectURL:  "http://localhost:8080/auth/google/callback",
        ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
        Scopes: []string{
            "https://www.googleapis.com/auth/userinfo.email",
            "https://www.googleapis.com/auth/userinfo.profile",
        },
        Endpoint: google.Endpoint,
    }
}

// generateStateToken creates a random string to be used as the OAuth2 state parameter.
func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ----- 2a. CREATE THE LOGIN INITIATION ROUTE (/auth/google) -----

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	// Get the config when needed
    googleOauthConfig := getGoogleOAuthConfig()
	// Generate a random state token to help protect against CSRF attacks.
	state, err := generateStateToken()
	if err != nil {
		http.Error(w, "Failed to generate state token", http.StatusInternalServerError)
		return
	}

	// Store the state token in the session.
	session, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, "Unable to get session", http.StatusInternalServerError)
		return
	}
	session.Values["state"] = state
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Generate the Google OAuth URL and redirect the user.
    authURL := googleOauthConfig.AuthCodeURL(state)
    http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}
// ----- 2b. CREATE THE CALLBACK ROUTE (/auth/google/callback) -----

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Get the config when needed
    googleOauthConfig := getGoogleOAuthConfig()
	// Retrieve the session to validate the state token.
	session, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, "Unable to get session", http.StatusInternalServerError)
		return
	}
	storedState, ok := session.Values["state"].(string)
	if !ok || storedState == "" {
		http.Error(w, "Invalid session state", http.StatusBadRequest)
		return
	}

	// Validate that the state parameter matches.
	queryState := r.URL.Query().Get("state")
	if queryState != storedState {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Retrieve the authorization code from the URL query.
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for an access token.
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	// Use the access token to fetch user information.
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read user info", http.StatusInternalServerError)
		return
	}

	// Parse the JSON response containing user information.
	var userInfo struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(data, &userInfo); err != nil {
		http.Error(w, "Failed to parse user info", http.StatusInternalServerError)
		return
	}

	// Establish a session for the authenticated user.
	session.Values["user_id"] = userInfo.ID
	session.Values["user_email"] = userInfo.Email
	session.Values["user_name"] = userInfo.Name
	session.Values["picture"] = userInfo.Picture
	// Optionally, you could store the user's picture or other details.
	// Remove the state token since it’s no longer needed.
	delete(session.Values, "state")
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}else{
	}

	// Redirect the user to the homepage (or a protected page).
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}



var tmpl *template.Template

func main() {
	// Use Port environment variable provided by Render
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default port if not set
	}

	// Load .env only in development
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Fatal("Error loading .env file")
		}
	}

	// Parse the base template
	tmpl = template.Must(template.ParseFiles("templates/index.html"))

	// Routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/grammar", handleGrammar)
	http.HandleFunc("/image-analysis", handleImageAnalysis)
	http.HandleFunc("/generate", handleGenerate)
	http.HandleFunc("/analyze-image", handleAnalyzeImage)

	// ----- Register the Google OAuth routes -----
	http.HandleFunc("/auth/google", handleGoogleLogin)
	http.HandleFunc("/auth/google/callback", handleGoogleCallback)

	// Serve manifest.json and service-worker.js from a "static" folder.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Printf("Server is running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	data := PageData{CurrentPage: "home"}
	tmpl.Execute(w, data)
}

func handleGrammar(w http.ResponseWriter, r *http.Request) {
	data := PageData{CurrentPage: "grammar"}
	tmpl.Execute(w, data)
}

func handleImageAnalysis(w http.ResponseWriter, r *http.Request) {
	data := PageData{CurrentPage: "image-analysis"}
	tmpl.Execute(w, data)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	originalText := r.FormValue("text")
	// Save to CSV
	if err := saveTextToCSV(originalText); err != nil {
		log.Printf("Error saving text: %v", err)
	}

	correctedText := callGrammarAPI(originalText)

	data := PageData{
		CurrentPage:   "grammar",
		OriginalText:  originalText,
		CorrectedText: correctedText,
	}

	// If this is an HTMX request, return only the corrected text fragment
	if r.Header.Get("HX-Request") != "" {
		partial := `
        {{if .CorrectedText}}
        <div id="grammar-result" class="skeleton animate-fade-in mt-6 p-5 bg-orange-50/50 rounded-lg border border-orange-100">
            <div class="flex justify-between items-center">
                <h3 class="text-lg font-medium text-gray-800 mb-3">Corrected Text:</h3>
                <button id="copy-btn" class="ripple text-sm bg-orange-500 hover:bg-orange-600 text-white px-3 py-1 rounded" onclick="copyResult()">Copy</button>
            </div>
            <div id="formatted-response" class="text-gray-700 leading-relaxed">{{.CorrectedText}}</div>
        </div>
        {{end}}
        `
		t, err := template.New("partial").Parse(partial)
		if err != nil {
			log.Printf("Error parsing partial template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		t.Execute(w, data)
		return
	}

	tmpl.Execute(w, data)
}

func handleAnalyzeImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".bin"
	}
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading the file", http.StatusInternalServerError)
		return
	}

	// Save image locally
	if err := saveImageToFile(imageData, filename); err != nil {
		log.Printf("Error saving image: %v", err)
	}

	analysis := analyzeImage(imageData)
	data := PageData{
		CurrentPage:   "image-analysis",
		ImageAnalysis: analysis,
	}

	// If HTMX, return a fragment with only the analysis result
	if r.Header.Get("HX-Request") != "" {
		partial := `
        {{if .ImageAnalysis}}
        <div id="image-result" class="skeleton animate-fade-in mt-6 p-5 bg-orange-50/50 rounded-lg border border-orange-100">
            <h3 class="text-lg font-medium text-gray-800 mb-3">Image Analysis:</h3>
            <div class="text-gray-700 leading-relaxed">{{.ImageAnalysis}}</div>
        </div>
        {{end}}
        `
		t, err := template.New("partial").Parse(partial)
		if err != nil {
			log.Printf("Error parsing partial template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		t.Execute(w, data)
		return
	}

	tmpl.Execute(w, data)
}

func saveTextToCSV(text string) error {
	// Create data directory if not exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	// Open CSV file in append mode
	file, err := os.OpenFile("data/text.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	return writer.Write([]string{text})
}

func saveImageToFile(data []byte, filename string) error {
	// Create images directory if not exists
	if err := os.MkdirAll("data/images", 0755); err != nil {
		return err
	}
	filePath := filepath.Join("data/images", filename)
	return os.WriteFile(filePath, data, 0644)
}

// callGrammarAPI returns a dummy response in test mode.
func callGrammarAPI(text string) string {

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Printf("Error creating client: %v", err)
		return "Error occurred while processing your request."
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")
	prompt := "Correct the grammar and improve the tone of the following text: " + text

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Error generating content: %v", err)
		return "Error occurred while processing your request."
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	}
	return "No response generated."
}

// analyzeImage returns a dummy analysis in test mode.
func analyzeImage(imageData []byte) string {
	// if os.Getenv("TEST_MODE") == "true" {
	// 	return "Test Mode: This is a dummy image analysis. The image is described as vibrant and meme-worthy."
	// }

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Printf("Error creating client: %v", err)
		return "Error occurred while processing your request."
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")
	prompt := []genai.Part{
		genai.ImageData("jpeg", imageData),
		genai.Text("Describe this image in detail."),
	}

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		log.Printf("Error generating content: %v", err)
		return "Error occurred while analyzing the image."
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	}
	return "No analysis generated."
}
