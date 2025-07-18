package service

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/rs/zerolog/log"

	"channelog/config"
)

// OpenAIService provides OpenAI client functionality
type OpenAIService struct {
	client openai.Client
	model  shared.ChatModel
}

// NewOpenAIService creates a new OpenAI service instance using the provided configuration
func NewOpenAIService(cfg *config.Config) *OpenAIService {
	opts := []option.RequestOption{
		option.WithBaseURL(cfg.OpenAIApiUrl),
	}

	client := openai.NewClient(opts...)

	log.Info().
		Str("model", cfg.OpenAIModel).
		Str("api_url", cfg.OpenAIApiUrl).
		Msg("OpenAI client initialized")

	return &OpenAIService{
		client: client,
		model:  shared.ChatModel(cfg.OpenAIModel),
	}
}

// GetClient returns the OpenAI client instance
func (s *OpenAIService) GetClient() *openai.Client {
	return &s.client
}

// CreateChatCompletion creates a chat completion using the configured model
func (s *OpenAIService) CreateChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {
	response, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    s.model,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create chat completion")
		return nil, err
	}
	return response, nil
}

// GenerateText is a helper function to generate text using OpenAI
// Example usage with user message
func (s *OpenAIService) GenerateText(ctx context.Context, prompt string) (string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	response, err := s.CreateChatCompletion(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return response.Choices[0].Message.Content, nil
}
