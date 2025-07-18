package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"channelog/config"
)

// LivenessService responds with 200 OK if the channelog can connect to the remote Git repository.
func LivenessService(c *fiber.Ctx, cfg *config.Config) error {
	// Check if we can reach the remote Git repository
	if err := checkGitRemoteConnectivity(cfg); err != nil {
		log.Error().Err(err).Msg("Failed to connect to remote Git repository")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "unhealthy",
			"error":  err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "healthy",
	})
}

// checkGitRemoteConnectivity uses git ls-remote to quickly check if we can reach the remote repository
// This is faster than git fetch as it doesn't download any data, just lists references
func checkGitRemoteConnectivity(cfg *config.Config) error {
	// Set a timeout for the git command (5 seconds should be sufficient for a connectivity check)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Prepare the git ls-remote command
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", cfg.GitRepo)

	// Set up environment variables for authentication if token is provided
	if cfg.GitToken != "" {
		// For HTTPS URLs with token authentication
		if strings.HasPrefix(cfg.GitRepo, "https://") {
			// Extract the URL parts to inject the token
			repoURL := strings.TrimPrefix(cfg.GitRepo, "https://")
			authenticatedURL := fmt.Sprintf("https://oauth2:%s@%s", cfg.GitToken, repoURL)
			cmd.Args[3] = authenticatedURL
		}
	}

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git connectivity check failed: %v, output: %s", err, string(output))
	}

	// If we got here, the command succeeded
	log.Debug().Str("repo", cfg.GitRepo).Msg("Git remote connectivity check passed")
	return nil
}
