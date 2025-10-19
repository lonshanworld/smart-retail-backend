package handlers

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

// HandleGenerateText generates text from a given prompt using the Gemini API.
// POST /api/v1/gemini/generate
func HandleGenerateText(c *fiber.Ctx) error {
	// Get the prompt from the request body
	var body struct {
		Prompt string `json:"prompt"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

	// Initialize the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Printf("Error creating Gemini client: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to initialize Gemini client",
		})
	}

	// Use the Gemini model to generate text
	model := client.GenerativeModel("gemini-1.5-pro")
	resp, err := model.GenerateContent(ctx, genai.Text(body.Prompt))
	if err != nil {
		log.Printf("Error generating content: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to generate text",
		})
	}

	// Return the generated text
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   resp,
	})
}
