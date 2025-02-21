package services

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// CallGrammarAPI processes the provided text using the generative AI API.
func CallGrammarAPI(text string) string {
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
