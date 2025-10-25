package handlers

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
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
	model := client.GenerativeModel("gemini-1.5-pro-latest")
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

// HandleMultimodalText generates text from a given prompt and image using the Gemini API.
// POST /api/v1/gemini/multimodal
func HandleMultimodalText(c *fiber.Ctx) error {
	var body struct {
		Prompt    string `json:"prompt"`
		ImageData string `json:"image_data"` // base64 encoded image with prefix e.g. "data:image/png;base64,"
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

    // Extract image format and data
    parts := strings.Split(body.ImageData, ";base64,")
    if len(parts) != 2 {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid image data format"})
    }
    
    mimeTypeParts := strings.Split(strings.TrimPrefix(parts[0], "data:"), "/")
    if len(mimeTypeParts) != 2 {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid image mime type"})
    }
    imageFormat := mimeTypeParts[1]
    
    imageData, err := base64.StdEncoding.DecodeString(parts[1])
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Failed to decode image data"})
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
	defer client.Close()

	// Use the Gemini model for multimodal input
	model := client.GenerativeModel("gemini-1.5-pro-latest")

	// Create the prompt with text and image
	prompt := []genai.Part{
		genai.Text(body.Prompt),
		genai.ImageData(imageFormat, imageData),
	}

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		log.Printf("Error generating content: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to generate text from multimodal input",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   resp,
	})
}
