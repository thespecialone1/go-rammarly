package services

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// AnalyzeImage processes the image data using the generative AI API.
func AnalyzeImage(imageData []byte) string {
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
