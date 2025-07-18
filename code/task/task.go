package task

import (
	"channelog/helpers"
	"channelog/models"
	"context"
	"encoding/json"

	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
)

func CommitMessage(review admissionv1.AdmissionReview, modelService *models.OpenAIService) {
	// Log key fields from the AdmissionRequest for observability.
	log.Info().
		Str("uid", string(review.Request.UID)).
		Str("kind", review.Request.Kind.String()).
		Str("resource", review.Request.Resource.String()).
		Str("name", review.Request.Name).
		Str("namespace", review.Request.Namespace).
		Str("operation", string(review.Request.Operation)).
		Msg("received AdmissionReview")

	// Get json objects from the request
	oldObject, newObject, err := getOldNewObjects(review)
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to get old and new objects from AdmissionReview")
		return
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
	commitMessage, err := modelService.GenerateChangelogEntry(ctx, oldObjectStr, newObjectStr, objectDiff)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate commit message")
		return
	}
	// Commit the changes to the git repository
	
}

func getOldNewObjects(
	review admissionv1.AdmissionReview,
) (map[string]any, map[string]any, error) {
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