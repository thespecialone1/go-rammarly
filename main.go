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

var tmpl *template.Template

func main() {
	// Use Port environment variable provided by Render
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default port if not set
	}

	// Load .env only in development
	if os.Getenv("GO_ENV") != "production" {
		err := godotenv.Load()
		if err != nil {
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
        <div id="grammar-result" class="animate-fade-in mt-6 p-5 bg-orange-50/50 rounded-lg border border-orange-100">
            <div class="flex justify-between items-center">
                <h3 class="text-lg font-medium text-gray-800 mb-3">Corrected Text:</h3>
                <button id="copy-btn" class="text-sm bg-orange-500 hover:bg-orange-600 text-white px-3 py-1 rounded" onclick="copyResult()">Copy</button>
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

	// Save image
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
        <div id="image-result" class="animate-fade-in mt-6 p-5 bg-orange-50/50 rounded-lg border border-orange-100">
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

	// Create file path
	filePath := filepath.Join("data/images", filename)
	return os.WriteFile(filePath, data, 0644)
}

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

func analyzeImage(imageData []byte) string {
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
