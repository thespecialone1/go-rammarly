package main

import (
    "context"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "os"

    "github.com/google/generative-ai-go/genai"
    "github.com/joho/godotenv"
    "google.golang.org/api/option"
)

type PageData struct {
    OriginalText  string
    CorrectedText string
    ImageAnalysis string
}

var tmpl *template.Template

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    tmpl = template.Must(template.ParseFiles("index.html"))

    http.HandleFunc("/", handleHome)
    http.HandleFunc("/generate", handleGenerate)
    http.HandleFunc("/analyze-image", handleImageAnalysis)

    fmt.Println("Server is running on http://127.0.0.1:8080")
    log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
    tmpl.Execute(w, nil)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    originalText := r.FormValue("text")
    correctedText := callGrammarAPI(originalText)

    data := PageData{
        OriginalText:  originalText,
        CorrectedText: correctedText,
    }

    tmpl.Execute(w, data)
}

func handleImageAnalysis(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    file, _, err := r.FormFile("image")
    if err != nil {
        http.Error(w, "Error retrieving the file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    imageData, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "Error reading the file", http.StatusInternalServerError)
        return
    }

    analysis := analyzeImage(imageData)

    data := PageData{
        ImageAnalysis: analysis,
    }

    tmpl.Execute(w, data)
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