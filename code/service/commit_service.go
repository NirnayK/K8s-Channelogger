// Package service provides HTTP handlers for Kubernetes admission Channelog
// endpoints. These handlers generate changelog entries and commit them to git
// based on Kubernetes resource changes.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/helpers"
	"channelog/models"
	"channelog/validation"
)

// CommitService handles AdmissionReview requests and records changelog entries.
// It skips requests that validation.ValidateValidRequest reports should be
// ignored, such as Pod objects.
//
//	c            - Fiber context wrapping the HTTP request/response.
//	cfg          - Application configuration.
//	modelService - OpenAI client for generating text responses.
func CommitService(
	c *fiber.Ctx,
	cfg *config.Config,
	modelService *models.OpenAIService,
) error {
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(c.Body(), &review); err != nil {
		log.Error().
			Err(err).
			Msg("could not unmarshal AdmissionReview request")
		return c.
			Status(fiber.StatusBadRequest).
			SendString("could not unmarshal AdmissionReview request")
	}

	review.Response = &admissionv1.AdmissionResponse{
		Allowed: true,
		UID:     review.Request.UID,
	}

	shouldSkip := validation.ValidateValidRequest(review)
	if shouldSkip {
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	reviewCopy := review.DeepCopy()

	go commitMessage(*reviewCopy, cfg, modelService)

	return c.
		Status(fiber.StatusOK).
		JSON(review)
}

// getOldNewObjects extracts old and new objects from the admission review
func getOldNewObjects(review admissionv1.AdmissionReview) (map[string]any, map[string]any, error) {
	var newObject map[string]any
	if review.Request.Object.Raw != nil {
		if err := json.Unmarshal(review.Request.Object.Raw, &newObject); err != nil {
			return nil, nil, err
		}
	}

	var oldObject map[string]any
	if review.Request.OldObject.Raw != nil {
		if err := json.Unmarshal(review.Request.OldObject.Raw, &oldObject); err != nil {
			return nil, nil, err
		}
	}

	return oldObject, newObject, nil
}

// commitMessage handles the git commit process for changelog entries
func commitMessage(
	review admissionv1.AdmissionReview,
	cfg *config.Config,
	modelService *models.OpenAIService,
) {
	logAdmissionRequest(review)

	changelogEntry, err := generateChangelogEntry(review, modelService)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate changelog entry")
		return
	}

	if err := commitChangelogEntry(review, cfg, changelogEntry); err != nil {
		log.Error().Err(err).Msg("failed to commit changelog entry")
		return
	}
}

// logAdmissionRequest logs key fields from the AdmissionRequest for observability
func logAdmissionRequest(review admissionv1.AdmissionReview) {
	log.Info().
		Str("uid", string(review.Request.UID)).
		Str("kind", review.Request.Kind.String()).
		Str("resource", review.Request.Resource.String()).
		Str("name", review.Request.Name).
		Str("namespace", review.Request.Namespace).
		Str("operation", string(review.Request.Operation)).
		Msg("received AdmissionReview")
}

// generateChangelogEntry processes the admission review and generates a changelog entry
func generateChangelogEntry(review admissionv1.AdmissionReview, modelService *models.OpenAIService) (string, error) {
	// Get json objects from the request
	oldObject, newObject, err := getOldNewObjects(review)
	if err != nil {
		return "", fmt.Errorf("failed to get old and new objects: %w", err)
	}

	// Generate a diff between the old and new objects
	objectDiff, err := helpers.ObjectDiff(oldObject, newObject)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate object diff")
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
	return modelService.GenerateChangelogEntry(ctx, oldObjectStr, newObjectStr, objectDiff)
}

// commitChangelogEntry creates and commits the changelog entry to git
func commitChangelogEntry(
	review admissionv1.AdmissionReview,
	cfg *config.Config,
	changelogEntry string,
) error {
	// Create git service
	gitService := NewGitService(cfg)

	// Generate filename based on resource information
	fileName := gitService.GenerateFileName(
		review.Request.Namespace,
		review.Request.Name,
		review.Request.Kind.Kind,
	)

	// Format the changelog entry with metadata
	changelogContent := formatChangelogContent(review, changelogEntry)

	// Create git commit with the changelog entry
	gitCommitMessage := fmt.Sprintf("Add changelog for %s/%s (%s)",
		review.Request.Kind.Kind,
		review.Request.Name,
		review.Request.Operation,
	)

	if err := gitService.CreateCommit(fileName, changelogContent, gitCommitMessage); err != nil {
		return fmt.Errorf("failed to create git commit for %s: %w", fileName, err)
	}

	log.Info().
		Str("filename", fileName).
		Str("commit_message", gitCommitMessage).
		Msg("successfully created changelog entry and committed to git")

	return nil
}

// formatChangelogContent formats the changelog entry with metadata
func formatChangelogContent(review admissionv1.AdmissionReview, changelogEntry string) string {
	ist, _ := time.LoadLocation("Asia/Kolkata")
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
