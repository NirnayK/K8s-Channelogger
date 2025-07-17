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

func NodeService(
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
			constants.NodeAddTask,
		)

	case admissionv1.Delete:
		// On deletion of an HPA, no special validation → enqueue delete task.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.NodeDeleteTask,
		)

	default:
		// Fallback path if the operation is unexpected.
		return HandleReview(
			c,
			cfg,
			rm,
			validation.ValidTask,
			constants.DummyTask,
		)
	}
}
