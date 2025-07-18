// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to Pod binding and status changes.
package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/openai/openai-go"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/helpers"
	"channelog/validation"
)

// PodBindingService handles AdmissionReview requests for Pod binding events.
// It delegates to HandleReview using validation.ValidateBindingPod to determine
// when a Pod has been scheduled onto a node, then enqueues constants.PodNodeBindingTask.
//
//		c   – Fiber context wrapping the HTTP request/response.
//		cfg – Application configuration, including queue settings.
//	 modelClient – OpenAI client for generating text responses.
func CommitService(
	c *fiber.Ctx,
	cfg *config.Config,
	modelClient *openai.Client,
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

	response := admissionv1.AdmissionResponse{
		Allowed: true,
		UID:     review.Request.UID,
	}
	review.Response = &response

	isEarlyExit := validation.ValidateValidRequest(review)
	if isEarlyExit {
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	// Log key fields from the AdmissionRequest for observability.
	log.Info().
		Str("uid", string(review.Request.UID)).
		Str("kind", review.Request.Kind.String()).
		Str("resource", review.Request.Resource.String()).
		Str("name", review.Request.Name).
		Str("namespace", review.Request.Namespace).
		Str("operation", string(review.Request.Operation)).
		Str("path", c.Path()).
		Msg("received AdmissionReview")

	oldObject, newObject, err := getOldNewObjects(review)
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to get old and new objects from AdmissionReview")

		return c.
			Status(fiber.StatusInternalServerError).
			SendString("failed to get old and new objects from AdmissionReview")
	}

	objectDiff, err := helpers.ObjectDiff(oldObject, newObject)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate object diff")

		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	log.Info().
		Str("object_diff", objectDiff).
		Msg("object diff generated")

	return c.
		Status(fiber.StatusOK).
		JSON(review)
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
