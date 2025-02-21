package handlers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/thespecialone1/go-rammerly/models"
	"github.com/thespecialone1/go-rammerly/services"
)

// HandleImageAnalysis renders the image analysis page.
func HandleImageAnalysis(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{CurrentPage: "image-analysis"}
	tmpl.Execute(w, data)
}

// HandleAnalyzeImage processes an uploaded image, saves it, and returns the analysis.
func HandleAnalyzeImage(w http.ResponseWriter, r *http.Request) {
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

	// Save the image (optional).
	if err := services.SaveImageToFile(imageData, filename); err != nil {
		log.Printf("Error saving image: %v", err)
	}

	analysis := services.AnalyzeImage(imageData)
	data := models.PageData{
		CurrentPage:   "image-analysis",
		ImageAnalysis: analysis,
	}

	// Return a partial if HTMX header is set.
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
