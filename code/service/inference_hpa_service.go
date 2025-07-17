// Package service wires up specific Kubernetes AdmissionReview endpoints
// to the generic HandleReview logic, selecting the appropriate validation
// function and Celery task name based on the operation.
package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
	"channelog/constants"
	"channelog/helpers"
	"channelog/rabbit"
	"channelog/validation"

	admissionv1 "k8s.io/api/admission/v1"
)

// InferenceHpaService handles AdmissionReview requests for HorizontalPodAutoscaler
// resources tied to inference workloads. It determines the operation (Create, Update,
// Delete) and dispatches to HandleReview with the appropriate validator and task name.
//
//	c   – Fiber context wrapping the HTTP request/response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
//
// Routes:
//   - On Create: uses ValidTask (no-op), enqueues InferenceHpaCreateTask.
//   - On Update: uses ValidateInferenceHpaStatus, enqueues InferenceHpaUpdateTask.
//   - On Delete: uses ValidTask (no-op), enqueues InferenceHpaDeleteTask.
//   - Default: falls back to the Update path.
func InferenceHpaService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	// 1) Extract the admission operation (CREATE, UPDATE, DELETE) from the request body.
	op, err := helpers.GetOperation(c)
	if err != nil {
		// Bad or malformed AdmissionReview JSON → return 400.
		return c.
			Status(fiber.StatusBadRequest).
			SendString("could not unmarshal request")
	}

	// 2) Select validation logic and Celery task based on the operation.
	switch op {

	case admissionv1.Create:
		// On creation of an HPA, no special validation → always enqueue create task.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.InferenceHpaCreateTask,
		)

	case admissionv1.Update:
		// On update of an HPA, validate via ValidateInferenceHpaStatus.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.InferenceHpaUpdateTask,
		)

	case admissionv1.Delete:
		// On deletion of an HPA, no special validation → enqueue delete task.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.InferenceHpaDeleteTask,
		)

	default:
		// Fallback to update path if the operation is unexpected.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.DummyTask,
		)
	}
}
