package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/helpers"
	"channelog/models"
)

// ChangelogService handles changelog generation and git operations
type ChangelogService struct {
	cfg          *config.Config
	modelService *models.OpenAIService
	gitService   *GitService
}

// NewChangelogService creates a new ChangelogService instance
func NewChangelogService(cfg *config.Config, modelService *models.OpenAIService) *ChangelogService {
	return &ChangelogService{
		cfg:          cfg,
		modelService: modelService,
		gitService:   NewGitService(cfg),
	}
}

// ProcessAndCommit handles the complete changelog process: generation and commit
func (cs *ChangelogService) ProcessAndCommit(review admissionv1.AdmissionReview) error {
	// Log the admission request for observability
	cs.logAdmissionRequest(review)

	// Generate changelog entry
	changelogEntry, err := cs.generateChangelogEntry(review)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate changelog entry")
		return err
	}

	// Commit the changelog entry
	if err := cs.commitChangelogEntry(review, changelogEntry); err != nil {
		log.Error().Err(err).Msg("failed to commit changelog entry")
		return err
	}

	return nil
}

// generateChangelogEntry processes the admission review and generates a changelog entry
func (cs *ChangelogService) generateChangelogEntry(review admissionv1.AdmissionReview) (string, error) {
	// Get json objects from the request
	oldObject, newObject, err := getOldNewObjects(review)
	if err != nil {
		return "", fmt.Errorf("failed to get old and new objects: %w", err)
	}

	// Generate a diff between the old and new objects
	objectDiff, err := helpers.ObjectDiff(oldObject, newObject)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate object diff")
		return "", err
	}

	// Convert the jsons to string
	var oldObjectStr, newObjectStr string
	if oldObject != nil {
		oldObjectStr = string(review.Request.OldObject.Raw)
	}
	if newObject != nil {
		newObjectStr = string(review.Request.Object.Raw)
	}

	// Use the OpenAI service to generate a commit message
	ctx := context.Background()
	return cs.modelService.GenerateChangelogEntry(ctx, oldObjectStr, newObjectStr, objectDiff)
}

// commitChangelogEntry creates and commits the changelog entry to git
func (cs *ChangelogService) commitChangelogEntry(review admissionv1.AdmissionReview, changelogEntry string) error {
	// Generate filename based on resource information
	fileName := cs.gitService.GenerateFileName(
		review.Request.Namespace,
		review.Request.Name,
		review.Request.Kind.Kind,
	)

	// Format the changelog entry with metadata
	changelogContent := cs.formatChangelogContent(review, changelogEntry)

	// Create git commit with the changelog entry
	gitCommitMessage := fmt.Sprintf("Add changelog for %s/%s (%s)",
		review.Request.Kind.Kind,
		review.Request.Name,
		review.Request.Operation,
	)

	if err := cs.gitService.CreateCommit(fileName, changelogContent, gitCommitMessage); err != nil {
		return fmt.Errorf("failed to create git commit for %s: %w", fileName, err)
	}

	log.Info().
		Str("filename", fileName).
		Str("commit_message", gitCommitMessage).
		Str("changelogContent", changelogContent).
		Msg("successfully created changelog entry and committed to git")

	return nil
}

// logAdmissionRequest logs key fields from the AdmissionRequest for observability
func (cs *ChangelogService) logAdmissionRequest(review admissionv1.AdmissionReview) {
	log.Info().
		Str("uid", string(review.Request.UID)).
		Str("kind", review.Request.Kind.String()).
		Str("resource", review.Request.Resource.String()).
		Str("name", review.Request.Name).
		Str("namespace", review.Request.Namespace).
		Str("operation", string(review.Request.Operation)).
		Msg("received AdmissionReview")
}

// formatChangelogContent formats the changelog entry with metadata
func (cs *ChangelogService) formatChangelogContent(review admissionv1.AdmissionReview, changelogEntry string) string {
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback to UTC if Asia/Kolkata timezone cannot be loaded
		ist = time.UTC
	}
	timestamp := time.Now().In(ist).Format(time.RFC1123)
	return fmt.Sprintf(`# Changelog Entry

**Resource:** %s/%s  
**Namespace:** %s  
**Operation:** %s  
**Timestamp:** %s  
**UID:** %s  

## Change Summary

%s

---
*Generated automatically by Channelog*
`,
		review.Request.Kind.Kind,
		review.Request.Name,
		review.Request.Namespace,
		review.Request.Operation,
		timestamp,
		review.Request.UID,
		changelogEntry,
	)
}
