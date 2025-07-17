// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to various resources, including Argo Workflows.
package service

import (
	"github.com/gofiber/fiber/v2"
	admissionv1 "k8s.io/api/admission/v1"

	"channelog/config"
	"channelog/constants"
	"channelog/helpers"
	"channelog/rabbit"
	"channelog/validation"
)

// WorkflowService handles AdmissionReview requests for Argo Workflow resources.
// It inspects the admission operation and chooses the appropriate validation logic
// and Celery task name.
//
//	c   – Fiber context wrapping the HTTP request and response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
//
// Behavior:
//   - On DELETE operations: uses validation.ValidTask (no-op) and enqueues WorkflowDeleteTask.
//   - On all other operations (CREATE, UPDATE, etc.): uses validation.IsValidWorkflowTask
//     to enqueue WorkflowTask when the Workflow’s Phase becomes non-empty.
func WorkflowService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	// 1) Extract the admission operation (CREATE, UPDATE, DELETE) from the request.
	op, err := helpers.GetOperation(c)
	if err != nil {
		// If we cannot parse the AdmissionReview, return HTTP 400.
		return c.
			Status(fiber.StatusBadRequest).
			SendString("could not unmarshal request")
	}

	// 2) Dispatch to HandleReview with the correct validator and task name.
	switch op {
	case admissionv1.Delete:
		// On deletion of a Workflow, always enqueue the delete task.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,         // no special validation
			constants.WorkflowDeleteTask, // delete-specific task
		)
	default:
		// On create/update (and any other) operations, validate phase changes.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.IsValidWorkflowTask, // enqueue only when Phase != ""
			constants.WorkflowTask,         // generic workflow task
		)
	}
}
