package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/thespecialone1/go-rammerly/models"
	"github.com/thespecialone1/go-rammerly/services"
)

// HandleGrammar renders the grammar page.
func HandleGrammar(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{CurrentPage: "grammar"}
	tmpl.Execute(w, data)
}

// HandleGenerate processes the text, calls the grammar API, and renders the result.
func HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	originalText := r.FormValue("text")

	// Save text via services.
	if err := services.SaveTextToCSV(originalText); err != nil {
		log.Printf("Error saving text: %v", err)
	}

	correctedText := services.CallGrammarAPI(originalText)
	data := models.PageData{
		CurrentPage:   "grammar",
		OriginalText:  originalText,
		CorrectedText: correctedText,
	}

	// Return a partial if this is an HTMX request.
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
