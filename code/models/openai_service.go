package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/rs/zerolog/log"

	"channelog/config"
)

// OpenAIService provides OpenAI client functionality
type OpenAIService struct {
	client              openai.Client
	model               shared.ChatModel
	systemPrompt        string
	userMessageTemplate string
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
		Bool("has_system_prompt", cfg.SystemPrompt != "").
		Bool("has_user_template", cfg.UserMessageTemplate != "").
		Msg("OpenAI client initialized")

	return &OpenAIService{
		client:              client,
		model:               shared.ChatModel(cfg.OpenAIModel),
		systemPrompt:        cfg.SystemPrompt,
		userMessageTemplate: cfg.UserMessageTemplate,
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

// CreateChatCompletionWithSystemPrompt creates a chat completion with the configured system prompt
func (s *OpenAIService) CreateChatCompletionWithSystemPrompt(ctx context.Context, userMessage string) (*openai.ChatCompletion, error) {
	messages := []openai.ChatCompletionMessageParamUnion{}

	// Add system prompt if configured
	if s.systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(s.systemPrompt))
	}

	// Add user message
	messages = append(messages, openai.UserMessage(userMessage))

	return s.CreateChatCompletion(ctx, messages)
}

// GenerateChangelogEntry generates a changelog entry using the configured templates
// oldObject, newObject should be YAML strings of the Kubernetes resources
// gitDiff should be the git diff string
func (s *OpenAIService) GenerateChangelogEntry(ctx context.Context, oldObject, newObject, gitDiff string) (string, error) {
	if s.userMessageTemplate == "" {
		return "", fmt.Errorf("user message template not configured")
	}

	// Validate inputs
	if oldObject == "" && newObject == "" {
		return "", fmt.Errorf("both oldObject and newObject cannot be empty")
	}

	// Replace placeholders in the template
	userMessage := s.userMessageTemplate
	userMessage = strings.ReplaceAll(userMessage, "{{.OldObject}}", oldObject)
	userMessage = strings.ReplaceAll(userMessage, "{{.NewObject}}", newObject)
	userMessage = strings.ReplaceAll(userMessage, "{{.GitDiff}}", gitDiff)

	log.Debug().
		Int("template_length", len(s.userMessageTemplate)).
		Int("final_message_length", len(userMessage)).
		Msg("Generated user message from template")

	response, err := s.CreateChatCompletionWithSystemPrompt(ctx, userMessage)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate changelog entry")
		return "", fmt.Errorf("failed to generate changelog entry: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	content := response.Choices[0].Message.Content
	log.Info().
		Int("response_length", len(content)).
		Msg("Successfully generated changelog entry")

	return content, nil
}
