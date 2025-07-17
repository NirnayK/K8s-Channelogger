// Package service provides a generic handler for Kubernetes AdmissionReview requests.
// It delegates the actual validation logic to a user-supplied ValidatorFunc and enqueues
// tasks for valid requests.
package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/constants"
	"channelog/rabbit"
	"channelog/task"
)

// ValidatorFunc defines the signature for a validation function.
// Given an AdmissionRequest, it returns whether the request is valid and any error encountered.
type ValidatorFunc func(request *admissionv1.AdmissionRequest) (bool, error)

// HandleReview processes an HTTP request containing an AdmissionReview JSON payload.
// It unmarshals the body, logs metadata, invokes the supplied validate function,
// enqueues a task if validation passes, and always returns an AdmissionResponse
// with Allowed=true.
//
// Parameters:
//
//	c        – Fiber context for the HTTP request/response.
//	cfg      – Application configuration (e.g., queue name).
//	rm       – RabbitManager used to publish tasks.
//	validate – ValidatorFunc to determine if the AdmissionRequest should trigger a task.
//	taskName – Name of the Celery task to enqueue on valid requests.
//
// Returns an error if JSON unmarshaling fails or if the response cannot be serialized.
func HandleReview(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
	validate ValidatorFunc,
	taskName string,
) error {
	// 1) Parse the incoming AdmissionReview JSON from the request body.
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(c.Body(), &review); err != nil {
		log.Error().
			Err(err).
			Msg("could not unmarshal AdmissionReview request")
		return c.
			Status(fiber.StatusBadRequest).
			SendString("could not unmarshal AdmissionReview request")
	}

	// 2) Log key fields from the AdmissionRequest for observability.
	log.Info().
		Str("uid", string(review.Request.UID)).
		Str("kind", review.Request.Kind.String()).
		Str("resource", review.Request.Resource.String()).
		Str("name", review.Request.Name).
		Str("namespace", review.Request.Namespace).
		Str("operation", string(review.Request.Operation)).
		Str("path", c.Path()).
		Msg("received AdmissionReview")

	// 3) Invoke the custom validation logic.
	valid, err := validate(review.Request)
	if err != nil {
		// Log validation errors but continue: we still return Allowed=true.
		log.Error().
			Err(err).
			Msg("validation error")
	}

	// 4) If validation passed (valid==true and no error), enqueue the corresponding task.
	if valid && err == nil && taskName != constants.DummyTask {
		task.PushTask(&review, taskName, rm, cfg)
	}

	// 5) Construct and return an AdmissionResponse with Allowed=true.
	//
	//    CRITICAL DESIGN DECISION: This channelog ALWAYS allows requests regardless of validation outcome.
	//    This is intentional because:
	//    - We implement an "allow-first, validate-later" strategy for non-blocking operations
	//    - The channelog responds immediately to prevent blocking Kubernetes API operations
	//    - Actual validation/enforcement happens asynchronously in Celery tasks
	//    - This ensures high availability and prevents channelog timeouts from affecting cluster operations
	//    - Any issues or violations are logged and handled by downstream consumers without blocking admission
	response := admissionv1.AdmissionResponse{
		Allowed: true,
		UID:     review.Request.UID,
	}
	review.Response = &response

	// 6) Send the modified AdmissionReview (with Response) back as JSON.
	return c.
		Status(fiber.StatusOK).
		JSON(review)
}
