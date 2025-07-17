// Package service provides HTTP handlers for various Kubernetes admission channelog services.
package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
	"channelog/constants"
	"channelog/rabbit"
	"channelog/validation"
)

// KedaService is the HTTP handler for KEDA-related AdmissionReview requests.
// It delegates the common flow to HandleReview, using IsValidKedaTask to decide
// whether to enqueue a task, and constants.KedaTask as the Celery task name.
//
// Parameters:
//
//	c   – Fiber context wrapping the incoming HTTP request and response writer.
//	cfg – Application configuration (e.g., queue name, AMQP URL).
//	rm  – RabbitManager for publishing tasks to RabbitMQ.
func KedaService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	// Delegate to the shared review handler with:
	// - validation function: validation.IsValidKedaTask
	// - task name:           constants.KedaTask
	return HandleReview(c, cfg, rm, validation.IsValidKedaTask, constants.KedaTask)
}
