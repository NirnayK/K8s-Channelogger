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
}

// LoadConfig reads required environment variables, applies defaults,
// and validates their values before returning a Config instance.
// Returns an error if any required variable is missing or malformed.
func LoadConfig() (*Config, error) {
	// 1) LOCATION is required for multi-region deployments.
	// This identifier helps downstream consumers route and process tasks
	// according to their geographic or logical region.
	location := os.Getenv("LOCATION")
	if location == "" {
		log.Error().Msg("LOCATION is required")
		return nil, fmt.Errorf("LOCATION is required")
	}

	// 2) GIT_REPO is required for repository access
	gitRepo := os.Getenv("GIT_REPO")
	if gitRepo == "" {
		log.Error().Msg("GIT_REPO is required")
		return nil, fmt.Errorf("GIT_REPO is required")
	}

	// 3) GIT_BRANCH is required to specify which branch to work with
	gitBranch := os.Getenv("GIT_BRANCH")
	if gitBranch == "" {
		log.Error().Msg("GIT_BRANCH is required")
		return nil, fmt.Errorf("GIT_BRANCH is required")
	}

	// 4) USERNAME is required for Git commits
	username := os.Getenv("USERNAME")
	if username == "" {
		log.Error().Msg("USERNAME is required")
		return nil, fmt.Errorf("USERNAME is required")
	}

	// 5) USER_EMAIL is required for Git commits
	userEmail := os.Getenv("USER_EMAIL")
	if userEmail == "" {
		log.Error().Msg("USER_EMAIL is required")
		return nil, fmt.Errorf("USER_EMAIL is required")
	}

	// 6) GIT_TOKEN is optional for HTTPS authentication
	gitToken := os.Getenv("GIT_TOKEN")

	// 7) Return the populated Config struct.
	return &Config{
		GitRepo:   gitRepo,
		GitBranch: gitBranch,
		Username:  username,
		UserEmail: userEmail,
		GitToken:  gitToken,
	}, nil
}
