// Package config provides functionality to load and validate
// environment-backed configuration for the channelog service.
package config

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

// Config holds all of the application's settings sourced from environment variables.
type Config struct {
	// GitRepo is the URL of the private GitLab repository
	// Example: "https://gitlab.example.com/group/project.git" (for token auth)
	// or "git@gitlab.example.com:group/project.git" (for SSH auth)
	GitRepo string

	// GitBranch is the branch to work with in the repository
	// Example: "main", "develop", "feature/xyz"
	GitBranch string

	// Username is the Git username for commits
	// Example: "John Doe"
	Username string

	// UserEmail is the Git email for commits
	// Example: "john.doe@example.com"
	UserEmail string

	// GitToken is the GitLab project token for authentication (optional)
	// If provided, will be used for HTTPS authentication
	GitToken string

	// OpenAI configuration
	// OpenAIApiUrl is the OpenAI API base URL
	OpenAIApiUrl string

	// AI Model is the model name to use for OpenAI Compatible requests
	OpenAIModel string

	// SystemPrompt is the system prompt for OpenAI chat completions
	SystemPrompt string

	// UserMessageTemplate is the template for user messages with placeholders
	UserMessageTemplate string
}

// LoadConfig reads required environment variables, applies defaults,
// and validates their values before returning a Config instance.
// Returns an error if any required variable is missing or malformed.
func LoadConfig() (*Config, error) {
	// 1) GIT_REPO is required for repository access
	gitRepo := os.Getenv("GIT_REPO")
	if gitRepo == "" {
		log.Error().Msg("GIT_REPO is required")
		return nil, fmt.Errorf("GIT_REPO is required")
	}

	// 2) GIT_BRANCH is required to specify which branch to work with
	gitBranch := os.Getenv("GIT_BRANCH")
	if gitBranch == "" {
		log.Error().Msg("GIT_BRANCH is required")
		return nil, fmt.Errorf("GIT_BRANCH is required")
	}

	// 3) USERNAME is required for Git commits
	username := os.Getenv("USERNAME")
	if username == "" {
		log.Error().Msg("USERNAME is required")
		return nil, fmt.Errorf("USERNAME is required")
	}

	// 4) USER_EMAIL is required for Git commits
	userEmail := os.Getenv("USER_EMAIL")
	if userEmail == "" {
		log.Error().Msg("USER_EMAIL is required")
		return nil, fmt.Errorf("USER_EMAIL is required")
	}

	// 5) GIT_TOKEN is optional for HTTPS authentication
	gitToken := os.Getenv("GIT_TOKEN")

	// 6) OPENAI_API_URL is required for OpenAI API access
	openAIApiUrl := os.Getenv("OPENAI_API_URL")
	if openAIApiUrl == "" {
		openAIApiUrl = "https://api.openai.com/v1" // Default to official OpenAI API
	}

	// 7) OPENAI_MODEL is required to specify which model to use
	openAIModel := os.Getenv("OPENAI_MODEL")
	if openAIModel == "" {
		openAIModel = "gpt-4" // Default to GPT-4
	}

	// 8) SYSTEM_PROMPT for OpenAI system messages
	systemPrompt := os.Getenv("SYSTEM_PROMPT")
	if systemPrompt == "" {
		log.Warn().Msg("SYSTEM_PROMPT not set, using empty system prompt")
	}

	// 9) USER_MESSAGE_TEMPLATE for formatting user messages
	userMessageTemplate := os.Getenv("USER_MESSAGE_TEMPLATE")
	if userMessageTemplate == "" {
		log.Warn().Msg("USER_MESSAGE_TEMPLATE not set, using empty template")
	}

	// 10) Return the populated Config struct.
	return &Config{
		GitRepo:             gitRepo,
		GitBranch:           gitBranch,
		Username:            username,
		UserEmail:           userEmail,
		GitToken:            gitToken,
		OpenAIApiUrl:        openAIApiUrl,
		OpenAIModel:         openAIModel,
		SystemPrompt:        systemPrompt,
		UserMessageTemplate: userMessageTemplate,
	}, nil
}
