// Package service provides HTTP handlers for Kubernetes admission Channelog
// endpoints. These handlers generate changelog entries and commit them to git
// based on Kubernetes resource changes.
package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/filters"
	"channelog/helpers"
	"channelog/models"
)

// CommitService handles AdmissionReview requests and records changelog entries.
// It skips requests that filters.ValidateValidRequest reports should be
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

	shouldSkip := filters.ValidateValidRequest(review)
	if shouldSkip {
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	// Check for meaningful differences before launching goroutine
	oldObject, newObject, err := getOldNewObjects(review)
	if err != nil {
		log.Error().Err(err).Msg("failed to get old and new objects")
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	// Apply filter conditions to check for meaningful changes
	filterConditions := filters.NewFilterConditions()
	filteredOld := filterConditions.ApplyAll(oldObject)
	filteredNew := filterConditions.ApplyAll(newObject)

	objectDiff, err := helpers.ObjectDiff(filteredOld, filteredNew)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate object diff")
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	// Early exit if no meaningful changes detected
	if objectDiff == "No differences found" {
		log.Debug().
			Str("uid", string(review.Request.UID)).
			Str("kind", review.Request.Kind.String()).
			Str("name", review.Request.Name).
			Str("namespace", review.Request.Namespace).
			Str("operation", string(review.Request.Operation)).
			Msg("skipping commit: no meaningful changes detected")
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	reviewCopy := review.DeepCopy()

	// Create changelog service and process the request in a goroutine
	changelogService := NewChangelogService(cfg, modelService)
	go changelogService.ProcessAndCommit(*reviewCopy)

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
