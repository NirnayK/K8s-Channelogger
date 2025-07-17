// Package service provides HTTP handlers for Kubernetes admission channelog endpoints
// related to Pod binding and status changes.
package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
	"channelog/constants"
	"channelog/rabbit"
	"channelog/validation"
)

// PodBindingService handles AdmissionReview requests for Pod binding events.
// It delegates to HandleReview using validation.ValidateBindingPod to determine
// when a Pod has been scheduled onto a node, then enqueues constants.PodNodeBindingTask.
//
//	c   – Fiber context wrapping the HTTP request/response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
func PodBindingService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	return HandleReview(c, cfg, rm, validation.ValidateBindingPod, constants.PodNodeBindingTask)
}

// PodStatusService handles AdmissionReview requests for Pod status updates.
// It unmarshals the incoming AdmissionReview, logs key metadata, invokes
// validation.ValidatePodStatusChange to determine if a task should be enqueued,
// and pushes a task if validation returns a non-empty taskName.
//
//	c   – Fiber context wrapping the HTTP request/response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
func PodStatusService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	return HandleReview(c, cfg, rm, validation.ValidatePodStatusChange, constants.PodStatusTask)
}

// PodDeleteService handles AdmissionReview requests for Pod delete events.
// It automatically approves the request and enqueues a task if the Pod is being deleted.
//
//	c   – Fiber context wrapping the HTTP request/response.
//	cfg – Application configuration, including queue settings.
//	rm  – RabbitManager for publishing tasks.
func PodDeleteService(
	c *fiber.Ctx,
	cfg *config.Config,
	rm *rabbit.RabbitManager,
) error {
	return HandleReview(c, cfg, rm, validation.ValidTask, constants.PodDeletionTask)
}
