// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to Pod binding and status changes.
package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
	"channelog/validation"
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
	return HandleReview(c, cfg, validation.ValidateBindingPod)
}
