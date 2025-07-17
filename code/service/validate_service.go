// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to Pod binding and status changes.
package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/helpers"
)

// PodBindingService handles AdmissionReview requests for Pod binding events.
// It delegates to HandleReview using validation.ValidateBindingPod to determine
// when a Pod has been scheduled onto a node, then enqueues constants.PodNodeBindingTask.
//
//	c   – Fiber context wrapping the HTTP request/response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
func ValidateService(
	c *fiber.Ctx,
	cfg *config.Config,
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

	// Check if the admission review is for a pod - if so, allow it and return early
	if review.Request.Kind.Kind == "Pod" {
		response := admissionv1.AdmissionResponse{
			Allowed: true,
			UID:     review.Request.UID,
		}
		review.Response = &response

		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	response := admissionv1.AdmissionResponse{
		Allowed: true,
		UID:     review.Request.UID,
	}
	review.Response = &response

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

	// Parse the new object if present
	var newObject map[string]interface{}
	if review.Request.Object.Raw != nil {
		if err := json.Unmarshal(review.Request.Object.Raw, &newObject); err != nil {
			log.Error().Err(err).Msg("could not parse new object raw JSON into map")
			return c.
				Status(fiber.StatusInternalServerError).
				SendString("could not parse new object")
		}
	}

	// Parse the old object if present (for UPDATE/DELETE operations)
	var oldObject map[string]interface{}
	if review.Request.OldObject.Raw != nil {
		if err := json.Unmarshal(review.Request.OldObject.Raw, &oldObject); err != nil {
			log.Error().Err(err).Msg("could not parse old object raw JSON into map")
			return c.
				Status(fiber.StatusInternalServerError).
				SendString("could not parse old object")
		}
	}

	objectDiff, err := helpers.ObjectDiff(oldObject, newObject)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate object diff")
	}

	log.Info().
		Str("object_diff", objectDiff).
		Msg("object diff generated")


	return c.
		Status(fiber.StatusOK).
		JSON(review)
}
