package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	oai "github.com/sashabaranov/go-openai"
)

// Client wraps the OpenAI API for franchise naming.
type Client struct {
	client *oai.Client
	model  string
}

// FranchiseNameResult holds the AI-generated franchise name.
type FranchiseNameResult struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// NewClient creates an OpenAI client if the API key is available.
func NewClient() *Client {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		log.Println("OPENAI_API_KEY not set — franchise naming will use fallback methods")
		return nil
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &Client{
		client: oai.NewClient(key),
		model:  model,
	}
}

// GenerateFranchiseName asks OpenAI for the best franchise name given a list of titles.
func (c *Client) GenerateFranchiseName(ctx context.Context, titles []string) (*FranchiseNameResult, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("OpenAI client not configured")
	}

	prompt := fmt.Sprintf(`Given these anime titles that belong to the same franchise, determine the best franchise name.
The name should be the most recognizable name for this group of anime.

Titles:
%s

Respond ONLY with a JSON object: {"name": "Franchise Name", "confidence": 0.95}
confidence should be between 0 and 1 indicating how confident you are.`, strings.Join(titles, "\n"))

	var result *FranchiseNameResult

	for attempt := 0; attempt < 3; attempt++ {
		resp, err := c.client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
			Model: c.model,
			Messages: []oai.ChatCompletionMessage{
				{Role: oai.ChatMessageRoleUser, Content: prompt},
			},
			Temperature: 0.3,
			MaxTokens:   100,
		})

		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate") {
				backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				log.Printf("[openai] Rate limited, retrying in %v", backoff)
				time.Sleep(backoff)
				continue
			}
			return nil, fmt.Errorf("OpenAI API error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no choices in response")
		}

		content := strings.TrimSpace(resp.Choices[0].Message.Content)
		// Strip markdown code fences if present.
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)

		result = &FranchiseNameResult{}
		if err := json.Unmarshal([]byte(content), result); err != nil {
			log.Printf("[openai] Failed to parse response: %s", content)
			return nil, fmt.Errorf("parse response: %w", err)
		}

		return result, nil
	}

	return result, nil
}

// RateLimitDelay is the minimum delay between OpenAI API calls (120ms = 500 req/min).
const RateLimitDelay = 120 * time.Millisecond
