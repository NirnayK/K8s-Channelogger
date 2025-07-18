// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to Pod binding and status changes.
package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/models"
	"channelog/task"
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

	isEarlyExit := validation.ValidateValidRequest(review)
	if isEarlyExit {
		return c.
			Status(fiber.StatusOK).
			JSON(review)
	}

	reviewCopy := review.DeepCopy()

	go task.CommitMessage(*reviewCopy, modelService)

	return c.
		Status(fiber.StatusOK).
		JSON(review)
}
