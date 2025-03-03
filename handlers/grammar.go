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
		<div id="grammar-result" class="output mt-4">
			<div class="result-text" style="color:#333;">{{.CorrectedText}}}</div>
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
