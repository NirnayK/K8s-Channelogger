// Package helpers provides utility functions for admission channelog handlers,
// including object decoding, panic recovery, and request parsing.
package helpers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// decoder deserializes raw Kubernetes objects using the client-go scheme.
	decoder = serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDeserializer()
)

// GetUrlType derives a simple mode string from the request path by removing
// the leading “/”. For example, "/pods" becomes "pods".
func GetUrlType(c *fiber.Ctx) string {
	path := c.Path()
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return path
}

// RecoverPanic logs any panic that occurs within a deferred context,
// tagging it with the provided context string.
//
// Panic recovery is critical for channelog reliability because:
// - Panics in channelog handlers can cause the entire channelog to crash
// - A crashed channelog blocks all Kubernetes operations that require validation
// - Recovery ensures the channelog continues processing other requests
// - Comprehensive logging helps debug issues without affecting service availability
func RecoverPanic(context string) {
	if r := recover(); r != nil {
		log.Error().
			Interface("panic", r).
			Str("context", context).
			Stack().
			Msg("recovered panic")
	}
}

// PanicCatcher returns a deferred function that will catch and log any panic.
//
// Usage example: defer PanicCatcher("MyFuncName")()
//
// Why panic recovery is better than failing:
// - Prevents a single malformed request from crashing the entire channelog service
// - Maintains channelog availability for other Kubernetes operations
// - Allows graceful degradation while logging errors for debugging
// - Essential for admission webhooks that control cluster operations
func PanicCatcher(context string) func() {
	return func() {
		RecoverPanic(context)
	}
}

// GetOperation parses an AdmissionReview from the HTTP request body
// and returns the contained AdmissionRequest.Operation. It logs and returns
// an error if the body cannot be unmarshaled.
func GetOperation(c *fiber.Ctx) (admissionv1.Operation, error) {
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(c.Body(), &review); err != nil {
		log.Error().Err(err).Msg("could not unmarshal AdmissionReview request")
		return "", fmt.Errorf("could not unmarshal request: %w", err)
	}
	return review.Request.Operation, nil
}
