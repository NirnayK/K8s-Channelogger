// Package helpers provides utility functions for admission channelog handlers,
// including object decoding, panic recovery, and request parsing.
package helpers

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ObjectDiff compares two objects and returns a git-style diff string.
// It takes two map[string]any objects representing the old and new versions,
// converts them to YAML and uses go-git to generate a proper git-style diff output.
func ObjectDiff(oldObj, newObj map[string]any) (string, error) {
	// Convert objects to YAML
	oldYAML, err := yaml.Marshal(oldObj)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal old object to YAML")
		return "", fmt.Errorf("failed to marshal old object to YAML: %w", err)
	}

	newYAML, err := yaml.Marshal(newObj)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal new object to YAML")
		return "", fmt.Errorf("failed to marshal new object to YAML: %w", err)
	}

	// Create in-memory filesystem for git repository
	fs := memfs.New()
	storage := memory.NewStorage()

	// Initialize a new git repository
	repo, err := git.Init(storage, fs)
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize git repository")
		return "", fmt.Errorf("failed to initialize git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		log.Error().Err(err).Msg("failed to get worktree")
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Write old version and commit
	file, err := fs.Create("object.yaml")
	if err != nil {
		log.Error().Err(err).Msg("failed to create file")
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	_, err = file.Write(oldYAML)
	if err != nil {
		log.Error().Err(err).Msg("failed to write old YAML")
		return "", fmt.Errorf("failed to write old YAML: %w", err)
	}
	file.Close()

	_, err = worktree.Add("object.yaml")
	if err != nil {
		log.Error().Err(err).Msg("failed to add file to git")
		return "", fmt.Errorf("failed to add file to git: %w", err)
	}

	oldCommit, err := worktree.Commit("old version", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "diff-generator",
			Email: "diff@example.com",
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to commit old version")
		return "", fmt.Errorf("failed to commit old version: %w", err)
	}

	// Write new version
	file, err = fs.OpenFile("object.yaml", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Error().Err(err).Msg("failed to open file for new version")
		return "", fmt.Errorf("failed to open file for new version: %w", err)
	}
	_, err = file.Write(newYAML)
	if err != nil {
		log.Error().Err(err).Msg("failed to write new YAML")
		return "", fmt.Errorf("failed to write new YAML: %w", err)
	}
	file.Close()

	_, err = worktree.Add("object.yaml")
	if err != nil {
		log.Error().Err(err).Msg("failed to add updated file to git")
		return "", fmt.Errorf("failed to add updated file to git: %w", err)
	}

	newCommit, err := worktree.Commit("new version", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "diff-generator",
			Email: "diff@example.com",
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to commit new version")
		return "", fmt.Errorf("failed to commit new version: %w", err)
	}

	// Generate diff between commits
	oldCommitObj, err := repo.CommitObject(oldCommit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get old commit object")
		return "", fmt.Errorf("failed to get old commit object: %w", err)
	}

	newCommitObj, err := repo.CommitObject(newCommit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get new commit object")
		return "", fmt.Errorf("failed to get new commit object: %w", err)
	}

	oldTree, err := oldCommitObj.Tree()
	if err != nil {
		log.Error().Err(err).Msg("failed to get old tree")
		return "", fmt.Errorf("failed to get old tree: %w", err)
	}

	newTree, err := newCommitObj.Tree()
	if err != nil {
		log.Error().Err(err).Msg("failed to get new tree")
		return "", fmt.Errorf("failed to get new tree: %w", err)
	}

	changes, err := object.DiffTree(oldTree, newTree)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate diff")
		return "", fmt.Errorf("failed to generate diff: %w", err)
	}

	if len(changes) == 0 {
		return "No differences found", nil
	}

	// Generate unified diff format
	var diffOutput strings.Builder
	for _, change := range changes {
		patch, err := change.Patch()
		if err != nil {
			log.Error().Err(err).Msg("failed to generate patch")
			continue
		}
		diffOutput.WriteString(patch.String())
	}

	if diffOutput.Len() == 0 {
		return "No differences found", nil
	}

	// Clean up the diff output to make it more readable
	return cleanDiffOutput(diffOutput.String()), nil
}

// cleanDiffOutput processes the git diff output to make it more readable
// by removing commit hashes and file paths, keeping only the meaningful diff content.
func cleanDiffOutput(diffOutput string) string {
	lines := strings.Split(diffOutput, "\n")
	var cleanedLines []string

	for _, line := range lines {
		// Skip commit hash lines and file header lines that are not useful for object comparison
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- a/") ||
			strings.HasPrefix(line, "+++ b/") {
			continue
		}

		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n")
}
