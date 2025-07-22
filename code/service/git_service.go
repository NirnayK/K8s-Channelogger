package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rs/zerolog/log"

	channelconfig "channelog/config"
)

const (
	// ClusterScopeFolder is the folder name for cluster-scoped resources
	// Using "__cluster-scope__" ensures it cannot be a valid k8s namespace name
	// (k8s namespace names cannot contain underscores)
	ClusterScopeFolder = "__cluster-scope__"
)

// GitService provides in-memory git repository operations using go-git
type GitService struct {
	repoURL   string
	branch    string
	username  string
	userEmail string
	token     string
	repo      *git.Repository
	worktree  *git.Worktree
	auth      transport.AuthMethod
}

// NewGitService creates a new git service instance
func NewGitService(cfg *channelconfig.Config) *GitService {
	service := &GitService{
		repoURL:   cfg.GitRepo,
		branch:    cfg.GitBranch,
		username:  cfg.Username,
		userEmail: cfg.UserEmail,
		token:     cfg.GitToken,
	}

	// Set up authentication
	service.setupAuth()

	return service
}

// setupAuth configures authentication based on repository URL and token
func (g *GitService) setupAuth() {
	if g.token != "" && strings.HasPrefix(g.repoURL, "https://") {
		// Use token for HTTPS authentication
		g.auth = &http.BasicAuth{
			Username: g.token, // GitHub/GitLab token as username
			Password: "",      // Empty password for token auth
		}
		log.Debug().Msg("Configured HTTPS token authentication")
	} else {
		log.Debug().Msg("No authentication configured (using default SSH or public access)")
	}
}

// InitializeRepo initializes an in-memory repository and fetches the target branch
func (g *GitService) InitializeRepo() error {
	// Clone the repository directly into memory
	storer := memory.NewStorage()
	fs := memfs.New()

	repo, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:           g.repoURL,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", g.branch)),
		SingleBranch:  true,
		Depth:         1, // Shallow clone
		Auth:          g.auth,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("branch", g.branch).
			Str("repo_url", g.repoURL).
			Msg("Failed to clone repository")
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	g.repo = repo

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get worktree")
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	g.worktree = worktree

	log.Info().
		Str("branch", g.branch).
		Str("repo_url", g.repoURL).
		Msg("Successfully cloned repository into memory")

	return nil
}

// CreateCommit creates a commit with the given file content and pushes it
func (g *GitService) CreateCommit(fileName, content, commitMessage string) error {
	if g.repo == nil {
		if err := g.InitializeRepo(); err != nil {
			return err
		}
	}

	// Ensure the directory exists
	dir := filepath.Dir(fileName)
	if dir != "." && dir != "" {
		err := g.worktree.Filesystem.MkdirAll(dir, 0755)
		if err != nil {
			log.Error().Err(err).Str("dir", dir).Msg("Failed to create directory")
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write the file content
	file, err := g.worktree.Filesystem.Create(fileName)
	if err != nil {
		log.Error().Err(err).Str("filename", fileName).Msg("Failed to create file")
		return fmt.Errorf("failed to create file %s: %w", fileName, err)
	}
	defer file.Close()

	_, err = file.Write([]byte(content))
	if err != nil {
		log.Error().Err(err).Str("filename", fileName).Msg("Failed to write file content")
		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Add the file to the index
	_, err = g.worktree.Add(fileName)
	if err != nil {
		log.Error().Err(err).Str("filename", fileName).Msg("Failed to add file to index")
		return fmt.Errorf("failed to add file to index: %w", err)
	}

	// Create the commit
	commitHash, err := g.worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.username,
			Email: g.userEmail,
			When:  time.Now(),
		},
		Committer: &object.Signature{
			Name:  g.username,
			Email: g.userEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create commit")
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Push the changes
	err = g.repo.Push(&git.PushOptions{
		Auth: g.auth,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("branch", g.branch).
			Msg("Failed to push commit")
		return fmt.Errorf("failed to push commit: %w", err)
	}

	log.Info().
		Str("filename", fileName).
		Str("commit_message", commitMessage).
		Str("commit_hash", commitHash.String()[:8]).
		Msg("Successfully created and pushed commit")

	return nil
}

// GenerateFileName generates a filename for the changelog entry
// Layout structure:
// - Cluster-scoped: __cluster-scope__/{kind}/{name}_{timestamp}.yaml
// - Namespace-scoped: {namespace}/{kind}/{name}_{timestamp}.yaml
func (g *GitService) GenerateFileName(namespace, name, kind string) string {
	// Use IST timezone (UTC+5:30) with RFC1123 format
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback to UTC if Asia/Kolkata timezone cannot be loaded
		ist = time.UTC
	}
	timestamp := time.Now().In(ist).Format(time.RFC1123)
	// Replace spaces and colons with underscores for filesystem safety
	timestamp = strings.ReplaceAll(timestamp, " ", "_")
	timestamp = strings.ReplaceAll(timestamp, ":", "-")

	// Sanitize the name to be filesystem-safe
	safeName := strings.ReplaceAll(name, "/", "_")
	safeName = strings.ReplaceAll(safeName, ":", "_")

	// Generate filename with name and timestamp
	fileName := fmt.Sprintf("%s_%s.yaml", safeName, timestamp)

	if namespace != "" {
		// Namespace-scoped resources: {namespace}/{kind}/{file}
		safeNamespace := strings.ReplaceAll(namespace, "/", "_")
		safeNamespace = strings.ReplaceAll(safeNamespace, ":", "_")
		return filepath.Join(safeNamespace, strings.ToLower(kind), fileName)
	}

	// Cluster-scoped resources: __cluster-scope__/{kind}/{file}
	// Using "__cluster-scope__" ensures it cannot be a valid k8s namespace name
	// (k8s namespace names cannot contain underscores)
	return filepath.Join(ClusterScopeFolder, strings.ToLower(kind), fileName)
}
